---
task_id: T-025
slug: download-bin-timeout-fix
stage: qa
author: QA Tester
date: 2026-05-23
mode: full
upstream_verdict_ra: READY (01 §9)
upstream_verdict_sd: READY-FOR-GATE-REVIEW (02 §14)
upstream_verdict_gate: APPROVED FOR DEVELOPMENT (03 §6)
upstream_verdict_dev: READY FOR REVIEW (04 §9)
upstream_verdict_review: APPROVED (05 §8)
---

# 06 — 测试报告（T-025 download-bin-timeout-fix）

## 1. 测试范围

- **生产代码**：`internal/downloader/downloader.go`（Manager 双 client 拆分 + `newDownloadHTTPClient` helper）
- **测试代码**：
  - `internal/downloader/downloader_test.go`（3 处既有用例改名为 `apiClient` / `downloadClient` 注入）
  - `internal/downloader/downloader_timeout_test.go`（新建，4 个 adversarial 用例 + 1 个 P1-2 静态守门）
- **AC 矩阵**：AC-1..AC-6（详 §2）
- **关卡**：`scripts/verify_all.sh` PASS=20（§4）
- **不在本轮验证范围**：生产 systemd 部署 e2e（GitHub release 真实 ~14 MB 下载），由用户在真环境完成 — 见 §6 limitation。

## 2. AC 覆盖矩阵

| AC | 描述 | 测试用例（文件:函数） | 实测结果 |
|---|---|---|---|
| AC-1 | 慢源 ≥10 分钟不被超时切断 | `downloader_timeout_test.go:TestTimeout_SlowDownload_Succeeds`（T1） + `downloader_timeout_test.go:TestNewDownloadHTTPClient_NoTotalTimeout`（P1-2 静态守门） | T1 真造 3.813s 慢传输完整跑通；P1-2 断言 `downloadClient.Timeout == 0` —— 联合证明"60s 总超时不再生效" |
| AC-2 | 死连接快速失败 | `downloader_timeout_test.go:TestTimeout_DeadConnection_ResponseHeaderTimeout`（T2） + `downloader_test.go:TestDownload_NetworkTimeout_StatusFailed`（既有 Test 6，注入路径已改 `downloadClient.Transport.ResponseHeaderTimeout`） | T2: 104ms 内 StatusFailed + "下载超时" 前缀；Test 6: 60ms 内 StatusFailed |
| AC-3 | GitHub API 短超时保留 | `downloader_timeout_test.go:TestTimeout_GitHubAPIHang_ApiClientTimeout`（T3，hang 路径） + `downloader_test.go:TestResolveLatest_NetworkFailure`（既有 Test 12，TCP 拒绝路径） | T3: 207ms 内 StatusFailed + "无法访问 GitHub" 前缀；Test 12: 10ms 内 StatusFailed |
| AC-4 | frpc/frps 双 kind 行为一致 | `downloader_timeout_test.go:TestTimeout_BothKinds_Symmetric`（T4，同一 Manager + 同一 root + 同一 server 串行各跑一遍） | T4: 21ms 内两个 kind 均 StatusSuccess + Progress=100 + 二进制落盘成功 |
| AC-5 | 无回归 | 整个 `internal/downloader/...` 包 28 个用例 + verify_all 全 20 项 | go test 28/28 PASS（5.197s 总耗时）；verify_all PASS=20 / FAIL=0 / Delta=0 |
| AC-6 | progress 持续推进 | `downloader_timeout_test.go:TestTimeout_SlowDownload_Succeeds`（T1 在 15s 内每 100ms 抓 progress 快照） | T1 实测 `sawMidProgress=true`，即慢传输期间至少抓到一次 `0 < progress < 100` |

每条 AC 至少有一个测试用例显式覆盖；AC-1 与 AC-2、AC-3 各有两条互补测试（动态行为 + 静态守门 / hang 路径 + TCP 拒绝路径）。

## 3. 完整测试结果（`go test -v ./internal/downloader/...`）

总用例数：**28**（PASS）/ 0（FAIL）/ 0（SKIP）；包总耗时 **5.197s**。

| # | 用例 | 耗时 | 类别 |
|---|---|---|---|
| 1 | `TestAdversarial_ResolveLatest_MalformedJSON` | 0.01s | 既有 |
| 2 | `TestAdversarial_ResolveLatest_MissingTagName` | 0.01s | 既有 |
| 3 | `TestAdversarial_ResolveLatest_HTTP500` | 0.01s | 既有 |
| 4 | `TestDownload_TarGz_Success_Progress` | 0.01s | 既有（共享 client 拆分后未受影响） |
| 5 | `TestDownload_Zip_Success` | 0.01s | 既有 |
| 6 | `TestDownload_ErrAlreadyInProgress` | 0.01s | 改名（注入 `apiClient`，C-3 保留 5s 值） |
| 7 | `TestDownload_HTTP404_StatusFailed` | 0.01s | 既有 |
| 8 | `TestDownload_ZipSlip_MaliciousEntryFiltered` | 0.01s | 既有（安全回归） |
| 9 | `TestDownload_NetworkTimeout_StatusFailed` | 0.06s | 改造（C-2 注入 `downloadClient.Transport.ResponseHeaderTimeout=50ms`） |
| 10 | `TestDownload_BadKind` | 0.00s | 既有 |
| 11 | `TestParseIPFromJSON` | 0.00s | 既有 |
| 12 | `TestResolveLatest_Success` | 0.02s | 既有 |
| 13 | `TestResolveLatest_RateLimited403` | 0.01s | 既有 |
| 14 | `TestResolveLatest_AssetNotMatched` | 0.01s | 既有 |
| 15 | `TestResolveLatest_NetworkFailure` | 0.01s | 改名（注入 `apiClient`，Test 12 与 T3 互补） |
| 16 | `TestTimeout_SlowDownload_Succeeds`（T1） | **3.82s** | 新增 adversarial（AC-1 + AC-6） |
| 17 | `TestTimeout_DeadConnection_ResponseHeaderTimeout`（T2） | 0.11s | 新增 adversarial（AC-2） |
| 18 | `TestTimeout_GitHubAPIHang_ApiClientTimeout`（T3） | 0.21s | 新增 adversarial（AC-3） |
| 19 | `TestTimeout_BothKinds_Symmetric`（T4） | 0.02s | 新增 adversarial（AC-4） |
| 20 | `TestNewDownloadHTTPClient_NoTotalTimeout`（P1-2） | 0.00s | 新增静态守门（AC-1 反向证伪） |
| 21 | `TestInstall_HappyPath` | 0.00s | 既有 |
| 22 | `TestInstall_TooLarge` | 0.00s | 既有 |
| 23 | `TestInstall_AtMaxBytes` | 0.01s | 既有 |
| 24 | `TestInstall_MaxBytesUnlimited` | 0.00s | 既有 |
| 25 | `TestInstall_BadKind` | 0.00s | 既有 |
| 26 | `TestInstall_UnsupportedGOOS` | 0.00s | 既有 |
| 27 | `TestInstall_WindowsFallback_OverwriteExisting` | 0.01s | 既有（T-019 引入） |
| 28 | `TestInstall_ReaderError` | 0.00s | 既有 |

T1 内部 log（QA 复跑实测）：

```
downloader_timeout_test.go:98: T1 archive size after gzip: 196389 bytes
downloader_timeout_test.go:136: T1 elapsed: 3.813s, sawMidProgress=true
```

`sawMidProgress=true` 直接证伪"progress 卡死或一步到 100"，是 AC-6 的动态证据。

## 4. verify_all 结果

执行：`bash C:/Programs/frp_easy/scripts/verify_all.sh`（POSIX 路径，等价于 PowerShell 版 `verify_all.ps1`，二者校验项一致）。

```
=== Summary ===
  PASS: 20
  WARN: 0
  FAIL: 0
  SKIP: 0
```

| 项 | 实测 | 基线 | Delta |
|---|---|---|---|
| PASS | 20 | 20 | 0 |
| WARN | 0 | 0 | 0 |
| FAIL | 0 | 0 | 0 |
| SKIP | 0 | 0 | 0 |
| `[E.6] Adversarial tests section in completed task reports` | PASS | — | 闸门通过 |

逐项 PASS 摘录：A.1/A.2/A.3（secrets/.env/TODO 预算）、G.1/G.2/G.3（go vet/test/build）、B.1..B.5（前端 typecheck/lint/test/baseline/无 tsc 残留）、C.1（playwright e2e smoke）、D.1（OpenAPI schema）、E.1..E.7（CLAUDE.md / workflow / 7 agents / binding / AI-GUIDE / **Adversarial tests 段** / .ps1 BOM）。

特别注意：**E.6 闸门 PASS** —— verify_all 已扫描本文件 `## Adversarial tests` 段并通过；标题确认为裸标题、无数字前缀（见 §5）。

## Adversarial tests

对抗测试段（裸标题，无数字前缀；遵循 insight L19 / L30 / L43；本段是 verify_all E.6 硬要求闸门）。

每条 adversarial 用例：哪个 AC 被故意攻击 / 失败假设 / 实测结果。

| ID | 关联 AC | 失败假设（"我期待 fail 当…"） | 攻击场景 / 边界 | 独立复现命令 | 实测结果 |
|---|---|---|---|---|---|
| T1 `TestTimeout_SlowDownload_Succeeds` | AC-1 + AC-6 | 若仍有 60s 总超时，14 MB 在国内 50 KB/s 必死 → 用 ~50 KB/s 真造 256 KB 慢传输证伪 | httptest server 以 chunkSize=4096 + sleep=80ms 节奏分块写 gzip 后 196 KB archive；用伪随机不可压缩内容（`incompressibleBlob` seed=42）让 gzip 压缩比 ~1.31:1 保持真传输负载 | `go test -v -run TestTimeout_SlowDownload_Succeeds ./internal/downloader/` | **PASS 3.813s**；`sawMidProgress=true`；二进制落盘 `<tmp>/frp_linux/frpc`。注：此用例**真造 ~3.8s** 证明"链路通畅性 + AC-6 推进"，不能反向证伪 60s 回归（3.8s < 60s 仍会 PASS）—— 后者由 P1-2 静态守门补全。 |
| T2 `TestTimeout_DeadConnection_ResponseHeaderTimeout` | AC-2 | 死服务器（accept TCP 但永不写 header）若无 ResponseHeaderTimeout 兜底，下载会无限挂 | httptest handler `/archive` 进入 `<-r.Context().Done()` 永不 `WriteHeader`；注入 `downloadClient.Transport.ResponseHeaderTimeout=100ms`（Gate C-2：改 Transport 子字段而非 Client.Timeout） | `go test -v -run TestTimeout_DeadConnection_ResponseHeaderTimeout ./internal/downloader/` | **PASS 104ms**；StatusFailed + Error 含 "下载超时" 中文前缀。 |
| T3 `TestTimeout_GitHubAPIHang_ApiClientTimeout` | AC-3 | API server hang（接受连接但永不应答）若 apiClient 也丢了超时，整 download 链路会卡死 | server 所有路径都 `<-r.Context().Done()`；注入 `apiClient.Timeout=200ms`；与既有 Test 12（TCP 立即拒绝）互补 —— 覆盖"hang"与"拒绝"两条物理失败路径 | `go test -v -run TestTimeout_GitHubAPIHang_ApiClientTimeout ./internal/downloader/` | **PASS 207ms**；StatusFailed + Error 含 "无法访问 GitHub" 中文前缀。 |
| T4 `TestTimeout_BothKinds_Symmetric` | AC-4 | 若拆分后某个 client 实例只被 frpc 路径调用、frps 走错分支 → 串行各跑一次同 Manager 同 root 同 server 必能暴露 | 单 archive 同时含 `frp_<v>_linux_amd64/frpc` 和 `frps` 两个 entry（`extractFromTarGz` 按 `filepath.Base` 各取所需）；用快传输（chunkSize=64KiB, sleep=0）让单次 ~10ms 量级，证明对称性而不拉长 verify_all | `go test -v -run TestTimeout_BothKinds_Symmetric ./internal/downloader/` | **PASS 21ms**；frpc + frps 均 StatusSuccess + Progress=100 + 二进制落盘。T4 用快传输不直接证慢传输对称性 —— 由"共享 downloadClient 实例 + T1 已证慢传输不超时"联合推断 AC-4。 |
| P1-2 `TestNewDownloadHTTPClient_NoTotalTimeout` | AC-1（反向证伪 60s 回归） | 若未来人 lint 习惯把 `Timeout: 0` "修复"为 `60 * time.Second`，T1 仍能 PASS（3.8s < 60s）—— 必须有静态字段断言瞬间 FAIL | 直接调 `newDownloadHTTPClient()` 拿返回 `*http.Client`，断言 `c.Timeout == 0` —— Code Review §3 P1-2 显式强烈推荐的反向证伪盲区补丁 | `go test -v -run TestNewDownloadHTTPClient_NoTotalTimeout ./internal/downloader/` | **PASS 0.00s**；`c.Timeout == 0` 验证通过。 |

### AC-1 "60s 不再生效" 联合证据链

按 Code Review §3 P1-2 与 §9 给 PM 动作清单第 4 条，AC-1 "60s 总超时不再生效"由三方联合保证：

1. **字段值的静态证据**：`downloader.go:96-97` `Timeout: 0` —— Code Reviewer 已字面核验（05_CODE_REVIEW.md §2 对齐表）。
2. **静态守门**：`TestNewDownloadHTTPClient_NoTotalTimeout`（P1-2）直接断言 `newDownloadHTTPClient().Timeout == 0` —— 任何回归都会让此用例 FAIL。
3. **动态链路通畅性**：T1 在 ~3.8s 真造慢传输下走通完整 download → 解压 → install 链路 + AC-6 progress 推进。

三条互补；任一缺失都无法独立证 AC-1。本报告显式记录此结构以闭合 Code Review P1-2 的"反向证伪盲区"。

### 边界 / 隐藏假设清单（adversarial 心态：哪些场景没测、为何接受）

| 隐藏假设 | 是否测了 | 缓解 / 说明 |
|---|---|---|
| 真生产 GitHub release 14 MB 在国内 60s+ 真下载 | **未测** | 成本 / 网络不可控；由用户在 systemd 真部署后验。代码 path 与 T1 一致（共享 `downloadClient`），数学保证"无总超时"。 |
| Trickle stall（每 30s 1 字节）数小时不断流 | **未测** | RA §7 R-1 + 02 §6 R-1 + Gate C-1 显式接受为已知 limitation，作 future task。 |
| HTTP/2 over TLS 协商成功路径 | **未测** | `ForceAttemptHTTP2=true` 仅在 TLS + ALPN 协商时升级；httptest 默认 HTTP/1.1 不会触发，生产 GitHub CDN 走 HTTP/2 与 client 字段无关。 |
| 用户运行期改 `HTTPS_PROXY` env | **未测** | 与 stdlib `DefaultTransport` 同款行为（02 §6 R-5），既有 frpc/frps 也不响应运行期 env 变化。 |
| 进程退出时 `Transport` idle conn 显式 close | **未测** | 02 §6 R-4 YAGNI，OS 兜底回收。 |
| 大文件 + 磁盘满 | 既有 `TestInstall_TooLarge` / `TestInstall_AtMaxBytes` 覆盖 install 段；download 段读流到磁盘的"写满"错误走既有 `下载写入失败:` 分支 | 不在 T-025 范围 |

## 5. 稳定性（Stability）

- T1 / T2 / T3 / T4 / P1-2 全套连续跑两次，结果一致：T1 elapsed 3.813s / 3.816s（开发 04 §5.1 实测）；T2 102 / 104ms；T3 207ms；T4 21 / 31ms；P1-2 0.00s。
- T1 ~3.8s 是 chunkSize × sleep 节奏决定（49 chunks × 80ms ≈ 3.92s），不依赖系统抖动；`waitForDone` 给 15s 余量（02 §6 R-3 / Gate C-5 软提醒 OK）。
- 全 28 用例无 flaky；包总耗时 5.197s 稳定。

## 6. 已知 limitation

1. **真生产 systemd e2e 待用户验**：本次单测/集成测试不访问真 GitHub release。用户应在 Linux systemd 部署后从 webUI 点 "一键下载 frpc/frps" 验证生产 path 通畅 —— 这是 RA 写本任务的源头证据，最终 close-loop 由用户在生产网络（国内 GFW 抖动 50–200 KB/s）完成。
2. **Trickle stall 数小时挂起**：02 §6 R-1 + Gate C-1 接受为 known limitation。未来如需作 T-026 引入 `SetReadDeadline` 周期刷新 + 自管 conn lease 解决。
3. **前端"取消下载"按钮**：out of scope（RA §2.2 / D-5）。
4. **断点续传 / 自动重试 / 镜像源切换**：out of scope（RA §2.2）。
5. **`TestTimeout_BothKinds_Symmetric` 用快传输**：AC-4 在"慢传输 + 双 kind"组合下无独立 ~5s 真造证据（避免 verify_all 拉长 +10s）—— 由"共享 downloadClient 实例 + T1 已证慢传输 OK + T4 已证 frpc/frps 各跑通"联合推断。Code Review P1-4 已 NIT 标记接受。

## 7. Verdict

**PASS — APPROVED FOR DELIVERY**

- AC-1..AC-6 全部覆盖（AC-1 由 T1 动态 + P1-2 静态联合；AC-4 由 T4 + 共享 client 派生）
- `go test ./internal/downloader/...` **28 / 28 PASS**（5.197s）
- `verify_all` PASS=20 / WARN=0 / FAIL=0 / SKIP=0 —— **零回归**，与基线 (`scripts/baseline.json` `test_count=337`) 一致；本任务新增 5 个 Go 用例（T1/T2/T3/T4 + P1-2），下个 baseline 更新点应将 `go_tests` 由 241 上调至 246
- `## Adversarial tests` 段为裸标题（无数字前缀），E.6 闸门 PASS
- Code Review §9 给 PM 的动作清单 1–4 条全部落实（P1-2 加测、QA 报告显式说明 60s 回归证据链、裸标题）
- 已知 limitation 显式记入 §6；生产 systemd e2e 由用户在真环境完成最后一公里

无 BLOCKER / CRITICAL / MAJOR 缺陷；任务可进入 PM 归档阶段（Stage 7）。
