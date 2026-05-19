# 02 · 方案设计 — T-001 · web-ui-mvp

> 模式：`full` · 编写：solution-architect · 日期：2026-05-16 · PM 自治模式
> 上游输入（只读）：`docs/features/web-ui-mvp/01_REQUIREMENT_ANALYSIS.md`（verdict=READY，24 条 In-scope、15 条验收）
> 上游显式转交本阶段的技术选型：Q-4 持久化介质、Q-8 密码哈希算法
> 决策原则（来自 INPUT.md）：① 用户体验 > ② 软件工程规范 > ③ 长期可维护性

---

## 1. Architecture summary（架构总览）

frp_easy 的 Web UI MVP 是一个**单 Go 二进制 + 内嵌 Vue 3 SPA** 的本地工具。Go 进程承担三重身份：
（a）HTTP API 服务器，向浏览器提供 REST 端点与静态资源（Vue 构建产物通过 `embed.FS` 编入二进制，无独立前端部署）；
（b）持久化层，使用 **SQLite (modernc.org/sqlite，纯 Go 驱动)** 存储管理员凭据、模式开关、frpc/frps 配置、代理规则与会话；
（c）进程编排器，按需 fork/监督 `frpc` 与 `frps` 子进程（从仓库随附的 `frp_win/` 或 `frp_linux/` 选对应 OS 的二进制），并在配置变更时调用 frpc 的 `GET /api/reload` 完成热加载（失败时回退到重启子进程）。

部署模型：**双产物**——开发用 `vite dev` + Go API（端口分离 + 代理）；发布用 `go build -tags embed` 产出单一可执行文件（Windows 与 Linux 各一份），用户克隆仓库后只需运行 `scripts/start.ps1` 或 `scripts/start.sh`。

---

## 2. Affected modules（受影响模块清单）

仓库当前**零业务代码**，仅有 `frp_win/`、`frp_linux/` 上游二进制与 Harness 骨架。下表中"existing"指 Harness 文档体系本身已存在但需追加，"new"指本期新建。

| 模块 | 路径（绝对） | 状态 |
|---|---|---|
| Go 模块根 | `C:\Programs\frp_easy\go.mod` | new |
| 程序入口 | `C:\Programs\frp_easy\cmd\frp-easy\main.go` | new |
| 内嵌前端资源 | `C:\Programs\frp_easy\internal\assets\embed.go` + `dist/` | new |
| HTTP 路由层 | `C:\Programs\frp_easy\internal\httpapi\` | new |
| 鉴权 / 会话 | `C:\Programs\frp_easy\internal\auth\` | new |
| 持久化层 | `C:\Programs\frp_easy\internal\storage\` | new |
| 数据库迁移 | `C:\Programs\frp_easy\migrations\` | new |
| 配置渲染器（DB ↔ TOML） | `C:\Programs\frp_easy\internal\frpconf\` | new |
| 进程编排器 | `C:\Programs\frp_easy\internal\procmgr\` | new |
| frpc 远程控制客户端 | `C:\Programs\frp_easy\internal\frpcadmin\` | new |
| 日志尾部读取 | `C:\Programs\frp_easy\internal\logtail\` | new |
| 二进制定位 | `C:\Programs\frp_easy\internal\binloc\` | new |
| 应用配置（UI 端口、绑定地址） | `C:\Programs\frp_easy\internal\appconf\` | new |
| 前端工程 | `C:\Programs\frp_easy\web\` | new |
| 启动脚本 | `C:\Programs\frp_easy\scripts\start.ps1` / `start.sh` | new |
| 构建脚本 | `C:\Programs\frp_easy\scripts\build.ps1` / `build.sh` | new |
| verify_all 实体化 | `C:\Programs\frp_easy\scripts\verify_all.ps1` / `.sh` | edit（占位脚本已存在，补 Go vet/build/test + npm build + lint） |
| dev-map 更新 | `C:\Programs\frp_easy\docs\dev-map.md` | edit（由开发阶段补 Go 包与 Vue 目录） |
| .gitignore | `C:\Programs\frp_easy\.gitignore` | edit（追加 `.frp_easy/`、`web/dist/`、`bin/frp-easy*`） |

---

## 3. Module decomposition（模块分解）

> 命名与公开 API 是契约，开发分区按本节落地。所有 Go 包名为最后一段（`auth`、`storage` …）。

### 3.1 `internal/appconf`

读取 / 写入 `frp_easy.toml`（应用自身配置，非 FRP 配置）。

```go
type AppConfig struct {
    UIBindAddr string // 默认 "127.0.0.1"，仅文件改，不暴露 UI 入口
    UIPort     int    // 默认 8080
    DataDir    string // 默认 "./.frp_easy"，绝对路径化后使用
    LogDir     string // 默认 DataDir + "/logs"
}

func Load(path string) (*AppConfig, error)                 // 文件不存在写默认
func (c *AppConfig) Validate() error                       // 端口范围、地址解析
```

### 3.2 `internal/storage`

封装 SQLite 句柄、迁移引导与所有 DAO。**不在其它包里写 SQL**。

```go
type Store struct { /* *sql.DB + sync.Mutex 守护写 */ }

func Open(dataDir string) (*Store, error)                  // 打开/创建 db，跑 migrations
func (s *Store) Close() error

// admin
type Admin struct { Username, PasswordHash string; UpdatedAt time.Time }
func (s *Store) GetAdmin(ctx) (*Admin, error)              // 无返回 nil,nil
func (s *Store) SetAdmin(ctx, username, hash string) error

// session
type Session struct { Token string; ExpiresAt time.Time }
func (s *Store) CreateSession(ctx, ttl time.Duration) (*Session, error)
func (s *Store) GetSession(ctx, token string) (*Session, error)
func (s *Store) DeleteSession(ctx, token string) error
func (s *Store) PurgeExpiredSessions(ctx) error

// kv（模式开关、frps 配置、frpc 服务器连接信息、CSRF key、登录失败计数）
func (s *Store) KVGet(ctx, key string) (string, bool, error)
func (s *Store) KVSet(ctx, key, value string) error

// proxies
type Proxy struct {
    ID         int64
    Name       string
    Type       string // tcp/udp/http/https
    LocalIP    string
    LocalPort  int
    RemotePort *int       // tcp/udp
    CustomDomains []string // http/https，JSON 列
    Enabled    bool
    Version    int64      // last-write-wins 校验
    UpdatedAt  time.Time
}
func (s *Store) ListProxies(ctx) ([]Proxy, error)
func (s *Store) UpsertProxy(ctx, p *Proxy) error            // INSERT 或 UPDATE
func (s *Store) DeleteProxy(ctx, id int64) error
```

### 3.3 `internal/auth`

```go
func HashPassword(plain string) (string, error)            // argon2id；见 §6 决策
func VerifyPassword(plain, encoded string) (bool, error)
func GenerateSessionToken() (string, error)                // crypto/rand 32B base64url
func GenerateCSRFToken() (string, error)
type RateLimiter struct{ /* per-IP 滑窗：5 次/60s */ }
func (r *RateLimiter) Allow(ip string) (allowed bool, retryAfter time.Duration)
```

### 3.4 `internal/frpconf`

唯一的"DB → TOML 文件"渲染器。输出 `.frp_easy/runtime/frpc.toml` 与 `frps.toml`。

```go
type FrpcRenderInput struct {
    ServerAddr string
    ServerPort int
    AuthMethod string // "" | "token"
    AuthToken  string
    Proxies    []storage.Proxy
    AdminAddr  string // 强制 127.0.0.1
    AdminPort  int    // 默认 7400
    AdminUser  string // 由 UI 服务在启动时生成并持久化
    AdminPass  string
    LogPath    string
}
func RenderFrpc(in FrpcRenderInput) ([]byte, error)        // 写出 TOML
func RenderFrps(in FrpsRenderInput) ([]byte, error)
func AtomicWrite(path string, content []byte) error        // 写临时文件 + rename
```

字段命名严格按 FRP 上游 TOML（camelCase、嵌套表 `webServer.*`、`auth.*`、`[[proxies]]` 数组段）—— 见 §附录 A。

### 3.5 `internal/procmgr`

子进程生命周期。提供事件驱动接口，HTTP 层订阅状态变化。

```go
type State string // "stopped" | "starting" | "running" | "stopping" | "error"

type ProcessInfo struct {
    State    State
    PID      int
    LastErr  string
    ChangedAt time.Time
}

type Manager struct { /* 守护 frpc + frps 各一个 supervisor */ }

func New(binLocator binloc.Locator, runtimeDir, logDir string) *Manager
func (m *Manager) Start(kind string) error                 // kind: "frpc"|"frps"
func (m *Manager) Stop(kind string) error
func (m *Manager) Restart(kind string) error
func (m *Manager) Status(kind string) ProcessInfo
func (m *Manager) Subscribe() <-chan StatusEvent           // 用于未来 SSE/WebSocket，MVP 内仅本地观察
func (m *Manager) ApplyConfigChange(kind string) error     // frpc → 调用 frpcadmin.Reload；frps → 重启
```

跨平台细节：

- Windows：`exec.Command` + `CREATE_NEW_PROCESS_GROUP` 标志；停止时 `taskkill /T /PID` 或对 frpc 先尝试 `POST /api/stop`（FRP 暂未提供，故直接 kill）。
- Linux：`os/exec` + `Setpgid: true`；停止时先 `SIGTERM`，3s 超时后 `SIGKILL`。
- 启动后 3s 内若子进程退出 → state=error，捕获 stderr 末尾写入日志（满足 4.3 错误路径）。

### 3.6 `internal/frpcadmin`

frpc admin API 客户端封装（HTTP basic auth + 短超时）。

```go
type Client struct { /* baseURL, user, pass, *http.Client */ }
func New(addr string, port int, user, pass string) *Client
func (c *Client) Reload(ctx, strict bool) error           // GET /api/reload?strictConfig=true
func (c *Client) Status(ctx) (map[string][]ProxyStatus, error) // GET /api/status
```

证据：context7 `/fatedier/frp` 确认 `GET /api/reload[?strictConfig=true]`、`GET /api/status` 与默认 `webServer.port = 7400`，basic auth `webServer.user/password`（见附录 A.1 / A.2）。

### 3.7 `internal/logtail`

读取子进程日志文件的最后 N 行 + 增量 polling 支持。

```go
func TailLines(path string, n int) ([]string, error)       // 末 n 行（默认 500）
func ReadFrom(path string, offset int64) ([]byte, int64, error) // 增量 + 新 offset
```

日志文件由 procmgr 启动时通过 `frpc -c <toml>` 的 `log.to = "..."` 字段渲染指向 `<DataDir>/logs/frpc.log`、`frps.log`，并按上游字段 `log.maxDays=7`、`log.level=info` 限制。

### 3.8 `internal/binloc`

```go
type Locator interface {
    FRPCPath() (string, error)                              // 选 frp_win/frpc.exe 或 frp_linux/frpc
    FRPSPath() (string, error)
    Missing() []string                                      // 返回未找到的 ("frpc"/"frps")
}
func NewDefault(repoRoot string) Locator                   // 按 runtime.GOOS 决策
```

二进制目录决策：**保留 `frp_win/` 与 `frp_linux/` 在 git 中**（作为 vendored 工件，单二进制部署 = 克隆即可跑）。`.gitignore` **不**追加这两个目录。依据：原则 ①（用户克隆即用，无需另装 FRP），原则 ③（版本与 UI 配对，避免上游字段漂移导致 UI 错乱）。代价是仓库体积 ~70 MB，可接受；后续若膨胀，T-002 再引入 git-lfs。

### 3.9 `internal/httpapi`

`chi` router（轻量、context-friendly、std net/http 兼容）。所有路由见 §5。统一中间件：
`Recover → RequestID → Logger → CORS(dev) → CSRF(写接口) → SessionAuth(受保护) → Handler`。

### 3.10 `internal/assets`

```go
//go:embed all:dist
var FS embed.FS
func Handler() http.Handler // 含 SPA fallback：未匹配文件 → 返回 index.html
```

### 3.11 前端 `web/`

Vue 3 + Vite + TypeScript + Pinia + Vue Router + Naive UI + Axios。详细脚手架见 §10。

---

## 4. Data model changes（数据模型）

> 介质 = SQLite 单文件 `.frp_easy/data.db`（Q-4 决策见 §6.1）。所有迁移在 `migrations/` 下，文件名 `NNNN_<slug>.up.sql` / `.down.sql`，按 NNNN 顺序由 `internal/storage` 启动时应用，应用记录写入 `schema_migrations(version INTEGER PRIMARY KEY, applied_at TEXT)`。

### 4.1 `migrations/0001_init.up.sql`

```sql
CREATE TABLE IF NOT EXISTS schema_migrations (
    version    INTEGER PRIMARY KEY,
    applied_at TEXT    NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE admin (
    id              INTEGER PRIMARY KEY CHECK (id = 1),
    username        TEXT    NOT NULL,
    password_hash   TEXT    NOT NULL,
    updated_at      TEXT    NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE sessions (
    token       TEXT    PRIMARY KEY,
    csrf_token  TEXT    NOT NULL,
    created_at  TEXT    NOT NULL DEFAULT (datetime('now')),
    expires_at  TEXT    NOT NULL
);
CREATE INDEX idx_sessions_expires ON sessions(expires_at);

CREATE TABLE kv (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
-- 预期 key：mode.frpc.enabled, mode.frps.enabled,
--          frps.config (JSON), frpc.serverConn (JSON),
--          frpc.admin (JSON: addr/port/user/pass),
--          loginfail.<ip> (JSON: count/firstAt)

CREATE TABLE proxies (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    name            TEXT    NOT NULL UNIQUE,
    type            TEXT    NOT NULL CHECK (type IN ('tcp','udp','http','https')),
    local_ip        TEXT    NOT NULL DEFAULT '127.0.0.1',
    local_port      INTEGER NOT NULL CHECK (local_port BETWEEN 1 AND 65535),
    remote_port     INTEGER,
    custom_domains  TEXT,    -- JSON 数组；http/https 才用
    enabled         INTEGER NOT NULL DEFAULT 1,
    version         INTEGER NOT NULL DEFAULT 1,  -- last-write-wins 校验
    updated_at      TEXT    NOT NULL DEFAULT (datetime('now')),
    CHECK (
        (type IN ('tcp','udp') AND remote_port IS NOT NULL AND custom_domains IS NULL)
     OR (type IN ('http','https') AND remote_port IS NULL AND custom_domains IS NOT NULL)
    )
);
CREATE UNIQUE INDEX idx_proxies_tcp_remote ON proxies(type, remote_port)
    WHERE type IN ('tcp','udp');
-- customDomain 唯一约束在应用层做（解析 JSON 后比对）。

INSERT INTO schema_migrations(version) VALUES (1);
```

### 4.2 `migrations/0001_init.down.sql`

```sql
DROP TABLE IF EXISTS proxies;
DROP TABLE IF EXISTS kv;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS admin;
DELETE FROM schema_migrations WHERE version = 1;
```

### 4.3 损坏处理（实现 AC-12）

`storage.Open` 启动时若 `sqlite3_open_v2` 失败或 `PRAGMA integrity_check` 返回非 `ok`：将 `data.db` 重命名为 `data.db.broken-<RFC3339>`，新建空库重跑迁移，并向上层返回 `ErrCorruptReset`；HTTP 层据此进入"首次启动"状态。

---

## 5. API contracts（REST 端点契约）

### 5.1 统一约定

- 路径前缀：`/api/v1`。
- 内容类型：请求 / 响应均 `application/json; charset=utf-8`（除日志拖尾文本流）。
- 鉴权：除 `/api/v1/setup`、`/api/v1/auth/login`、`/api/v1/system/ready`、静态资源外，全部要求有效 `frp_easy_sid` cookie（`HttpOnly; SameSite=Lax`，HTTPS 时附 `Secure`）。
- 写接口需带 `X-CSRF-Token` 头，与 session 内 `csrf_token` 比对（NF-S3 同源 + token 双保险）。
- 状态码：2xx 正常 / 302 重定向（仅 HTML 路由）/ 401 未登录 / 403 CSRF 失败或权限不足 / 422 校验失败 / 429 限流 / 500 服务器错误。
- 错误体统一：

```json
{ "error": { "code": "VALIDATION_FAILED", "message": "name 已存在", "field": "name" } }
```

`code` 列表：`SETUP_REQUIRED`、`ALREADY_INITIALIZED`、`UNAUTHENTICATED`、`CSRF_FAILED`、`RATE_LIMITED`、`VALIDATION_FAILED`、`CONFLICT`、`NOT_FOUND`、`BIN_MISSING`、`PROC_BUSY`、`INTERNAL`。

### 5.2 路由清单

| 方法 + 路径 | 鉴权 | 请求体 | 200/2xx 响应 | 主要错误 |
|---|---|---|---|---|
| `GET /api/v1/system/ready` | 否 | — | `{ "initialized": bool, "binMissing": ["frpc"], "version": "0.1.0" }` | — |
| `POST /api/v1/setup` | 否（仅 `initialized=false` 时可调） | `{ "username": "admin", "password": "..." }` | `{ "ok": true }` + 自动登录 set-cookie | 409 `ALREADY_INITIALIZED`, 422 `VALIDATION_FAILED` |
| `POST /api/v1/auth/login` | 否 | `{ "username": "...", "password": "..." }` | `{ "ok": true }` + set-cookie | 401, 429 `RATE_LIMITED`（带 `Retry-After`） |
| `POST /api/v1/auth/logout` | 是 | — | `{ "ok": true }` | — |
| `POST /api/v1/auth/password` | 是 | `{ "oldPassword": "...", "newPassword": "..." }` | `{ "ok": true }` | 401, 422 |
| `GET /api/v1/auth/me` | 是 | — | `{ "username": "..." }` | 401 |
| `GET /api/v1/auth/csrf` | 是 | — | `{ "csrfToken": "..." }`（也通过响应头 `X-CSRF-Token`） | 401 |
| `GET /api/v1/mode` | 是 | — | `{ "frpc": true, "frps": false }` | — |
| `PUT /api/v1/mode` | 是 | `{ "frpc": bool, "frps": bool }` | `{ "frpc": ..., "frps": ... }` | 422 |
| `GET /api/v1/proxies` | 是 | — | `[ Proxy, ... ]` | — |
| `POST /api/v1/proxies` | 是 | `ProxyInput` | `Proxy` | 422, 409 `CONFLICT` |
| `PUT /api/v1/proxies/{id}` | 是 | `ProxyInput` + `version` | `Proxy` | 404, 409 |
| `DELETE /api/v1/proxies/{id}` | 是 | — | `{ "ok": true }` | 404 |
| `GET /api/v1/server` | 是 | — | `FrpsConfig`（`auth.token` 默认脱敏为 `"***"`，加 `?reveal=1` 返回明文） | — |
| `PUT /api/v1/server` | 是 | `FrpsConfig` | `FrpsConfig` | 422 |
| `GET /api/v1/client` | 是 | — | `FrpcServerConn`（脱敏同上） | — |
| `PUT /api/v1/client` | 是 | `FrpcServerConn` | `FrpcServerConn` | 422 |
| `POST /api/v1/proc/{kind}/start` | 是 | — | `ProcessInfo` | 409 `PROC_BUSY`, 422 `BIN_MISSING` |
| `POST /api/v1/proc/{kind}/stop` | 是 | — | `ProcessInfo` | 409 |
| `POST /api/v1/proc/{kind}/restart` | 是 | — | `ProcessInfo` | 409 |
| `GET /api/v1/proc/status` | 是 | — | `{ "frpc": ProcessInfo, "frps": ProcessInfo }` | — |
| `GET /api/v1/logs/{kind}` | 是 | query：`?lines=500` 或 `?offset=N` | 增量模式：`{ "data": "text", "nextOffset": N }`；末 N 行：`{ "lines": [...] }` | 404 |

`kind ∈ {"frpc","frps"}`。`ProxyInput`：

```ts
type ProxyInput = {
  name: string;          // ^[A-Za-z0-9_-]{1,64}$
  type: "tcp"|"udp"|"http"|"https";
  localIP?: string;      // 默认 "127.0.0.1"
  localPort: number;     // 1..65535
  remotePort?: number;   // tcp/udp 必填
  customDomains?: string[]; // http/https 必填，每项是合法域名
  enabled?: boolean;     // 默认 true
};
```

字段命名（响应体）一律 camelCase，与前端类型对齐；后端→FRP TOML 渲染时由 `internal/frpconf` 负责字段名映射，不让前后端关心 TOML 拼写。

### 5.3 SPA 路由 fallback

未匹配 `/api/` 前缀且未匹配 `/assets/...` 静态文件的 GET 请求 → 返回 `index.html`；浏览器侧 Vue Router（history 模式）接管 `/setup`、`/login`、`/dashboard`、`/proxies`、`/server`、`/client`、`/logs/{kind}`、`/settings`。

---

## 6. 关键技术决策（Q-4 / Q-8 / 嵌入 / 进程 / 二进制）

### 6.1 Q-4 持久化介质 → **SQLite (modernc.org/sqlite)**

候选评估：

| 候选 | 用户体验 | 工程规范 | 长期可维护性 |
|---|---|---|---|
| A. 单 JSON/TOML 文件 | 直观可手编 | 并发写易丢；无事务 | 数据演进只能整文件改 |
| B. SQLite 单文件 | 用户无需感知 | 事务、约束、索引、迁移成熟 | schema 演进有标准范式 |
| C. 多文件分目录 | 接近 A | 跨文件原子性需手工实现 | 演进成本高 |

**决策：B**。依据：
- 原则 ②：proxy 表存在唯一约束（name 全局唯一、`(type,remotePort)` 唯一）、版本号字段（last-write-wins），用 SQL 约束比应用层重复造轮子稳。
- 原则 ③：迁移文件机制是行业标准；未来要加 `users` 表（解锁 O-1 RBAC）或 `audit_log` 直接加迁移。
- 原则 ①：用户对持久化介质无感知；损坏恢复（AC-12）用 `PRAGMA integrity_check` 比 JSON 解析失败更可靠。
- **驱动选 `modernc.org/sqlite`**（纯 Go，**无 cgo**），保证 `go build` 跨平台单二进制零 toolchain 依赖；牺牲少量性能换可移植性，MVP 量级（≤200 proxy）完全够用。

> 这与项目元数据 `Go + Vue 3 + SQLite` 一致，无需 PM 修正 `.harness/rules/`。

### 6.2 Q-8 密码哈希 → **argon2id (`golang.org/x/crypto/argon2`)**

候选评估：

| 候选 | 安全强度 | Go 生态成熟度 | 性能/可用性 |
|---|---|---|---|
| A. bcrypt | 行业默认，但抗 GPU 弱于 argon2 | `golang.org/x/crypto/bcrypt` 一等公民 | 快、单参数 |
| B. argon2id | OWASP 2023 首选；抗 GPU/ASIC | `golang.org/x/crypto/argon2` 官方维护 | 慢，需调 m/t 参数 |
| C. scrypt | 老牌内存硬 | `golang.org/x/crypto/scrypt` | 调参更繁 |

**决策：B（argon2id）**。依据：
- 原则 ②：OWASP Password Storage Cheat Sheet 当前首推 argon2id；项目元数据"Web UI 管理 FRP"涉及生产网络入口，凭据强度直接关联可暴露面。
- 原则 ③：`golang.org/x/crypto/argon2` 是 Go 官方扩展库，长期稳定。
- 参数：`m=64 MiB`、`t=3`、`p=2`、`saltLen=16`、`keyLen=32`；存储格式 `$argon2id$v=19$m=65536,t=3,p=2$<salt>$<hash>`（PHC 标准串）。
- 单次哈希在普通笔记本 ≈80–150 ms，远低于 NF-P3 的 5 s 启停预算，登录路径完全可接受。

### 6.3 嵌入策略 → **`embed.FS` 单二进制**

候选：
- A. 独立部署前端（nginx / file server）。
- B. `embed.FS` 将 `web/dist/` 编入 Go 二进制，HTTP 层挂 `http.FS`。

**决策：B**。依据：原则 ①（用户克隆 → 一个可执行文件 → 跑起来），原则 ③（部署单元唯一，无前后端版本错配）。Vite 输出 hashed 文件名 + `index.html` SPA fallback 已是成熟模式。

### 6.4 进程模型 → **`os/exec` + supervisor goroutine per kind**

- 每个 kind（frpc/frps）一个 supervisor goroutine：负责 fork、读 stdout/stderr 到日志文件、监听退出、广播状态事件。
- 启动 = 写 TOML（`AtomicWrite`）→ fork 子进程 → 等 3 秒确认未退出 → state=running。
- 配置变更 frpc = 写 TOML → 调用 `frpcadmin.Reload`（5 s 超时）→ 失败回退到 Restart。
- 配置变更 frps = 始终 Restart（上游无 reload）。
- 退出：Linux SIGTERM→3s→SIGKILL；Windows 直接 `cmd.Process.Kill()`（FRP 在 Win 下对 Ctrl+Break 支持需 detached console，复杂度不值，MVP 阶段直接 kill 已可接受 — 配置/数据由 UI 侧持久化，子进程崩了无状态丢失）。

### 6.5 二进制查找 → `runtime.GOOS` switch；**保留 `frp_win/` `frp_linux/` 在 git 中**

```go
switch runtime.GOOS {
case "windows": return filepath.Join(root, "frp_win", "frpc.exe"), nil
case "linux":   return filepath.Join(root, "frp_linux", "frpc"), nil
}
```

缺失时不崩溃：UI 顶部 banner 提示 + 启动按钮 disable（AC-13）。

`root` 优先级：`FRP_EASY_ROOT` 环境变量 > 可执行文件所在目录 > `os.Getwd()`。

---

## 7. Sequence / flow（关键流程）

### 7.1 首次启动

```
浏览器 GET /dashboard
  → SPA 加载 → 调 GET /api/v1/system/ready
  → { initialized: false } → 路由跳 /setup
  → 用户填 username/password → POST /api/v1/setup
    → 校验（长度≥12 含字母数字）→ argon2id 哈希 → storage.SetAdmin
    → 创建 session + csrf → 返回 set-cookie
    → 客户端跳 /dashboard
```

### 7.2 新增一条 tcp proxy 并 5 秒内生效

```
POST /api/v1/proxies { name:"demo", type:"tcp", localPort:22, remotePort:6000 }
  → httpapi 校验字段 + 调 storage.UpsertProxy（DB 唯一约束兜底）
  → 触发 procmgr.ApplyConfigChange("frpc")
    → frpconf.RenderFrpc(...) → AtomicWrite runtime/frpc.toml
    → frpcadmin.Reload(ctx, strict=true)
      → 成功：返回 201 + Proxy{...}
      → 失败：procmgr.Restart("frpc")
        → 失败再降级：返回 500 + error.code=PROC_BUSY，原 DB 行打 enabled=false
  → 全过程总超时 5 s（HTTP handler 用 context.WithTimeout）
```

### 7.3 持久化损坏恢复

```
storage.Open(dataDir)
  → sqlite3 open OK，PRAGMA integrity_check 返回 "garbage..."
  → 改名 data.db → data.db.broken-2026-05-16T14-50-12Z
  → 重新 Open + Migrate
  → 返回 (store, ErrCorruptReset)
httpapi 启动后 system/ready 返回 initialized=false → SPA 走 /setup
```

---

## 8. Reuse audit（复用审计）

| 需求 | 是否有现成代码 | 文件路径 | 决策 |
|---|---|---|---|
| FRP 子进程二进制 | 是 — 上游 vendored | `C:\Programs\frp_easy\frp_win\frpc.exe` `frps.exe`、`C:\Programs\frp_easy\frp_linux\frpc` `frps` | 直接 fork 调用；不重新打包 |
| 示例 frpc/frps TOML | 是 | `C:\Programs\frp_easy\frp_win\frpc.toml` `frps.toml` | 作为字段命名验证样本；不直接读取 |
| HTTP framework | 否（仓库零业务代码） | — | 新建：`go-chi/chi v5`（理由：std net/http 兼容、零反射、中间件链清晰；比 gin 更轻） |
| ORM/SQL builder | 否 | — | 不引入 ORM；用 `database/sql` + 手写 SQL（schema 简单，引 ORM 反加噪音） |
| 密码哈希 | 否 | — | 新建：`golang.org/x/crypto/argon2`（理由见 6.2） |
| TOML 渲染 | 否 | — | 新建依赖：`github.com/pelletier/go-toml/v2`（理由：FRP 上游字段需精确 camelCase 序列化；标准库无 TOML） |
| SQLite 驱动 | 否 | — | 新建依赖：`modernc.org/sqlite`（理由见 6.1：纯 Go，免 cgo） |
| 前端组件库 | 否 | — | 新建：Naive UI（理由：Vue 3 原生，TS 类型完整，组件覆盖表单/表格/Modal/Toast/Tabs，无需自造） |
| HTTP 客户端 | 否 | — | 新建：axios（理由：拦截器/CSRF/统一错误处理成熟） |
| 状态管理 | 否 | — | 新建：Pinia（Vue 3 官方） |
| 路由 | 否 | — | 新建：Vue Router 4 |
| Harness 流水线骨架 | 是 | `C:\Programs\frp_easy\.harness\` | 复用：本文件即流水线产物 |
| verify_all 脚本 | 占位存在 | `C:\Programs\frp_easy\scripts\verify_all.{ps1,sh}` | Stage 4 dev-backend 扩充：加 `go vet ./...`、`go test ./...`、`go build`、`npm run build`、`npm run lint` |

**没有发现可直接复用的业务代码** —— 仓库零业务代码状态合理，所有 internal 包都是首建。

---

## 9. Risk analysis（风险与缓解）

| # | 风险 | 影响 | 缓解 |
|---|---|---|---|
| R-1 | frpc 热加载在某些字段变更（如 admin 段、auth 段）下静默失效 | 用户改 proxy 后 5s 内未生效，AC-5 失败 | 实现"reload → 1s 后调 `/api/status` 校验新 proxy 出现"；未出现即降级 restart；HTTP 响应附 `reloadStrategy: "reload"|"restart"` 字段供前端 toast 区分 |
| R-2 | Windows 下 `cmd.Process.Kill()` 是硬 kill，子进程未及时释放端口 | 后续 Start 时报 address already in use | 在 procmgr.Stop 之后跑一个"端口可用性"探测（最多 2s），失败则把 ProcessInfo.LastErr 标 `port still bound`，UI 醒目提示 |
| R-3 | argon2id 在低配机器上单次哈希 > 300 ms，登录被刷时压力大 | 登录 P95 退化 | 1) 内置参数针对最低 4C/4G 调优（m=32MiB 备选）；2) `/api/v1/auth/login` 路径独立限流（5 次/60s 已落 NF）；3) Stage 4 在 README 给出"超低配机器把 m 改 32768" 的注释 |
| R-4 | `modernc.org/sqlite` 是 SQLite 的 C → Go 翻译，体积大（编译产物 +10 MB） | 单二进制变胖 | 接受。文档化在 README "为何不用 cgo 版"。若 0.2 版本要瘦身，可切 `mattn/go-sqlite3` + 提供 CGO 构建脚本 |
| R-5 | `frp_win/` `frp_linux/` 共 ~70 MB 在 git，仓库克隆变慢 | git clone P95 拉长 | 接受。FRP 二进制不频繁更新（年级），git 增量小。若升级频繁，T-002 起改用 git-lfs；本期不引入避免额外工具链 |
| R-6 | last-write-wins 在两个浏览器 tab 同时编辑同一 proxy 时静默覆盖 | 用户体验受损 | 422 + 错误码 `CONFLICT`，body 含当前版本号与对端版本号；前端按 `version` 检测并提示"已被另一会话修改，是否覆盖" |
| R-7 | embed.FS 在 dev 模式不灵活（每次改前端要重 `go build`） | 开发效率低 | 提供"dev 双进程"模式：Go 启动时若 `--dev` 则不挂 embed，转而 proxy 到 `http://localhost:5173`（vite dev）；prod 构建用 `-tags embed` 启用 embed |

---

## 10. 项目脚手架决策（dev-* 落地指南）

### 10.1 后端 `go.mod`

```
module github.com/frp-easy/frp-easy

go 1.22

require (
    github.com/go-chi/chi/v5            v5.x  // HTTP router；轻量、context 友好
    github.com/pelletier/go-toml/v2     v2.x  // FRP TOML 渲染与解析
    golang.org/x/crypto                 v0.x  // argon2id
    modernc.org/sqlite                   v1.x  // 纯 Go SQLite，免 cgo 跨平台
)
```

构建入口：`go build -o bin/frp-easy ./cmd/frp-easy`（Linux）或 `go build -o bin/frp-easy.exe ./cmd/frp-easy`（Windows）。无 cgo 依赖，跨编译开箱即用。

测试入口：`go test ./...`，覆盖率：`go test -coverprofile=coverage.out ./...`。

### 10.2 前端 `web/package.json`

```
{
  "name": "frp-easy-web",
  "private": true,
  "type": "module",
  "scripts": {
    "dev":   "vite",
    "build": "vue-tsc --noEmit && vite build",
    "lint":  "eslint . --ext .ts,.vue",
    "test":  "vitest run"
  },
  "dependencies": {
    "vue":          "^3.4.0",
    "vue-router":   "^4.3.0",
    "pinia":        "^2.1.0",
    "naive-ui":     "^2.38.0",
    "axios":        "^1.6.0"
  },
  "devDependencies": {
    "vite":               "^5.2.0",
    "@vitejs/plugin-vue": "^5.0.0",
    "typescript":         "^5.4.0",
    "vue-tsc":            "^2.0.0",
    "eslint":             "^8.57.0",
    "eslint-plugin-vue":  "^9.25.0",
    "vitest":             "^1.5.0",
    "@types/node":        "^20.12.0"
  }
}
```

`vite.config.ts` 关键配置：

```ts
export default defineConfig({
  plugins: [vue()],
  base: './',
  build: { outDir: '../internal/assets/dist', emptyOutDir: true },
  server: { proxy: { '/api': 'http://127.0.0.1:8080' } }
})
```

构建产物直接落到 `internal/assets/dist/`，被 `embed.FS` 拾取。

### 10.3 启动与构建脚本

- `scripts/build.ps1`（Windows）/ `scripts/build.sh`（Linux）：
  1. `cd web && npm ci && npm run build`
  2. `go build -tags embed -o bin/frp-easy[.exe] ./cmd/frp-easy`
- `scripts/start.ps1` / `scripts/start.sh`：直接跑 `bin/frp-easy[.exe]`；可选 `--dev` 时跳过 embed 走 vite proxy。
- `scripts/verify_all.{ps1,sh}`：依次跑 `go vet`、`go test`、`go build`、`npm run lint`、`npm run build`、`npm run test`。

### 10.4 目录最终形态

```
C:\Programs\frp_easy\
├── cmd/frp-easy/main.go
├── internal/
│   ├── appconf/
│   ├── auth/
│   ├── binloc/
│   ├── frpcadmin/
│   ├── frpconf/
│   ├── httpapi/
│   ├── logtail/
│   ├── procmgr/
│   ├── storage/
│   └── assets/{embed.go, dist/}
├── migrations/{0001_init.up.sql, 0001_init.down.sql}
├── web/{src/, public/, index.html, vite.config.ts, package.json, tsconfig.json}
├── frp_win/         (保留)
├── frp_linux/       (保留)
├── scripts/{start.ps1, start.sh, build.ps1, build.sh, verify_all.ps1, verify_all.sh}
├── .frp_easy/       (运行时生成，gitignore)
│   ├── data.db
│   ├── runtime/{frpc.toml, frps.toml}
│   └── logs/{frpc.log, frps.log, ui.log}
├── go.mod / go.sum
└── ...（Harness 既有产物保留不变）
```

### 10.5 `.gitignore` 追加

```
# frp_easy runtime
.frp_easy/
internal/assets/dist/
web/node_modules/
bin/
```

注意：`frp_win/` `frp_linux/` **不**加 ignore。

---

## 11. Migration / rollout plan（迁移 / 上线）

- **零数据迁移**：项目首版，无历史数据。
- **首次启动等同迁移**：`storage.Open` 自动跑 `0001_init.up.sql`；后续版本追加 `0002_*.up.sql`，**绝不修改已合并迁移**（dev-db 红线）。
- **回滚**：每个迁移配套 `*.down.sql`；本期仅 0001。
- **Feature flag**：MVP 范围内无需。后续 `O-*` 上线时按需用 `kv` 表的 `features.<name>` key 控制。
- **二进制升级**：用户拉新版本 git pull → 重启进程；SQLite 文件原地兼容（schema 增量）。
- **配置兼容**：FRP 上游字段命名 1.x 系列稳定（context7 校验已确认），若 FRP 升级出现 breaking change，`internal/frpconf` 单点改即可。
- **降级路径**：用户可手动停 UI 进程，直接编辑 `.frp_easy/runtime/frpc.toml` + `frpc -c` 运行，绕过本工具 —— UI 故障不锁死 FRP。

---

## 12. Out-of-scope clarifications（设计边界）

本设计明确**不**覆盖以下项（保持与 01 文档 §3 Out-of-scope 一致，并在技术层重申）：

- 集群 / 多节点 frps 集中管理（不引入消息总线、分布式存储）。
- 多账号 / RBAC（`admin` 表硬约束 `id=1`，仅允许单行）。
- HTTPS / TLS 自动签发（UI 仅 HTTP，监听 `127.0.0.1`）。
- 子进程的资源隔离（cgroup / Job Object）—— 信任本地用户。
- Prometheus / OpenTelemetry —— 仅结构化日志到文件。
- WebSocket / SSE 实时推送 —— MVP 用 polling（前端 2s 拉一次状态，符合 NF-P3 即可）。
- 前端 i18n / 主题切换 —— 仅中文 + 默认主题。
- 自动升级 FRP 二进制 —— 用户手动替换 `frp_win/` `frp_linux/` 内容。

---

## 13. Partition assignment（分区分配 — REQUIRED）

> 仓库存在 `.harness/agents/dev-db.md`、`dev-backend.md`、`dev-frontend.md` 与兜底 `developer.md`。本项目用三分区。下表"Partition"对应这三个 agent 的 owned paths（已校准 owned globs 未覆盖本项目布局 — 详见 §13.3 备注）。

### 13.1 文件级分配

| 文件 / 目录 | Partition | New / Edit | 依赖 |
|---|---|---|---|
| `migrations/0001_init.up.sql` | dev-db | new | — |
| `migrations/0001_init.down.sql` | dev-db | new | — |
| `internal/storage/store.go`（连接 + Migrate） | dev-db | new | 依赖 migrations |
| `internal/storage/admin.go` | dev-db | new | store.go |
| `internal/storage/sessions.go` | dev-db | new | store.go |
| `internal/storage/kv.go` | dev-db | new | store.go |
| `internal/storage/proxies.go` | dev-db | new | store.go |
| `internal/storage/storage_test.go`（DAO 单测，含损坏恢复） | dev-db | new | 全 DAO |
| `go.mod` / `go.sum` | dev-backend | new | — |
| `cmd/frp-easy/main.go` | dev-backend | new | 依赖所有 internal/* |
| `internal/appconf/*.go` | dev-backend | new | — |
| `internal/auth/*.go` | dev-backend | new | storage |
| `internal/binloc/*.go` | dev-backend | new | — |
| `internal/frpconf/*.go` | dev-backend | new | storage |
| `internal/frpcadmin/*.go` | dev-backend | new | — |
| `internal/procmgr/*.go` | dev-backend | new | frpconf + binloc + frpcadmin |
| `internal/logtail/*.go` | dev-backend | new | — |
| `internal/httpapi/router.go` + `handlers_*.go` | dev-backend | new | 全部 internal/* |
| `internal/assets/embed.go` | dev-backend | new | 等待前端 dist/ 产物存在 |
| `internal/**/*_test.go`（backend 单测 + httptest） | dev-backend | new | 对应实现 |
| `web/package.json` `tsconfig.json` `vite.config.ts` `index.html` | dev-frontend | new | — |
| `web/src/main.ts`、`router.ts`、`stores/*.ts` | dev-frontend | new | — |
| `web/src/api/*.ts`（axios 客户端 + 类型） | dev-frontend | new | 后端契约 §5 |
| `web/src/pages/Setup.vue` `Login.vue` `Dashboard.vue` `Proxies.vue` `Server.vue` `Client.vue` `Logs.vue` `Settings.vue` | dev-frontend | new | api 模块 |
| `web/src/components/*.vue`（ProxyForm、StatusBadge、LogViewer、ConfirmDialog 等） | dev-frontend | new | — |
| `web/src/**/__tests__/*.spec.ts`（Vitest） | dev-frontend | new | 对应组件 |
| `scripts/start.ps1` / `start.sh` | dev-backend | new | bin 产物 |
| `scripts/build.ps1` / `build.sh` | dev-backend | new | go + npm |
| `scripts/verify_all.ps1` / `verify_all.sh` | dev-backend | edit（占位 → 实体化） | go + npm |
| `.gitignore` | dev-backend | edit（追加 §10.5） | — |
| `docs/dev-map.md` | 各分区在各自 stage 末追加自己负责的目录索引 | edit | — |

### 13.2 Dispatch order

1. **dev-db**：迁移 + storage 包 + DAO 单测。完成后产物可被 dev-backend 引入。
2. **dev-backend**：业务逻辑 + HTTP + procmgr + 启动 / 构建 / verify_all 脚本。**先做不依赖前端的部分**（除 `internal/assets/embed.go`），跑通 `go build`。
3. **dev-frontend**：脚手架 + 页面 + 组件 + 类型对齐 §5 契约 + Vitest。
4. **dev-backend（第二轮）**：在 `web/dist/` 存在后补 `internal/assets/embed.go` + 跑最终 `verify_all`。

### 13.3 Parallelism

- **dev-db ↔ dev-backend**：严格顺序（backend storage 调用依赖 dev-db 完成的接口与迁移）。
- **dev-backend ↔ dev-frontend**：**可并行**。前提是契约 §5 已冻结（本设计已冻结），前端按本文件类型先写假 mock 调用，后端独立推进。**集成在 dev-backend 第二轮回收**。
- **dev-frontend 内部**：页面间独立可并行（同一 agent 内 TodoWrite 调度即可）。

### 13.4 Owned-path 备注（给 PM 与 dev-* 的提示）

本项目实际布局（Go `cmd/` `internal/`、前端 `web/`）与 `.harness/agents/dev-*.md` 当前列出的 owned globs（`apps/`、`src/server/` 等基于 TS/Next.js 模板）不完全对齐。**建议**在 Stage 4 启动前，PM 在 `.harness/agents/dev-backend.md` 与 `dev-frontend.md` 追加：

- dev-backend owned：`cmd/**`、`internal/**`、`scripts/**`、`go.mod`、`go.sum`
- dev-frontend owned：`web/**`
- dev-db owned：`migrations/**`

并执行 `scripts/harness-sync` 同步 `.claude/`。本动作不属 Solution Architect 范畴，仅作提醒（红线第 8 条："优先让 AI 帮你编辑 `.harness/`"）。

---

## 14. Verdict（裁定）

**READY**

依据：
- 12 节齐备（§1–§12），并按需补 §13 分区表与 §14 verdict。
- Q-4 / Q-8 两项显式技术选型已在 §6 给出决策 + 三原则依据 + 备选对比。
- 嵌入策略、进程模型、二进制查找均给出明确选择与代价说明。
- 完整 REST 契约 §5、数据模型 §4、迁移文件路径 §4.1 已落定。
- Reuse audit §8 非空（vendored FRP 二进制 + Harness 骨架被显式复用），所有新代码有依据。
- 风险 7 条 + 缓解（≥3 的硬要求超额满足）。
- 分区分配表 + dispatch order + parallelism 完整，给 dev-db / dev-backend / dev-frontend 直接可执行。
- 所有 FRP 字段命名（`webServer.addr/port/user/password`、`auth.method/auth.token/auth.oidc.*`、`[[proxies]]` 数组段的 `serverAddr/serverPort/localIP/localPort/remotePort/customDomains`、frpc admin 默认端口 7400 与 `GET /api/reload`/`GET /api/status`）已通过 context7 校验（见附录 A）。

下一阶段：**gate-reviewer** 评审本设计；通过后按 §13.2 顺序派 dev-db → dev-backend → dev-frontend。

---

## 附录 A · context7 关键证据（FRP 字段 / API 校准）

### A.1 frpc admin API：热加载与状态

来源：`/fatedier/frp` README + llms.txt（context7 拉取，2026-05-16）

```toml
# frpc.toml — 启用 admin API 的最小配置
webServer.addr     = "127.0.0.1"
webServer.port     = 7400
webServer.user     = "admin"
webServer.password = "admin"
```

端点：

- `GET /api/reload[?strictConfig=true]` — basic auth，成功 200 空 body。
- `GET /api/status` — 返回按 type 分组的 proxy 状态数组，示例字段：`name / type / status / err / local_addr / plugin / remote_addr`。

### A.2 frps 鉴权与 dashboard

```toml
# 鉴权
auth.method = "token"     # 默认 token；可选 "oidc"
auth.token  = "s3cr3t"
# OIDC 时：
# auth.method = "oidc"
# auth.oidc.issuer   = "https://..."
# auth.oidc.audience = "..."

# Dashboard
webServer.addr     = "127.0.0.1"   # 默认 127.0.0.1
webServer.port     = 7500
webServer.user     = "admin"       # 可选
webServer.password = "admin"       # 可选
```

本项目 MVP：`auth.method` 仅在 `auth.token` 非空时设为 `"token"`，否则整段不写（FRP 默认行为 = 无鉴权）；OIDC 不纳入 MVP（Out-of-scope）。

### A.3 `[[proxies]]` 数组与字段

```toml
serverAddr = "x.x.x.x"
serverPort = 7000

[[proxies]]
name       = "ssh"
type       = "tcp"
localIP    = "127.0.0.1"
localPort  = 22
remotePort = 6000

[[proxies]]
name          = "web"
type          = "http"
localPort     = 80
customDomains = ["www.example.com"]
```

字段命名 **camelCase**（FRP 上游 1.x 已稳定）；`[[proxies]]` 是 TOML 数组表语法；`type` 互斥规则与 4.1 一致（tcp/udp 用 `remotePort`，http/https 用 `customDomains`）。

`internal/frpconf` 渲染时**逐字段照搬**这套命名，不做风格转换 —— UI 内部用 camelCase JSON，FRP TOML 也是 camelCase，两边天然一致。
