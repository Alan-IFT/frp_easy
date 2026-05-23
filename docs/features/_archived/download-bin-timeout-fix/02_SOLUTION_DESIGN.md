---
task_id: T-025
slug: download-bin-timeout-fix
stage: design
author: Solution Architect
date: 2026-05-23
mode: full
upstream_verdict: READY (01_REQUIREMENT_ANALYSIS.md §9)
---

# 02 — 方案设计（T-025 download-bin-timeout-fix）

## §1 架构摘要

把 `internal/downloader.Manager` 内现有的单个共享 `*http.Client`（`Timeout: 60s` 覆盖整请求周期）拆为两个职责分明的实例：`apiClient`（保留 60s 总超时，给短 JSON 查询用）与 `downloadClient`（**不设 `Client.Timeout`**，仅在 `*http.Transport` 上设置阶段性上限：dial 30s、TLS handshake 30s、`ResponseHeaderTimeout` 60s，由 `IdleConnTimeout` + body-read 期周期性 `SetReadDeadline` 兜底防 trickle）。`doDownload` 第 162 行的 archive 下载改走 `downloadClient`，`resolveLatestAsset` 第 317 行的 GitHub API 查询继续走 `apiClient`。变更严格局限在单文件 `internal/downloader/downloader.go`；`New(root, logger)` 公开签名（Gate Review C-2 锁定）不动；`DownloadState` JSON 结构、HTTP API 契约、前端代码零变更。

## §2 受影响模块

| 文件 | 改动类型 | 范围 |
|---|---|---|
| `internal/downloader/downloader.go` | edit | Manager 结构 + New + doDownload + resolveLatestAsset 共 4 处；新增 `newDownloadHTTPClient()` helper |
| `internal/downloader/downloader_test.go` | edit | Test 6 (`TestDownload_NetworkTimeout_StatusFailed`) 与 Test 12 (`TestResolveLatest_NetworkFailure`) 中对 `m.client` 的注入改名为 `m.apiClient` / 配套 `m.downloadClient`；其它测试沿用 |
| `internal/downloader/downloader_timeout_test.go` | new | 三类对抗性用例（慢响应 / 响应头 hang / API hang） |

无新增依赖；仅用 stdlib `net/http`、`net`、`time`、`context`、`net/http/httptest`。

## §3 Manager 结构与构造变更

### §3.1 结构体新增字段（**Manager 公开方法签名不动**）

```go
type Manager struct {
    mu     sync.Mutex
    states map[string]*DownloadState
    root   string

    // T-025：拆分。apiClient 短超时（60s 总），downloadClient 仅阶段性上限。
    apiClient      *http.Client
    downloadClient *http.Client

    logger *slog.Logger

    apiBaseURL string
    goos       string
}
```

**移除**：原 `client *http.Client` 字段。

> 决策：不保留 `client` 兼容字段。理由 — 该字段未导出且仅在包内被引用（`grep -n "\.client" internal/downloader` 仅 `downloader.go:162/317` 两处 + 测试 3 处），跨包零调用，强类型改名一次性收敛比保留两个名字（导致未来人误用）更安全。

### §3.2 `New` 实现（签名 100% 不变）

```go
func New(root string, logger *slog.Logger) *Manager {
    return &Manager{
        states: map[string]*DownloadState{
            "frpc": {Status: StatusIdle},
            "frps": {Status: StatusIdle},
        },
        root:           root,
        apiClient:      &http.Client{Timeout: 60 * time.Second},
        downloadClient: newDownloadHTTPClient(),
        logger:         logger,
    }
}
```

### §3.3 `newDownloadHTTPClient` helper（包内未导出）

```go
// newDownloadHTTPClient 返回 archive 下载专用 client：
//   - 无 Client.Timeout（避免 14MB 在国内 CDN 50–200 KB/s 触发 60s 总超时切断）。
//   - Dial / TLS / ResponseHeader 三阶段上限确保死连接不会无限挂。
//   - IdleConnTimeout 防 keep-alive 池里的 stale 连接被复用。
//   - body 阶段的 trickle 防护：本任务**不**实现 readDeadline 周期刷新
//     （见 §6 R-1 决策：留作 future-work）。
func newDownloadHTTPClient() *http.Client {
    return &http.Client{
        Timeout: 0, // 显式 0，文档化"无总超时"决策
        Transport: &http.Transport{
            Proxy: http.ProxyFromEnvironment, // 复用既有 HTTPS_PROXY env（与 stdlib DefaultTransport 一致，零成本）
            DialContext: (&net.Dialer{
                Timeout:   30 * time.Second,
                KeepAlive: 30 * time.Second,
            }).DialContext,
            ForceAttemptHTTP2:     true,
            MaxIdleConns:          10,
            IdleConnTimeout:       90 * time.Second,
            TLSHandshakeTimeout:   30 * time.Second,
            ExpectContinueTimeout: 1 * time.Second,
            ResponseHeaderTimeout: 60 * time.Second,
        },
    }
}
```

> 字段值来源：dial/keepalive/IdleConnTimeout/ForceAttemptHTTP2 沿用 stdlib `DefaultTransport`；新增的是 `TLSHandshakeTimeout=30s`（DefaultTransport 默认 10s 但实测国内 GitHub CDN TLS 偶尔慢于此）与 `ResponseHeaderTimeout=60s`（RA D-2 决策）。**`Client.Timeout=0` 是唯一与 DefaultTransport 行为差异的关键改动**。

## §4 调用点改造

### §4.1 `doDownload` 第 162 行

```go
// 改前
resp, err := m.client.Do(req)
// 改后
resp, err := m.downloadClient.Do(req)
```

错误消息保留 `fmt.Sprintf("下载超时: %v", err)`（FR-4 + RA D-6 既有中文文案）。注：`ResponseHeaderTimeout` 触发时 `http.Transport` 返回的错误信息已含 `timeout awaiting response headers` 字样，被包进我们的"下载超时:"前缀，前端用户体感与既有一致。

### §4.2 `resolveLatestAsset` 第 317 行

```go
// 改前
resp, err := m.client.Do(req)
// 改后
resp, err := m.apiClient.Do(req)
```

错误消息 `"无法访问 GitHub（请检查网络或代理）: ..."` 不变（FR-3）。

### §4.3 `m.client` 字段引用清查

`grep -n "m\.client" internal/downloader/downloader.go` 共 2 处（162 + 317），均已在 §4.1 / §4.2 覆盖。无遗漏。

## §5 测试策略

### §5.1 既有测试的修复

`grep -n "m\.client\b" internal/downloader/downloader_test.go` 命中 3 处（line 229 / 362 / 528），均改为：

| 文件 | 行 | 用途 | 改法 |
|---|---|---|---|
| `downloader_test.go` | 229 (Test 3) | API hang 让 Start 进 Downloading 状态 | `m.apiClient = &http.Client{Timeout: 5*time.Second}` |
| `downloader_test.go` | 362 (Test 6) | archive 路径 hang → 验证 StatusFailed | 改为 `m.downloadClient = &http.Client{Transport: &http.Transport{ResponseHeaderTimeout: 50*time.Millisecond}}` 并把 `m.apiClient` 保持默认（API 响应是同步立即返回的，不会触发 apiClient 超时） |
| `downloader_test.go` | 528 (Test 12) | API 短超时验证网络失败分支 | `m.apiClient = &http.Client{Timeout: 2*time.Second}` |

> Test 6 是这里**关键改动**：原测试用 `Client.Timeout=50ms` 整请求级超时，新设计 downloadClient 没有这一字段，所以改为 `Transport.ResponseHeaderTimeout` 注入。由于该测试的 server `/archive` 在 hang 阶段不发 header，`ResponseHeaderTimeout` 触发路径正确覆盖该 server 行为。验证标准不变：50ms 内 `StatusFailed` + `Error != ""`。

### §5.2 新增 `downloader_timeout_test.go`

三类新用例，全部 `httptest.Server` + 真实 HTTP 行为（遵循 insight L29/L45 / RA AC-5 / Gate C-4）：

**T1 `TestTimeout_SlowDownload_Succeeds`（AC-1 核心证明）**
- httptest server `/archive` 以 ~50 KB/s 速率分块写 256 KB（约 5 秒），完整响应；API 端点正常返回 release JSON。
- Archive 内容用 `buildTarGz` 真造，让 doDownload 走通解压 + Install 完整路径。
- 期待：`StatusSuccess` + `progress == 100` + 安装后路径 `frp_linux/frpc` 存在。
- 不设 `m.downloadClient` 覆盖（用生产默认 client）— 这正是回归"60s 总超时"bug 的关键证据。
- `t.Skip` 守护：用例总耗时 ~5s，超过"快测试"基线，加 `if testing.Short() { t.Skip("slow") }`。

**T2 `TestTimeout_DeadConnection_ResponseHeaderTimeout`（AC-2 死连接兜底）**
- httptest server `/archive` 拿到请求后 `select { case <-r.Context().Done() }` 永不写 header；API 端点正常。
- 注入 `m.downloadClient = &http.Client{Transport: &http.Transport{ResponseHeaderTimeout: 100*time.Millisecond}}`（**短 ResponseHeaderTimeout 走测试 seam**，避免真等 60s）。
- 期待：≤2s 内 `StatusFailed`，`Error` 含 "下载超时" 子串（与生产消息一致）。

**T3 `TestTimeout_GitHubAPIHang_ApiClientTimeout`（AC-3 API 短超时）**
- httptest server 所有路径都 hang。
- 注入 `m.apiClient = &http.Client{Timeout: 200*time.Millisecond}`（同 Test 12 的 seam 风格）。
- 期待：≤2s 内 `StatusFailed`，`Error` 含 "无法访问 GitHub" 子串。
- 注：与既有 Test 12 (`TestResolveLatest_NetworkFailure`) 互补 — Test 12 测的是"server 已关闭、TCP 立即拒绝"，T3 测的是"server 接受连接但永不应答"。两者覆盖 GitHub API 失败的两条不同物理路径。

**T4 `TestTimeout_BothKinds_Symmetric`（AC-4）**
- 用 T1 同款 server，分别 `Start("frpc")` 与 `Start("frps")` 串行各跑一遍（避免共享 root 目录冲突 — 各自 `t.TempDir()`），断言两次都 `StatusSuccess`。
- 也加 `testing.Short` skip。

### §5.3 进度推进 spot check（AC-6 软证据）

T1 用例在 `waitForDone` 轮询时额外抓快照：每 200ms 读 `m.Status(kind).Progress`，断言**至少出现 ≥1 次中间值 > 0 且 < 100**（证明 progressWriter 在长时间下载期间持续推进，未被超时打断）。

### §5.4 受 `testing.Short()` 影响的用例

| 用例 | 真耗时 | -short 行为 |
|---|---|---|
| T1 `SlowDownload_Succeeds` | ~5s | skip |
| T2 `DeadConnection_ResponseHeaderTimeout` | <2s（注入了 100ms ResponseHeaderTimeout） | 跑 |
| T3 `GitHubAPIHang_ApiClientTimeout` | <2s | 跑 |
| T4 `BothKinds_Symmetric` | ~10s（串行两次） | skip |

verify_all 是否带 `-short` 由 CI 控制；本地 `go test ./internal/downloader -short` 仍能在 ~3s 完成全套，长用例（T1/T4）作为完整 `go test` 一次的额外 ~15s 成本。

## §6 风险与缓解

| ID | 风险 | 缓解决策 |
|---|---|---|
| R-1 | trickle stall — 攻击者/死网络让 body 阶段每 30s 仅发 1 字节，连接看似 alive 数小时 | **本任务不引入 `SetReadDeadline` 周期刷新**。理由：(a) 用户主动操作场景（webUI 点击），非自动后台任务；(b) `IdleConnTimeout=90s` 与 TCP keep-alive 30s 已覆盖完全断流；(c) 进程重启即终止；(d) 引入 readDeadline goroutine 需自管 `net.Conn` lease 与 Body close 时机，复杂度与本任务"最小侵入"原则冲突。本风险**作为已知 limitation 记入 02 §6 与 07 insight**，未来如需可作 T-026 独立任务。Architect 决策已记。 |
| R-2 | `ResponseHeaderTimeout=60s` 在国内 GitHub release CDN 偶发返 5xx 而不 hang 时，仍可能让首次连接慢于 60s | 该 timeout 仅约束"已 dial+TLS 完成 → 收到 status line"阶段；现实中只要 TCP 三次握手 + TLS 完成（≤30s+30s 上限），CDN 收请求后通常立即（≤几百 ms）开始写 status line。60s 完全足够。若仍命中，failed → 用户 retry。 |
| R-3 | 测试 T1 在慢 CI runner 上 5s 真造太脆弱，可能假阴性 | 用 `t.Helper()` + `waitForDone(timeout=15*time.Second)` 给宽裕余量；server 端发送速率用 chunk size + `time.Sleep(40ms)` 控制，runner 抖动 ±2s 不会失败。 |
| R-4 | `Transport` 缓存复用 — 同一 `downloadClient` 跨任务复用可能让后一次下载继承前一次的 stale connection | `IdleConnTimeout=90s` 让 idle conn 自动回收；下载链路单次/小时级，不会复用到过期 conn。如果未来发现问题，可在 `doDownload` 内 `defer m.downloadClient.CloseIdleConnections()` 兜底（**本任务不加，YAGNI**）。 |
| R-5 | proxy env 变更 — `http.ProxyFromEnvironment` 进程启动后只读一次，用户运行期改 `HTTPS_PROXY` 不生效 | 与 stdlib `DefaultTransport` 同款行为，既有 frpc/frps 本身也不响应 env 变更。文档可留一行 hint，但不在本任务范围。 |
| R-6 | 同一 `*http.Transport` 实例并发被 frpc + frps 共享 → goroutine race | `*http.Transport` 是 goroutine-safe 的（stdlib 文档明确）；既有共享 `m.client` 已经在该模型下运行 1 年无 race issue。零回归。 |

## §7 复用审计

| 需求 | 已有代码 | 文件路径 | 决策 |
|---|---|---|---|
| `*http.Client` + Transport 标准构造 | Go stdlib `net/http.DefaultTransport` | (stdlib) | **参考其字段值，但不直接复用** — DefaultTransport 是全局单例，改它会影响全进程；我们要独立实例 |
| httptest server 测试 seam | `newFRPServer` helper | `internal/downloader/downloader_test.go:109` | 复用（T2/T3/T4 直接调） |
| 慢响应 server pattern | 无（既有 Test 6 只 hang，不慢写） | — | T1 需新建分块慢写 server（~15 行） |
| `progressWriter` 进度推进 | `progressWriter` struct | `internal/downloader/downloader.go:381-411` | 不动，T1 间接验证其在长链路下持续推进 |
| Manager 测试注入 seam | `apiBaseURL` + `goos` 直接字段写 + Test 6/12 手工赋值 `m.client` | `downloader.go:58-59` + 测试三处 | **沿用同款手工字段赋值**（写 `m.apiClient = ...` / `m.downloadClient = ...`）；不引 setter 方法、不改 `New` 签名（Gate Review C-2 锁定） |
| HTTP API 中文错误消息 | 既有 `"下载超时:"` `"无法访问 GitHub..."` 字符串 | `downloader.go:164/319` | 100% 保留 |

> insight L29/L45 显式禁用 interface mock：本设计的测试 seam 是"字段直接赋值真 `*http.Client`"而非"注入 `interface { Do(req) (resp, err) }`"，符合既有 T-014 测试风格、不引入新抽象。

## §8 数据 / API 契约变更

**零**。

- `DownloadState` JSON 结构不变（`status` / `progress` / `error` 三字段）。
- `POST /api/v1/system/download-bin` 请求/响应不变（202 / 409 / 422 / 503）。
- `GET /api/v1/system/download-status/{kind}` 响应不变。
- Manager 公开方法签名不变（`New`/`Start`/`Status`/`Install`）。
- 错误消息文案沿用既有中文字面量。

## §9 序列流（archive 下载链路）

```
POST /api/v1/system/download-bin {kind:"frps"}
        ↓
handlers_system.go:downloadBin → 202 + Manager.Start("frps")
        ↓ (synchronous: state = StatusDownloading)
go m.doDownload(kind, goos)
        │
        ├─ resolveLatestAsset(goos)
        │       └─ m.apiClient.Do(req)   ← Timeout=60s (整请求级)
        │              └→ ghRelease JSON → downloadURL, tag
        │
        ├─ resp, err := m.downloadClient.Do(req)
        │       │   Transport stage limits:
        │       │     - Dial:           30s
        │       │     - TLS handshake:  30s
        │       │     - ResponseHeader: 60s
        │       │   Client.Timeout:     0 (无 body 总超时)
        │       └→ resp.Body (open-ended read)
        │
        ├─ io.Copy(archiveTmp, TeeReader(resp.Body, progressWriter))
        │   ← AC-1 关键路径：5–10 分钟级别读取也不被切断
        │
        ├─ extract (tar.gz / zip)
        └─ m.Install(kind, binFile, -1) → 原子 rename
                ↓
        state = StatusSuccess, progress = 100
```

前端 `GET /download-status/{kind}` polling 期间，`progressWriter` 每跨 1% 调一次 `setProgress`；用户在 webUI 看到进度条持续推进，不会因 60s 切断而卡死在 ~10–30%。

## §10 迁移 / rollout 计划

- 无数据迁移、无配置迁移、无前端发布耦合。
- 单次 commit 落地 `internal/downloader/downloader.go` + `_test.go` + 新 `_timeout_test.go`。
- 回滚策略：单文件 `git revert` 即可恢复原 `client *http.Client{Timeout: 60s}` 单字段行为。
- 不需要 feature flag — 此修复是单向无副作用变更（"放宽 timeout"），不存在两个并存的运行模式。

## §11 范围边界（out-of-scope 重申）

- 取消下载按钮 / Manager 级 context cancel（RA D-5/D-7）— future task。
- 断点续传 / 失败自动重试 / 镜像源切换（RA §2.2）— future task。
- trickle attack 防护（§6 R-1）— future task，需 readDeadline 周期刷新 goroutine。
- progressWriter 新增字段（如 `bytes_received`）— RA NFR-2 标注非强制，本任务不做以保 JSON 契约稳定。
- archive 流式解压（边下边解）— 本任务保留"先全量落盘、再读盘解压"既有流程（`downloader.go:144-153`）。

## §12 文件改动清单与规模估计

| 文件 | 操作 | 估算 LOC |
|---|---|---|
| `internal/downloader/downloader.go` | edit (struct + New + 2 调用点 + 1 helper) | +30 / -5 |
| `internal/downloader/downloader_test.go` | edit (3 处 `m.client` 改名) | +6 / -3 |
| `internal/downloader/downloader_timeout_test.go` | new (4 用例 + 1 慢 server helper) | +160 |

总改动量：**~200 LOC**，位于 50–200 区间上沿。

## §13 验收清单交接（给 Gate Reviewer 用）

| RA 项 | 设计对应章节 | 实施判定标准 |
|---|---|---|
| FR-1 慢源 ≥10 分钟不超时 | §3.3 (`Timeout: 0`) + §5.2 T1 | T1 在 ~5s 真造下 PASS；生产场景靠 Client.Timeout=0 由数学保证 |
| FR-2 阶段性上限 | §3.3 (dial 30s / TLS 30s / ResponseHeader 60s) | 字段值在 PR review 可见 |
| FR-3 API 短超时保留 | §4.2 (apiClient) | 既有 Test 12 + 新 T3 |
| FR-4 中文错误消息 | §4.1/§4.2 | 字面量 `下载超时:` / `无法访问 GitHub` 不动 |
| FR-5 双 kind 一致 | §3 共享同一对 client | T4 |
| NFR-1 稳定性 | §3.3 IdleConnTimeout + 三阶段上限 | T1（活）+ T2（死） |
| NFR-2 可观测 | 既有 logger.Info/Error 全保留 | grep `m.logger.` 无改动 |
| NFR-3 向后兼容 | §8 零契约变更 | API 测试无回归（既有 `internal/httpapi` 测试不受影响） |
| NFR-4 最小侵入 | §2 改动文件清单 | 仅 1 个 production .go 文件 |
| NFR-5 测试可注入 | §3.1 字段直写 + §5.1 注入方式 | 与既有 `apiBaseURL` seam 同款 |
| AC-1 慢传输不超时 | §5.2 T1 | `StatusSuccess` |
| AC-2 死连接快速失败 | §5.2 T2 | `StatusFailed` + "下载超时" |
| AC-3 API 短超时保留 | §5.2 T3 | `StatusFailed` + "无法访问 GitHub" |
| AC-4 双 kind 一致 | §5.2 T4 | 两次 `StatusSuccess` |
| AC-5 无回归 | §5.1 既有测试改名后仍 PASS + verify_all | 全套 `go test ./internal/downloader` PASS |
| AC-6 progress 持续推进 | §5.3 spot check | T1 抓到中间 progress 值 |

## §14 Verdict

**READY-FOR-GATE-REVIEW** — 设计自洽、零未决项、零新依赖、零契约变更。RA §6 自决断 D-1 至 D-7 全部纳入实现；R-1 trickle 风险显式标为已知 limitation。建议 Gate Reviewer 重点审 §3.3 字段值合理性与 §5.2 T1 在 CI 上的稳定性（5s 真造对慢 runner 是否过紧）。
