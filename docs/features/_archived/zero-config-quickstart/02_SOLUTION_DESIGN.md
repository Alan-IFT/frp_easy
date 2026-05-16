# 02 · 方案设计 — T-002 · zero-config-quickstart

> 模式：`full` · 编写：solution-architect · 日期：2026-05-16
> 上游输入（只读）：`docs/features/zero-config-quickstart/01_REQUIREMENT_ANALYSIS.md`（verdict=READY）
> 上游历史：T-001 `docs/features/web-ui-mvp/02_SOLUTION_DESIGN.md`（架构已全部落地）
> 决策原则（沿用 T-001）：① 用户体验 > ② 软件工程规范 > ③ 长期可维护性

---

## 1. Architecture summary（架构总览）

T-002 在 T-001 已落地的"单 Go 二进制 + 内嵌 Vue 3 SPA"架构基础上做四处增量扩展，不改变核心架构：

1. 新增 `internal/downloader` 包——管理 frp 二进制的异步下载、解压、原子安装，向 HTTP 层暴露状态轮询接口；
2. 扩展 `internal/httpapi` 的 system 域——新增下载触发、下载状态、公网 IP 检测三条受保护端点；新增独立 handlers_wizard.go 处理向导状态持久化（KV 复用现有 `kv` 表，不增 schema）；
3. 前端新增 `/wizard` 独立页面、`FirewallHint` / `PublicIpDetector` 两个复用组件、两个 Pinia store（downloader / wizard），并对 `AppLayout`、`Server.vue`、`Proxies.vue`、`router.ts` 做最小化修改；
4. 无新 SQL 迁移文件：所有新的持久化状态（wizard.handled）均通过现有 `kv` 表的 KV 接口存取。

---

## 2. Affected modules（受影响模块清单）

| 模块 | 路径 | 状态 |
|---|---|---|
| FRP 二进制下载器 | `C:\Programs\frp_easy\internal\downloader\downloader.go` | new |
| 下载器单测 | `C:\Programs\frp_easy\internal\downloader\downloader_test.go` | new |
| HTTP system handler 扩展 | `C:\Programs\frp_easy\internal\httpapi\handlers_system.go` | edit（追加 3 handler + ipCache 内部类型） |
| HTTP wizard handler | `C:\Programs\frp_easy\internal\httpapi\handlers_wizard.go` | new |
| HTTP 路由层 | `C:\Programs\frp_easy\internal\httpapi\router.go` | edit（追加 5 条路由 + Dependencies.Downloader 字段） |
| 程序入口 | `C:\Programs\frp_easy\cmd\frp-easy\main.go` | edit（注入 downloader.Manager） |
| Wizard 页面 | `C:\Programs\frp_easy\web\src\pages\Wizard.vue` | new |
| 下载器 API 模块 | `C:\Programs\frp_easy\web\src\api\downloader.ts` | new |
| Wizard API 模块 | `C:\Programs\frp_easy\web\src\api\wizard.ts` | new |
| 防火墙提示组件 | `C:\Programs\frp_easy\web\src\components\FirewallHint.vue` | new |
| 公网 IP 检测组件 | `C:\Programs\frp_easy\web\src\components\PublicIpDetector.vue` | new |
| 下载器 Pinia store | `C:\Programs\frp_easy\web\src\stores\downloader.ts` | new |
| Wizard Pinia store | `C:\Programs\frp_easy\web\src\stores\wizard.ts` | new |
| 路由 + 导航守卫 | `C:\Programs\frp_easy\web\src\router.ts` | edit（添加 /wizard 路由 + 守卫逻辑） |
| 服务端配置页 | `C:\Programs\frp_easy\web\src\pages\Server.vue` | edit（嵌入 PublicIpDetector + FirewallHint） |
| 布局组件（下载按钮） | `C:\Programs\frp_easy\web\src\components\AppLayout.vue` | edit（banner 加一键下载按钮） |
| 代理列表页 | `C:\Programs\frp_easy\web\src\pages\Proxies.vue` | edit（保存成功后展示 FirewallHint） |
| 前端类型定义 | `C:\Programs\frp_easy\web\src\types.ts` | edit（追加 4 个新接口） |
| System API 模块 | `C:\Programs\frp_easy\web\src\api\system.ts` | edit（追加 public-ip 调用函数） |

---

## 3. Module decomposition（新模块分解）

### 3.1 `internal/downloader`

**职责**：异步下载 frp GitHub Releases 二进制，追踪每个 kind（frpc/frps）的下载进度，原子安装到 binloc 对应目录。

**公开 API**（Go）：

```go
package downloader

// FRPVersion 是目标下载版本，与 frp_linux/ frp_win/ 现有版本保持一致。
// 开发者需在实现时运行 `frp_linux/frpc --version` 确认版本号后填入。
const FRPVersion = "0.61.1"  // 占位符；实现时以实际 vendored 版本为准

// Status 值
const (
    StatusIdle        = "idle"
    StatusDownloading = "downloading"
    StatusSuccess     = "success"
    StatusFailed      = "failed"
)

// DownloadState 是单个 kind 的当前下载状态（用于 JSON 序列化）。
type DownloadState struct {
    Status   string `json:"status"`              // idle|downloading|success|failed
    Progress int    `json:"progress"`             // 0-100，仅 downloading 状态有意义
    Error    string `json:"error,omitempty"`
}

var (
    ErrAlreadyInProgress = errors.New("downloader: download already in progress")
    ErrUnsupportedOS     = errors.New("downloader: unsupported OS (only windows/linux amd64)")
    ErrBadKind           = errors.New("downloader: kind must be 'frpc' or 'frps'")
)

// Manager 管理 frpc/frps 的并发下载状态。
// root 是 binloc.Locator.Root() 返回的仓库根目录。
type Manager struct { /* sync.Mutex + states map[string]*DownloadState + root string + *http.Client */ }

func New(root string) *Manager

// Start 触发异步下载。kind ∈ {"frpc","frps"}。
// 已在下载中返回 ErrAlreadyInProgress；frpc 与 frps 互不影响可并发。
func (m *Manager) Start(kind string) error

// Status 返回 kind 当前状态副本。ok=false 表示 kind 不合法。
func (m *Manager) Status(kind string) (DownloadState, bool)
```

**内部实现要点**：

- 下载 URL（HTTPS，NF-S2 要求）：
  - Linux amd64：`https://github.com/fatedier/frp/releases/download/v{FRPVersion}/frp_{FRPVersion}_linux_amd64.tar.gz`
  - Windows amd64：`https://github.com/fatedier/frp/releases/download/v{FRPVersion}/frp_{FRPVersion}_windows_amd64.zip`
- 下载超时：`context.WithTimeout(ctx, 60*time.Second)`（Q-8 决策）
- 进度计算：若响应头含 `Content-Length`，则 `progress = bytesWritten*100/contentLength`；若无 Content-Length，则伪进度（每 512KB 递增 2%，最高 95%，完成后跳 100）
- 解压：
  - Linux `.tar.gz`：`compress/gzip` + `archive/tar`，找 entry 路径匹配 `*/frpc` 或 `*/frps`，复制到临时文件
  - Windows `.zip`：`archive/zip`，找 entry 匹配 `*/frpc.exe` 或 `*/frps.exe`
  - **安全校验**：提取前检查 entry.Name 不含 `..` 或绝对路径前缀（防 Zip Slip，R-2）
- 原子安装：
  - 下载到 `os.CreateTemp(targetDir, ".dl-*.tmp")`
  - `os.Rename(tmp, targetPath)` 与 `frpconf.AtomicWrite` 同模式（NF-S1）
  - Linux：`os.Chmod(targetPath, 0o755)`
- 日志（NF-O1）：下载开始/完成/失败均通过注入的 `*slog.Logger` 写 JSON 结构化日志；进度更新不写日志（避免刷爆）
- 失败时清理临时文件；不覆盖已存在的有效二进制（B-4：下载失败不破坏已有文件）

### 3.2 `internal/httpapi/handlers_wizard.go`（新文件）

**职责**：wizard 状态读写，不含配置保存逻辑（配置保存复用现有 PUT /api/v1/client、PUT /api/v1/server、PUT /api/v1/mode）。

```go
const kvWizardHandled = "wizard.handled"

// WizardStatus 是 GET /api/v1/wizard/status 的响应体。
type WizardStatus struct {
    Handled    bool `json:"handled"`
    ShouldShow bool `json:"shouldShow"`
}

// wizardStatus：GET /api/v1/wizard/status
// shouldShow = !handled && !hasAnyConfig
// hasAnyConfig = (frpc.serverConn 在 KV 中存在)
//             || (frps.config 在 KV 中存在)
//             || mode.frpc.enabled == "true"
//             || mode.frps.enabled == "true"
func (h *handlers) wizardStatus(w http.ResponseWriter, r *http.Request)

// wizardComplete：POST /api/v1/wizard/complete（跳过和完成均调此接口）
// 持久化 wizard.handled = "true"，响应 { "ok": true }
func (h *handlers) wizardComplete(w http.ResponseWriter, r *http.Request)
```

### 3.3 `handlers_system.go` 扩展（新增内部类型 + 3 个 handler）

```go
// --- 公网 IP 缓存（进程内，非持久化）---
type ipResult struct {
    IP       string
    Advisory string // IPv6 advisory，空 = 无
    ErrMsg   string
}

type ipCache struct {
    mu        sync.Mutex
    result    *ipResult
    fetchedAt time.Time
}

// handlers 结构体增加 ipCache 字段（零值合法）。
type handlers struct {
    deps    Dependencies
    ipCache ipCache
}

// PublicIPResponse 是 GET /api/v1/system/public-ip 的响应体。
// 始终 HTTP 200（B-14）。
type PublicIPResponse struct {
    IP       string `json:"ip,omitempty"`
    Error    string `json:"error,omitempty"`
    Advisory string `json:"advisory,omitempty"` // IPv6 场景提示
}

// DownloadBinRequest 是 POST /api/v1/system/download-bin 的请求体。
type DownloadBinRequest struct {
    Kind string `json:"kind"` // "frpc" | "frps"
}

// systemPublicIP：GET /api/v1/system/public-ip
// 缓存 TTL 5 分钟；超时 3 秒；主备两个外部服务（见下方）
func (h *handlers) systemPublicIP(w http.ResponseWriter, r *http.Request)

// downloadBin：POST /api/v1/system/download-bin → 202 or 409
func (h *handlers) downloadBin(w http.ResponseWriter, r *http.Request)

// downloadStatus：GET /api/v1/system/download-status/{kind} → 200 DownloadState
func (h *handlers) downloadStatus(w http.ResponseWriter, r *http.Request)
```

IP 检测服务（按顺序尝试，单服务超时 1.5s，总预算 3s）：

| 优先级 | URL | 响应格式 |
|---|---|---|
| 主 | `https://api.ipify.org?format=json` | `{"ip":"x.x.x.x"}` |
| 备 | `https://api.my-ip.io/json` | `{"ip":"x.x.x.x","country":"..."}` |

IPv6 判断：`net.ParseIP(ip).To4() == nil` → advisory = `"IPv6 地址，frpc serverAddr 填写时请加方括号 [ip]"`。

---

## 4. Data model changes（数据模型）

**无新迁移文件**。T-001 的 `kv` 表已具备所有能力。

新 KV key：

| Key | 类型 | 取值 | 说明 |
|---|---|---|---|
| `wizard.handled` | string | `"true"` | 存在且为 "true" 时 wizard 不再自动展示 |

wizard.handled 不存在时等同于 `false`（KV miss = 未处理），与 `KVGet` 的 `ok=false` 语义一致，无需初始化。

现有 KV keys 用于 wizard `hasAnyConfig` 判断（只读，不修改）：

| Key | 判断条件 |
|---|---|
| `frpc.serverConn` | KV 行存在（ok=true），表明用户曾保存过 frpc 配置 |
| `frps.config` | KV 行存在（ok=true） |
| `mode.frpc.enabled` | 值为 "true" |
| `mode.frps.enabled` | 值为 "true" |

---

## 5. API contracts（REST 端点契约）

### 5.1 新增端点一览

| 方法 + 路径 | 鉴权 | CSRF | 请求体 | 成功响应 | 主要错误 |
|---|---|---|---|---|---|
| `GET /api/v1/system/public-ip` | session | 否 | — | 200 `PublicIPResponse` | — |
| `POST /api/v1/system/download-bin` | session | 是 | `{"kind":"frpc"}` | 202 `{"ok":true}` | 409 `PROC_BUSY`, 422 `VALIDATION_FAILED` |
| `GET /api/v1/system/download-status/{kind}` | session | 否 | — | 200 `DownloadState` | 404 kind 不合法 |
| `GET /api/v1/wizard/status` | session | 否 | — | 200 `WizardStatus` | — |
| `POST /api/v1/wizard/complete` | session | 是 | — | 200 `{"ok":true}` | — |

### 5.2 响应体形状

```json
// GET /api/v1/system/public-ip — 成功
{ "ip": "203.0.113.1" }

// GET /api/v1/system/public-ip — IPv6
{ "ip": "2001:db8::1", "advisory": "IPv6 地址，frpc serverAddr 填写时请加方括号 [2001:db8::1]" }

// GET /api/v1/system/public-ip — 超时或不可达
{ "error": "检测超时，请手动查询" }

// GET /api/v1/system/download-status/{kind}
{ "status": "downloading", "progress": 42 }
{ "status": "failed", "error": "下载超时" }
{ "status": "success", "progress": 100 }

// GET /api/v1/wizard/status
{ "handled": false, "shouldShow": true }
```

### 5.3 既有端点不变

T-002 wizard 配置步骤直接调用现有端点：`PUT /api/v1/client`、`PUT /api/v1/server`、`PUT /api/v1/mode`——无任何修改，完整复用 T-001 契约。

### 5.4 Dependencies struct 变更

```go
// internal/httpapi/router.go — Dependencies 追加一个字段
type Dependencies struct {
    // ... 全部 T-001 字段不变 ...
    Downloader *downloader.Manager // nil 时 download 端点返回 503（测试安全）
}
```

---

## 6. Sequence / flow（关键流程）

### 6.1 FRP 二进制下载流程

```
Browser                 Go Handler              downloader.Manager     GitHub CDN
  │──POST /download-bin──►│                           │                     │
  │◄──202 ok──────────────│──Start("frpc")───────────►│                     │
  │                        │                      goroutine: GET ──────────►│
  │──GET /download-status──►│                           │◄── 200+bytes ──────│
  │◄──{downloading, 20%}───│◄──Status("frpc")──────────│   progress++        │
  │  (每 1s 轮询)           │                           │                     │
  │◄──{downloading, 80%}───│                           │                     │
  │◄──{success, 100%}──────│                      AtomicRename(frp_linux/frpc)
  │──GET /system/ready─────►│                           │                     │
  │◄──{binMissing: []}──────│  (Locator.Missing() 重新扫描)                   │
```

### 6.2 Wizard 跳转流程（首次 /setup 后）

```
Browser (Setup.vue.handleSubmit)      router beforeEach          Go Handler
  │──POST /setup──────────────────────►│ (已有，无变化)             │
  │◄──200 + set-cookie──────────────────│                           │
  router.push('/dashboard')
         │──navigation to /dashboard──►│                           │
         │    auth.user !== null?  yes  │                           │
         │    wizard.checked? false     │──GET /wizard/status──────►│
         │                             │◄──{shouldShow:true}────────│  KVGet(wizard.handled): miss
         │                             │                            │  hasAnyConfig(): false
         │◄──return '/wizard'──────────│                           │
  浏览器跳到 /wizard
  用户选 frps 角色 → 填 bindPort → 点"完成配置"
         │──PUT /api/v1/server────────►│ (复用 T-001 handler)       │
         │──PUT /api/v1/mode──────────►│ (复用 T-001 handler)       │
         │──POST /wizard/complete─────►│──KVSet("wizard.handled","true")
         │◄──{ok:true}─────────────────│                           │
  router.push('/dashboard')
  再次登录（后续）:
         │──GET /wizard/status─────────────────────────────────────►│
         │◄──{handled:true, shouldShow:false}────────────────────────│
         不跳转 /wizard
```

### 6.3 公网 IP 检测流程（含缓存）

```
Browser             handlers.ipCache     ipify.org / my-ip.io
  │──GET /public-ip──►│                        │
  │                    lock; cache miss         │
  │                    │──GET api.ipify.org ───►│ (1.5s timeout)
  │                    │◄──{"ip":"1.2.3.4"}─────│
  │                    cache result; unlock     │
  │◄──{"ip":"1.2.3.4"}─│                        │
  (5 分钟内再次请求):
  │──GET /public-ip──►│                        │
  │                    lock; cache hit          │
  │◄──{"ip":"1.2.3.4"}─│  (不调外部)           │
```

### 6.4 防火墙提示展示流程（纯前端，无后端调用）

```
Server.vue.handleSave()
  ──► PUT /api/v1/server (已有)
  ──► 成功后：savedPorts = [{port: bindPort, proto:'tcp', label:'frps 监听端口'}]
              if dashboardEnabled: push {port: dashboardPort, ...}
  ──► <FirewallHint :ports="savedPorts" /> 渲染

Proxies.vue.handleSubmit()
  ──► POST/PUT /api/v1/proxies (已有)
  ──► 成功后，仅 type ∈ {tcp, udp}：
        firewallPorts = [{port: proxy.remotePort, proto: proxy.type, label:'...'}]
  ──► <FirewallHint :ports="firewallPorts" :serverContext="false" />
  http/https type → firewallPorts = []（B-20：不显示）
```

---

## 7. Reuse audit（复用审计）

| 需求 | 现有代码 | 文件路径 | 决策 |
|---|---|---|---|
| 二进制路径定位 / 缺失检测 | `binloc.Locator.Root()` + `Missing()` | `internal/binloc/binloc.go` | 复用：下载器用 Root() 确定安装目录；/system/ready 的 Missing() 不变 |
| 原子文件写入模式 | `frpconf.AtomicWrite` | `internal/frpconf/render.go` | 下载器**不直接调用**（下载器只需 os.CreateTemp + os.Rename，逻辑自含），但遵循相同模式 |
| KV 持久化 | `Store.KVGet / KVSet` | `internal/storage/kv.go` | 复用：wizard.handled 写入现有 kv 表 |
| HTTP 错误响应格式 | `writeError / writeJSON` | `internal/httpapi/errors.go` | 复用：所有新 handler 统一调用 |
| CSRF + SessionAuth 中间件 | 现有中间件链 | `internal/httpapi/middleware.go` | 复用：新写接口自动受 CSRF 保护（在 protected group 内） |
| 错误码 `PROC_BUSY` | 已存在 | `internal/httpapi/errors.go` | 复用：下载已在进行中时使用此码（语义吻合：同种进程操作冲突） |
| Pinia store 模式 | `stores/app.ts`, `stores/proxies.ts` | `web/src/stores/` | 复用模式，新 store 跟随相同 `defineStore` + 接口声明约定 |
| axios 拦截器 + CSRF | `apiClient` | `web/src/api/client.ts` | 复用：所有新 API 模块 import apiClient，不新建 axios 实例 |
| Naive UI 组件 | `NCollapse / NButton / NAlert / NProgress` | `web/src/` | 复用：FirewallHint 用 NCollapse，AppLayout 下载进度用 NProgress |
| frpc/frps 配置保存端点 | `PUT /api/v1/client` `PUT /api/v1/server` `PUT /api/v1/mode` | `internal/httpapi/handlers_server.go`, `handlers_mode.go` | 复用：wizard 表单提交直接调用这些现有端点，无需新增 |

**新依赖情况**：无需引入新的 Go 或 npm 依赖。下载、解压所用 `archive/tar`、`archive/zip`、`compress/gzip`、`net` 均为 Go 标准库。公网 IP 检测使用 `net/http`。

---

## 8. Risk analysis（风险分析）

| # | 风险 | 影响 | 缓解 |
|---|---|---|---|
| R-1 | GitHub Releases CDN 在中国大陆访问缓慢或超时 | 下载超时（60s），用户体验差 | ① 显示进度条，用户看到下载中不会误以为卡住；② 失败后展示 GitHub Releases 手动下载链接；③ 备注：超时时长已是 Q-8 最宽松选择（60s），无更好的 pure-code 方案 |
| R-2 | Zip Slip 攻击（tar.gz / zip 内含路径穿越 entry） | 下载器将文件写到非预期目录，安全漏洞 | 提取前校验每个 entry.Name：不含 `..`、不以 `/` 或 `C:\` 开头；不符合则跳过并报错 |
| R-3 | wizard 状态检查 hasAnyConfig 太松，frps.config KV 由 getServer 写入默认值时误触发"已配置" | wizard 条件误判，不应出现向导但出现了 | 分析代码确认：`getServer` **不**写 KV，只读（见 handlers_server.go line 37-47）；`putServer` 才写 KV。因此 frps.config 的 KV 行存在意味着用户曾显式调用 PUT，判据成立 |
| R-4 | 下载成功后 `Locator.Missing()` 仍返回 kind | 前端 banner 不消失，用户困惑 | binloc.Missing() 每次调用都重新 `os.Stat`（见 binloc.go line 134-143），下载完成后立即生效；前端 fetchReady() 刷新后 banner 消失（AC-3） |
| R-5 | wizard 在 /wizard 路由直接访问时绕过守卫 | 未登录用户访问 wizard | router.beforeEach 守卫已有"已初始化但未登录 → /login"逻辑；/wizard 落在此条之后被正常保护 |
| R-6 | ipify.org / my-ip.io 在某些内网环境返回内网 IP | IP 显示不准 | 需求侧已接受（4.3 boundary：私有 IP 原样展示，由用户判断）；advisory 信息不过滤 |
| R-7 | download-bin 并发触发（双 tab 同时点击） | 409 PROC_BUSY 响应，其中一个请求被拒绝 | 正确行为：已在下载中时返回 409，前端按钮在 downloading 状态禁用（B-2），409 时前端展示"正在下载中"提示即可 |
| R-8 | FRPVersion 常量与 vendored 版本不一致 | 下载后二进制与现有 frpc.toml 字段不兼容 | 实现时通过 `frp_linux/frpc --version` 确认版本并更新常量；FRP 1.x 系列 camelCase TOML 格式稳定（已由 T-001 验证），版本间字段变化风险低 |

---

## 9. Migration / rollout plan（迁移/上线）

- **无 schema 迁移**：`wizard.handled` KV key 不存在等同 false，零配置初始状态自动正确。
- **向后兼容**：T-001 全部已有端点（22 条路由）不做任何修改；现有前端页面除最小化 edit 外不受影响。
- **T-001 测试基线保护**：`go test ./...` 测试基线 ≥ 146 条（T-001 交付时测试数量）。T-002 新增测试覆盖 5 个新 AC 范围（AC-2、AC-6、AC-12/13、AC-14/15/16）。
- **Feature flag**：不引入。wizard 跳转逻辑由 `wizard.handled` KV 控制，已初始化系统的老用户因 hasAnyConfig=true，shouldShow=false，自动跳过 wizard（不感知此功能）。
- **回滚**：如需回滚 T-002，因无 schema 变更，直接回滚二进制即可。KV 中新增的 `wizard.handled` 行对旧版本无影响（旧代码不读该 key）。
- **二进制升级**：`FRPVersion` 常量在 `internal/downloader/downloader.go` 单点维护；升级版本只需修改该常量并测试。

---

## 10. Out-of-scope clarifications（设计边界）

| 项 | 依据 |
|---|---|
| FRP 二进制 SHA-256 校验 | 01 O-2；MVP 接受 HTTPS 传输安全 |
| arm64 / Apple Silicon 下载支持 | 01 O-3；仅 amd64 |
| wizard 完成后自动启动进程 | 01 O-4；启动由用户在 /dashboard 手动触发 |
| frpc 页面公网 IP 检测 | 01 O-5；frpc 客户端在 NAT 内网场景下本机公网 IP 无助于 frpc 配置 |
| frps vhostHTTPPort/vhostHTTPSPort 防火墙提示 | 01 O-6；字段未在当前 frps 表单暴露 |
| Windows 防火墙命令（netsh） | 01 O-7；frps 固定部署 Ubuntu 22+ |
| wizard 多语言 | 01 O-8；仅中文 |
| wizard "返回上一步" | 01 O-10；"两者都配置"仅向前 |
| 下载进度的 SSE/WebSocket 推送 | T-001 Out-of-scope 继续延续（使用 polling，NF-U4 ≤1s 间隔） |
| downloader 的版本检查（已存在二进制是否需要更新） | T-001 R-5 / 01 O-1；升级逻辑单独任务 |

---

## 11. Partition assignment（分区分配）

### 11.1 文件级分配

| 文件 | Partition | New / Edit | 依赖 |
|---|---|---|---|
| `internal/downloader/downloader.go` | dev-backend | new | `internal/binloc`（Root()）、标准库 archive/* |
| `internal/downloader/downloader_test.go` | dev-backend | new | downloader.go |
| `internal/httpapi/handlers_system.go` | dev-backend | edit（追加 3 handler + ipCache 类型 + handlers 字段） | `downloader.Manager` |
| `internal/httpapi/handlers_wizard.go` | dev-backend | new | `internal/storage`（KVGet/KVSet） |
| `internal/httpapi/router.go` | dev-backend | edit（5 路由 + Dependencies.Downloader 字段） | handlers_system + handlers_wizard |
| `cmd/frp-easy/main.go` | dev-backend | edit（创建 downloader.Manager，注入 Dependencies） | downloader |
| `web/src/types.ts` | dev-frontend | edit（追加 4 接口） | 后端契约 §5 |
| `web/src/api/system.ts` | dev-frontend | edit（追加 apiGetPublicIP） | types.ts |
| `web/src/api/downloader.ts` | dev-frontend | new | types.ts |
| `web/src/api/wizard.ts` | dev-frontend | new | types.ts |
| `web/src/stores/downloader.ts` | dev-frontend | new | api/downloader.ts |
| `web/src/stores/wizard.ts` | dev-frontend | new | api/wizard.ts |
| `web/src/pages/Wizard.vue` | dev-frontend | new | stores/wizard、api/frpclient、api/server、api/mode |
| `web/src/components/FirewallHint.vue` | dev-frontend | new | — |
| `web/src/components/PublicIpDetector.vue` | dev-frontend | new | api/system.ts |
| `web/src/router.ts` | dev-frontend | edit（/wizard 路由 + guard 逻辑） | stores/wizard |
| `web/src/pages/Server.vue` | dev-frontend | edit（嵌入 PublicIpDetector + FirewallHint） | components/* |
| `web/src/components/AppLayout.vue` | dev-frontend | edit（下载按钮 + 进度条） | stores/downloader、stores/app |
| `web/src/pages/Proxies.vue` | dev-frontend | edit（FirewallHint 展示逻辑） | components/FirewallHint |

### 11.2 Dispatch order

1. **dev-backend**：`internal/downloader` + 扩展 `internal/httpapi` + 修改 `main.go`
2. **dev-frontend**：全部前端文件（可与 dev-backend 并行；前端按本设计契约 §5 开发，后端 API Mock 不需要）

### 11.3 Parallelism

dev-backend 与 dev-frontend **可并行**。
- 契约（§5 响应体形状、端点路径、状态码）已在本文件冻结。
- 前端在 dev 模式下可 mock 5 个新端点（Vite proxy + MSW 或直接 hardcode 假数据）。
- 集成点：dev-backend 跑通 `go test ./...` 后，前端切换到真实后端做 E2E 验证。

**dev-db**：本任务无新迁移文件，不派发 dev-db。

---

## 12. Verdict（裁定）

**READY**

依据：

1. 上游 `01_REQUIREMENT_ANALYSIS.md` verdict=READY，0 条 BLOCKED ON USER，20 条 In-scope 行为已逐一被本设计覆盖。
2. 架构总览（§1）、受影响模块（§2）、新模块分解（§3）、数据模型（§4）、API 契约（§5）、流程图（§6）均已给出，开发者无需做追加设计决策。
3. Reuse audit（§7）非空：7 处现有代码明确复用，0 处新库引入（所有新能力用 Go 标准库实现）。
4. 风险 8 条（超过 ≥3 要求），每条有具体缓解。
5. 分区表 + dispatch order + parallelism 完整；dev-backend 与 dev-frontend 可并行，无 dev-db 任务。
6. 不破坏 T-001：无 schema 变更、无现有端点修改、无 handler 行为改变；T-001 测试基线 ≥146 继续通过。

下一阶段：**Gate Reviewer**（`03_GATE_REVIEW.md`）。重点评审：① downloader 并发安全模型；② wizard shouldShow 条件边界；③ 前端路由守卫调用时机。
