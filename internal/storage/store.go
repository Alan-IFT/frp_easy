// Package storage 封装 SQLite 句柄、迁移引导与所有 DAO。
//
// 设计契约见 docs/features/web-ui-mvp/02_SOLUTION_DESIGN.md §3.2 与 §4。
// 调用方（dev-backend）必须通过本包提供的方法访问数据库；其它包不写 SQL。
//
// 驱动选用 modernc.org/sqlite（纯 Go，无 cgo），驱动名注册为 "sqlite"。
package storage

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

// 迁移 SQL 物理文件：
//   - 权威源：仓库根 migrations/0001_init.{up,down}.sql（供文档 / 人审 / CLI 回滚使用）。
//   - 包内副本：internal/storage/sqlmigrations/*.sql（被 go:embed 编入二进制，
//     运行时迁移引擎读这一份）。
//
// 两份必须字节一致。verifyEmbeddedMigrationsMatchDisk 的测试用例（见
// storage_test.go）会比较两边内容，防止静默 drift。

// 公共哨兵错误。
var (
	// ErrCorruptReset 表示启动时检测到现有 data.db 损坏（PRAGMA integrity_check 非 "ok"
	// 或基础打开失败），已经将原文件改名为 data.db.broken-<RFC3339> 并重建空库 + 跑迁移。
	// 上层（httpapi）据此进入"首次启动"状态（见 AC-12）。
	ErrCorruptReset = errors.New("storage: existing database was corrupt; renamed to *.broken-<ts> and reinitialized")

	// ErrVersionConflict 表示 UpsertProxy 时调用者传入的 Version 与 DB 内当前版本不一致
	// （last-write-wins 校验失败，对应 02 §5.2 `409 CONFLICT`）。
	ErrVersionConflict = errors.New("storage: version conflict")

	// ErrNotFound 表示请求的实体（按 id 或 token / key）在 DB 中不存在。
	ErrNotFound = errors.New("storage: not found")

	// ErrDuplicateName 表示 UpsertProxy 时与已有 proxies.name 唯一约束冲突。
	// 调用方（httpapi）应据此返回 409 Conflict 而非 422，与 (type,remote_port)
	// 组合冲突区分开（后者继续走原 422 兜底分支）。
	// 触发条件：proxies 表 name 列 column-level UNIQUE 约束（见
	// internal/storage/sqlmigrations/0001_init.up.sql 第 32 行）。
	ErrDuplicateName = errors.New("storage: duplicate proxy name")

	// ErrDuplicateTcpRemote 表示批量 / 单条 Upsert 触发 (type, remote_port) 部分唯一
	// 索引冲突（idx_proxies_tcp_remote）。
	// T-018 引入用于 UpsertProxiesTx 事务回滚后的分流：调用方据此返回 409 + field=remotePort。
	// 错误文本格式：`UNIQUE constraint failed: proxies.type, proxies.remote_port`
	// （已在 proxies_test.go TestIsDuplicateNameError_DirectChecks "type-remote-conflict"
	// 用例中实证，T-018 B-8）。
	ErrDuplicateTcpRemote = errors.New("storage: duplicate (type, remote_port)")
)

// Store 是 SQLite 持久化层句柄。所有写操作受内部 mu 守护（避免单连接 + 并发写时
// "database is locked"）；读操作直接走 db。
type Store struct {
	db      *sql.DB
	dataDir string
	mu      sync.Mutex // 守护写入
}

// Open 打开 / 创建 dataDir 下的 data.db，应用所有未应用的迁移。
//
// 行为：
//  1. 若 dataDir 不存在则创建（含父目录）。
//  2. 若 data.db 不存在 → 新建空库 + 跑迁移 + 返回 (store, nil)。
//  3. 若 data.db 存在但 PRAGMA integrity_check 非 "ok" → 改名为
//     data.db.broken-<RFC3339> + 新建空库 + 跑迁移 + 返回 (store, ErrCorruptReset)。
//  4. 若 data.db 存在且健康 → 跑未应用的迁移 + 返回 (store, nil)。
//
// 调用方应判断 errors.Is(err, ErrCorruptReset)：是 = 软错误（store 可用，但提示进入 setup 流程）；
// 其它非 nil = 硬错误。
func Open(dataDir string) (*Store, error) {
	abs, err := filepath.Abs(dataDir)
	if err != nil {
		return nil, fmt.Errorf("storage: abs dataDir: %w", err)
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return nil, fmt.Errorf("storage: mkdir dataDir: %w", err)
	}
	dbPath := filepath.Join(abs, "data.db")

	var corruptReset bool
	if info, statErr := os.Stat(dbPath); statErr == nil && info.Size() > 0 {
		// 文件存在且非空：探测健康。
		healthy, probeErr := probeIntegrity(dbPath)
		if probeErr != nil || !healthy {
			brokenName := fmt.Sprintf("data.db.broken-%s", time.Now().UTC().Format("20060102T150405Z"))
			brokenPath := filepath.Join(abs, brokenName)
			if renameErr := os.Rename(dbPath, brokenPath); renameErr != nil {
				return nil, fmt.Errorf("storage: rename corrupt db (probe err=%v): %w", probeErr, renameErr)
			}
			// 同时把 sqlite 可能创建的 -journal / -wal / -shm 副产物挪开（best-effort）。
			for _, suf := range []string{"-journal", "-wal", "-shm"} {
				_ = os.Rename(dbPath+suf, brokenPath+suf)
			}
			corruptReset = true
		}
	} else if statErr != nil && !errors.Is(statErr, fs.ErrNotExist) {
		return nil, fmt.Errorf("storage: stat data.db: %w", statErr)
	}

	db, err := openSqlite(dbPath)
	if err != nil {
		return nil, fmt.Errorf("storage: open sqlite: %w", err)
	}

	s := &Store{db: db, dataDir: abs}
	if err := s.migrate(context.Background()); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("storage: migrate: %w", err)
	}

	if corruptReset {
		return s, ErrCorruptReset
	}
	return s, nil
}

// Close 关闭底层 *sql.DB。多次调用安全。
func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

// DataDir 返回 Open 时使用的绝对 dataDir，供调用方定位日志 / TOML 输出。
func (s *Store) DataDir() string {
	return s.dataDir
}

// openSqlite 用启用 foreign_keys、WAL、busy_timeout 的 DSN 打开 DB。
func openSqlite(path string) (*sql.DB, error) {
	// modernc.org/sqlite 接受 URI 形式的 query 参数（_pragma=...）。
	// 这里在 Open 后再单独 PRAGMA，更直观可控。
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	// 单连接 + sync.Mutex 守护写，避免 "database is locked"。
	// 读操作仍然受单连接限制，但 MVP 量级（≤200 proxy）可接受。
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	if _, err := db.Exec("PRAGMA foreign_keys=ON;"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("pragma foreign_keys: %w", err)
	}
	if _, err := db.Exec("PRAGMA journal_mode=WAL;"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("pragma journal_mode: %w", err)
	}
	if _, err := db.Exec("PRAGMA busy_timeout=5000;"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("pragma busy_timeout: %w", err)
	}
	return db, nil
}

// probeIntegrity 临时打开 path 跑 `PRAGMA integrity_check`。
// 返回 true 仅当能正常打开且首行结果为 "ok"。
// 任何 open / query 失败或结果非 ok 都视为不健康（false, nil）。
// 仅当严重 IO 错误（无法关闭等）时返回 err。
func probeIntegrity(path string) (bool, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return false, nil
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	rows, err := db.Query("PRAGMA integrity_check;")
	if err != nil {
		return false, nil
	}
	defer rows.Close()
	if !rows.Next() {
		return false, nil
	}
	var status string
	if err := rows.Scan(&status); err != nil {
		return false, nil
	}
	// 进一步保险：sqlite 损坏时可能 Next() == false 而无错；上面 if 已覆盖。
	return strings.TrimSpace(status) == "ok", nil
}

// --- migrations ---

//go:embed sqlmigrations/*.sql
var migrationsFS embed.FS

// migration 是一条已加载的迁移。
type migration struct {
	version int64
	name    string
	upSQL   string
}

// migrate 应用所有未应用的迁移（按 version 升序）。
func (s *Store) migrate(ctx context.Context) error {
	// 先建 schema_migrations（自举）。若 0001 自带 CREATE TABLE IF NOT EXISTS schema_migrations，
	// 这里 idempotent。
	if _, err := s.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version    INTEGER PRIMARY KEY,
		applied_at TEXT    NOT NULL DEFAULT (datetime('now'))
	);`); err != nil {
		return fmt.Errorf("bootstrap schema_migrations: %w", err)
	}

	applied := map[int64]bool{}
	rows, err := s.db.QueryContext(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return fmt.Errorf("read schema_migrations: %w", err)
	}
	for rows.Next() {
		var v int64
		if err := rows.Scan(&v); err != nil {
			rows.Close()
			return fmt.Errorf("scan version: %w", err)
		}
		applied[v] = true
	}
	rows.Close()

	migs, err := loadMigrations()
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, m := range migs {
		if applied[m.version] {
			continue
		}
		if err := s.applyOne(ctx, m); err != nil {
			return fmt.Errorf("apply %04d_%s: %w", m.version, m.name, err)
		}
	}
	return nil
}

// applyOne 在单个事务中执行 m.upSQL。迁移文件本身已带
// `INSERT INTO schema_migrations(version) VALUES (N)`，无需重复插入。
func (s *Store) applyOne(ctx context.Context, m migration) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, m.upSQL); err != nil {
		_ = tx.Rollback()
		return err
	}
	// 兜底：若迁移文件没插 schema_migrations，这里补一次（idempotent）。
	if _, err := tx.ExecContext(ctx,
		`INSERT OR IGNORE INTO schema_migrations(version) VALUES (?)`, m.version); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

// loadMigrations 从嵌入的 sqlmigrations/ 目录读取所有 *.up.sql，
// 按文件名前缀解析 version（NNNN_<slug>.up.sql）。
func loadMigrations() ([]migration, error) {
	entries, err := fs.ReadDir(migrationsFS, "sqlmigrations")
	if err != nil {
		return nil, fmt.Errorf("read embedded migrations dir: %w", err)
	}
	var out []migration
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".up.sql") {
			continue
		}
		// NNNN_<slug>.up.sql
		base := strings.TrimSuffix(name, ".up.sql")
		parts := strings.SplitN(base, "_", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("malformed migration name: %s", name)
		}
		var v int64
		if _, err := fmt.Sscanf(parts[0], "%d", &v); err != nil {
			return nil, fmt.Errorf("parse version from %s: %w", name, err)
		}
		data, err := fs.ReadFile(migrationsFS, "sqlmigrations/"+name)
		if err != nil {
			return nil, fmt.Errorf("read embedded %s: %w", name, err)
		}
		out = append(out, migration{version: v, name: parts[1], upSQL: string(data)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].version < out[j].version })
	return out, nil
}
