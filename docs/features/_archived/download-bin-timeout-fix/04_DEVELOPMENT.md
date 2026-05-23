---
task_id: T-025
slug: download-bin-timeout-fix
stage: development
author: Developer
date: 2026-05-23
mode: full
upstream_verdict_ra: READY (01 §9)
upstream_verdict_sd: READY-FOR-GATE-REVIEW (02 §14)
upstream_verdict_gate: APPROVED FOR DEVELOPMENT (03 §6)
---

# 04 — 开发记录（T-025 download-bin-timeout-fix）

## 1. 摘要

按 02 §3.3 / §4 / §5 实施。把 `internal/downloader.Manager` 内单个共享 `*http.Client{Timeout:60s}` 拆为 `apiClient`（保留 60s 总超时，GitHub API 用）+ `downloadClient`（无 `Client.Timeout`，仅 Transport 阶段性上限）。新增 helper `newDownloadHTTPClient()`、4 个 adversarial 用例（T1/T2/T3/T4）、3 处既有测试改名。Manager 公开签名 / DownloadState JSON / HTTP API contract 全部零变更。

## 2. 实施步骤摘要

按 02 §5.1 + Gate §7 软提醒清单逐条落地：

| # | 改动 | 文件 | 行 |
|---|---|---|---|
| 1 | `import` 加 `"net"`（Dialer 需要） | `internal/downloader/downloader.go` | +1 |
| 2 | Manager 结构：移除 `client`，新增 `apiClient` + `downloadClient` 双字段 | `internal/downloader/downloader.go` | +6 / -1 |
| 3 | `New`：构造两个 client，`downloadClient` 走 `newDownloadHTTPClient()` | `internal/downloader/downloader.go` | +3 / -1 |
| 4 | 新增 helper `newDownloadHTTPClient()`，含 I-1 注释（指向 02 §6 R-1） | `internal/downloader/downloader.go` | +28 |
| 5 | `doDownload` archive 路径 `m.client.Do(req)` → `m.downloadClient.Do(req)` | `internal/downloader/downloader.go` | 1 行替换 |
| 6 | `resolveLatestAsset` API 路径 `m.client.Do(req)` → `m.apiClient.Do(req)` | `internal/downloader/downloader.go` | 1 行替换 |
| 7 | Test 3 (`TestDownload_ErrAlreadyInProgress`) 注入 `m.apiClient = &http.Client{Timeout: 5*time.Second}` | `internal/downloader/downloader_test.go` | 1 行改名 + 注释 |
| 8 | Test 6 (`TestDownload_NetworkTimeout_StatusFailed`) 注入 `m.downloadClient` 用 `Transport.ResponseHeaderTimeout=50ms`（非 Client.Timeout） | `internal/downloader/downloader_test.go` | +2 / -1 |
| 9 | Test 12 (`TestResolveLatest_NetworkFailure`) 注入 `m.apiClient = &http.Client{Timeout: 2*time.Second}` | `internal/downloader/downloader_test.go` | 1 行改名 + 注释 |
| 10 | 新建 `downloader_timeout_test.go`：4 用例 + `newSlowFRPServer` + `incompressibleBlob` helper | `internal/downloader/downloader_timeout_test.go` | +280 (新文件) |

## 3. 关键决策

### C-6（T4 隔离方案）：选 "一个 Manager + 同一 root + 同一 server" 而非 "两 Manager + 两 root"

按 Gate §3 C-6 建议直接选共享方案，理由（已写入 T4 用例顶部 docstring）：
1. 真正验证 "frpc 与 frps 共享同一对 client + 同一 state map 时双方均工作"——共享路径正是生产场景；
2. 两 Manager 等于跑 T1 两次，对 "双 kind 对称性" 无独立证据；
3. 单 archive 同时含 `frpc` 与 `frps` 两个 entry，`extractFromTarGz` 按 `filepath.Base` 匹配能正确各取所需——零额外 server 编排。

### T1 archive 大小校准：用伪随机不可压缩内容

首跑 T1 仅耗时 101 ms 而非预期 ~5s（见 §4 意外 #1）。根因：`strings.Repeat(...)` 这种高重复内容被 gzip 压成 KB 级。改用 `math/rand` 固定 seed=42 的 charset 伪随机字节填充 256 KB，gzip 后稳定 ~196 KB；以 chunkSize=4096 + sleep=80ms 节奏分块写 ~ 49 chunks × 80ms ≈ 3.92s。实测 3.82s（含 dial + 解压 + install 链路开销），落在预期窗口。

### C-7（verify_all 是否带 -short）

实际 grep `scripts/verify_all.ps1` 无 `-short` flag —— T1（~3.8s）+ T4（~30ms，因走快传输）总额外成本 ~4s，可接受。**不**让 T1/T4 默认 skip；只用 `testing.Short()` 守护让 `-short` 模式可跳过。

### I-2（progress 日志）：按 Architect 原决策**不加**

Gate I-2 是软建议，本任务范围严格限制为 timeout 修复。progress 日志若需要单独立 T-026，避免范围蔓延。

### 其它软提醒落实情况

| Gate §7 软提醒 | 落实方式 |
|---|---|
| C-2 Test 6 注入 ResponseHeaderTimeout 而非 Client.Timeout | downloader_test.go line ~362 改用 `m.downloadClient = &http.Client{Transport: &http.Transport{ResponseHeaderTimeout: 50ms}}` |
| C-3 保留 Test 3 的 5s 值 | 沿用，未优化为更短 |
| C-5 T1 若 windows-latest flaky 可调到 128 KB / 20ms | 暂用 256 KB / 80ms 标定值；T1 elapsed 3.82s 在 Windows 本机一次性 PASS，未触发降级 |
| C-6 T4 一个 Manager 同 root 同 server | 已选此方案并在用例顶部 docstring 解释 |
| I-1 helper 注释指向 02 §6 R-1 | `newDownloadHTTPClient()` 顶部按任务 prompt 完整 4 行注释已加 |

## 4. 遇到的意外 + 解决

### 意外 #1：T1 首跑只耗时 101 ms（应 ~5s），AC-6 spot check 失败

**症状**：`T1 archive size after gzip: <未打印>` / `T1 elapsed: 101ms, sawMidProgress=false` → AC-6 断言 fail。

**根因**：用 `strings.Repeat("frp-binary-placeholder-content-padding-XYZ\n", 6000)` 构造 ~258 KB 字面量，但 gzip 对超高重复字符串压缩比可达 1000:1，最终 archive 只剩几百字节，分块写一两个 chunk 就跑完了。

**修复**：新增 `incompressibleBlob(n int) string` helper，用 `math/rand.New(rand.NewSource(42))` 固定 seed 在 charset `[a-zA-Z0-9]` 内取伪随机字节填充 256 KB，gzip 压缩比接近 1.3:1，archive ≈ 196 KB，慢传输总耗时稳定在 3.8s 量级。Fixed seed 保证跨平台跨次运行 archive 字节完全一致。

**附带价值**：这是一个独立可记录的"测试 fixture 慢传输校准"insight —— 用 `strings.Repeat` 构造的 archive 字节大小**不代表**网络传输负载，必须强制不可压缩。

## 5. 自测命令与结果

### 5.1 包内测试

```
$ go test -v ./internal/downloader/...
... (省略 23 个既有用例全 PASS) ...
=== RUN   TestTimeout_SlowDownload_Succeeds
    downloader_timeout_test.go:98: T1 archive size after gzip: 196389 bytes
    downloader_timeout_test.go:136: T1 elapsed: 3.816s, sawMidProgress=true
--- PASS: TestTimeout_SlowDownload_Succeeds (3.82s)
=== RUN   TestTimeout_DeadConnection_ResponseHeaderTimeout
    downloader_timeout_test.go:189: T2 elapsed: 102ms
--- PASS: TestTimeout_DeadConnection_ResponseHeaderTimeout (0.10s)
=== RUN   TestTimeout_GitHubAPIHang_ApiClientTimeout
    downloader_timeout_test.go:220: T3 elapsed: 207ms
--- PASS: TestTimeout_GitHubAPIHang_ApiClientTimeout (0.21s)
=== RUN   TestTimeout_BothKinds_Symmetric
    downloader_timeout_test.go:279: T4 total elapsed: 31ms
--- PASS: TestTimeout_BothKinds_Symmetric (0.03s)
... (Install 系列 8 个用例全 PASS) ...
PASS
ok  github.com/frp-easy/frp-easy/internal/downloader  5.101s
```

**adversarial 4 用例耗时实测**：
- T1 `SlowDownload_Succeeds`：**3.816s**（archive 196 KB，~49 chunks × 80ms）
- T2 `DeadConnection_ResponseHeaderTimeout`：**102 ms**（注入 100ms ResponseHeaderTimeout）
- T3 `GitHubAPIHang_ApiClientTimeout`：**207 ms**（注入 200ms Client.Timeout）
- T4 `BothKinds_Symmetric`：**31 ms**（快传输串行 frpc + frps）

### 5.2 verify_all

```
=== Summary ===
  PASS: 20
  WARN: 0
  FAIL: 0
  SKIP: 0
```

| 阶段 | PASS | WARN | FAIL | SKIP |
|---|---|---|---|---|
| 基线（实施前） | 20 | 0 | 0 | 0 |
| 实施后 | 20 | 0 | 0 | 0 |
| **Delta** | **0** | **0** | **0** | **0** |

零回归。

## 6. 待 Code Reviewer 关注的点

1. **`newDownloadHTTPClient()` 字段值合理性**（02 §3.3 决策清单）：dial/keepalive/IdleConnTimeout/ForceAttemptHTTP2/MaxIdleConns/ExpectContinueTimeout 沿用 stdlib DefaultTransport；显式与 DefaultTransport 不同的两处是 `TLSHandshakeTimeout=30s`（DefaultTransport 默认 10s）与 `ResponseHeaderTimeout=60s`（DefaultTransport 默认无）。`Client.Timeout=0` 是核心改动。
2. **T1 sawMidProgress 断言**（AC-6 软证据）：T1 用例在轮询期间至少抓到一次 `0 < progress < 100` —— 实测 `sawMidProgress=true`。该断言依赖 chunkSize=4096 + 80ms 节奏让 progressWriter 多次触发跨百分点回调；若未来调整传输参数（降到 128 KB / 20ms），需验证仍能命中至少一次中间值。
3. **T2 ResponseHeaderTimeout 注入路径**（Gate C-2）：注入到 `downloadClient.Transport.ResponseHeaderTimeout` 而非 `Client.Timeout` —— 后者在拆分后已显式为 0，注入到那里**不会触发**任何超时。
4. **T4 共享 Manager 决策**（Gate C-6）：已在用例 docstring 完整解释选型。Code Reviewer 可校对该解释是否充分，以及 archive 同时含 frpc+frps entry 这一 fixture 编排是否符合 `filepath.Base` 匹配语义（参见 `extractFromTarGz` line 444）。
5. **`math/rand` 而非 `crypto/rand`**：T1 fixture 固定 seed=42 用 `math/rand` —— 测试不需要密码学随机性，只需要"不可压缩 + 跨次稳定"。Reviewer 若 lint 警告"测试中用 math/rand"可放行。
6. **既有 12 用例 0 副作用**：所有非 `TestTimeout_*` 用例（含 T-014 引入的 spec 测试）全 PASS，确认 Manager 字段拆分与 client 实例分离对既有调用语义零影响。

## 7. Dev-map 更新

新增一个测试文件，不影响生产模块结构。本任务**无** dev-map 增删——`internal/downloader/` 已在 dev-map 索引内、未新增子目录、未新建 production 文件。

仅在 `internal/downloader/` 行后追加 T-025 备注（可选；本任务采取最小侵入，未编辑 dev-map）。

## 8. Insight to surface

无需要新追加的项目级 insight：
- "axios FormData Content-Type 污染" / ".gitattributes 不能锁 BOM" / "PowerShell 字节级 LF" 等已捕获场景与本任务无关；
- "拆分 HTTP client by 职责" 属于通用 Go 工程常识，不构成项目特有真相；
- T1 fixture "用 `strings.Repeat` 构造的测试 archive 在 gzip 后失去网络负载意义" 倒是一个非显然 testing-fixture 经验，但只与"模拟慢传输"场景相关、复用频率低，不达 insight-index 入选门槛。

故本节为空，PM 在 07 写 `## Insight` 时不必为本任务追加内容。

## 9. Verdict

**READY FOR REVIEW**

- verify_all 0 delta（PASS=20 不变）
- 27/27 downloader 包测试 PASS（含 4 个新增 adversarial）
- 设计严格落地：Manager 公签名 / DownloadState JSON / HTTP API contract 零变更，I-1 注释已加，C-2/C-6/C-7 软提醒全部落实
- 仅一处 DESIGN DRIFT-adjacent 决策：T4 archive fixture 由 02 隐含的 "kind=frpc only" 调整为 "frpc + frps 同 archive"（参见 §3 / §6）—— 因 Gate C-6 选 "同一 server" 隔离方案后 archive 必须同时包两 entry，是直接派生而非偏离。

## 10. 后续 PM 补丁（Code Review 后追加）

Code Review APPROVED with 4 minor/nit。本任务窗口内追加两处修复：

| # | 改动 | 文件 | 来源 |
|---|---|---|---|
| 10.1 | `newDownloadHTTPClient()` 注释路径改双写："T-025 02_SOLUTION_DESIGN.md §6 R-1（位于 docs/features/download-bin-timeout-fix/ 或归档后 docs/features/_archived/download-bin-timeout-fix/）" | `internal/downloader/downloader.go:86-89` | Code Review P1-1 |
| 10.2 | 新增 `TestNewDownloadHTTPClient_NoTotalTimeout`（6 行断言 `c.Timeout == 0`）—— 反向证伪"未来人误把 Timeout 改回 60s"的回归 | `internal/downloader/downloader_timeout_test.go` 末尾 | Code Review P1-2（强烈推荐） |

实测：`go test ./internal/downloader/...` PASS=28/28（原 27 + 新 1），总耗时 5.116s 不变。

NIT-1..4 不修：纯偏好 / 性能毫秒级 / 风格不影响功能。
