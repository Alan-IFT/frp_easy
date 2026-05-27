# 02 — Solution Design · T-039 frpsadmin-server-runtime-api

> Stage 2 / 7。基于 `01_REQUIREMENT_ANALYSIS.md` 的"做什么"，给出"怎么做"的详细方案。

## 1. 总览（一图流）

```
┌──────────────┐ HTTP basic auth ┌─────────────────────────────┐
│ chi handler  │ ──────────────▶ │ frps:7500 dashboard         │
│ (httpapi)    │ ◀──── JSON ──── │  /api/serverinfo            │
└──────┬───────┘                 │  /api/proxy/{type}          │
       │                         │  /api/proxy/{type}/{name}   │
       │ uses                    │  /api/traffic/{name}        │
       ▼                         └─────────────────────────────┘
┌──────────────┐                          ▲
│ frpsadmin    │                          │
│  .Client     │──────────────────────────┘
└──────┬───────┘
       │ creds from
       ▼
┌──────────────────────────────────────────┐
│ KV store                                 │
│  - frps.config       (user-edited)       │
│  - frps.dashboard.autogen  (fallback)    │
└──────────────────────────────────────────┘
       ▲
       │ ensure on startup
┌──────┴───────┐
│ main.go      │
│ ensureFrpsDashboardCreds()  ← 与 ensureFrpcAdminCreds 对称
└──────────────┘
```

## 2. 包结构

```
internal/frpsadmin/
├── client.go              ← Client + 4 个方法 + sentinel error + 类型定义
└── client_test.go         ← httptest 模拟 frps admin server，4 方法 × 4 状态分支
```

## 3. 详细设计

### 3.1 `internal/frpsadmin/client.go`

```go
// Package frpsadmin 封装 frps 自带 admin API 的 HTTP basic auth 客户端。
//
// 上游 endpoint（参考 frp v0.58+ webServer routes）：
//   - GET /api/serverinfo       → 全局服务状态 + 配置摘要 + 总流量
//   - GET /api/proxy/{type}     → 按类型列出 proxy（tcp/udp/http/https/stcp/sudp/xtcp）
//   - GET /api/proxy/{type}/{name} → 单条 proxy 详情
//   - GET /api/traffic/{name}   → 单条 proxy 日级流量序列
//
// 5s 超时由 Client 持有的 http.Client 控制（比 frpcadmin 的 3s 略宽，
// 因 frps 端可能聚合多 client 数据）。
//
// 错误模型（sentinel，可类型断言）：
//   - ErrUnauthorized：上游 401（凭据失效）
//   - ErrNotFound：上游 404（proxy 不存在）
//   - ErrUnavailable：连接拒绝 / DNS 失败 / 超时 / 5xx（frps 进程未跑或 dashboard 未启用）
package frpsadmin

import (
    "context"
    "encoding/json"
    "errors"
    "fmt"
    "io"
    "net/http"
    "strings"
    "time"
)

// 错误模型
var (
    ErrUnauthorized = errors.New("frpsadmin: unauthorized")
    ErrNotFound     = errors.New("frpsadmin: not found")
    ErrUnavailable  = errors.New("frpsadmin: upstream unavailable")
)

type Client struct {
    baseURL string
    user    string
    pass    string
    http    *http.Client
}

const defaultTimeout = 5 * time.Second

func New(addr string, port int, user, pass string) *Client { ... }
func NewWithTimeout(addr string, port int, user, pass string, timeout time.Duration) *Client { ... }
func NewWithBaseURL(baseURL, user, pass string, timeout time.Duration) *Client { ... }

// ServerInfo 是 GET /api/serverinfo 的响应（仅取常用字段；其它字段忽略）。
type ServerInfo struct {
    Version         string `json:"version"`
    BindPort        int    `json:"bindPort"`
    KCPBindPort     int    `json:"kcpBindPort,omitempty"`
    QUICBindPort    int    `json:"quicBindPort,omitempty"`
    VhostHTTPPort   int    `json:"vhostHTTPPort,omitempty"`
    VhostHTTPSPort  int    `json:"vhostHTTPSPort,omitempty"`
    TCPMuxHTTPCONNECTPort int    `json:"tcpmuxHTTPConnectPort,omitempty"`
    SubdomainHost   string `json:"subdomainHost,omitempty"`
    MaxPoolCount    int64  `json:"maxPoolCount,omitempty"`
    MaxPortsPerClient int64 `json:"maxPortsPerClient,omitempty"`
    HeartbeatTimeout int64 `json:"heartbeatTimeout,omitempty"`
    ClientCounts    int    `json:"clientCounts"`
    CurConns        int    `json:"curConns"`
    ProxyTypeCount  map[string]int `json:"proxyTypeCount,omitempty"`
    TotalTrafficIn  int64  `json:"totalTrafficIn,omitempty"`
    TotalTrafficOut int64  `json:"totalTrafficOut,omitempty"`
}

// ProxyStatus 是 GET /api/proxy/{type} 数组项。
type ProxyStatus struct {
    Name            string         `json:"name"`
    Type            string         `json:"type"`
    Status          string         `json:"status"`           // "online" | "offline"
    LastStartTime   string         `json:"lastStartTime,omitempty"`
    LastCloseTime   string         `json:"lastCloseTime,omitempty"`
    TodayTrafficIn  int64          `json:"todayTrafficIn,omitempty"`
    TodayTrafficOut int64          `json:"todayTrafficOut,omitempty"`
    CurConns        int            `json:"curConns,omitempty"`
    ClientVersion   string         `json:"clientVersion,omitempty"`
    Conf            map[string]any `json:"conf,omitempty"`   // 透传 frps 上游字段
}

// proxiesEnvelope 是 frps /api/proxy/{type} 实际返回的顶层包装。
// frps 端返回 {"proxies":[...]}，本包解包后只返回数组给调用方。
type proxiesEnvelope struct {
    Proxies []ProxyStatus `json:"proxies"`
}

// ProxyDetail 是 GET /api/proxy/{type}/{name} 响应（与 ProxyStatus 同结构，但单独类型让未来扩展更安全）。
type ProxyDetail = ProxyStatus

// Traffic 是 GET /api/traffic/{name} 响应。
type Traffic struct {
    Name       string  `json:"name"`
    TrafficIn  []int64 `json:"trafficIn"`
    TrafficOut []int64 `json:"trafficOut"`
}

func (c *Client) ServerInfo(ctx context.Context) (ServerInfo, error) { ... }
func (c *Client) Proxies(ctx context.Context, proxyType string) ([]ProxyStatus, error) { ... }
func (c *Client) ProxyDetail(ctx context.Context, proxyType, name string) (ProxyDetail, error) { ... }
func (c *Client) Traffic(ctx context.Context, name string) (Traffic, error) { ... }

// 内部：构造请求 + basic auth + 执行 + 分类错误 + 反序列化
func (c *Client) doGet(ctx context.Context, path string, out any) error {
    u := c.baseURL + path
    req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
    if err != nil {
        return fmt.Errorf("frpsadmin: build request: %w", err)
    }
    if c.user != "" || c.pass != "" {
        req.SetBasicAuth(c.user, c.pass)
    }
    resp, err := c.http.Do(req)
    if err != nil {
        // 连接拒绝 / DNS / 超时 → ErrUnavailable
        return fmt.Errorf("%w: %v", ErrUnavailable, err)
    }
    defer resp.Body.Close()
    switch {
    case resp.StatusCode == http.StatusOK:
        // happy path
    case resp.StatusCode == http.StatusUnauthorized:
        return ErrUnauthorized
    case resp.StatusCode == http.StatusNotFound:
        return ErrNotFound
    case resp.StatusCode >= 500:
        return fmt.Errorf("%w: HTTP %d", ErrUnavailable, resp.StatusCode)
    default:
        body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
        return fmt.Errorf("frpsadmin: unexpected status %d: %s",
            resp.StatusCode, strings.TrimSpace(string(body)))
    }
    if out == nil {
        return nil
    }
    data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
    if err != nil {
        return fmt.Errorf("frpsadmin: read body: %w", err)
    }
    if err := json.Unmarshal(data, out); err != nil {
        return fmt.Errorf("frpsadmin: decode: %w", err)
    }
    return nil
}
```

**关键决策**：
- D-3.1（错误模型）：错误用 `errors.Is(err, ErrUnauthorized)` 可断言。`net/http` 的 dial 错误（`*url.Error` wrapping `*net.OpError`）用 `fmt.Errorf("%w: ...", ErrUnavailable, err)` 双层包装让上层可同时拿到 sentinel + 详情。
- D-3.2（容错）：5xx 也视为 `ErrUnavailable`（与连接失败同语义"上游不可用"）。frps panic / OOM 这类场景前端友好"等会再试"。
- D-3.3（body 上限）：1 MiB 读上限（与 frpcadmin 1<<20 同款）；frps `/api/proxy/{type}` 极端大集群可能 > 1 MiB，但 MVP 单 frps server 场景充裕。
- D-3.4（json 反序列化）：所有 struct 用 `json:"..."` 标签匹配 frps 上游 camelCase；`omitempty` 让旧版 frps 缺字段时不报错。
- D-3.5（envelope）：`/api/proxy/{type}` 返回 `{"proxies":[...]}`，需双层结构 `proxiesEnvelope` 解包，`Proxies()` 方法对外返回扁平数组——隐藏上游包装让调用方代码简洁。

### 3.2 `internal/httpapi/handlers_server_runtime.go`（新文件）

```go
package httpapi

import (
    "context"
    "encoding/json"
    "errors"
    "fmt"
    "net/http"

    "github.com/frp-easy/frp-easy/internal/frpsadmin"
    "github.com/go-chi/chi/v5"
)

const (
    kvFrpsDashboardAutogen = "frps.dashboard.autogen"
)

// FrpsDashboardCreds 是 KV 持久化的 dashboard 自动生成凭据。
type FrpsDashboardCreds struct {
    User string `json:"user"`
    Pass string `json:"pass"`
}

// resolveFrpsDashboard 从 KV 读 frps.config + frps.dashboard.autogen，
// 合并出"实际生效的 dashboard 凭据 + 端口"。
//
// 优先级：
//   1) frps.config.DashboardUser/Pass 非空 → 用用户填写值
//   2) 否则从 frps.dashboard.autogen 读 autogen 值
//   3) 都没有 → 返回 (zero, false, nil) 让 handler 走 503 分支
//
// DashboardEnabled 视为"用户意图"——false 时直接 (zero, false, nil) 让 503。
// 这也是 renderAndApplyFrps 的同款决策（D-1）。
func (h *handlers) resolveFrpsDashboard(ctx context.Context) (addr string, port int, user, pass string, enabled bool, err error) {
    var cfg FrpsConfig
    if v, ok, _ := h.deps.Store.KVGet(ctx, kvFrpsConfig); ok {
        _ = json.Unmarshal([]byte(v), &cfg)
    }
    if !cfg.DashboardEnabled {
        // FR-3.3：dashboard 未启用 → handler 503
        return "", 0, "", "", false, nil
    }
    addr = cfg.DashboardAddr
    if addr == "" {
        addr = "127.0.0.1"
    }
    port = cfg.DashboardPort
    if port == 0 {
        port = 7500
    }
    user = cfg.DashboardUser
    pass = cfg.DashboardPass
    if user == "" || pass == "" {
        // FR-3.5 fallback：从 autogen 读
        var auto FrpsDashboardCreds
        if v, ok, _ := h.deps.Store.KVGet(ctx, kvFrpsDashboardAutogen); ok {
            _ = json.Unmarshal([]byte(v), &auto)
        }
        if user == "" {
            user = auto.User
        }
        if pass == "" {
            pass = auto.Pass
        }
    }
    if user == "" || pass == "" {
        // KV 也没有 autogen（应该不会发生，但兜底）
        return addr, port, "", "", true, fmt.Errorf("dashboard 凭据缺失")
    }
    return addr, port, user, pass, true, nil
}

// buildFrpsAdminClient 是 handler 通用的 client 构造路径。
// 失败时 w 已写出响应；返回 nil 让 handler 直接 return。
func (h *handlers) buildFrpsAdminClient(w http.ResponseWriter, r *http.Request) *frpsadmin.Client {
    addr, port, user, pass, enabled, err := h.resolveFrpsDashboard(r.Context())
    if !enabled {
        writeError(w, http.StatusServiceUnavailable, CodeInternal,
            "frps dashboard 未启用。请到 Server 设置页打开 'Dashboard' 开关并保存（frp_easy 会自动生成凭据并应用配置）。", "")
        return nil
    }
    if err != nil {
        writeError(w, http.StatusServiceUnavailable, CodeInternal,
            "frps dashboard 凭据不可用："+err.Error(), "")
        return nil
    }
    return frpsadmin.New(addr, port, user, pass)
}

// 4 个 handler 共用错误映射
func (h *handlers) writeFrpsadminError(w http.ResponseWriter, err error) {
    switch {
    case errors.Is(err, frpsadmin.ErrUnauthorized):
        writeError(w, http.StatusBadGateway, CodeInternal,
            "frps dashboard 凭据验证失败（401）。请到 Server 设置页清空 user/pass 让 frp_easy 重新生成，或检查 frps.toml [webServer] 段。", "")
    case errors.Is(err, frpsadmin.ErrNotFound):
        writeError(w, http.StatusNotFound, CodeNotFound, "未找到对应的 proxy", "")
    case errors.Is(err, frpsadmin.ErrUnavailable):
        writeError(w, http.StatusServiceUnavailable, CodeInternal,
            "frps 进程不可达。请确认 frps 已启动，且 dashboard 监听端口与配置一致。", "")
    default:
        writeError(w, http.StatusBadGateway, CodeInternal,
            "调用 frps dashboard 失败："+err.Error(), "")
    }
}

func (h *handlers) serverRuntimeInfo(w http.ResponseWriter, r *http.Request) {
    c := h.buildFrpsAdminClient(w, r)
    if c == nil {
        return
    }
    info, err := c.ServerInfo(r.Context())
    if err != nil {
        h.writeFrpsadminError(w, err)
        return
    }
    writeJSON(w, http.StatusOK, info)
}

// frpsProxyTypes 是 GET /api/v1/server/runtime/proxies 聚合用的 type 列表。
var frpsProxyTypes = []string{"tcp", "udp", "http", "https", "stcp", "sudp", "xtcp"}

// ServerRuntimeProxiesResponse 是聚合 N 个 type 的响应。
// 部分 type 失败 → errors[type] = 错误文案；整体仍 200（Q-1 PM-DECIDED）。
type ServerRuntimeProxiesResponse struct {
    Proxies map[string][]frpsadmin.ProxyStatus `json:"proxies"`
    Errors  map[string]string                  `json:"errors,omitempty"`
}

func (h *handlers) serverRuntimeProxies(w http.ResponseWriter, r *http.Request) {
    c := h.buildFrpsAdminClient(w, r)
    if c == nil {
        return
    }
    resp := ServerRuntimeProxiesResponse{
        Proxies: map[string][]frpsadmin.ProxyStatus{},
        Errors:  map[string]string{},
    }
    fatalCount := 0
    for _, t := range frpsProxyTypes {
        list, err := c.Proxies(r.Context(), t)
        if err != nil {
            // 上游"unauthorized" / "unavailable" 是 fatal（凭据 / 进程问题影响所有 type）
            // 其它视为 per-type 错误（如未启用 xtcp 上游可能返回怪异）。
            if errors.Is(err, frpsadmin.ErrUnauthorized) || errors.Is(err, frpsadmin.ErrUnavailable) {
                fatalCount++
            }
            resp.Errors[t] = err.Error()
            continue
        }
        resp.Proxies[t] = list
    }
    // 全部 type 都 fatal → 视为整体失败，返回 503/502（与 single endpoint 同款）
    if fatalCount == len(frpsProxyTypes) {
        // 用第一个 type 的错误判 fatal 类型（同源所以等价）
        for _, t := range frpsProxyTypes {
            // 任取一个错误的 type 重新构造 client 调用以拿到 sentinel
            list, err := c.Proxies(r.Context(), t)
            _ = list
            if err != nil {
                h.writeFrpsadminError(w, err)
                return
            }
        }
    }
    writeJSON(w, http.StatusOK, resp)
}

func (h *handlers) serverRuntimeProxyDetail(w http.ResponseWriter, r *http.Request) {
    c := h.buildFrpsAdminClient(w, r)
    if c == nil {
        return
    }
    pt := chi.URLParam(r, "type")
    name := chi.URLParam(r, "name")
    detail, err := c.ProxyDetail(r.Context(), pt, name)
    if err != nil {
        h.writeFrpsadminError(w, err)
        return
    }
    writeJSON(w, http.StatusOK, detail)
}

func (h *handlers) serverRuntimeTraffic(w http.ResponseWriter, r *http.Request) {
    c := h.buildFrpsAdminClient(w, r)
    if c == nil {
        return
    }
    name := chi.URLParam(r, "name")
    traffic, err := c.Traffic(r.Context(), name)
    if err != nil {
        h.writeFrpsadminError(w, err)
        return
    }
    writeJSON(w, http.StatusOK, traffic)
}
```

**关键决策**：
- D-3.6（路径合并）：4 个 handler 写在同一文件 `handlers_server_runtime.go`（仿照 `handlers_system.go` 的合并模式）。
- D-3.7（聚合错误）：`Q-1` PM-DECIDED 部分成功 + per-type error map；全 fatal 才整体 5xx。
- D-3.8（错误码透传）：`writeFrpsadminError` 把上游 401 映射到 frp_easy HTTP **502 Bad Gateway**（区别于本应用 401 SessionAuth 失败）。让前端 UX 能区分"我没登录"vs"我登录了但 frps dashboard 凭据坏了"。

### 3.3 `internal/httpapi/router.go` 改动

在 `r.Get("/system/service-status", h.systemServiceStatus)` 后追加：

```go
// T-039: server runtime monitoring — frps admin API proxy.
r.Get("/server/runtime/info", h.serverRuntimeInfo)
r.Get("/server/runtime/proxies", h.serverRuntimeProxies)
r.Get("/server/runtime/proxy/{type}/{name}", h.serverRuntimeProxyDetail)
r.Get("/server/runtime/traffic/{name}", h.serverRuntimeTraffic)
```

中间件链不动；归属现有 SessionAuth + CSRF 分组（CSRF 只作用于写方法，对 GET 透明）。

### 3.4 `internal/httpapi/config_helper.go` 改动（FR-3.3 / FR-3.4）

在 `renderAndApplyFrps` 函数内，第 2 步（TOML 生成）之前插入凭据 fallback 逻辑：

```go
// T-039 (FR-3.3 / FR-3.4)：dashboard 凭据自动生成补齐。
// - DashboardEnabled=false：用户明示禁用 → 不动（保留原行为）。
// - DashboardEnabled=true 但 user/pass 空 → 从 KV frps.dashboard.autogen 补齐。
// 这让用户在 Server 页只勾选 "启用 dashboard" + 保存 即可（零配置）。
if cfg.DashboardEnabled && (cfg.DashboardUser == "" || cfg.DashboardPass == "") {
    var auto FrpsDashboardCreds
    if v, ok, _ := h.deps.Store.KVGet(ctx, kvFrpsDashboardAutogen); ok {
        _ = json.Unmarshal([]byte(v), &auto)
    }
    if cfg.DashboardUser == "" {
        cfg.DashboardUser = auto.User
    }
    if cfg.DashboardPass == "" {
        cfg.DashboardPass = auto.Pass
    }
}
```

> **注意**：本任务 PM-DECIDED **不**改 FR-3.3 中"DashboardEnabled=false → 自动翻 true"的描述。RA 原文中的"DashboardEnabled=false → 自动翻 true"会**反客为主**违反 D-2（用户体验好）+ 用户对配置的控制权。SA 调整为：用户没明确点 Enable 时 frps.toml 不写 webServer 段（即 frps 端 dashboard 自然不启用），handler 走 503 + 友好引导。"用户点了 Enable 但忘填凭据"才是真正的零配置场景，被本段 fallback 完美覆盖。
>
> **Design drift 记号**：本节相对 01 §FR-3.3 的"DashboardEnabled=false → 自动翻 true"做了**收敛性调整**。RA 同意（01 §10 D-1 原文"用户体验好 + 不能覆盖用户填的值"暗示——禁用即用户意图也应尊重）。GR 将在 stage 3 复核此 drift 是否合理。

### 3.5 `cmd/frp-easy/main.go` 改动（FR-3.2）

在 `ensureFrpcAdminCreds` 调用之后、`procmgr.New` 之前插入：

```go
// T-039: 自动生成 frps dashboard 凭据（与 ensureFrpcAdminCreds 对称镜像）。
// 仅在 KV 无值时生成；用户在 Server 设置页填的值优先（resolveFrpsDashboard 解析时按优先级合并）。
ensureFrpsDashboardCreds(store, logger)
```

新增函数：

```go
const kvFrpsDashboardAutogen = "frps.dashboard.autogen"

// ensureFrpsDashboardCreds 读 kv.frps.dashboard.autogen；不存在则生成 + 持久化。
// 与 ensureFrpcAdminCreds 对称镜像（同款 3s 超时、auth.GenerateCSRFToken、
// fail-soft logger.Warn 不阻塞启动）。
func ensureFrpsDashboardCreds(store *storage.Store, logger *slog.Logger) {
    ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
    defer cancel()
    if v, ok, err := store.KVGet(ctx, kvFrpsDashboardAutogen); err == nil && ok && v != "" {
        return // 已有，不动
    }
    user, _ := auth.GenerateCSRFToken()
    pass, _ := auth.GenerateCSRFToken()
    payload := map[string]string{
        "user": "frp_easy_" + user[:8],
        "pass": pass,
    }
    b, _ := json.Marshal(payload)
    if err := store.KVSet(ctx, kvFrpsDashboardAutogen, string(b)); err != nil {
        logger.Warn("persist frps dashboard creds failed", "err", err)
    }
}
```

**注**：函数签名故意**不返回值**——handler 自己从 KV 读，避免 `Dependencies` 结构扩展。这与 `ensureFrpcAdminCreds` 区别：后者凭据在 `Dependencies.FrpcAdmin` 持有（frpc reload 需要恒定 client），本任务 handler 是 lazy 构造（每次请求新建 5s timeout 小对象，对象池化无收益）。

### 3.6 openapi.yaml 改动

在末尾追加 4 条路径 + components 段追加 4 schema。详见 stage 4 实现。

### 3.7 dev-map.md 改动

- "目录布局" `internal/` 子树追加：
  ```
  │   ├── frpsadmin/ ← frps admin HTTP 客户端（T-039：/api/serverinfo / proxy / traffic）
  ```
- "功能在哪里" 表追加一行 `internal/frpsadmin/client.go`。
- "HTTP 路由层" 行追加：`T-039: +4 条（server/runtime/info|proxies|proxy/{type}/{name}|traffic/{name}）。`

## 4. 测试计划

### 4.1 `internal/frpsadmin/client_test.go`

| 测试 | 覆盖 |
|---|---|
| `TestServerInfo_Success` | 200 happy path + JSON 反序列化字段 |
| `TestServerInfo_Unauthorized` | 401 → `errors.Is(err, ErrUnauthorized)` |
| `TestServerInfo_Unavailable_5xx` | 502 → `errors.Is(err, ErrUnavailable)` |
| `TestServerInfo_Unavailable_ConnRefused` | 关掉 server 后调用 → `errors.Is(err, ErrUnavailable)` |
| `TestProxies_Envelope_Unwrap` | 上游返回 `{"proxies":[...]}` → 客户端返回扁平数组 |
| `TestProxies_BadType` | 上游 404 → `ErrNotFound`（虽然语义稍怪，但 frps 实测 type 不存在确实可能 404） |
| `TestProxyDetail_NotFound` | 404 → `ErrNotFound` |
| `TestProxyDetail_Success` | 200 + 字段 |
| `TestTraffic_Success` | 200 + TrafficIn/TrafficOut 数组 |
| `TestTraffic_NotFound` | 404 → `ErrNotFound` |
| `TestBasicAuth_Applied` | httptest 端断言 r.BasicAuth() 拿到正确 user/pass |
| `TestNew_BuildsBaseURL` | New("127.0.0.1", 7500, "u", "p").baseURL = "http://127.0.0.1:7500" |

### 4.2 `internal/httpapi/handlers_server_runtime_test.go`

| 测试 | 覆盖 |
|---|---|
| `TestServerRuntimeInfo_DashboardDisabled_503` | KV `frps.config` DashboardEnabled=false → 503 |
| `TestServerRuntimeInfo_DashboardEnabled_NoCreds_Fallback` | DashboardEnabled=true，user/pass 空，autogen KV 有值 → handler 用 autogen 调上游，httptest mock 返 200 → handler 200 |
| `TestServerRuntimeInfo_UpstreamUnauthorized_502` | mock 上游 401 → handler 502 + 友好错误体 |
| `TestServerRuntimeInfo_UpstreamUnavailable_503` | KV 指向不存在的端口 → handler 503 |
| `TestServerRuntimeProxies_Aggregation_PartialSuccess` | mock 上游对 tcp 返 200 对 xtcp 返 5xx → handler 200，response 含 `proxies.tcp` + `errors.xtcp` |
| `TestServerRuntimeProxies_AllFatal_503` | mock 上游所有 type 都 5xx → handler 503 |
| `TestServerRuntimeProxyDetail_404` | mock 上游 404 → handler 404 |
| `TestServerRuntimeTraffic_200` | mock 上游 200 + 数组 → handler 透传 |
| `TestRenderAndApplyFrps_AutogenFallback` | DashboardEnabled=true + user/pass 空 + KV autogen 有值 → 渲染出的 frps.toml 字面含 autogen user/pass |
| `TestRenderAndApplyFrps_UserCredsTakePrecedence` | DashboardEnabled=true + user/pass 填了 → autogen 不被用，渲染出的 frps.toml 含用户填值 |

**测试技术**：用 `httptest.NewServer` 启动 mock frps；handler 测试通过 `Dependencies` 注入 mock URL（通过预先在 KV 写入"指向 mock URL host:port"的 frps.config）。

### 4.3 Adversarial tests（QA stage 6）

| ID | 反向证伪 |
|---|---|
| ADV-1 | 删 `ErrUnauthorized` 的 401 分支 → `TestServerInfo_Unauthorized` FAIL → 恢复 → PASS |
| ADV-2 | 删 `Proxies()` 的 envelope unwrap → `TestProxies_Envelope_Unwrap` FAIL → 恢复 → PASS |
| ADV-3 | 删 `resolveFrpsDashboard` 的 autogen fallback → `TestServerRuntimeInfo_DashboardEnabled_NoCreds_Fallback` FAIL → 恢复 → PASS |
| ADV-4 | 把 `kvFrpsDashboardAutogen` 字面改成不同字符串 → `TestRenderAndApplyFrps_AutogenFallback` FAIL → 恢复 → PASS |

## 5. 风险

| 风险 | 影响 | 缓解 |
|---|---|---|
| R-1：frps 上游 API 在 v0.59+ 字段重命名 | 字段 missing → JSON 解出零值（不报错），UI 显示空 | 用 `omitempty` 容忍；T-040 / T-041 前端做空值兜底 |
| R-2：聚合 `Proxies()` 7 个 type 串行调用 → 单次 handler 耗时 7×单调用时间 | 慢，长尾 | 串行实现简洁。如 T-041 UX 实测瓶颈再改 goroutine 并发（与本任务 KISS 原则一致） |
| R-3：autogen 凭据生成后用户重启 frp_easy 但**未**点 Server 页保存 → 凭据写 KV 但 frps.toml 还没被重新渲染 → handler 调 frps 仍 401 | 用户困惑 | 渲染只在 PUT /server 时触发（既有逻辑）。UX 引导：handler 503 文案明示"请到 Server 页保存后重启 frps"。**这是约定不是 bug**——本任务范围内可接受 |
| R-4：handler 503 错误文案太长 → 前端 UI 排版坏 | UX | 文案控制在 ≤ 100 字符 + 一行 |
| R-5：mock httptest server 在 Windows 下 port reuse 偶发冲突 | 测试 flake | 用 `httptest.NewServer` 自动选可用端口；不写死 |

## 6. 回滚计划

如果实现后 verify_all FAIL > 1：
- 单文件回滚：`git checkout HEAD -- <file>` 按 stage 4 文档列出的文件清单。
- 影响范围：本任务**全新增量**（除 router.go / config_helper.go / main.go / dev-map.md / openapi.yaml 5 处微调），回滚相对安全。
- 数据：KV `frps.dashboard.autogen` 在回滚后 orphan（无 reader），不污染。

## 7. 实现顺序（dev stage 推荐）

1. 新建 `internal/frpsadmin/client.go` + `client_test.go`。先跑 `go test ./internal/frpsadmin/` PASS。
2. 新建 `internal/httpapi/handlers_server_runtime.go`。
3. 新建 `internal/httpapi/handlers_server_runtime_test.go`。先 `go test ./internal/httpapi/ -run ServerRuntime` PASS。
4. 改 `internal/httpapi/router.go` 注册 4 条路由。
5. 改 `internal/httpapi/config_helper.go` 凭据 fallback。
6. 改 `cmd/frp-easy/main.go` `ensureFrpsDashboardCreds` + 调用点。
7. 改 `openapi.yaml` 新 4 条路径 + 4 schema。
8. 改 `docs/dev-map.md`。
9. 跑 `pwsh scripts/verify_all.ps1`，目标 PASS ≥ 32, FAIL = 1。

## 8. 分区分配

本项目本任务**单 developer**模式（无 `dev-*` 分区文件）。所有改动归"backend / Go" 范畴。

## 9. SA self-check

| 项 | 状态 |
|---|---|
| 与 01 验收标准每条都有实现路径 | ✅ |
| 与既有架构（frpcadmin / KV / chi router / SessionAuth）对齐 | ✅ |
| 错误模型可类型断言 | ✅ |
| 测试覆盖每个错误分支 | ✅ |
| 不引入新依赖 | ✅（仅 net/http + encoding/json） |
| Design drift 已标记（§3.4 关于 FR-3.3） | ✅ |
| insight L38 适用性已评估（autogen 不需要 retry，是一次性配置生成而非运行时恢复） | ✅ |

---

**Verdict**：READY FOR GATE REVIEW.
