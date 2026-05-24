package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Proxy 表示一条端口转发规则（02 §3.2、§4.1）。
//
// 互斥规则（由迁移 SQL 的 CHECK 约束 + 应用层共同保证）：
//   - type ∈ {"tcp","udp"} 必须有 RemotePort，CustomDomains 必须为 nil/空。
//   - type ∈ {"http","https"} 必须有非空 CustomDomains，RemotePort 必须为 nil。
//
// Version 字段用于 last-write-wins 校验（对应 02 §5.2 `409 CONFLICT` / R-6）。
type Proxy struct {
	ID            int64
	Name          string
	Type          string // tcp/udp/http/https
	LocalIP       string
	LocalPort     int
	RemotePort    *int     // tcp/udp 必填
	CustomDomains []string // http/https 必填；DB 列存 JSON 字符串
	Enabled       bool
	Version       int64
	UpdatedAt     time.Time
}

// ListProxies 返回全部 proxy，按 id 升序。
func (s *Store) ListProxies(ctx context.Context) ([]Proxy, error) {
	const q = `
		SELECT id, name, type, local_ip, local_port, remote_port,
		       custom_domains, enabled, version, updated_at
		  FROM proxies ORDER BY id ASC`
	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("storage.ListProxies: %w", err)
	}
	defer rows.Close()

	var out []Proxy
	for rows.Next() {
		p, err := scanProxy(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("storage.ListProxies iter: %w", err)
	}
	return out, nil
}

// GetProxy 按 id 取单条。不存在返回 (nil, ErrNotFound)。
func (s *Store) GetProxy(ctx context.Context, id int64) (*Proxy, error) {
	const q = `
		SELECT id, name, type, local_ip, local_port, remote_port,
		       custom_domains, enabled, version, updated_at
		  FROM proxies WHERE id = ?`
	row := s.db.QueryRowContext(ctx, q, id)
	p, err := scanProxy(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// UpsertProxy 插入或更新一条 proxy 规则，并执行 last-write-wins 版本校验。
//
// 语义：
//
//   - 新建（p.ID == 0）：忽略 p.Version，写入后 p.ID / p.Version=1 / p.UpdatedAt 被回填。
//   - 更新（p.ID > 0）：
//   - 若 DB 中该行不存在 → 返回 ErrNotFound。
//   - 若 p.Version != 当前 DB version → 返回 ErrVersionConflict（前端应拉新值再 retry）。
//   - 成功更新后 version += 1 并回填 p.Version / p.UpdatedAt。
//
// 互斥字段冲突（tcp/udp 带 customDomains，或 http/https 带 remotePort）触发 SQL CHECK
// 失败，会作为普通错误返回（不映射为 sentinel）—— 上层 handler 应在 422 路径报字段错。
// name 重复或 (type,remotePort) 重复也由 SQL UNIQUE / 部分索引拦截。
func (s *Store) UpsertProxy(ctx context.Context, p *Proxy) error {
	if p == nil {
		return errors.New("storage.UpsertProxy: nil proxy")
	}
	if err := validateProxyShape(p); err != nil {
		return err
	}
	cdJSON, err := encodeCustomDomains(p.CustomDomains)
	if err != nil {
		return fmt.Errorf("storage.UpsertProxy encode customDomains: %w", err)
	}
	enabledInt := 0
	if p.Enabled {
		enabledInt = 1
	}
	localIP := p.LocalIP
	if localIP == "" {
		localIP = "127.0.0.1"
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if p.ID == 0 {
		// 新建
		const insertQ = `
			INSERT INTO proxies (name, type, local_ip, local_port, remote_port,
			                     custom_domains, enabled, version, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, 1, datetime('now'))`
		res, err := s.db.ExecContext(ctx, insertQ,
			p.Name, p.Type, localIP, p.LocalPort,
			nullableInt(p.RemotePort), nullableString(cdJSON),
			enabledInt)
		if err != nil {
			if isDuplicateNameError(err) {
				return ErrDuplicateName
			}
			return fmt.Errorf("storage.UpsertProxy insert: %w", err)
		}
		id, err := res.LastInsertId()
		if err != nil {
			return fmt.Errorf("storage.UpsertProxy lastid: %w", err)
		}
		p.ID = id
		p.Version = 1
		// 回填 UpdatedAt：直接读一遍最稳。
		fresh, err := s.getProxyLocked(ctx, id)
		if err == nil && fresh != nil {
			p.UpdatedAt = fresh.UpdatedAt
		}
		return nil
	}

	// 更新：先在事务中校验版本，再 +1。
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("storage.UpsertProxy begin: %w", err)
	}
	var curVersion int64
	err = tx.QueryRowContext(ctx, `SELECT version FROM proxies WHERE id = ?`, p.ID).Scan(&curVersion)
	if errors.Is(err, sql.ErrNoRows) {
		_ = tx.Rollback()
		return ErrNotFound
	}
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("storage.UpsertProxy read version: %w", err)
	}
	if curVersion != p.Version {
		_ = tx.Rollback()
		return ErrVersionConflict
	}
	const updQ = `
		UPDATE proxies
		   SET name = ?, type = ?, local_ip = ?, local_port = ?, remote_port = ?,
		       custom_domains = ?, enabled = ?, version = version + 1,
		       updated_at = datetime('now')
		 WHERE id = ?`
	if _, err := tx.ExecContext(ctx, updQ,
		p.Name, p.Type, localIP, p.LocalPort,
		nullableInt(p.RemotePort), nullableString(cdJSON), enabledInt,
		p.ID); err != nil {
		_ = tx.Rollback()
		if isDuplicateNameError(err) {
			return ErrDuplicateName
		}
		return fmt.Errorf("storage.UpsertProxy update: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("storage.UpsertProxy commit: %w", err)
	}
	p.Version = curVersion + 1
	if fresh, err := s.getProxyLocked(ctx, p.ID); err == nil && fresh != nil {
		p.UpdatedAt = fresh.UpdatedAt
	}
	return nil
}

// DeleteProxy 按 id 删除一条 proxy；不存在时返回 ErrNotFound。
func (s *Store) DeleteProxy(ctx context.Context, id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	res, err := s.db.ExecContext(ctx, `DELETE FROM proxies WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("storage.DeleteProxy: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("storage.DeleteProxy rows: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// --- 内部辅助 ---

// scanProxy 同时支持 *sql.Row 与 *sql.Rows。
type scannable interface {
	Scan(dest ...any) error
}

func scanProxy(sc scannable) (Proxy, error) {
	var (
		p          Proxy
		remoteN    sql.NullInt64
		cdN        sql.NullString
		enabledN   int64
		updatedStr string
	)
	if err := sc.Scan(&p.ID, &p.Name, &p.Type, &p.LocalIP, &p.LocalPort,
		&remoteN, &cdN, &enabledN, &p.Version, &updatedStr); err != nil {
		return p, err
	}
	if remoteN.Valid {
		rp := int(remoteN.Int64)
		p.RemotePort = &rp
	}
	if cdN.Valid && cdN.String != "" {
		var arr []string
		if err := json.Unmarshal([]byte(cdN.String), &arr); err != nil {
			return p, fmt.Errorf("decode custom_domains for id=%d: %w", p.ID, err)
		}
		p.CustomDomains = arr
	}
	p.Enabled = enabledN != 0
	ts, err := parseSQLiteTime(updatedStr)
	if err != nil {
		return p, fmt.Errorf("parse updated_at for id=%d: %w", p.ID, err)
	}
	p.UpdatedAt = ts
	return p, nil
}

// getProxyLocked 内部使用：调用方已持锁，直接走 db。
func (s *Store) getProxyLocked(ctx context.Context, id int64) (*Proxy, error) {
	const q = `
		SELECT id, name, type, local_ip, local_port, remote_port,
		       custom_domains, enabled, version, updated_at
		  FROM proxies WHERE id = ?`
	row := s.db.QueryRowContext(ctx, q, id)
	p, err := scanProxy(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func validateProxyShape(p *Proxy) error {
	switch p.Type {
	case "tcp", "udp":
		if p.RemotePort == nil {
			return fmt.Errorf("storage.UpsertProxy: %s proxy requires remotePort", p.Type)
		}
		if len(p.CustomDomains) > 0 {
			return fmt.Errorf("storage.UpsertProxy: %s proxy must not set customDomains", p.Type)
		}
	case "http", "https":
		if p.RemotePort != nil {
			return fmt.Errorf("storage.UpsertProxy: %s proxy must not set remotePort", p.Type)
		}
		if len(p.CustomDomains) == 0 {
			return fmt.Errorf("storage.UpsertProxy: %s proxy requires customDomains", p.Type)
		}
	default:
		return fmt.Errorf("storage.UpsertProxy: invalid type %q", p.Type)
	}
	if p.Name == "" {
		return errors.New("storage.UpsertProxy: name required")
	}
	if p.LocalPort < 1 || p.LocalPort > 65535 {
		return fmt.Errorf("storage.UpsertProxy: localPort %d out of range", p.LocalPort)
	}
	if p.RemotePort != nil && (*p.RemotePort < 1 || *p.RemotePort > 65535) {
		return fmt.Errorf("storage.UpsertProxy: remotePort %d out of range", *p.RemotePort)
	}
	return nil
}

func encodeCustomDomains(d []string) (string, error) {
	if len(d) == 0 {
		return "", nil
	}
	b, err := json.Marshal(d)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func nullableInt(p *int) any {
	if p == nil {
		return nil
	}
	return *p
}

func nullableString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// isDuplicateNameError 判断 sqlite 错误是否为 proxies.name 列的 UNIQUE 冲突。
//
// 约束来源：internal/storage/sqlmigrations/0001_init.up.sql 第 32 行
// `name TEXT NOT NULL UNIQUE` —— 是 column-level UNIQUE，sqlite 在违规时输出文本
// `UNIQUE constraint failed: proxies.name`。
//
// 区分另一处 UNIQUE 约束（同文件第 46 行的部分唯一索引
// `idx_proxies_tcp_remote ON proxies(type, remote_port)`），其错误文本含
// `proxies.type, proxies.remote_port` 而非 `proxies.name`，本函数据此精确区分。
//
// 驱动：modernc.org/sqlite（internal/storage/store.go L23 blank import）。
// 未来如果驱动升级改了错误文本，本任务 AC-6.1 / AC-6.2 测试会立即捕获回归。
func isDuplicateNameError(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "UNIQUE constraint failed") &&
		strings.Contains(s, "proxies.name")
}

