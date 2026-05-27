# 01 — Requirement Analysis · T-039 frpsadmin-server-runtime-api

> Stage 1 / 7。本文档定义"做什么 / 怎么验"，不涉及实现细节。

## 1. 任务一句话

新增 `internal/frpsadmin/` 包封装 frps 自带 admin HTTP API，并在 `internal/httpapi` 暴露 4 条 `/api/v1/server/runtime/*` REST 路由让后续 T-040 / T-041 / T-042 的前端 UI 能查询 frpc 在线状态、连接状态与流量。同时让 `frps.toml` 的 dashboard 凭据在用户零配置时自动生成稳定值，做到"装完即可被监管"。

## 2. 背景与上下文

- 批次：`docs/batches/frps-monitor-and-mgmt-suite/` 第 1 个任务（共 4 个；T-040 / T-041 / T-042 均依赖本任务的 API 输出）。
- 现有 `internal/frpcadmin/` 是 frpc 端 admin API 客户端（`Reload` / `Status`），本任务给 frps 端做**对称镜像**。
- 现有 `internal/httpapi/handlers_server.go` 持有 `FrpsConfig`（含 `DashboardEnabled` / `DashboardAddr` / `DashboardPort` / `DashboardUser` / `DashboardPass` 5 字段，T-002 引入），KV `frps.config`；本任务 handler 直接复用。
- 现有 `cmd/frp-easy/main.go::ensureFrpcAdminCreds` 是凭据自生成 + 持久化的范式（KV `frpc.admin`，启动期一次性），本任务对 frps dashboard 做同款（KV `frps.dashboard.autogen`）。

## 3. 用户故事

- US-1：**作为 frp_easy 运维**，我希望在后端 API 已就绪后让前端能查询"当前有哪些 frpc 连上来、每个 proxy 跑在什么状态、流量多少"，而无需自己用 curl 调 frps admin。
- US-2：**作为初装用户**，我希望"装好 frp_easy + 启用 frps 即可被监控"——不需要自己去 `Server` 设置页填 dashboard user / pwd，但如果填了我自己的值，系统不能覆盖。
- US-3：**作为接下来要写前端的开发者（T-041 / T-042）**，我希望 API 形状稳定、错误码可区分（503 = frps 未跑 / 401 = 凭据失效 / 200 = 数据），handler 内部不暴露 frps 上游 URL 细节。
- US-4：**作为长期维护者**，我希望 frpsadmin 包结构与 frpcadmin 包对称，错误模型一致（sentinel error 可类型断言），未来加上游新 endpoint 时按既有模式扩展即可。

## 4. 功能需求 FR

### FR-1 frpsadmin 包

| ID | 描述 | 上游 endpoint |
|---|---|---|
| FR-1.1 | `New(addr string, port int, user, pass string) *Client` 构造（与 frpcadmin 同款签名） | — |
| FR-1.2 | `NewWithTimeout(...)` / `NewWithBaseURL(...)` 测试用 seam | — |
| FR-1.3 | `ServerInfo(ctx) (ServerInfo, error)` | GET `/api/serverinfo` |
| FR-1.4 | `Proxies(ctx, proxyType string) ([]ProxyStatus, error)` | GET `/api/proxy/{type}`，type ∈ {tcp,udp,http,https,stcp,sudp,xtcp} |
| FR-1.5 | `ProxyDetail(ctx, proxyType, name string) (ProxyDetail, error)` | GET `/api/proxy/{type}/{name}` |
| FR-1.6 | `Traffic(ctx, name string) (Traffic, error)` | GET `/api/traffic/{name}` |
| FR-1.7 | 5s 默认超时（基于 02 §3.6 frpcadmin 用 3s，本包给 5s 因 frps server 端可能聚合多 client 数据） | — |
| FR-1.8 | basic auth（user / pass 通过 `req.SetBasicAuth`） | — |
| FR-1.9 | 错误模型分类返回 sentinel：`ErrUnauthorized`（401）/ `ErrNotFound`（404）/ `ErrUnavailable`（连接拒绝 / DNS / EOF / 超时 / 5xx）。其余错误用 `fmt.Errorf` 包装。 | — |

### FR-2 REST 路由

| ID | 路由 | 内部调用 | 期望 HTTP |
|---|---|---|---|
| FR-2.1 | `GET /api/v1/server/runtime/info` | `frpsadmin.ServerInfo()` | 200 / 503 / 401 |
| FR-2.2 | `GET /api/v1/server/runtime/proxies` | `Proxies()` × N 个 type 聚合 | 200 / 503 / 401 |
| FR-2.3 | `GET /api/v1/server/runtime/proxy/{type}/{name}` | `ProxyDetail()` | 200 / 404 / 503 / 401 |
| FR-2.4 | `GET /api/v1/server/runtime/traffic/{name}` | `Traffic()` | 200 / 404 / 503 / 401 |
| FR-2.5 | 所有路由走既有 SessionAuth 中间件（与 `/api/v1/proxies` 同分组） | — | 401 if no session |
| FR-2.6 | handler 内根据 KV `frps.config` 读 `DashboardEnabled` / `DashboardAddr` / `DashboardPort` / `DashboardUser` / `DashboardPass` 构建 client；`DashboardEnabled=false` 或字段缺失 → 503 + 友好错误体（含"如何打开 frps dashboard"指引）。 | — |
| FR-2.7 | frpsadmin 错误 → HTTP 状态映射：`ErrUnauthorized` → 502（上游 401，让前端可区分"frp_easy 自身 401"和"frps dashboard 401"）；`ErrNotFound` → 404；`ErrUnavailable` → 503；其余 → 502。 | — |

### FR-3 frps.toml 渲染：dashboard 凭据自动生成

- FR-3.1：`internal/frpconf.RenderFrps()` 不变接口签名（已稳定）；调用方 handler 在写 KV 时做自动生成。
- FR-3.2：新增启动期函数 `ensureFrpsDashboardCreds`（在 `cmd/frp-easy/main.go`，与 `ensureFrpcAdminCreds` 同款；KV key = `frps.dashboard.autogen`），用 `auth.GenerateCSRFToken()` 生成 user / pass，**仅在 KV 中无值时生成一次**；存到独立 KV 让后续渲染始终能拿到稳定值。
- FR-3.3：`renderAndApplyFrps`（`internal/httpapi/config_helper.go`）在调 `RenderFrps` 前，若 `FrpsConfig.DashboardEnabled=true` 但 `DashboardUser` / `DashboardPass` 为空 → 从 KV `frps.dashboard.autogen` 读 fallback；若 `DashboardEnabled=false` → 自动翻 true（让"装完即用"成立）。
- FR-3.4：用户在 `Server` 设置页明确填写自己的 user/pass → KV `frps.config` 持久化的字段非空 → 渲染优先用用户填写值（不被自动生成值覆盖），即 FR-3.3 fallback 仅在用户没填时生效。
- FR-3.5：handler 构建 frpsadmin client 时也走同样的"优先 frps.config 字段 → fallback 到 frps.dashboard.autogen"逻辑，确保 client 拿到的凭据与 frps.toml 渲染出的凭据**字节一致**。

### FR-4 openapi.yaml

- FR-4.1：新增 4 条路径（FR-2.1 ~ FR-2.4）schema 定义。
- FR-4.2：新增 4 个 schema：`ServerRuntimeInfo` / `ServerRuntimeProxiesResponse` / `ServerRuntimeProxyDetail` / `ServerRuntimeTraffic`。
- FR-4.3：4 条路径都标 `cookieAuth: []` 与既有 read 端点一致。

### FR-5 dev-map.md 同步

- FR-5.1：`internal/frpsadmin/` 包条目新增在"目录布局"与"功能在哪里"两节。
- FR-5.2：新增路由分组在"HTTP 路由层"行追加 "T-039: +4 条"。

## 5. 非功能需求 NFR

| ID | 描述 |
|---|---|
| NFR-1 | 不引入新外部依赖（用 `net/http` + `encoding/json`，与 frpcadmin 同款）。go.sum 不动。 |
| NFR-2 | 默认 5s 超时；handler 不阻塞 chi router goroutine 池。 |
| NFR-3 | 凭据自动生成使用 `crypto/rand`（继承 `auth.GenerateCSRFToken` 已有实现，无新代码）。 |
| NFR-4 | dashboard 凭据持久化值与 KV `frpc.admin` 一样仅在 KV 内，**不**落 frp_easy.toml（避免明文配置漂移）。 |
| NFR-5 | 错误响应体走既有 `writeError(w, status, code, msg, field)`；用既有 code 常量（`CodeInternal` / `CodeUnauthenticated` / `CodeNotFound`），不新增 code。 |
| NFR-6 | 友好错误体含**修复指引**（如 503 时附 "请在 Server 设置页启用 dashboard 或检查 frps 进程是否运行"）。 |
| NFR-7 | frpsadmin 包单测 + handler 单测覆盖**全部**错误码分支（200 / 401 / 404 / 503）。 |
| NFR-8 | go vet / go test ./... PASS。 |

## 6. 约束 / 红线

- C-1：禁动 `internal/frpcadmin/`（对称镜像即可，不改其字节）。
- C-2：禁改 `cmd/frp-easy/main.go::run()` 的启动序列字节级语义（T-019 / T-038 NFR-9 保留）；只在适当位置插入 `ensureFrpsDashboardCreds` 一行。
- C-3：禁动 `internal/storage/` SQL（用既有 KV 接口）。
- C-4：禁碰前端代码（T-041 / T-042 owned）。
- C-5：禁动既有路由顺序 / 中间件链。
- C-6：禁触发 verify_all 新闸门（既有 G.1 D.1 已守 OpenAPI 字面；T-037 H.1 不涉及本任务字面）。

## 7. 验收标准 AC

| ID | 描述 | 验证方法 |
|---|---|---|
| AC-1 | `internal/frpsadmin/client.go` 存在，导出 `Client` / `New` / `NewWithTimeout` / `NewWithBaseURL` / `ServerInfo` / `Proxies` / `ProxyDetail` / `Traffic` / `ErrUnauthorized` / `ErrNotFound` / `ErrUnavailable`。 | `go doc` |
| AC-2 | `internal/frpsadmin/client_test.go` 覆盖 4 个方法 × 4 种状态分支（200 / 401 / 404 / 5xx），使用 `httptest.NewServer`。 | `go test ./internal/frpsadmin/ -v` |
| AC-3 | 4 条新路由注册到 chi router，归属 SessionAuth 分组（不在 ReadyGate 之前，不暴露未鉴权）。 | grep `router.go` + handler 单测 |
| AC-4 | handler 单测覆盖：200 happy path / 401（上游 frps 拒绝）/ 503（dashboard 未启用 / 未配置 / 连接失败）。 | `go test ./internal/httpapi/ -v -run ServerRuntime` |
| AC-5 | `ensureFrpsDashboardCreds` 实现：KV 无值时生成 + 持久化；KV 有值时直接返回；与 `ensureFrpcAdminCreds` 同款 3s 超时；user / pass 用 `auth.GenerateCSRFToken()`。 | grep + 单测（在 main_test.go 不可加时跳过 e2e） |
| AC-6 | `renderAndApplyFrps` 渲染逻辑：用户未配置 dashboard → 自动启用 + fallback 凭据；用户配置了完整凭据 → 不被覆盖。 | 单测（修改 `kvFrpsConfig` + KV `frps.dashboard.autogen` 不同组合，验证渲染出的 `frps.toml` 字面） |
| AC-7 | openapi.yaml 新增 4 条路径 schema；既有 D.1 OpenAPI 闸门保持 PASS。 | `verify_all D.1` |
| AC-8 | dev-map.md 新增 frpsadmin 条目；E.5 索引闸门不需要（dev-map 不在 AI-GUIDE 强索引列表）。 | grep |
| AC-9 | `verify_all.ps1` Full PASS ≥ 31，FAIL ≤ 1（baseline 31 PASS / 1 FAIL；C.1 已知豁免）。 | `pwsh scripts/verify_all.ps1` |
| AC-10 | 06_TEST_REPORT.md 含 `## Adversarial tests` 段（≥ 3 个反向证伪用例）。 | grep `^##\s+Adversarial\s+tests` |
| AC-11 | 07_DELIVERY.md 含 `## Insight` bullet 列表（若有 insight；无则省略段）。 | grep `^##\s+Insight$` |

## 8. 范围内 / 范围外

**范围内**：frpsadmin 包、4 条 REST 路由、凭据自动生成、openapi 同步、dev-map 同步、单测、verify_all 通过。

**范围外**：
- 前端 UI（T-041 / T-042 owned）。
- frps admin API 写操作（reload / kill client / disconnect proxy）—— 上游未稳定 API；若 T-040 阶段确认有需求再扩展。
- 多个 frps server 监管（当前架构只支持单 frps）。
- Prometheus metrics 路径 `/metrics`（OOS-1，可后续任务）。

## 9. 关联任务

| ID | 关系 | 文档目录 |
|---|---|---|
| T-002 | 引入 `FrpsConfig.Dashboard*` 5 字段 + 路由 `/server` | `docs/features/_archived/zero-config-quickstart/` |
| T-005 | openapi.yaml 28 条路由基线 | `docs/features/_archived/docs-and-api-schema/` |
| T-006 | 既有 frpcadmin 包模式 | `docs/features/_archived/e2e-smoke-tests/` |
| T-038 | KV `system.autorestore.*` + dashboard 卡片 UI 范本 | `docs/features/_archived/boot-autostart-hardening/` |
| T-040 | （下游）frps 在线 client 列表 UI | （未启动） |
| T-041 | （下游）frpc 状态卡片 UI | （未启动） |
| T-042 | （下游）流量图表 UI | （未启动） |

## 10. 决策点（用户已明确）

- **D-1**：dashboard 凭据自动生成 = on by default（"用户体验好"原则）。
- **D-2**：包结构与 frpcadmin 对称（"长期易使用易维护"原则）。
- **D-3**：错误分类用 sentinel error（"长期易维护"原则）。
- **D-4**：不引入新依赖（"软件工程标准"原则，最小依赖增量）。
- **D-5**：测试覆盖 + openapi 同步（"软件工程标准"原则）。

## 11. 开放问题（PM-DECIDED）

- **Q-1**：聚合 `Proxies()` × N 个 type 时部分 type 失败的策略？
  → **decision**：部分成功返回（per-type 报错 collect 在 `errors` 字段，整体仍 200）。frps 对未启用的 type（如未配置 xtcp）可能返回空数组而非错误，应能正常工作。这种"软容错"是 T-041 前端 UX 友好的基础。
- **Q-2**：用户在 Server 设置页明确填了空字符串 → 视为"清空 = 想用 autogen"还是"明确禁用"？
  → **decision**：视为"想用 autogen"（DashboardEnabled=true 且字段空 → fallback；DashboardEnabled=false → 不启用，handler 503）。这与 D-1 一致。
- **Q-3**：autogen 凭据何时旋转？
  → **decision**：MVP 不旋转。后续如有"凭据轮换"需求另立任务（与本任务范围解耦）。这与 insight L38 ("一次性恢复必须配 backoff") 不冲突，因为 autogen 是"配置生成"非"运行时恢复"，无重试场景。

---

**Verdict**：READY FOR ARCHITECT。
