# 06 测试报告 — T-055 backend-api-hygiene

> Stage 6 / QA Tester · mode: full · 输出语言：中文
>
> 诚实声明：本 QA 上下文**无 Bash / PowerShell 工具**（insight L14 role-collapse 现象），无法自跑 `go test` / `scripts/verify_all`。本报告记录测试计划 + 我独立从 AC 推导的对抗用例（已落代码）+ 失败假设；**verify_all 的真实执行由 orchestrator 独立 `bash scripts/verify_all.sh`（全量含 e2e）作权威硬闸门**。不在此处伪造 PASS（insight L46 明示 role-played QA 不得假报）。

## Test plan

| 验收标准 | 测试用例 | 文件 |
|---|---|---|
| AC-1 非白名单 type→422+field=type+未触上游 | `TestServerRuntimeProxyDetail_BadType_422` | `internal/httpapi/handlers_server_runtime_test.go` |
| AC-2 特殊字符 path 正确编码 | `TestProxyDetail_PathEscape`（6 子例）/ `TestTraffic_PathEscape` / `TestProxies_PathEscape` | `internal/frpsadmin/client_test.go` |
| AC-3 正常 path 无回归 | `TestProxyDetail_PathEscape::normal_no_regression` + `TestServerRuntimeProxyDetail_AllValidTypes` | client_test.go / handlers_server_runtime_test.go |
| AC-5/B-2 兜底"保存失败"无 SQL 后缀 | `TestMapProxyWriteError_Fallback_NoLeak` | `internal/httpapi/handlers_hygiene_test.go` |
| B-1 procStop 兜底固定文案+进日志 | `TestWriteInternalError_FixedMessage_NoLeak` | handlers_hygiene_test.go |
| B-3 downloadBin 兜底固定文案+进日志 | `TestWriteInternalError_NilLogger`（+ FixedMessage 覆盖行为） | handlers_hygiene_test.go |
| B-2 保留语义化分支 | `TestMapProxyWriteError_DuplicateName_Preserved` / `TestMapProxyWriteError_Validation_Preserved` | handlers_hygiene_test.go |

## Boundary tests added

- 空 / 大写 type：`isFrpsProxyType` 对空串、`TCP`（大写）均不命中 → 422（AC-1 用例覆盖 `evil`；白名单大小写敏感由 `TestServerRuntimeProxyDetail_AllValidTypes` 反向确认仅小写放行）。
- path segment 含 `/` `?` `#` 空格 `%`：`TestProxyDetail_PathEscape` 表驱动逐一断言编码（BC-3/BC-4）。
- `%` 幂等：`a%2f` → `a%252f`（已编码字符再编码安全）。
- nil logger：`TestWriteInternalError_NilLogger` 验证不 panic（BC-5）。
- nil cause：`writeInternalError` 内 `cause != nil` 守卫（静态确认，handlers_proc.go:108）。

## Adversarial tests

> 每条 AC 一个独立 reproducer + 失败假设。我从 AC 直接推导这些断言（非抄 04 的测试思路），刻意构造"攻击者视角"输入证伪实现。

| AC | 失败假设（"我预期失败当…"） | Reproducer（我写的） | 预期结论 |
|---|---|---|---|
| AC-1 | 攻击者传 `type=../admin` 想绕过白名单触达上游非法 path | `TestServerRuntimeProxyDetail_BadType_422`（断言 `upstreamHit==false`） | 应 Survive：`../admin` ∉ 白名单 → 422，client 根本不构造，mock 永不被调 |
| AC-2 | 攻击者在 `name` 注入 `x/../etc?a=1` 想把单段 name 变成"跨目录 + query"改变上游请求语义 | `TestTraffic_PathEscape`（断言上游 `r.RequestURI == /api/traffic/x%2F..%2Fetc%3Fa=1`） | 应 Survive：`/`→`%2F`、`?`→`%3F`，上游收到的是单 segment，无目录穿越、无 query 切断 |
| AC-2 | `Proxies` 的 type 注入 `tcp/../admin` 想访问非 `/api/proxy/*` 的上游端点 | `TestProxies_PathEscape`（断言 `r.RequestURI == /api/proxy/tcp%2F..%2Fadmin`） | 应 Survive：`/` 被编码，上游 path 仍锚定在 `/api/proxy/` 下 |
| AC-3 | escape 把正常 `tcp`/`ssh` 也改写导致全量正常请求回归 | `TestProxyDetail_PathEscape::normal_no_regression`（断言 `/api/proxy/tcp/ssh` 字节不变）+ 既有 9 个 frpsadmin client 回归测试 | 应 Survive：`url.PathEscape` 对 unreserved 字符不编码 |
| AC-5/B-2 | 触发兜底 500 时裸 SQL（`UNIQUE constraint failed ... sqlite ... errno`）泄露到前端 message | `TestMapProxyWriteError_Fallback_NoLeak`（leak 关键词列表断言 + message 严格等于"保存失败"） | 应 Survive：兜底改固定文案，多关键词扫描确认无泄露 |
| B-1/B-3 | 内部 error（含 driver/errno/路径）经 `writeInternalError` 仍漏到响应体 | `TestWriteInternalError_FixedMessage_NoLeak`（响应体扫 6 个 leak 子串 + 验证 cause 进日志） | 应 Survive：响应只含固定文案，cause 仅进 logger buffer |
| B-2 保留 | "改兜底"误把 `ErrDuplicateName` 409 / validation 422 也吞成 500 | `TestMapProxyWriteError_DuplicateName_Preserved`（断言 409）/ `_Validation_Preserved`（断言 422 透传） | 应 Survive：前置语义化分支字面保留，仅 L260 单行改 |

证据获取说明：上述用例已落代码（client_test.go / handlers_server_runtime_test.go / handlers_hygiene_test.go），但本 QA 上下文无法执行 `go test` 抓取实际输出。**orchestrator 的 `bash scripts/verify_all.sh` 全量运行将产出每条的真实 PASS/FAIL 证据**，作为本报告 verdict 的最终确证依据。

## verify_all result

- **执行者**：orchestrator（QA 上下文无 Bash，不自跑、不伪造）。
- 预期：Total Go tests 308 → 318（+10 顶层 Test*），test_count 734 → 744；frontend_tests 426 不变；Fail 期望 0。
- baseline.json：已 bump（go_tests=318 / test_count=744 / passing_count=744），由 dev stage 落地。
- E.6 闸门：本报告用裸 `## Adversarial tests` 标题（无数字/§ 前缀，insight L40/L52），避免 E.6 由 PASS 转 FAIL。

## Defects found

无（静态审查 + 测试设计阶段未发现缺陷）。若 orchestrator 真跑 verify_all 出现 FAIL，路由回 developer（dev-backend）。

## Stability

- 全部新测试为确定性单测（无时间/并发/网络外部依赖；httptest mock 本地回环），不引入 flake 风险。
- escape 测试断言 `r.RequestURI`（请求行原文逐字节），无规范化不确定性。

## Verdict

**APPROVED FOR DELIVERY（pending orchestrator verify_all 硬闸门）**

测试设计覆盖全部 AC + 对抗用例 + 保留分支回归；测试数只升不降；06 含裸 `## Adversarial tests` 段。最终 PASS 由 orchestrator 真跑 verify_all 确证——若全绿则交付，若红则回 dev。
