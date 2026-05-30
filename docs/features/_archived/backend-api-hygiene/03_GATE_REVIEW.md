# 03 闸门评审 — T-055 backend-api-hygiene

> Stage 3 / Gate Reviewer · mode: full · 输出语言：中文
>
> 独立核实：已逐处 Read 设计引用的真实代码（handlers_server_runtime.go / frpsadmin/client.go / handlers_proc.go / handlers_proxies.go / handlers_system.go / router.go / errors.go / 既有测试文件），验证符号存在与可复用性。

## 1. 8 维审计清单

| # | 维度 | 结论 | 理由（一句话） |
|---|---|---|---|
| 1 | 需求完整性 | PASS | A-1/A-2/A-3、B-1/B-2/B-3 均给出可测断言（status / code / field / 文案 / 不含子串），无模糊词。 |
| 2 | 设计完整性 | PASS | 每条 in-scope 行为映射到具体文件+改动；A 双层防御 + B helper 统一化，覆盖全部 AC。 |
| 3 | 复用正确性 | PASS | `frpsProxyTypes`（handlers_server_runtime.go:46 实测 `[]string{"tcp","udp","http","https","stcp","sudp","xtcp"}`）、logger nil 守卫范式（handlers_proxies.go:147-148 实测存在）、`writeError`（errors.go:50 签名一致）、`frpsAdminFactory`+`NewWithBaseURL` seam 均实测可复用。 |
| 4 | 风险覆盖 | PASS | R-1~R-5 覆盖 escape 回归 / PathEscape 语义 / 误删语义化分支 / 日志旁路 / e2e 契约；均带缓解。 |
| 5 | 迁移安全 | PASS | 无 schema / 无数据迁移；A-1 向更严格兼容、B 仅文案变更，回滚 = git revert。 |
| 6 | 边界处理 | PASS | BC-1~BC-6 覆盖空 type / 大小写 / name 含 `/` `..` `?` `#` / logger nil / 上游 404。 |
| 7 | 测试可行性 | PASS（见 C-1） | A 走既有 black-box seam；B 的 500 兜底经测试 seam 论证（§6）抽 `writeInternalError` helper 直测，绕开具体类型不可达问题。 |
| 8 | 范围外清晰 | PASS | O-1~O-6 明确；uploadBin errno 透传（O-3）、(type,remote_port) sentinel 化（O-2 backlog）显式排除，防过度构建。 |

## 2. Findings（WARN / FAIL）

无 FAIL。无阻塞 WARN。以下为 development 期应主动消化的 conditions（见 §4）。

## 3. Development 期高概率提问（预答）

- **Q1：`url.PathEscape` 会编码 `/` 吗？** 会。`url.PathEscape` 面向单 path segment，`url.PathEscape("a/b")` = `"a%2Fb"`、`url.PathEscape("x y#z")` = `"x%20y%23z"`、`url.PathEscape("..")` = `".."`（`.` 是 unreserved 不编码，但作为单 segment 编码后整体不含 `/` 分隔符故不构成穿越）。`url.PathEscape("tcp")` = `"tcp"`（unreserved 字符不编码，无回归）。设计 R-2 已锁定，AC-2 以 `a/b`→`a%2Fb` 断言。
- **Q2：A-1 校验放 `serverRuntimeProxyDetail` 还是 `serverRuntimeTraffic`？** 仅 `serverRuntimeProxyDetail`（它有 `{type}` 段可对白名单）；`serverRuntimeTraffic` 只有 `{name}` 无白名单可依，靠 client 层 escape 根防御（O-1）。设计 §6 已明示。
- **Q3：B 的 helper 命名与放置？** 设计 §3 给出 `(h *handlers) writeInternalError(w, userMsg, cause)`，放 handlers_proc.go 或就近；3 处兜底改调用它。注意 helper 内 logger 用 `Error` 级（比 proxies.go:148 的 `Warn` 高一档，因 500 是真错误）——dev 可自行决定 Warn/Error，测试只断言响应文案与"不含内部子串"。
- **Q4：A-1 的 422 文案？** 设计 §5 给"不支持的代理类型"，field=`type`，code=`VALIDATION_FAILED`。与项目既有 422 文案风格（handlers_system.go:136 "kind 必须为 frpc 或 frps"）一致。
- **Q5：测试如何断言"未触上游"（AC-1）？** mock frps server 设一个 `var hit bool`，handler 走 422 早返时 `hit` 应保持 false（白名单校验在 `buildFrpsAdminClient` 之后、`c.ProxyDetail` 之前，故 mock 不会被调）。注意：A-1 校验需放在 `buildFrpsAdminClient` 之前还是之后？设计流程图把校验放 `pt := URLParam` 之后、`c.ProxyDetail` 之前——dev 可把 type 校验**前移到 `buildFrpsAdminClient` 之前**，让非法 type 在不构造 client 时即 422（更省，且 AC-1 的 hit=false 更稳）。这是 C-2。

## 4. Conditions（APPROVED WITH CONDITIONS）

- **C-1**：B-1/B-2/B-3 必须以 `writeInternalError` helper 的直接单测覆盖（含捕获型 logger 验证"不含内部子串"+ 固定文案）；不得因"黑盒不可达"而跳过 B 的测试。测试数只升不降（baseline bump，insight L46）。
- **C-2**：A-1 type 白名单校验建议前移到 `buildFrpsAdminClient` 之前，让非法 type 不构造 client 即 422（AC-1 的"未触上游"更稳）。非阻塞，dev 自然顺手消化。
- **C-3**：06_TEST_REPORT.md 的对抗测试段必须是裸 `## Adversarial tests`（无数字/§ 前缀，insight L40/L52，否则 verify_all E.6 FAIL）。
- **C-4**：07_DELIVERY.md 的 Insight 段裸 `## Insight`（insight L18）。

## 5. 裁决

**APPROVED WITH CONDITIONS**

设计与需求完整、可测、零依赖、零扩散；条件 C-1~C-4 均为 development 期可自然消化项，不阻塞 stage 4 启动。
