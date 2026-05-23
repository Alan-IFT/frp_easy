---
task_id: T-025
slug: download-bin-timeout-fix
stage: delivery
author: PM Orchestrator
date: 2026-05-23
mode: full
upstream_verdict_ra: READY (01 §9)
upstream_verdict_sd: READY-FOR-GATE-REVIEW (02 §14)
upstream_verdict_gate: APPROVED FOR DEVELOPMENT (03 §6)
upstream_verdict_dev: READY FOR REVIEW (04 §9)
upstream_verdict_review: APPROVED (05 §8)
upstream_verdict_qa: PASS (06 §7)
---

# 07 — Delivery（T-025 download-bin-timeout-fix）

## 1. 摘要

修复生产 bug：webUI 上点击「一键下载 frps / frpc」精确 60 秒后报「下载写入失败: context deadline exceeded」。

根因：`internal/downloader/downloader.go` 旧版第 71 行 `&http.Client{Timeout: 60 * time.Second}` 是 Go 标准库**整请求总超时**（含 connect / TLS / response header / **body 读取**全过程）。frp 1.x archive ~14MB，国内访问 GitHub Release CDN 速率不稳（50-300 KB/s 常见），60s 内几乎不可能读完 body。

修复方案：拆 `Manager.client` 为两个专用 client：
- `apiClient`：保留 60s 总超时，给 GitHub Release API JSON 查询用（小响应足够）
- `downloadClient`：`Client.Timeout=0`（无总超时），用 Transport 阶段性上限防御死连接：dial 30s / TLS 30s / ResponseHeader 60s / IdleConn 90s

公开 API（`New(root, logger)` 签名、`DownloadState` JSON、HTTP API contract）零变更；前端代码、其它模块零改动。

## 2. 改动文件

| 文件 | 变化 |
|---|---|
| `internal/downloader/downloader.go` | +39 / -3：Manager 字段拆分、新 helper `newDownloadHTTPClient()`、2 处 client 调用点改名 |
| `internal/downloader/downloader_test.go` | +5 / -2：3 处 `m.client = ...` 改名（Test 3/6/12），Test 6 注入位置改为 `Transport.ResponseHeaderTimeout` |
| `internal/downloader/downloader_timeout_test.go` | +307（新建）：T1/T2/T3/T4 adversarial + `TestNewDownloadHTTPClient_NoTotalTimeout` 静态守门 + helper `newSlowFRPServer` / `incompressibleBlob` |
| `scripts/baseline.json` | 版本 10→11，go_tests 241→246，test_count 337→342 |
| `docs/tasks.md` | T-025 任务条目（进行中→已完成） |
| `docs/features/download-bin-timeout-fix/01..07.md` | 七阶段全套文档 |

总生产代码改动 ~42 LOC；总测试增量 ~307 LOC。

## 3. 7 阶段路径

| 阶段 | 产出 | 关键决策 |
|---|---|---|
| 1 RA | 01_REQUIREMENT_ANALYSIS.md | FR-1..5 + AC-1..6；D-3 决断"拆 client" |
| 2 Architect | 02_SOLUTION_DESIGN.md | §3.3 helper 设计；trickle stall 作 known limitation（R-1） |
| 3 Gate | 03_GATE_REVIEW.md | APPROVED；9 个 NIT、3 个 Improvement；R-1 在用户取舍授权下接受 |
| 4 Dev | 04_DEVELOPMENT.md | 严格按 02；T1 incompressibleBlob 解决高重复字符串被 gzip 压崩 |
| 5 Review | 05_CODE_REVIEW.md | APPROVED；P1-1 注释路径、P1-2 静态守门测试 → PM 本任务窗口内补丁 |
| 6 QA | 06_TEST_REPORT.md | PASS；`## Adversarial tests` 裸标题；28/28 包测试 PASS |
| 7 Delivery | 本文件 | verify_all 终验 PASS=20/0/0 |

## 4. verify_all 终验

跑 `scripts/verify_all.ps1`（Stage 7）：

```
PASS: 20    WARN: 0    FAIL: 0    SKIP: 0
```

零回归，与基线一致。包内 `go test -count=1 ./internal/downloader/...` 28/28 PASS（5.367s）。全仓 `go test -count=1 ./...` 全部 ok。

## 5. AC 状态

| AC | 描述 | 状态 |
|---|---|---|
| AC-1 | 慢源 ≥10 分钟不被总超时切断 | ✅ 静态字段值（`Timeout: 0`）+ `TestNewDownloadHTTPClient_NoTotalTimeout` + T1 联合证据 |
| AC-2 | 死连接快速失败（不超过 ResponseHeaderTimeout） | ✅ T2 实测 102ms < 60s |
| AC-3 | GitHub API 短超时保留（≤60s） | ✅ T3 实测 207ms（短超时注入下）+ Test 12 互补 |
| AC-4 | frpc/frps 行为一致 | ✅ T4（共享 Manager 串行）；慢传输对称性靠"共享 client + T1 已证"联合推断 |
| AC-5 | 现有单测全部 PASS | ✅ verify_all PASS=20 不变；27 既有用例 0 副作用 |
| AC-6 | progress 持续推进 | ✅ T1 `sawMidProgress=true` |

## 6. 生产验证建议

本次修复在 unit/integration 层完整覆盖，但**生产 e2e 验证**需用户在真环境跑：
1. 服务模式部署到 `VM-20-7-ubuntu`（或同款国内 VM）
2. webUI 点击「一键下载 frps」与「一键下载 frpc」
3. 观察 systemd journal，确认：
   - 不再出现"下载写入失败: context deadline exceeded"
   - download progress 在长链路下持续推进（前端 polling 见 progress > 0 走动）
   - 最终 `status: success`，`frps` / `frpc` 落到 `frp_linux/` 目录可执行

若网络极差（>15 分钟仍未完成），可在 systemd Environment 注入 `HTTPS_PROXY=http://127.0.0.1:7890` 走代理（helper 支持 `Proxy: http.ProxyFromEnvironment`）。

## 7. 已知 limitation

- **trickle stall**（对端每 30s 发 1 字节让连接永不死）：本任务**显式不防御**。原因：用户授权"长期易维护，不为边缘风险引入复杂控制逻辑"；trickle 是恶意构造模式而非自然 CDN 抖动；用户主动 webUI 操作可随时重启进程终止。若未来出现该问题，可新开 T-026 引入 `SetReadDeadline` 周期刷新。
- 真"≥10 分钟" archive 不被切断的运行时边界，靠数学（`Timeout: 0` → 无总超时）+ 静态守门测试保证，未做真造 10 分钟的运行时验证（成本不可接受）。

## Insight

- **2026-05-23** · Go `http.Client.Timeout` 是**整请求总超时**（包括 connect / TLS / response header / **body 读取**全过程），用一个 client 同时跑"短 JSON 查询"和"长 archive 下载"会因后者 body 远超 60s 导致整请求被切断。修复模式：**拆两个 client**——`apiClient`（短总超时，给 JSON 查询）+ `downloadClient`（`Client.Timeout=0`，仅靠 Transport 阶段性上限 dial/TLS/ResponseHeader/IdleConn 防御死连接）。这与 stdlib `DefaultTransport` 的设计哲学一致：`net.Dialer.Timeout` / `TLSHandshakeTimeout` / `ResponseHeaderTimeout` / `IdleConnTimeout` 都是阶段性，唯独 `Client.Timeout` 是整请求性——绝大多数场景滥用后者 · evidence: T-025 用户实测精确 60s 失败（systemd journal 21:26:33 download started → 21:27:33 ERROR），根因 `internal/downloader/downloader.go:71`
- **2026-05-23** · 动态慢造测试（httptest.Server chunk-write + sleep 真造 ~5s）能证明"链路通畅 + progress 推进"，但**无法反向证伪"未来人误把 Timeout 改回 60s"的回归** —— 因为真造 5s 仍 << 60s。这类"配置字段值"类修复必须配静态守门测试（`if c.Timeout != 0 { t.Fatal(...) }` 6 行），断言 helper 返回 client 的关键字段值。动态测试 + 静态测试是**互补的**两层防线 —— 缺一就让"silent regression"成为可能 · evidence: T-025 Code Review P1-2 发现 T1 反向证伪盲区 → 加 TestNewDownloadHTTPClient_NoTotalTimeout 弥补
- **2026-05-23** · 测试 fixture 用 `strings.Repeat("xxx", 100000)` 类高重复字符串作慢传输负载会被 `gzip.NewWriter` 压成 KB 级，**让慢传输用例在毫秒内跑完**（实测从 ~5s 期望塌到 101ms）。修复模式：用 `math/rand.New(rand.NewSource(固定 seed))` 在 `charset` 上生成伪随机字节，gzip 后稳定 ~1.3:1 压缩比 · evidence: T-025 04 §4 意外 #1 实测：256 KB 输入 / 196 KB gzip 后，与 chunk=4096 + sleep=80ms 节奏配合得到 ~3.8s 真造耗时
- **2026-05-23** · 阶段文档（特别是注释代码中的引用路径）若包含归档后路径 `_archived/`，则在归档前 commit 落 main 时该路径暂不存在；维护期内有人按图索骥会 404。修复模式：**注释中**双路径都提，或仅引用任务 ID 让读者用 grep 找。这与 insight L43（archive-task 标题禁数字前缀）属同一类"跨 stage 文档协调"陷阱 · evidence: T-025 Code Review P1-1（`downloader.go:88` 注释路径）→ PM 补丁双路径
