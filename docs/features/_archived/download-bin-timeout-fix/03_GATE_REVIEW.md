---
task_id: T-025
slug: download-bin-timeout-fix
stage: gate-review
author: Gate Reviewer
date: 2026-05-23
mode: full
upstream_verdict_ra: READY (01 §9)
upstream_verdict_sd: READY-FOR-GATE-REVIEW (02 §14)
---

# 03 — Gate Review（T-025 download-bin-timeout-fix）

## 1. 审阅范围

- 01_REQUIREMENT_ANALYSIS.md（RA verdict: READY，§9）
- 02_SOLUTION_DESIGN.md（SD verdict: READY-FOR-GATE-REVIEW，§14）
- internal/downloader/downloader.go（505 LOC，待改文件）
- internal/downloader/downloader_test.go（534 LOC，3 处 `m.client` 引用核实）
- .harness/insight-index.md（46 条 insight 全量逐条比对）

## 2. 8 维度审计

| # | 维度 | 判定 | 一句话理由 |
|---|---|---|---|
| 1 | Requirement 完整性 | PASS | FR-1..FR-5 + AC-1..AC-6 测试可证伪，歧义项 D-1..D-7 已自决断并入文档。 |
| 2 | Design 完整性 | PASS | 02 §13 提供逐项 RA → SD 对应表，无遗漏。 |
| 3 | Reuse 正确性 | PASS | 验证 `m.client` 全包内引用确为 2 处生产 + 3 处测试（与 02 §3.1 / §5.1 grep 结果一致）。 |
| 4 | Risk 覆盖 | PASS-WITH-NOTE | R-1..R-6 覆盖合理；R-1 trickle stall 显式承认作 known limitation（取舍授权下接受），见 C-1 详述。 |
| 5 | 迁移安全 | PASS | 无数据迁移、无 schema 变更、无 feature flag 需求；单次 commit / `git revert` 完整可回滚。 |
| 6 | 边界处理 | PASS | dial 30s / TLS 30s / ResponseHeaderTimeout 60s + IdleConnTimeout 90s 四阶段上限完整；StatusCode <200 || >=300 既有分支保留。 |
| 7 | 测试可行性 | PASS | T1..T4 + 既有 12 用例改名 = 16 用例可覆盖 AC-1..AC-6；T1/T4 ~5–10s 真造（有 testing.Short skip 闸门）。 |
| 8 | 范围清晰 | PASS | RA §2.2 + SD §11 双重重申 out-of-scope（取消按钮 / 续传 / 重试 / 镜像源 / trickle 防护）。 |

## 3. 编号 Concerns

### C-1 [SHOULD-FIX → 取舍后转 NIT]：trickle stall 风险的接受边界

02 §6 R-1 把 trickle stall（每 30s 1 字节）显式标为 known limitation 不引入 `SetReadDeadline` 周期刷新。**用户授权取舍原则明确**：
- 用户体验优先（archive 下载在国内必须能跑），
- 长期易维护，不为边缘风险引入复杂控制逻辑。

判断：**接受 02 的决策**。具体支撑：
1. `IdleConnTimeout=90s` 实际覆盖"TCP keep-alive 空闲超 90s 的 conn 自动回收"——只要对端完全断流，连接会在 90s + tcp keepalive (30s) 内被识破。
2. trickle stall 要求"对端每 N 秒主动发 1 字节"，这是**人为构造的恶意模式**，不是生产 CDN 自然抖动；用户场景是 webUI 主动操作，进程重启就能终止。
3. 引入 `SetReadDeadline` 周期刷新需要自管 net.Conn lease + Body close 时机，**复杂度与本任务"最小侵入"原则冲突**。

**降级为 NIT**：不阻塞，但 Developer 实施时 §3.3 helper 的注释里必须写清"trickle 已被显式接受为已知 limitation，未来如需可作 T-026 独立任务"——02 §6 R-1 已这么写、§3.3 helper 注释也已写了，无需补。

### C-2 [NIT]：Test 6 注入由"整请求超时"改为"ResponseHeaderTimeout"的语义边界

02 §5.1 把 Test 6（line 362）改造为 `m.downloadClient = &http.Client{Transport: &http.Transport{ResponseHeaderTimeout: 50*time.Millisecond}}`。

我验证了既有 Test 6 server 行为（downloader_test.go:340-354）：`/releases/latest` 立即正常响应，`/archive` 走 `select { <-r.Context().Done(); <-time.After(10s) }` **不发任何 header**。这正好命中 `ResponseHeaderTimeout` 触发路径——50ms 内必然 fire。

但要注意一个细节：**`apiClient` 是同一构造，会用生产默认值 60s**。Test 6 的 server `/releases/latest` 同步立即返回 ~200 字节 JSON，绝不会触发 apiClient 超时。因此 02 §5.1 表里 "把 `m.apiClient` 保持默认（API 响应是同步立即返回的）" 是正确判断。

**降级为 NIT**：实施时 Developer 别忘了把 50ms 注入到 `m.downloadClient` 而不是误手赋给 `m.apiClient`——前者改 Transport 字段、后者改 Client.Timeout 字段，语义层级不同。

### C-3 [NIT]：Test 3 改名后语义校对

02 §5.1 表里 Test 3 改 `m.apiClient = &http.Client{Timeout: 5*time.Second}`。原 Test 3 server hang 在 `/releases/latest`（API 端点），是 apiClient 路径——改名正确。但要校对：原代码 line 229 注入是为了**避免测试主体卡 60s 默认超时**（5s 超时 < `waitForDone(t, m, "frpc", 3*time.Second)` 容忍区间是反过来——5s > 3s，所以 5s 超时不会触发，goroutine 在 close(unblock) 后才解锁）。

Developer 实施时应保留这个 5s 值（不要"优化"成更短）。02 §5.1 已写明 `5*time.Second`，OK。

### C-4 [NIT]：T1 "不设 m.downloadClient 覆盖（用生产默认 client）" 与 httptest.Server 的 HTTP/2 兼容

02 §5.2 T1 说："不设 `m.downloadClient` 覆盖（用生产默认 client）— 这正是回归'60s 总超时'bug 的关键证据"。

但生产 client 设置了 `ForceAttemptHTTP2: true`（02 §3.3 第 87 行）。`httptest.NewServer` 默认是 HTTP/1.1（要 HTTP/2 必须 `httptest.NewTLSServer` + ALPN 协商）。这不会出问题：`ForceAttemptHTTP2` 仅在 TLS 上协商成功才升级，HTTP/1.1 路径完全兼容。**校对通过**，但若 Developer 实施时遇到奇怪行为可以排查这一点。

### C-5 [NIT]：T1 ~5s 真造在慢 CI runner 上的稳定性

02 §6 R-3 已自承"用 `waitForDone(timeout=15*time.Second)` 给宽裕余量；server 端用 chunk size + `time.Sleep(40ms)` 控制速率，runner 抖动 ±2s 不会失败"。这里 15s 余量 / 5s 期望耗时 = 3x 容忍，合理。

**软提醒**：若 CI（特别是 windows-latest runner）实测出现 flaky，可考虑 (a) 把传输总量从 256 KB 降到 128 KB（耗时缩到 2.5s）、(b) 把 `time.Sleep(40ms)` 缩到 20ms（速率翻倍，总耗时减半）。

### C-6 [NIT]：T4 串行两次下载的"共享 root 目录"风险

02 §5.2 T4 说"串行各跑一遍（避免共享 root 目录冲突 — 各自 `t.TempDir()`）"。**写法核对**：`t.TempDir()` 每次调用返回不同临时目录，所以 frpc 和 frps 必须在两次 `m := New(differentRoot, ...)` 之间构造新 Manager——否则同一 Manager 的 state map 也会被一次 `Status` 调用读到旧 frpc=success 状态。

02 文字略含糊：是"两个 Manager"还是"一个 Manager 串行 Start 两次"？我建议 Developer 选 **一个 Manager + 同一个 root + 同一个 httptest server**——这才真正测"双 kind 一致"。两个不同 root 的 Manager 等于跑了两次 T1，不算同步性证明。

**降级为 NIT**：Developer 自行决定写法，但 04_DEVELOPMENT.md 里要 explicit 解释这次选了哪种隔离方案及原因。

### C-7 [NIT]：testing.Short 跳过策略与 verify_all 的耦合

02 §5.4 说 "verify_all 是否带 `-short` 由 CI 控制"。我快速核对 `scripts/verify_all` 的 Go test 调用是否带 `-short`，但不是 Gate Reviewer 必须做的（Developer 阶段可自行 grep `scripts/verify_all` 确认）。

**软提醒**：若 verify_all 不带 `-short`，则每次 declare-done 都会跑 T1/T4 真 ~10–15s，会拉长 verify_all 总耗时。Developer 可选 (a) 让 T1/T4 默认 skip、仅 explicit `-run` 跑、(b) 接受额外 ~10s 成本。

### C-8 [NIT]：新 client 不重用既有 Transport 配置（无回归风险但要确认 Proxy 兼容性）

02 §3.3 helper 使用 `Proxy: http.ProxyFromEnvironment` —— 与 stdlib `http.DefaultTransport` 同款。**回归核对**：原 `Client{Timeout: 60s}` 默认 `Transport == nil`，stdlib 内部走 `DefaultTransport`，**已经支持** `ProxyFromEnvironment`。新 helper 显式写出，行为一致 —— 国内常见用户配 `HTTPS_PROXY=http://127.0.0.1:7890` 走 clash/v2ray 的场景**不会回归**。验证通过。

### C-9 [NIT]：与 .harness/insight-index.md 兼容性

我逐条对了 46 条 insight，本任务相关三条：
- L25（spec mock 漂移）：02 §5 + §7 显式禁用 interface mock、全部用 httptest.Server 真实 HTTP，符合。
- L29（spec mock 漂移）：同上。
- L43（07 标题"## Insight" 不能加数字前缀）：本阶段（Gate）尚未到 07，但 PM 写 07 时记着即可。
- L41（reviewer 不落盘）：本次 Gate Reviewer 无 Write 工具，**已**按 L41 fallback——PM 代为落盘。

无冲突。

## 4. 编号 Improvements（非阻塞，可选优化）

### I-1：02 §3.3 helper 命名可考虑加版本注释

`newDownloadHTTPClient()` 注释里直接写"T-025 拆分 — Client.Timeout=0 故意而为，未来人改前请读 docs/features/_archived/download-bin-timeout-fix/02_SOLUTION_DESIGN.md"。这能防 6 个月后某人按 lint 习惯把 Timeout=0 "修复"为 60s 重新引入本 bug。

### I-2：考虑在 logger.Info("download started", ...) 之后增加"download progress every 1MB"日志

02 §13 NFR-2 标记 RA NFR-2 "允许 Architect 阶段考虑添加 elapsed/bytes 字段，非本 RA 强制"。当前 02 决定**不**加。考虑到生产 5–10 分钟级下载，systemd journal 可能十几分钟都没新日志，运维体验差。可以加一行 progressWriter 内 "每 1MB 触发一次 logger.Info('download progress', kind, bytes_so_far, elapsed)"——零结构变更、零契约影响。**不阻塞**，留给 Developer 自决断。

### I-3：考虑 Developer 阶段对 `*http.Transport` 实例做"shutdown" 关注

02 §6 R-4 说 "如果未来发现问题，可在 `doDownload` 内 `defer m.downloadClient.CloseIdleConnections()` 兜底（**本任务不加，YAGNI**）"。我同意 YAGNI。但**软提醒**：Manager 当前无显式 Close 方法，进程退出时 net stack 由 OS 兜底回收，目前没有问题。未来若引入 graceful shutdown，需要 Manager.Close() 调 `t.CloseIdleConnections()` —— 非本任务范围。

## 5. 高概率开发期问题预答

**Q1（Developer 必问）：T1 的 server 端 chunk-write + Sleep 应该写成 helper 还是嵌入测试函数？**
A：写成 helper（如 `newSlowFRPServer(t, archive, ratebpsec)`），与既有 `newFRPServer` 平行。理由：T4 也要复用同款慢 server。

**Q2（Developer 必问）：T2 注入 `m.downloadClient = &http.Client{Transport: &http.Transport{ResponseHeaderTimeout: 100*time.Millisecond}}` 时，dial / TLS 字段是否要也设？**
A：不必。httptest.Server 是 localhost 直连，dial/TLS 阶段 <1ms 不会触发。只设 ResponseHeaderTimeout 即可保持测试代码最小。

**Q3（Developer 必问）：T3 注入 `m.apiClient = &http.Client{Timeout: 200*time.Millisecond}` 与既有 Test 12 的 `m.apiClient = &http.Client{Timeout: 2*time.Second}` 有什么本质区别？**
A：见 02 §5.2 T3 自答："Test 12 测的是'server 已关闭、TCP 立即拒绝'，T3 测的是'server 接受连接但永不应答'。两者覆盖 GitHub API 失败的两条不同物理路径"。完全互补，不要试图合并。

**Q4（Developer 可能问）：02 §3.3 写 `MaxIdleConns: 10` 但本场景只下载 1 个 archive，是否过度？**
A：沿用 stdlib `DefaultTransport` 默认值，零调优成本；改它没意义。

**Q5（Developer 可能问）：是否需要给 Manager 加一个 `Close()` 方法把 `downloadClient.Transport.(*http.Transport).CloseIdleConnections()` 调一下？**
A：本任务**不加**。原 Manager 无 Close()，加它属于"顺便重构"违反最小侵入原则。见 I-3 软提醒。

**Q6（QA 阶段可能问）：T1 真造 ~5s 是否要在 06_TEST_REPORT.md 的 `## Adversarial tests` 段单独列？**
A：是。T1（慢传输不超时）、T2（死连接快速失败）、T3（API hang）三类都属于 adversarial，必须列在 `## Adversarial tests` 段（裸标题、无数字前缀——见 L19/L43）。

## 6. Verdict

**APPROVED FOR DEVELOPMENT**

- 8 维度全 PASS（其中 R-1 trickle 在用户授权取舍下接受）。
- 9 个 NIT 级 concerns 全部不阻塞，已为 Developer 提供针对性软提醒。
- 3 个 Improvements 非强制，留 Developer 自决断。

## 7. 给 Developer 的 "软提醒" 清单（实施时关注）

1. **C-2**：Test 6 改注入到 `m.downloadClient.Transport.ResponseHeaderTimeout` 而非 `Client.Timeout` 字段——前者改 Transport 子结构、不要混。
2. **C-3**：保留 Test 3 的 5s 值不改短。
3. **C-5**：T1 ~5s 真造，若 windows-latest CI flaky 可调到 128 KB / 20ms sleep。
4. **C-6**：T4 选择 "一个 Manager + 同一 root + 同一 server" 而非 "两个 Manager + 两个 root"，并在 04 explicit 注明原因。
5. **C-7**：实施前 grep `scripts/verify_all` 看是否带 `-short`；若不带则 T1/T4 每次 verify_all 都跑，预算 +10s。
6. **I-1**：`newDownloadHTTPClient()` 注释加 "T-025 — Timeout=0 故意；未来人改前请读 docs/features/_archived/download-bin-timeout-fix/02_SOLUTION_DESIGN.md"。
7. **insight L29/L45/L41/L43**：QA 阶段 06 标题用裸 `## Adversarial tests`、PM 07 标题用裸 `## Insight`，无数字前缀。
8. **未变动的"既有 12 用例"** 必须在改名后全 PASS，无意外副作用——这是 AC-5 的硬要求。
