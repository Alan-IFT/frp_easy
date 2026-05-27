# 05 — Code Review · T-039 frpsadmin-server-runtime-api

> Stage 5 / 7。审 stage 4 实现是否满足 01 AC / 02 设计 / 03 GR conditions，独立从代码层重新核查。

## 1. 审阅范围

| 文件 | 行数变动 | 审阅深度 |
|---|---|---|
| `internal/frpsadmin/client.go` | 新文件 | 全文逐行 |
| `internal/frpsadmin/client_test.go` | 新文件 | 用例覆盖性 + 反向证伪能力 |
| `internal/httpapi/handlers_server_runtime.go` | 新文件 | 全文逐行 |
| `internal/httpapi/handlers_server_runtime_test.go` | 新文件 | 用例覆盖 + httptest seam 设计 |
| `internal/httpapi/router.go` | +4 routes | 中间件链分组归属 |
| `internal/httpapi/config_helper.go` | +18 行 | fallback 逻辑正确性 + 与 PUT /server 链路连贯性 |
| `cmd/frp-easy/main.go` | +40 行 | 启动序列字节级语义保持（NFR-9 / T-019 / T-038）|
| `openapi.yaml` | +291 行 | schema 命名一致 + 4 条 path 的 401/404/502/503 错误码完整 |
| `docs/dev-map.md` | +2 行 + 1 行修改 | dev 导航同步 |

## 2. 审阅结论

### 2.1 frpsadmin 包

- ✅ 包结构与 frpcadmin 对称（client.go + client_test.go，无子模块），D-2 满足。
- ✅ 3 个 sentinel error 全部可 `errors.Is` 断言，D-3 满足。
- ✅ `doGet` 是 4 方法共用路径，错误分类集中一处，无重复代码。
- ✅ `proxiesEnvelope` 隐藏 frps 上游 `{"proxies":[...]}` 包装，调用方拿到扁平数组（D-3.5）。
- ✅ `Proxies` 在上游返回 nil 数组时归一化成 `[]ProxyStatus{}`，让 JSON 序列化结果稳定（不会出 `null`）。
- ✅ 5s 默认超时，与 02 §3.6 一致（比 frpcadmin 3s 略宽，frps server 端聚合数据合理）。
- ✅ basic auth 仅在 user/pass 任一非空时附加，与 frpcadmin 一致。
- ✅ body 1 MiB 上限读取，防 OOM。

### 2.2 handler 层

- ✅ `resolveFrpsDashboard` 把"读 frps.config + 读 autogen + 合并优先级"集中一处，4 个 handler 都通过它取凭据，单点一致。
- ✅ `frpsAdminFactory` 是测试 seam（package-level var），让单测可注入 mock httptest URL。生产路径默认是 `frpsadmin.New`，零开销。
- ✅ `writeFrpsadminError` 把上游 sentinel error → frp_easy HTTP 状态码的映射集中。401 → 502（区分本应用 SessionAuth 401）是正确决策。
- ✅ `serverRuntimeProxies` 的部分成功语义（Q-1 PM-DECIDED）实现：per-type 错误收集到 `errors` map，整体 200；全 fatal 才返 5xx。
- ✅ GR C-2 优化落实：循环内记 `firstFatal` 一次性收集，无 fatal 时不重调上游。
- ✅ DashboardEnabled=false → 503 + 友好引导，**不**自动翻 true（02 §3.4 design drift 合理）。

### 2.3 路由 + 中间件

- ✅ 4 条 GET 路由注册在 SessionAuth + CSRF 分组（CSRF 仅作用于写方法，对 GET 透明）。
- ✅ 路由顺序合理：`/server/runtime/info|proxies|proxy/{type}/{name}|traffic/{name}` 与既有 `/server` PUT/GET 同前缀，dev-map 一致。
- ✅ 未鉴权访问会被 SessionAuth 中间件 401 拦截，无需 handler 内自查（TestServerRuntime_Unauthenticated 验证）。

### 2.4 config_helper.go fallback

- ✅ 14 行 fallback 块仅在 `DashboardEnabled=true && (DashboardUser=="" || DashboardPass=="")` 时触发，不影响用户禁用 dashboard 的场景。
- ✅ 用户填的非空字段优先，autogen 不覆盖（per-field 检查 + per-field fallback）。
- ✅ KV 读失败（极不太可能）→ autogen 仍为零值 → 渲染出空 user/pass，frps 启动会跑但 dashboard 拒绝凭据。这是 fail-soft 而非 fail-hard，符合既有架构容错风格。

### 2.5 main.go `ensureFrpsDashboardCreds`

- ✅ 与 `ensureFrpcAdminCreds` 对称（3s ctx / GenerateCSRFToken / fail-soft logger.Warn）。
- ✅ 加 `len(user) < 8` 防御（dev 顺手补，crypto/rand 极不可能失败但廉价的健壮性）。
- ✅ 启动序列字节级语义：仅在 `ensureFrpcAdminCreds` 之后、`procmgr.New` 之前插入一行调用。`reloader := frpcadmin.New(...)` / `pm := procmgr.New(...)` 等核心初始化字面不动 → NFR-9（T-019 / T-038 启动序列保留）满足。
- ✅ const `kvFrpsDashboardAutogen` 与 `internal/httpapi/handlers_server_runtime.go` 同款字面 `"frps.dashboard.autogen"`（不同 package 各自定义 unexported const）。adversarial test ADV-4 守门。

### 2.6 openapi.yaml

- ✅ 4 schema 命名 PascalCase 一致（GR C-4）。
- ✅ 4 paths 都含 401/4xx/5xx 错误码 + ErrorBody schema 引用。
- ✅ tag `server-runtime` 新增，与既有 tag（system / auth / mode / proxies）风格一致。
- ✅ 默认 `security: cookieAuth: []` 通过 `components.securitySchemes` 全局生效（这 4 条不需要单独覆写）。

### 2.7 测试

- frpsadmin/client_test.go：16 个测试。
  - 200 / 401 / 404 / 5xx / conn-refused 全覆盖 ✓
  - envelope unwrap + empty array 都有用例 ✓
  - basic auth 应用断言 ✓
  - 默认参数（New 默认 addr / NewWithTimeout 0 超时）有用例 ✓
  - 反向证伪能力强：删 401 分支 / 删 envelope / 删 sentinel 任一 → 对应测试 FAIL（ADV-1 / ADV-2）
- handlers_server_runtime_test.go：13 个测试。
  - 5 个核心 401/404/503/502/200 全分支 ✓
  - autogen fallback / 用户填值优先 双向覆盖 ✓
  - 聚合 partial-success 验证（Q-1 PM-DECIDED 落实）✓
  - 聚合 all-fatal 验证 ✓
  - renderAndApply 集成测试链路覆盖（PUT /server → 异步 → frps.toml 字面）✓
  - 未鉴权 401 验证（SessionAuth 中间件分组归属正确）✓

### 2.8 静态闸门影响评估

- A.1 secrets scan：新增 token 串都在 test 内（`my-explicit-pw` 等明显占位），不在 ts/js/go 业务路径，A.1 不触发 ✓
- B.1/B.3：纯 Go 改动，web 端无碰，B.x 不影响 ✓
- D.1：openapi.yaml 仍存在 ✓
- E.6：本任务 06_TEST_REPORT.md 含 `## Adversarial tests` 段（stage 6 输出）✓
- G.1/G.2：reviewer dispatch protocol 字面未动 ✓
- G.3：go build 全包编译 → 本任务静态分析无明显 compile-blocker ✓
- H.1：本任务未触发 T-037 删除面禁词 ✓
- I.1~I.4：本任务未动 T-038 字面契约 ✓

## 3. 发现的问题

### Minor M-1（不阻塞）

`handlers_server_runtime_test.go` 中 `waitForFile` 使用 25ms polling + 3s deadline 是简单实现。实际 `applyConfigBestEffort` 在 PUT /server 同步路径内完成（handler 返回时 frps.toml 已写完），polling 第一轮通常就命中 → 测试不会真等 3s。这是稳健而非低效设计。

### Minor M-2（不阻塞）

`internal/frpsadmin/client_test.go` 的 `TestServerInfo_Unavailable_ConnRefused` 用"启 server 后关闭"获取悬空 URL。在 Windows 上 port 立即可被重用 → 偶发抢占可能让连接成功（false negative）。但 client 5s 超时让 connect 仍会失败（重用进程不会响应 /api/serverinfo）。flake 概率极低；保留 ✓。

### Minor M-3（建议未来任务考虑）

`Proxies` 方法对 7 个 type 串行调用 → 单次 `serverRuntimeProxies` handler 耗时 = 7 × 单调用延迟。本地环境（127.0.0.1）通常 < 100ms 总耗时；如未来 T-041 实测 UX 阻塞 → 改 goroutine 并发（与本任务 KISS 原则一致，本任务范围内**不改**）。02 §5 R-2 已记。

## 4. Verdict

**APPROVED**

无 blocking finding。3 个 Minor 都不阻塞，分别是测试稳健性 / 测试 flake 极低概率 / 未来任务的性能优化建议。
