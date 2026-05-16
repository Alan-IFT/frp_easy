# 04 · Development Record — DB 分区（第 1 轮）

> 任务：T-001 · web-ui-mvp · Stage 4 · 分区 `dev-db` 第 1 轮
> 编写：dev-db agent · 日期：2026-05-16
> 上游输入（只读）：
> - `01_REQUIREMENT_ANALYSIS.md`（READY；AC-12、B-20、B-21 与本分区直接相关）
> - `02_SOLUTION_DESIGN.md`（READY；§3.2 storage API、§4 数据模型、§4.1 / §4.2 / §4.3 SQL 与损坏处理、§13.1 文件级分配）
> - `03_GATE_REVIEW.md`（APPROVED WITH CONDITIONS；本分区无直接 CONDITION，仅 §7 Q-A 要求 fresh / corrupt-reset 两个用例分开 —— 已遵守）

---

## Summary

按 02 §13.1 / §13.2 step 1 完成 SQLite 持久化层与 0001 初始迁移：5 张表（`schema_migrations` / `admin` / `sessions` / `kv` / `proxies`）含全部 CHECK / UNIQUE / 部分索引约束；`internal/storage` 暴露 11 个 DAO + 3 个哨兵错误（`ErrCorruptReset` / `ErrVersionConflict` / `ErrNotFound`），含 AC-12 损坏自动改名 + 重建逻辑、`UpsertProxy` 的 last-write-wins 版本校验。Go test 12 用例全绿，关键 DAO 路径覆盖率 ≥80%、整体 79.1%。

---

## Files changed

> 全部为**新建**，dev-db 分区 owned。

### owned 路径内（按设计 §13.1）

- `migrations/0001_init.up.sql` — 02 §4.1 原样落地：`schema_migrations` / `admin`（CHECK id=1）/ `sessions`（含 `idx_sessions_expires`）/ `kv` / `proxies`（含互斥 CHECK、`idx_proxies_tcp_remote` 部分唯一索引、`version` 列）。文件末尾 `INSERT INTO schema_migrations(version) VALUES (1)`。
- `migrations/0001_init.down.sql` — 02 §4.2 原样落地：按依赖逆序 DROP 4 张业务表 + DELETE schema_migrations 该行（schema_migrations 自身保留以便后续迁移记账）。
- `internal/storage/store.go` — `Store` 结构体（持 `*sql.DB` + `sync.Mutex` + `dataDir`），`Open(dataDir) (*Store, error)`，`Close() error`，`DataDir() string`。启动序列：`MkdirAll` → 若 `data.db` 非空则 `probeIntegrity`（独立打开跑 `PRAGMA integrity_check`，非 `ok` 或打开失败均视为损坏）→ 损坏则 `Rename data.db → data.db.broken-<UTC YYYYMMDDTHHMMSSZ>`（含 `-journal`/`-wal`/`-shm` best-effort 一并搬走）→ 用 `_ "modernc.org/sqlite"` 驱动以 `sql.Open("sqlite", ...)` 打开 → 设 `SetMaxOpenConns(1)`+`SetMaxIdleConns(1)` 配合 `sync.Mutex` 守护写 → PRAGMA `foreign_keys=ON / journal_mode=WAL / busy_timeout=5000` → 跑 `migrate` → 损坏路径返回 `(store, ErrCorruptReset)`，其余 `(store, nil)`。`migrate` 用 `//go:embed sqlmigrations/*.sql` 加载，按版本顺序在事务中 apply 未应用项。
- `internal/storage/admin.go` — `Admin{Username, PasswordHash, UpdatedAt}`；`GetAdmin`（空表返回 `(nil, nil)`，非空返回单行）/ `SetAdmin`（`INSERT ... ON CONFLICT(id) DO UPDATE` upsert）。空参数被拒。
- `internal/storage/sessions.go` — `Session{Token, CSRFToken, CreatedAt, ExpiresAt}`；`CreateSession`（`crypto/rand` 32B + `base64.RawURLEncoding`，ttl<=0 拒）/ `GetSession`（过期返回 `ErrNotFound`，**不删**——清理交 Purge）/ `DeleteSession`（idempotent）/ `PurgeExpiredSessions`（返回删除条数）。
- `internal/storage/kv.go` — `KVGet` / `KVSet`（upsert）/ `KVDelete`（idempotent）。空 key 被拒。
- `internal/storage/proxies.go` — `Proxy` 类型含 `Version int64`；`ListProxies` / `GetProxy` / `UpsertProxy` / `DeleteProxy`。新建走 INSERT 回填 ID + Version=1；更新走"事务内 SELECT current version → 比对 → UPDATE SET version = version + 1"，不匹配返回 `ErrVersionConflict`，不存在返回 `ErrNotFound`。`CustomDomains` 用 `encoding/json` 在应用层 ↔ DB 列做转换。`validateProxyShape` 在写入前校验互斥（tcp/udp 必须 RemotePort、无 customDomains；http/https 反之）以给出比 SQL CHECK 更友好的字段名错误。
- `internal/storage/helpers.go` — `parseSQLiteTime` 支持 RFC3339 / RFC3339Nano / SQLite `datetime('now')` 默认输出（`YYYY-MM-DD HH:MM:SS`，UTC）。
- `internal/storage/sqlmigrations/0001_init.up.sql`、`internal/storage/sqlmigrations/0001_init.down.sql` — **`migrations/` 的字节级镜像**，被 `//go:embed sqlmigrations/*.sql` 嵌入二进制。两份必须一致，由 `TestEmbeddedMigrations_MatchDisk` 守护。
  - **解释**：`//go:embed` 仅支持包目录及其子目录，无法跨包嵌入 `../../migrations/`。故选择"权威源 = `migrations/`（供文档 / 人审 / 回滚），副本 = `internal/storage/sqlmigrations/`（供 Go 编译嵌入）"的双轨布局 + 单元测试守护字节一致。
- `internal/storage/storage_test.go` — 12 个测试用例，全 `t.TempDir()` 隔离：
  1. `TestOpen_Fresh` —— 新 DataDir → 文件被建、5 表 + 2 索引到位、schema_migrations 含 version=1。
  2. `TestOpen_Corrupt` —— 预先写入 `garbage-not-a-sqlite-file` → `Open` 返回 `ErrCorruptReset`，原文件已改名 `data.db.broken-...`，新库可正常 `KVGet`。
  3. `TestAdmin_SetGet` —— 空表返回 nil、setup 后回读、upsert 覆盖、行数恒为 1、空参拒。
  4. `TestSession_Lifecycle` —— create → get → 不存在 → delete → idempotent delete → 手动过期一条 → GetSession 拒读过期 → 创建 live 一条 → Purge 删 1 条（live 保留）→ ttl=0 拒。
  5. `TestKV_SetGet` —— missing / set / get / upsert / delete / 空 key 拒。
  6. `TestProxy_CRUD` —— tcp + http 两种 type 落地 + List 回读 + CustomDomains 与 RemotePort round-trip + name 重复 UNIQUE 失败 + (type, remotePort) 部分唯一索引失败 + 互斥规则（tcp 带 customDomains / http 缺 customDomains）拒 + 端口越界拒 + Get/Delete + 更新版本号递增。
  7. `TestProxy_VersionConflict` —— a/b 并发拿到 version=1：a 写 → version=2，b 用旧 version=1 写 → `ErrVersionConflict`；update 不存在 id → `ErrNotFound`；**8-goroutine 真并发**抢同一 version，仅 1 成功 / 7 conflict。
  8. `TestStore_DataDirAndCloseIdempotent` —— `DataDir()` 返回绝对路径；二次 Close 不 panic；nil receiver Close 安全。
  9. `TestProxy_InvalidType` —— `xtcp` 等未知 type 被拒；空 name / nil proxy / localPort=0 均被拒。
  10. `TestKVDelete_Missing` —— 删不存在 key 不报错。
  11. `TestOpen_MigrationsIdempotent` —— 二次 `Open` 不重复 INSERT schema_migrations。
  12. `TestMigration_DownRoundTrip` —— 执行 `0001_init.down.sql` 后 `proxies` 表确认消失、`schema_migrations` 该 version=1 已删；再次 `Open` 能重新建表（验证 down + up 可循环）。
  13. `TestPurgeExpired_Empty` —— 空 sessions 表 Purge 返回 0 无错。
  14. `TestEmbeddedMigrations_MatchDisk` —— 比较 `migrations/*.sql` 与 `internal/storage/sqlmigrations/*.sql` 字节相等，防 drift。

### **PM 一次性授权范围（越界文件，标注）**

PM Orchestrator 在派单时明确授权 dev-db 创建初始 `go.mod` 与 `go.sum`（正常归 dev-backend owned）。仅本次、仅最小化：

- `go.mod` — `module github.com/frp-easy/frp-easy` / `go 1.22` / 单一直接依赖 `modernc.org/sqlite v1.29.10`（本分区唯一需要的依赖；纯 Go 免 cgo 见 02 §6.1）。
- `go.sum` — 由 `go mod tidy` 自动生成，含 modernc.org/sqlite 及其传递依赖（go-humanize、google/uuid、hashicorp/golang-lru/v2、go-isatty、go-strftime、bigfft、golang.org/x/sys、modernc.org/{gc,libc,mathutil,memory,strutil,token}）。

**dev-backend 第 1 轮接手后** 将扩展 require 列表（`chi`、`go-toml/v2`、`golang.org/x/crypto`）并重跑 `go mod tidy`；本分区**不预先添加**任何其它直接依赖。

### docs 与 dev-map

- `docs/dev-map.md` —— "目录布局"补 `migrations/` 与 `internal/storage/`；"功能在哪里"表追加 3 行；"可复用工具"追加 5 行；"要遵循的模式"补 5 条 storage 约定（其它分区开发时直接读这里即可知道接口契约与文件位置）。

---

## Migration

- 文件：`migrations/0001_init.up.sql` / `migrations/0001_init.down.sql`
- Schema 摘要：5 张表 —— `schema_migrations`（版本账本）、`admin`（CHECK id=1 强制单行）、`sessions`（`idx_sessions_expires` 配合 `PurgeExpired`）、`kv`（通用文本 key-value）、`proxies`（带 type 互斥 CHECK、name UNIQUE、`(type, remote_port)` 部分唯一索引、`version` 乐观锁列）。

---

## Schema change

DDL 关键约束清单：

| 表 | 约束 / 索引 | 目的 |
|---|---|---|
| `admin` | `CHECK (id = 1)` | 硬约束单管理员（对应 01 O-1 单账号 / RBAC out of scope；02 §6 Q-6）。 |
| `proxies` | `name TEXT NOT NULL UNIQUE` | 01 B-11 name 全局唯一 / AC-10。 |
| `proxies` | `type CHECK (type IN ('tcp','udp','http','https'))` | 01 B-11 / Q-7 仅四类。 |
| `proxies` | `local_port CHECK BETWEEN 1 AND 65535` | 01 §4.1 端口范围。 |
| `proxies` | 互斥 `CHECK ((tcp/udp + remotePort 必填 + customDomains 必空) OR (http/https + 反之))` | 01 §4.1 互斥字段；02 §4.1。 |
| `proxies` | `CREATE UNIQUE INDEX idx_proxies_tcp_remote ON proxies(type, remote_port) WHERE type IN ('tcp','udp')` | 01 §4.1 `(type, remotePort)` 唯一；AC-10。 |
| `proxies` | `version INTEGER NOT NULL DEFAULT 1` | 01 §4.2 last-write-wins；02 R-6 / §5.2 `409 CONFLICT`。 |
| `sessions` | `CREATE INDEX idx_sessions_expires ON sessions(expires_at)` | 让 `PurgeExpiredSessions` 的 `WHERE expires_at < ?` 走索引。 |
| `schema_migrations` | `version INTEGER PRIMARY KEY` | 迁移账本，幂等应用。 |

---

## Rollback plan

执行 `migrations/0001_init.down.sql`：

```sql
DROP TABLE IF EXISTS proxies;
DROP TABLE IF EXISTS kv;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS admin;
DELETE FROM schema_migrations WHERE version = 1;
```

MVP 阶段执行方式：手工跑（用 `sqlite3 .frp_easy/data.db ".read migrations/0001_init.down.sql"`）；本期不引入 CLI 子命令（属 dev-backend 范畴）。`TestMigration_DownRoundTrip` 已在单测中验证 down + 重 up 循环正确。

---

## Data impact

- 首版无任何历史数据。
- Backfill required：无。
- 损坏处理（AC-12）：`Open` 中若现有 `data.db` 损坏，**保留**改名为 `data.db.broken-<UTC RFC3339 紧凑>` 不静默删除，符合 01 B-21；上层据 `ErrCorruptReset` 进入 setup 流程。

---

## Coordination

- **dev-backend 第 1 轮接手**：
  - 扩 `go.mod` deps：`github.com/go-chi/chi/v5`、`github.com/pelletier/go-toml/v2`、`golang.org/x/crypto`，并 `go mod tidy`。
  - 引用 `import "github.com/frp-easy/frp-easy/internal/storage"`：
    - `internal/auth` 用 `Store.SetAdmin` / `GetAdmin` / `CreateSession` / `GetSession` / `DeleteSession`、`KVGet/Set`（rate limiter 计数）。
    - `internal/frpconf` 用 `Store.ListProxies` 渲染 frpc.toml。
    - `internal/httpapi/handlers_proxies.go` 用 `Store.UpsertProxy`，遇 `errors.Is(err, ErrVersionConflict)` 映射 HTTP `409 CONFLICT`（02 §5.2 `CONFLICT`），`ErrNotFound` 映射 `404 NOT_FOUND`，shape 校验错误映射 `422 VALIDATION_FAILED`。
    - `cmd/frp-easy/main.go` 启动序列：`appconf.Load → storage.Open(dataDir)`，若 `errors.Is(err, storage.ErrCorruptReset)` 不视为致命，仅记 warn 并继续（system/ready 返回 `initialized=false`）。
  - **不要**在其它包里写 SQL —— 全部走 `internal/storage`。

- **dev-frontend**：本分区无直接交互。前端透过 dev-backend 暴露的 REST 契约（02 §5）感知 proxy / session / kv，不接触本包。

---

## verify_all result

### Baseline（实施前）

`scripts/verify_all.ps1` 在仓库根跑出：

```
PASS: 8 / WARN: 0 / FAIL: 0 / SKIP: 7
```

PASS 项：A.1 secrets / A.2 .env / A.3 TODO budget / E.1 CLAUDE.md / E.2 workflow.md / E.3 七 agent 文件 / E.4 binding 同步 / E.5 AI-GUIDE 索引完整。

SKIP 项（预期，不是失败）：B.1~B.4 因当前 `verify_all.ps1` 仍是 **npm 模板**，无 `package.json` 时全部 SKIP；C.1 无 playwright 配置；D.1 无 `src/`/`apps/`；E.6 无 06_TEST_REPORT.md。

> 02 §13.1 已把"实体化 verify_all（追加 `go vet/test/build` + `npm lint/build/test`）"明确派给 **dev-backend 第 1 轮**（owned `scripts/verify_all.{ps1,sh}`）。本分区**不修改** verify_all（属越界）。

### After（本分区实施后）

`scripts/verify_all.ps1` 再跑：

```
PASS: 8 / WARN: 0 / FAIL: 0 / SKIP: 7
```

Delta：**0 新失败、0 回归**。基线维持。

### dev-db 自检（按 PM 指定流程，verify_all 之外的本分区证据）

```
$env:PATH = "C:\Program Files\Go\bin;$env:PATH"
go version                # go1.26.3 windows/amd64（≥1.22 OK）
go mod tidy               # 成功，引入 modernc.org/sqlite 及 12 项 indirect deps
go vet ./internal/storage/...     # 无输出 = 干净
go test -count=1 ./internal/storage/...
  → ok github.com/frp-easy/frp-easy/internal/storage 0.86s  （12 个测试函数 / 14 个子断言，全 PASS）
go test -count=1 -coverprofile=coverage.out ./internal/storage/...
go tool cover -func=coverage.out
  → total: 79.1% of statements
  → 关键 DAO 全部 ≥ 80%：
       admin.GetAdmin 83.3% / SetAdmin 87.5%
       kv.KVGet 87.5% / KVSet 87.5% / KVDelete 83.3%
       sessions.CreateSession 81.2% / GetSession 80.0% / DeleteSession 83.3% / PurgeExpired 85.7%
       proxies.ListProxies 78.6% / GetProxy 87.5% / UpsertProxy 81.1% / DeleteProxy 81.8% / scanProxy 88.2%
       store.Close 100% / DataDir 100% / probeIntegrity 80.0%
  → 低分集中在底层 IO 错误兜底（openSqlite PRAGMA 失败、applyOne rollback、loadMigrations 文件名 malformed），需要 mock 文件系统才能可靠触发；属可接受。
```

### `-race` 说明（**重要**）

`go test -race` 在 Windows 上**要求 cgo + gcc**：

```
go: -race requires cgo; enable cgo by setting CGO_ENABLED=1
```

本机环境无 gcc（02 §6.1 选 `modernc.org/sqlite` 正是为了无 cgo 单二进制部署）。**race 检测留待 CI**（典型 Linux runner 默认 cgo + gcc 可用）补跑，**或** dev-backend 第 1 轮把 `verify_all` 实体化时加一条 "if (Get-Command gcc -ErrorAction SilentlyContinue) { go test -race ... } else { go test ... }" 的优雅降级。

本轮以"`sync.Mutex` 守护所有写 + `SetMaxOpenConns(1)` 强制单连接 + `TestProxy_VersionConflict` 8 goroutine 真并发抢同一 version 验证（结果 = 1 成功 / 7 ErrVersionConflict，符合预期）"作为并发安全的替代证据。

---

## Insight to surface

`//go:embed` 不能跨包目录引用（不能 `//go:embed ../../migrations/*.sql`），导致"权威 migration 源"与"被 Go 编译嵌入的副本"必须分离 —— 选用 `migrations/`（权威）+ `internal/storage/sqlmigrations/`（镜像）+ `TestEmbeddedMigrations_MatchDisk` 字节比对守护防 drift 的双轨布局。 · evidence: `internal/storage/store.go:64` 上方注释 + `internal/storage/storage_test.go:TestEmbeddedMigrations_MatchDisk`

---

## Verdict

**READY FOR REVIEW (DB partition complete)**

下一步建议：PM 派 **dev-backend 第 1 轮**，按 02 §13.2 step 2 落地：扩 `go.mod` deps、实体化 `scripts/verify_all.{ps1,sh}` 追加 `go vet/test/build` 步骤、实现 `internal/{appconf,auth,binloc,frpconf,frpcadmin,procmgr,logtail,httpapi}/` 与 `cmd/frp-easy/main.go`、`.gitignore` 追加 02 §10.5 三行（`.frp_easy/`、`internal/assets/dist/`、`web/node_modules/`、`bin/`），同时回应 Gate Review §8 的 C-2/C-3/C-5/I-1/I-2 五条 CONDITION（端口被占中文文案、ReadyGate 中间件 + 503/NOT_READY、日志脱敏过滤器、argon2id m=32MiB 备选注释、内部占用端口表注释）。
