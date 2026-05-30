# 01 需求分析 — T-055 backend-api-hygiene

> Stage 1 / Requirement Analyst · mode: full · 输出语言：中文

## 1. 目标（一句话）

修复 frps 运行态代理端点的 path 注入缺口，并停止 3 处后端 500 兜底分支向前端透传内部错误细节。

## 2. 范围内行为（可测试，无模糊词）

### A. Path 注入防御

A-1. `GET /api/v1/server/runtime/proxy/{type}/{name}`（`serverRuntimeProxyDetail`）收到不在 `frpsProxyTypes` 白名单（`tcp/udp/http/https/stcp/sudp/xtcp`）内的 `{type}` 时，返回 HTTP 422、错误码 `VALIDATION_FAILED`、`field="type"`、固定中文文案（含"代理类型"语义），且**不**调用上游 frps。

A-2. `internal/frpsadmin/client.go` 的 `Proxies`、`ProxyDetail`、`Traffic` 三个方法在拼接上游 path 时，对 `type`、`name` 这两类来自外部输入的 path segment 用 `net/url.PathEscape` 编码后再拼接。编码后：含 `/`、`?`、`#`、`..`、空格、`%` 等特殊字符的 segment 不改变上游请求的 path/query 语义边界。

A-3. A-2 的 escape 行为对"普通无特殊字符"的 `type`/`name`（如 `tcp`、`ssh`）输出与改动前**字节一致**的 path（即 `/api/proxy/tcp/ssh` 不变），不引入回归。

### B. 内部错误细节不外泄

B-1. `procStop`（`handlers_proc.go:41`）的 500 兜底分支返回固定中文文案（语义为"停止进程失败"），message **不含** `err.Error()` 注入的内部细节子串。原始 error 通过 `h.deps.Logger`（若非 nil）以 Warn/Error 级别记录。

B-2. `mapProxyWriteError`（`handlers_proxies.go:260`）的兜底 500 分支返回固定中文文案"保存失败"（去掉 `": "+msg` 拼接），message **不含**裸 SQL/驱动错误子串。原始 error 通过 `h.deps.Logger`（若非 nil）记录。L241（`ErrDuplicateName` 409）、L248-255（`(type,remote_port)` 冲突 422）、L256-259（validation 透传）**保持不变**。

B-3. `downloadBin`（`handlers_system.go:140`）的 default 500 分支返回固定中文文案（语义为"启动下载失败"），message **不含** `err.Error()` 注入的内部细节子串。原始 error 通过 `h.deps.Logger`（若非 nil）记录。

## 3. 范围外（本迭代明确不做）

- O-1. `serverRuntimeTraffic` / `serverRuntimeProxyDetail` 的 `{name}` segment 业务层白名单/格式校验（A 的 client 层 escape 已是根防御；handler 层 name 校验无现成白名单可用，引入会扩散）。
- O-2. `(type,remote_port)` 冲突 422 分支（handlers_proxies.go:248-255）的 sentinel 化（记为 backlog，本任务不做，避免 scope drift）。
- O-3. `uploadBin`（handlers_system.go:586）的 errno 透传——B-A.12 / R-7 有意决策，**不碰**。
- O-4. `serverRuntimeProxies`（聚合端点）的 type 校验——其 type 来自后端 hardcode 的 `frpsProxyTypes` 循环，非外部输入，无注入面。
- O-5. 其余 handler 的 500 兜底分支审计（仅限上下文点名的 3 处）。
- O-6. 新增日志依赖或日志框架——只用现成 `h.deps.Logger`。

## 4. 边界条件

- BC-1. `{type}` 为空字符串：chi 路由 `{type}` 段不可为空（路由不匹配），但若到达 handler，空串不在白名单 → 走 A-1 的 422。
- BC-2. `{type}` 大小写（如 `TCP`）：白名单是小写字面，大写不命中 → 422（与上游 frps 大小写敏感一致，不做大小写归一化）。
- BC-3. `{name}` 含 `/`：chi 默认不跨 `/` 匹配单段；若前端 URL-encode 了 `/`（`%2F`），chi 解码后传入含 `/` 的 name → client 层 `url.PathEscape` 重新编码为 `%2F`，上游收到单段 name。
- BC-4. `{name}` 含 `..`、`?`、`#`：client 层 `url.PathEscape` 编码（`..` 不被 PathEscape 改写，但因整体作为单 segment 编码后不含路径分隔符，不构成目录穿越；`?`/`#` 被编码为 `%3F`/`%23` 不再切断 query/fragment）。
- BC-5. `h.deps.Logger == nil`：B-1/B-2/B-3 的日志记录跳过（nil 守卫，与 handlers_proxies.go:147 现有模式一致），固定文案照常返回。
- BC-6. 上游 frps 对编码后的非法 name 返回 404：handler 走现有 `writeFrpsadminError` 的 404 映射（`ErrNotFound` → 404），不变。

## 5. 验收标准（可验证）

- AC-1.（A-1）单测：`serverRuntimeProxyDetail` 传非白名单 `type`（如 `evil`）→ 响应 422 + `code=VALIDATION_FAILED` + `field=type`，且 mock frps server **未收到任何请求**。
- AC-2.（A-2/A-3）frpsadmin client 层单测：`ProxyDetail(ctx, "tcp", "a/b?c")`、`Traffic(ctx, "x y#z")` 时，httptest mock server 收到的 `r.URL.EscapedPath()` 中特殊字符被正确编码（`/`→`%2F`、`?`→`%3F`、`#`→`%23`、空格→`%20`）；`ProxyDetail(ctx, "tcp", "ssh")` 收到的 path 仍是 `/api/proxy/tcp/ssh`（无回归）。
- AC-3.（B-1）单测：构造 `procStop` 500 兜底返回固定文案、不含注入的内部错误子串。
- AC-4.（B-3）单测：构造 `downloadBin` default 500 返回固定文案、不含注入的内部错误子串。
- AC-5.（B-2）单测：`mapProxyWriteError` 兜底 500 返回 message 严格等于"保存失败"（不含 `: ` 后缀的 SQL 子串）。
- AC-6. `scripts/verify_all`（全量含 e2e）PASS；测试数只升不降（baseline.json `go_tests`/`test_count` 同步 bump）。
- AC-7. 06_TEST_REPORT.md 含裸 `## Adversarial tests` 段。

## 6. 非功能需求

- NFR-1.（安全）path 注入防御是纵深防御：handler 层 type 白名单 + client 层 escape 双层，即使 handler 漏校验，client 层仍安全（A-2 是根防御位置）。
- NFR-2.（信息安全）前端可见 message 不含 SQL 约束文本 / 驱动细节 / 文件系统路径等内部信息，降低信息泄露面。
- NFR-3.（一致性）3 处兜底分支与项目"固定友好中文 + 内部细节进日志"的既定 handler 模式对齐。
- NFR-4.（可诊断性）原始 error 不丢——进 logger，配合 middleware RequestID 可关联定位。

## 7. 关联任务

- **T-039 frpsadmin-server-runtime-api**（`docs/features/_archived/frpsadmin-server-runtime-api/`）：本任务改的两个文件由 T-039 引入；`frpsProxyTypes` 白名单（handlers_server_runtime.go:46）、envelope unwrap（insight L31）均来自 T-039。
- **T-027 download-cancel-and-upload-decouple**：uploadBin errno 透传决策（O-3）来自 T-027。
- **T-007 hardening-pass-audit**：mapProxyWriteError 的 sentinel 分类（B-2 保留部分）来自 T-007。

## 8. 待澄清问题

无。技术上下文由 orchestrator 已逐处核实（精确文件/行号/符号名/方法签名），范围边界明确，无歧义。

## 9. 裁决

**READY**
