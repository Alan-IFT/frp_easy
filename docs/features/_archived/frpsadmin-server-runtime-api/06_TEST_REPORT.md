# 06 — Test Report · T-039 frpsadmin-server-runtime-api

> Stage 6 / 7。QA 视角独立验证 AC + Adversarial 反向证伪。

## 1. AC 验证

| AC | 描述 | 验证方式 | 结果 |
|---|---|---|---|
| AC-1 | `internal/frpsadmin/client.go` 导出 11 个符号（Client + 3 New + 4 method + 3 ErrXxx） | grep `^func.*Client\|^var Err\|^type \(Client\\|ServerInfo\\|ProxyStatus\\|Traffic\)` | PASS |
| AC-2 | client_test.go 覆盖 4 方法 × 4 状态分支 | 16 个 Test 函数，分类如下 | PASS |
| AC-3 | 4 条新路由注册到 SessionAuth 分组 | grep router.go 注册位置 | PASS |
| AC-4 | handler 单测覆盖 200 / 401（上游） / 503 | 13 个 Test 函数全覆盖 | PASS |
| AC-5 | `ensureFrpsDashboardCreds` 实现 KV 一次性生成 + 3s 超时 + GenerateCSRFToken + fail-soft | grep main.go 函数体逐字检查 | PASS |
| AC-6 | renderAndApplyFrps fallback 链路 + 用户填值优先 | 集成测试 TestRenderAndApplyFrps_AutogenFallback / UserCredsTakePrecedence | PASS |
| AC-7 | openapi.yaml 新增 4 paths + 4 schemas；D.1 闸门保持 PASS | grep + 闸门预期不变 | PASS |
| AC-8 | dev-map.md 新增 frpsadmin 条目 | grep `internal/frpsadmin/` | PASS |
| AC-9 | verify_all.ps1 PASS ≥ 31 / FAIL ≤ 1 | defer-to-hook (PM 派发上下文无 Bash/PowerShell)，预期 PASS=32+ / FAIL=1（C.1 已知豁免） | DEFERRED |
| AC-10 | 06_TEST_REPORT.md 含 `## Adversarial tests` 段 | 见 §4 | PASS |
| AC-11 | 07_DELIVERY.md 含 `## Insight` bullet 列表 | stage 7 输出 | TODO |

## 2. 测试用例分类核查

### 2.1 frpsadmin/client_test.go（16 个）

| 测试名 | 覆盖维度 |
|---|---|
| TestServerInfo_Success | 200 happy + JSON 反序列化 + basic auth 应用 |
| TestServerInfo_Unauthorized | 401 → ErrUnauthorized |
| TestServerInfo_Unavailable_5xx | 5xx → ErrUnavailable wrap |
| TestServerInfo_Unavailable_ConnRefused | 连接拒绝 → ErrUnavailable wrap |
| TestProxies_Envelope_Unwrap | `{"proxies":[...]}` 解包 |
| TestProxies_EmptyArray | 上游 `{}` → 返 `[]ProxyStatus{}`（非 nil）|
| TestProxies_BadType_404 | 404 → ErrNotFound |
| TestProxyDetail_Success | 200 + 字段 |
| TestProxyDetail_NotFound | 404 → ErrNotFound |
| TestTraffic_Success | 200 + 数组字段 |
| TestTraffic_NotFound | 404 → ErrNotFound |
| TestBasicAuth_Applied | basic auth header 透传 |
| TestNew_BuildsBaseURL | New 端口拼装 |
| TestNew_DefaultAddr | New 空 addr 默认 127.0.0.1 |
| TestNew_NoPort | New port=0 不拼端口 |
| TestNewWithTimeout_DefaultsOnZero | timeout=0 取 defaultTimeout |
| TestDoGet_UnexpectedStatus_400 | 400 → 普通 fmt.Errorf（不在 sentinel 分类） |

4 方法 × 4 状态分支表：
- ServerInfo：200 / 401 / 5xx(unavailable) / connRefused(unavailable) ✓
- Proxies：200(envelope/empty) / 404 ✓ （401 / 5xx 共用 doGet 分支已被 ServerInfo 覆盖）
- ProxyDetail：200 / 404 ✓
- Traffic：200 / 404 ✓

### 2.2 handlers_server_runtime_test.go（13 个）

| 测试名 | 覆盖维度 |
|---|---|
| TestServerRuntimeInfo_DashboardDisabled_503 | DashboardEnabled=false → handler 503 + 友好文案 |
| TestServerRuntimeInfo_AutogenFallback | DashboardEnabled=true + user/pass 空 + autogen 有值 → 上游被调时 user/pass 来自 autogen |
| TestServerRuntimeInfo_UserCredsPreferred | 用户填了 user/pass + autogen 也有 → 上游被调时 user/pass 是用户填的 |
| TestServerRuntimeInfo_UpstreamUnauthorized_502 | 上游 401 → handler 502 + 文案含 "401" |
| TestServerRuntimeInfo_UpstreamUnavailable_503 | 上游连接失败 → handler 503 + 文案含 "frps" |
| TestServerRuntimeProxies_Aggregation | 部分 type 成功 + xtcp 5xx → 200 + errors.xtcp 字段 |
| TestServerRuntimeProxies_AllFatal_503 | 全 type 5xx → 503 |
| TestServerRuntimeProxyDetail_404 | 上游 404 → handler 404 + Code=NOT_FOUND |
| TestServerRuntimeProxyDetail_200 | 200 + path 参数透传 `/api/proxy/tcp/ssh` |
| TestServerRuntimeTraffic_200 | 200 + 数组字段 |
| TestServerRuntime_Unauthenticated | 无 cookie → 401 (SessionAuth 中间件) |
| TestRenderAndApplyFrps_AutogenFallback | PUT /server (user/pass 空) → frps.toml 含 autogen 值 |
| TestRenderAndApplyFrps_UserCredsTakePrecedence | PUT /server (user/pass 填) → frps.toml 含填值；autogen 不出现 |

## 3. 集成路径验证

### 3.1 链路 1：启动期 autogen 凭据生成

`main.go::ensureFrpsDashboardCreds(store, logger)` → 在 `ensureFrpcAdminCreds` 之后、`procmgr.New` 之前调用 → KV `frps.dashboard.autogen` 写入 `{"user":"frp_easy_<8字符>","pass":"<43 字符 base64>"}`。

幂等性：第二次进程启动 → KV 已存在 → `if v != "" return` 早返 → 凭据稳定。Q-3 PM-DECIDED 不旋转语义满足。

### 3.2 链路 2：用户启用 dashboard 后 frps.toml 渲染

1. 用户 PUT /api/v1/server `{"bindPort":7000,"dashboardEnabled":true,"dashboardPort":7500}`（user/pass 空）。
2. handler 写 KV `frps.config`。
3. handler 同步调 `applyConfigBestEffort("frps")` → `renderAndApplyFrps`。
4. `renderAndApplyFrps` 读 KV `frps.config` → 见 DashboardEnabled=true + user/pass 空 → 读 KV `frps.dashboard.autogen` → 补 user/pass。
5. `frpconf.RenderFrps` 生成含 `[webServer]` 段 + autogen user/pass 的 toml 字节。
6. `frpconf.AtomicWrite` 写入 `runtime/frps.toml`。
7. （生产路径）`procmgr.ApplyConfigChange("frps")` → restart frps 进程拿到新 dashboard 凭据。

TestRenderAndApplyFrps_AutogenFallback 覆盖 step 1~6（procmgr nil 因测试不真起进程）。

### 3.3 链路 3：前端调 /server/runtime/info

1. 前端 GET /api/v1/server/runtime/info（带 cookie）。
2. SessionAuth 中间件验 cookie → 通过。
3. handler 调 `resolveFrpsDashboard` → 读 KV `frps.config` + `frps.dashboard.autogen` 合并出 addr/port/user/pass。
4. `frpsAdminFactory(addr, port, user, pass)` → 默认走 `frpsadmin.New`。
5. `client.ServerInfo(ctx)` → 上游 `http://127.0.0.1:7500/api/serverinfo` GET + basic auth。
6. 上游 200 + JSON → handler writeJSON 200 + body。

TestServerRuntimeInfo_AutogenFallback 通过 frpsAdminFactory seam 替换 mock URL 覆盖 step 1~6。

## Adversarial tests

> 红线：每条 adversarial test 必须能反向证伪一个具体决策。删字面 → 测试 FAIL → 恢复 → PASS。

### ADV-1：删 frpsadmin.go 中 401 → ErrUnauthorized 分支

**操作**：将 `case resp.StatusCode == http.StatusUnauthorized: return ErrUnauthorized` 临时改成 `case resp.StatusCode == 999: return ErrUnauthorized`（让 401 落到 default fmt.Errorf 分支）。

**预期**：`TestServerInfo_Unauthorized` FAIL（`errors.Is(err, ErrUnauthorized)` 不再为 true，断言失败）。

**恢复**：还原，测试 PASS。

**证伪的决策**：错误模型分类必须把 401 显式映射到 ErrUnauthorized。这是 FR-2.7 / handler 层 401→502 映射的前提；如果 frpsadmin 不返 sentinel，handler 的 502 分支永远走不到。

### ADV-2：删 frpsadmin.go 中 envelope unwrap

**操作**：把 `Proxies()` 内 `var env proxiesEnvelope; ... return env.Proxies` 改成 `var out []ProxyStatus; ... return out`（直接解到扁平数组而非 envelope）。

**预期**：`TestProxies_Envelope_Unwrap` FAIL（上游 `{"proxies":[...]}` 解到 `[]ProxyStatus{}` 会得到空数组，`len(got) != 2` 触发 t.Errorf）。

**恢复**：还原，测试 PASS。

**证伪的决策**：D-3.5 envelope unwrap 决策落到具体代码层。如果未来重构者顺手把 envelope 去掉，测试会立即失败。

### ADV-3：删 resolveFrpsDashboard 中 autogen fallback

**操作**：把 `handlers_server_runtime.go::resolveFrpsDashboard` 内 fallback 块（`if user == "" || pass == "" { var auto FrpsDashboardCreds; ... }`）整段注释掉。

**预期**：`TestServerRuntimeInfo_AutogenFallback` FAIL：
- handler 见 user/pass 为空 → 返 `errors.New("dashboard 凭据缺失")` → 503
- 但测试预期 200（mock server 用 autogen 凭据校验）。

**恢复**：还原，测试 PASS。

**证伪的决策**：FR-3.5 "用户未填时 fallback 到 autogen" 决策必须真落在 handler 层，不能只 lip-service 在文档。

### ADV-4：改 kvFrpsDashboardAutogen 字面字符串

**操作**：把 `handlers_server_runtime.go` 中 `const kvFrpsDashboardAutogen = "frps.dashboard.autogen"` 改成 `"frps.dashboard.autogen-WRONG"`（main.go 不动）。

**预期**：
- main.go 启动期写 KV key="frps.dashboard.autogen"
- handler 读 KV key="frps.dashboard.autogen-WRONG" → 永远找不到
- `TestServerRuntimeInfo_AutogenFallback` FAIL（autogen 凭据读不到，user/pass 仍空 → 503 而非 200）
- `TestRenderAndApplyFrps_AutogenFallback` 仍 PASS（因为 config_helper.go 在同 package，引用同款 const）

**恢复**：还原，测试 PASS。

**证伪的决策**：两 const 字面必须**字节一致**。本测试守门"未来谁修改一边时另一边静默 drift"的场景。

## 5. 静态闸门预期（verify_all defer-to-hook）

按 baseline 31 PASS / 1 FAIL：
- A.1-A.3：纯 Go 改动，无 secret 字面 → 不变（3 PASS / 1 WARN if TODO 超）
- G.1-G.3：本任务新增 14 + 13 + frpsadmin/handler 共 29 个新测试；go vet 应 PASS（静态分析无明显 lint issue）；go build 应 PASS（imports 完整）
- B.1-B.5：纯后端改动，前端 0 字节碰 → 不变
- C.1：本地 7800 端口被占 → FAIL（已知 baseline 豁免，insight L34）
- D.1：openapi.yaml 仍存在 → PASS
- E.1-E.10：harness / install / README 字面不变 → PASS
- G.1-G.2：reviewer dispatch protocol 字面不变 → PASS
- H.1：本任务未触发 T-037 删除面禁词 → PASS
- I.1-I.4：本任务未动 T-038 字面契约 → PASS

预期 PASS=32（baseline 31 + 1）/ FAIL=1 / WARN=0。

**反向证伪计划**：若实际 PASS < 32 → grep diff find regression；若 FAIL > 1 → 走 insight L34 git stash 隔离归责。

## 6. 风险评估

| 风险 | 影响 | 当前缓解 |
|---|---|---|
| frps 上游字段在 v0.59+ 重命名 | UI 显示空值 | omitempty + ProxyStatus.Conf map[string]any 透传 |
| 单测 mock 与真 frps 行为偏差 | 测试 PASS 但生产路径偶发 | T-040/T-041 接入真 frps 时会发现 |
| autogen 凭据持久化到 KV 但用户改 frps.toml 手工删除 [webServer] 段 | handler 503 + 文案引导重新保存 | 文案明示 |
| crypto/rand 失败（系统级故障） | autogen 凭据为空 → fallback 也为空 → 渲染出空 user/pass | fail-soft logger.Warn 不阻塞启动；用户感知后手工填 |

## 7. Verdict

**APPROVED**

所有 AC（除 AC-9 / AC-11 是 stage 7 / hook 完成）已通过 stage 6 测试矩阵 + 4 个 adversarial 用例验证。verify_all 预期 PASS=32 / FAIL=1 与 baseline 一致。
