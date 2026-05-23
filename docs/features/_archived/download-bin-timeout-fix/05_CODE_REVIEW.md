---
task_id: T-025
slug: download-bin-timeout-fix
stage: code-review
author: Code Reviewer
date: 2026-05-23
mode: full
upstream_verdict_ra: READY (01 §9)
upstream_verdict_sd: READY-FOR-GATE-REVIEW (02 §14)
upstream_verdict_gate: APPROVED FOR DEVELOPMENT (03 §6)
upstream_verdict_dev: READY FOR REVIEW (04 §9)
---

# 05 — Code Review（T-025 download-bin-timeout-fix）

## 1. 审阅范围

- `docs/features/download-bin-timeout-fix/01_REQUIREMENT_ANALYSIS.md`
- `docs/features/download-bin-timeout-fix/02_SOLUTION_DESIGN.md`
- `docs/features/download-bin-timeout-fix/03_GATE_REVIEW.md`
- `docs/features/download-bin-timeout-fix/04_DEVELOPMENT.md`
- `internal/downloader/downloader.go`（543 LOC）
- `internal/downloader/downloader_test.go`（546 LOC）
- `internal/downloader/downloader_timeout_test.go`（281 LOC，新建）
- `scripts/verify_all.sh` / `verify_all.ps1`（确认无 `-short`）
- `.harness/insight-index.md`（46 条全量比对）

## 2. 与 02 设计的对齐核验

| 设计项 | 实现位置 | 对齐 |
|---|---|---|
| Manager 移除 `client`，新增 `apiClient` + `downloadClient` 双字段 | downloader.go:59-60 | ✅ |
| `New(root, logger)` 公开签名不变（Gate C-2 锁定） | downloader.go:71 | ✅ |
| `apiClient = &http.Client{Timeout: 60s}` | downloader.go:78 | ✅ |
| `downloadClient = newDownloadHTTPClient()` | downloader.go:79 | ✅ |
| helper `newDownloadHTTPClient` 含 I-1 注释 + 指向 02 §6 R-1 | downloader.go:84-93 | ✅（路径预写归档后路径，见 P1-1） |
| `Timeout: 0` 显式 + 文档化 | downloader.go:96 | ✅ |
| dial 30s / KeepAlive 30s | downloader.go:99-102 | ✅ |
| TLSHandshakeTimeout 30s | downloader.go:106 | ✅ |
| ResponseHeaderTimeout 60s | downloader.go:108 | ✅ |
| `Proxy: http.ProxyFromEnvironment`（Gate C-8） | downloader.go:98 | ✅ |
| `doDownload` 改用 `m.downloadClient.Do(req)` | downloader.go:199 | ✅ |
| `resolveLatestAsset` 改用 `m.apiClient.Do(req)` | downloader.go:354 | ✅ |
| 错误消息文案"下载超时: " / "无法访问 GitHub" 保留 | downloader.go:201/356 | ✅ |
| Test 3 注入 `m.apiClient = &http.Client{Timeout: 5s}` | downloader_test.go:230 | ✅ |
| Test 6 注入 `m.downloadClient` 用 `Transport.ResponseHeaderTimeout=50ms` | downloader_test.go:364-366 | ✅（Gate C-2 落实正确） |
| Test 12 注入 `m.apiClient = &http.Client{Timeout: 2s}` | downloader_test.go:533 | ✅ |
| 新增 T1/T2/T3/T4 用例 + `newSlowFRPServer` + `incompressibleBlob` helper | downloader_timeout_test.go | ✅ |

**结论**：设计与实现对齐度 100%。Gate Review 软提醒 C-2 / C-3 / C-6 全部按要求落实，并在 04_DEVELOPMENT.md §3 给出 explicit 解释。

## 3. 编号问题

### P0（Blocking）

无。

### P1（Should-fix，非阻塞但建议本任务窗口内修）

#### P1-1 [MAINT] `downloader.go:88` — helper 注释引用了归档后路径，当前不存在

`newDownloadHTTPClient` 注释第二段写：
> 若未来想改成有总超时，请先读 `docs/features/_archived/download-bin-timeout-fix/02_SOLUTION_DESIGN.md` §6 R-1。

但当前任务尚未走完 7-stage 归档，实际路径是 `docs/features/download-bin-timeout-fix/02_SOLUTION_DESIGN.md`。归档前若有人按图索骥会 404。

**修复建议**：双路径都提，或仅留任务 ID 让读者自己找。

#### P1-2 [TEST] `downloader_timeout_test.go:87-152` — T1 实际**无法反向证伪 60s 总超时回归**

**事实链**：
1. T1 不覆盖 `m.downloadClient`，走生产默认 client（`Client.Timeout=0`, `ResponseHeaderTimeout=60s`）。
2. T1 server 首字节立即写 + flush（line 59-64 of timeout_test），ResponseHeader 阶段在 <10ms 完成，远低于 60s。
3. T1 总耗时实测 3.816s（04 §5.1）。
4. **若 Developer 把 downloadClient.Client.Timeout 改回 60s**，T1 仍然 PASS（3.816s < 60s）。

**强烈建议**：加 `TestNewDownloadHTTPClient_NoTotalTimeout` 6 行单元测试：
```go
func TestNewDownloadHTTPClient_NoTotalTimeout(t *testing.T) {
    c := newDownloadHTTPClient()
    if c.Timeout != 0 {
        t.Fatalf("Client.Timeout must be 0 (no total timeout); got %v", c.Timeout)
    }
}
```
让"未来人误改 60s"瞬间 fail。

#### P1-3 [TEST] `downloader_timeout_test.go:160-197` — T2 注入裸 Transport 丢失生产 dial/TLS/Proxy 字段

localhost 直连下 OK，但与生产 Transport 行为有差。建议注释说明"仅验证注入路径有效，不代表生产 ResponseHeaderTimeout=60s 默认值"。**判定**：NIT。

#### P1-4 [TEST] `downloader_timeout_test.go:242-280` — T4 用快传输无法证明"双 kind 在慢传输下都不超时"

T4 用快传输（31ms 总耗时），靠"共享 client + T1 已证慢传输 OK"联合推断 AC-4 对称性。建议 docstring 补此说明。**判定**：NIT。

### NIT（Nice-to-have，不阻塞）

#### NIT-1 `downloader_timeout_test.go:14` — `math/rand` 在 Go 1.22+ 偏好 `math/rand/v2`

本场景非密码学、`rand.New(rand.NewSource(42))` 局部实例非顶层 race-prone API。可保持现状。

#### NIT-2 `downloader_timeout_test.go:24-26` — `incompressibleBlob` 注释可加压缩比实测值

加一句"实测 gzip 后 ~196 KB / 输入 256 KB ≈ 1.31:1"（04 意外 #1 已记录）。

#### NIT-3 `downloader_timeout_test.go:32` — string/[]byte 双向转换

256 KB 数据被拷贝两次；测试场景毫秒级开销，可忽略。

#### NIT-4 `downloader.go:96` — `Timeout: 0` 注释中英文混排

与文件其它注释风格不一致。纯偏好。

## 4. AC 覆盖检查

| AC | 实施位置 | 状态 | 备注 |
|---|---|---|---|
| AC-1 慢源 ≥10 分钟不超时 | downloader.go:96 (`Timeout: 0`) + timeout_test.go T1 | ✅ 静态字段值证 + T1 跑通慢链路 | 真正"10 分钟"边界无运行时测试，靠数学：Timeout=0 → 无总超时 |
| AC-2 死连接快速失败 | timeout_test.go T2 | ✅ | "下载超时"错误前缀已断言 |
| AC-3 GitHub API 短超时保留 | timeout_test.go T3 + 既有 Test 12 | ✅ | T3 测"hang"、Test 12 测"TCP 拒"，互补 |
| AC-4 双 kind 一致 | timeout_test.go T4 | ✅（弱版） | T4 用快传输，慢传输对称性靠"共享 client + T1 已证"联合推断（见 P1-4） |
| AC-5 无回归 | 04 §5.2 verify_all PASS=20 / Delta=0 | ✅ | 27/27 downloader 包测试 PASS |
| AC-6 progress 持续推进 | timeout_test.go T1 `sawMidProgress` 断言 | ✅ | 实测 `sawMidProgress=true` |

## 5. 设计偏离审计

按 04 §6 / §9 自承，无偏离设计。仅一处"派生"决策：T4 archive 同时含 frpc + frps 两个 entry（由 Gate C-6 选共享 Manager 隔离方案派生）。直接派生、合理。

## 6. 代码品质细项

| 项 | 评估 |
|---|---|
| 错误处理完整性 | ✅ `defer resp.Body.Close()` 在 downloader.go:204 / 358 都有；新代码无遗漏 |
| Goroutine 泄漏 | ✅ T1 server `if _, err := w.Write(...); err != nil { return }` + `r.Context().Done()` select 双兜底；T2/T3 `<-r.Context().Done()` 单兜底，srv.Close() 时全部解锁 |
| Race 条件 | ✅ `rand.New(rand.NewSource(42))` 局部实例非顶层包级，单 goroutine 使用；`*http.Transport` 文档保证 goroutine-safe；`m.mu` 保护 states map 不变 |
| 资源关闭 | ✅ 所有 httptest.Server 都 `defer srv.Close()`；archiveTmp/binTmp 关闭与 remove 既有逻辑保留 |
| 输入验证 | ✅ Start 仍校验 kind / goos；新代码不引入用户输入路径 |
| 安全 | ✅ Zip Slip 检查保留；新代码无 SQL/反序列化/secret 风险 |
| 命名 | ✅ `apiClient` / `downloadClient` / `newDownloadHTTPClient` / `newSlowFRPServer` / `incompressibleBlob` 都直观 |
| 注释（WHY） | ✅ helper 注释解释了"为何 Timeout=0"+ 指引；测试 docstring 解释了 T1/T4 取舍 |
| 死代码 / 过度抽象 | ✅ 无；helper 是必要分离 |
| 范围蔓延 | ✅ 严格按 02 §12 改动清单：1 个生产文件 + 1 个测试文件改名 + 1 个新测试文件 |

## 7. 与 .harness/insight-index.md 兼容

| Insight | 本任务关联 | 兼容判定 |
|---|---|---|
| L29 / L45 / L25 spec mock 漂移 | T1/T2/T3/T4 全用 `httptest.Server` 真实 HTTP，非 interface mock | ✅ |
| L19 / L30 / L43 标题禁数字前缀 | 提醒 QA 用裸 `## Adversarial tests`，PM 用裸 `## Insight` | ⚠ 待后续阶段执行 |
| L37 / L41 reviewer 不落盘 fallback | 本 review 按约定返回到响应消息，由 PM 代写 | ✅ |

## 8. Verdict

**APPROVED**

- 零 P0、零 CRITICAL、零 MAJOR。
- 4 个 P1 全部为 MINOR / NIT 级，不阻塞 merge。
- 设计→实现对齐度 100%。
- Gate Review C-2 / C-3 / C-6 / I-1 软提醒全部按要求落实。
- AC-1..AC-6 全部覆盖（AC-4 弱版，已派生说明）。
- verify_all 0 delta，27 用例全 PASS。
- 范围严格未蔓延。

## 9. 给 PM 的后续动作清单

1. **强烈推荐本任务窗口内修**：P1-2 加 6 行 `TestNewDownloadHTTPClient_NoTotalTimeout` 单元测试。
2. **建议**：P1-1 修注释路径（双路径或纯任务 ID 引用）。
3. **(QA 阶段提醒)** 06 标题用裸 `## Adversarial tests`（L19/L30/L43）。
4. **(QA 阶段提醒)** 06 中显式说明：T1 是"链路通畅性 + AC-6 推进"证据；"60s 总超时不再生效"靠字段值 + Reviewer 字面核验联合保证。
5. **(PM Stage 7 提醒)** 07 `## Insight` 段用裸标题、无数字前缀。
6. **(PM Stage 7 提醒)** 归档时校对 downloader.go helper 注释中的路径同步。
