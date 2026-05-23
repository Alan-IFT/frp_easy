# 05 Code Review — T-027 download-cancel-and-upload-decouple

> 审查人：code-reviewer sub-agent · 报告时间：2026-05-23
> 输入：01_REQUIREMENT_ANALYSIS / 02_SOLUTION_DESIGN / 03_GATE_REVIEW / 04_DEVELOPMENT
> 工具集：Read / Glob / Grep（无 Write，本内容由 PM 代为落盘 — insight L41/L44 复现）

## Verdict

**APPROVED**

设计与实施严密匹配 02 §2-§4 全部步骤；03 SHOULD-FIX 4 条（F-1/F-2/F-3/F-4）+ NIT 6 条（F-5..F-10）逐条在 04 §2/§3 落地，代码侧逐字核实一致；三层 `canceled` enum / 端点路径 verbatim 一致（Go L35 / openapi L332 / TS L102）；FR-1..FR-10 / NFR-1..NFR-6 / AC×22 全部映射到具体代码位置与测试用例；ctx 化 6 处 err 分支重检 + 单调 guard 双层防御 R-2 完整落地；F-3 强写路径保 FR-7 不变量；F-4 resolveLatestAsset 已 ctx 化；T-025 `downloadClient.Timeout=0` 不变（NFR-6 / L38）。仅发现 5 处 P2 + 3 处 NIT，无 P0 / P1。可直接放行 QA 阶段。

---

## 发现清单

| ID | 严重度 | 文件 | 主题 | 描述 | 修复建议 |
|---|---|---|---|---|---|
| R-1 | P2 | `internal/downloader/downloader.go:228-235` | F-3 强写路径未带 `Progress` 字段、与 `setCanceled` 不一致 | F-3 选项 A 兜底路径写 `Status=Canceled + Error="用户取消下载（goroutine 卡住，强写）"`，但**不写 Progress**。`setCanceled` 路径下 progress 自然保留 cancel 那一刻的值（OQ-2 决策"保留"），强写路径完全等价的语义本应也保留——目前实际就是"保留"（因为没动 Progress 字段）。**语义本身没错**，但缺一行注释说明"Progress 保留不变（与 setCanceled 对称）"会让未来读者怀疑是不是漏写。 | 加 1 行注释 `// Progress 保留 cancel 那一刻的值，与 setCanceled OQ-2 决策对称` 即可。 |
| R-2 | P2 | `web/src/api/downloader.ts:17` | `apiClient.post(url)` 无 body 仍受 default `Content-Type: application/json` 污染 | `web/src/api/client.ts:9` 的 axios 实例 default `headers: { 'Content-Type': 'application/json' }` 让 cancel POST 携带 `Content-Type: application/json`、空 body。后端 chi handler 不读 body、不解析 JSON，因此**实际无害**；但与 insight L37 教训"实例 default 污染所有 per-request"的精神相悖。 | （可选）`apiCancelDownload` 显式传 `{ headers: { 'Content-Type': undefined } }`。当前实现工作正常，QA 不阻塞。 |
| R-3 | P2 | `internal/httpapi/handlers_cancel_test.go:142-162` | AC-http-cancel-then-upload-200（关键 AC）未做"端到端 cancel→upload 200"集成测试 | 04 §5.2 自陈"`AC-http-cancel-then-upload-200`（cancel → upload 200）由 cancel 端点契约 + downloader cancel 行为 + upload 既有 happy path 三层组合保证"。但 01 §5.2 把这条 AC 标为"**关键 AC**"。"组合保证"是逻辑推断而非测试断言。 | QA 阶段可补 1 个 HTTP 集成测试：慢 mock GitHub API → Start frpc → 等 status=downloading → POST download-cancel/frpc 拿 200 → 立即 POST upload-bin 合法 ELF 拿 200。不阻塞当前 review。 |
| R-4 | P2 | `internal/downloader/downloader.go:418-422` | success 写入前 ctx 重检后调 `setCanceled` 但已持锁释放 → 二次 Lock | line 417 `m.mu.Lock()`，line 418 ctx.Err 命中，line 419 `m.mu.Unlock()`，line 420 `m.setCanceled(kind)` —— `setCanceled` 内部再次 `m.mu.Lock()`。**正确性 OK**（无死锁，单 goroutine 顺序拿放锁）。 | 可选优化（不阻塞）：把分支改为内联状态写入避免锁切换；保留现状亦可。 |
| R-5 | P2 | `internal/downloader/downloader_cancel_test.go:423-460` | `TestCancel_Concurrent_N10` 未用 `-race` flag 跑、04 自陈"未跑 -race" | 04 §5.1 表格注释："本机环境无 gcc，未跑 -race 但 mu 守护语义明确"。NFR-5 明确要求"go test -race PASS"。**verify_all PASS 22 是不带 -race 的 baseline**。 | 建议在 `scripts/verify_all.sh`（Linux 端）的 `go test ./internal/downloader/...` 步骤加 `-race`，或单独一个 step。当前本机限制可接受，QA 阶段可在 CI 跑一次确认。 |
| R-6 | NIT | `internal/httpapi/handlers_cancel_test.go:148-162` | `TestUploadBin_409MessageContainsCancel` 是源码静态扫描而非 HTTP 集成断言 | 用 `os.ReadFile("handlers_system.go")` + `strings.Contains` 校验源码文本，没有真起 httptest server。源码扫描足以防回归，但**不能保证文案真的进 response.body**。 | （可选）追加 1 个真起 server + 真触发 409 的集成测试。当前静态防回归已足够。 |
| R-7 | NIT | `web/src/components/__tests__/UploadBinButton.spec.ts:69-93` | T-027 sibling-downloading=true 的 tooltip 文案没有真校验文本 | 注释承认"tooltip 文案不能直接 expect.text()（n-tooltip 默认 hover 才渲染）"。仅校验 disabled 属性 + 按钮主文案不变。"先点击左侧'取消'按钮再上传"这句关键 UX 文案没有任何断言保护。 | 可在 wrapper.html() 里 grep 该字符串，或直接断言 `wrapper.html().includes("先点击左侧")`。 |
| R-8 | NIT | `web/src/stores/downloader.ts:13` | F-7 注释 OK，但 `idleState` 调用方仍是 `idleState()` 直接赋值给 `state.frpc/frps` | F-7 NIT 要求"显式标注 `: DownloadState`"，已落地（line 13）。该处实际上无问题。NIT 记录以收尾。 | 无修复建议，注记已闭合。 |

---

## 评审记录（A-H）

### A. 设计契约一致性

- **A-1 ✅** 02 §11 实施步骤 8 步全部完成：OpenAPI / Go downloader / Go httpapi / TS types / TS api / TS store / Vue AppLayout / Vue UploadBinButton 全 verbatim 一致。
- **A-2 ✅** 03 SHOULD-FIX 4 条全部在 04 §2 + 代码侧落地（F-1 tooltip 保留并标 DRIFT-1、F-2 422+DRIFT-2、F-3 强写路径 line 222-235 + TestCancel_3sTimeoutForceWrite 验证、F-4 resolveLatestAsset 签名改 ctx + 调用 + 测试齐全）。
- **A-3 ✅** 03 NIT 6 条全部采纳。
- **A-4 ✅** DESIGN DRIFT 3 条完整记录附理由与回退成本评估。

### B. Cancel 实现正确性

- **B-1 ✅** `Manager.cancels map[string]context.CancelFunc` 由 `m.mu` 守护，读 / 写 / delete 全在 Lock 区间内。
- **B-2 ✅** `doDownload` 顶端 `context.WithCancel(context.Background())` → 注册 → defer 反注册 + 调 cancel() 释放。
- **B-3 ✅** archive 下载 + resolveLatestAsset 两处都改 `NewRequestWithContext`。
- **B-4 ✅** 6 处 err 分支前置 `errors.Is(ctx.Err(), context.Canceled) → setCanceled` 覆盖完整。
- **B-5 ✅** success 写入前 ctx 重检 + 状态机单调 guard。R-4 列出可优化点但不构成 bug。
- **B-6 ✅** Cancel(kind) 5 态处理完整符合 02 §2.3 + 03 F-3 决策。
- **B-7 ✅** `setCanceled` Info 级日志；`setFailed` Error 级（语义正确：cancel 不是错误）。
- **B-8 ✅** `setFailed` 加单调 guard 与 setCanceled 对称（F-6 采纳）。

### C. HTTP / OpenAPI

- **C-1 ✅** 新路由挂在受保护 Group（SessionAuth + CSRF）。
- **C-2 ✅** 返回码：200 / 422 / 503 / 500 一致。
- **C-3 ✅** uploadBin 409 文案改为 `下载进行中，请先点击"取消下载"按钮后再上传`；旧文案 grep 不命中。
- **C-4 ✅** OpenAPI canceled enum 同步；download-cancel path block 完整。
- **C-5 ✅** 三层共契 verbatim 核实（Go L35 / openapi L332 / TS L102）；端点路径 verbatim 一致。L29 漂移防御闭合。

### D. 前端

- **D-1 ✅** `web/src/types.ts:102` DownloadState.status 加 `'canceled'`。
- **D-2 ✅** `apiCancelDownload(kind)` 路径与方法严格匹配后端。
- **D-3 ✅** `cancelDownload` action + idleState 标注。
- **D-4 ✅** AppLayout 取消按钮 + canceled→warning + 文案 + 进度条隐藏齐全。
- **D-5 ✅** UploadBinButton siblingDownloading 接入 + tooltip 文案分支。

### E. 测试

- **E-1 ✅** Go 10 用例齐全（含 F-3 + F-4 + 并发 N=10）。
- **E-2 ✅** `randomBytes` 使用 `math/rand` 防 L40 gzip 压塌。
- **E-3 ✅** HTTP 6 用例。
- **E-4 ✅** Vitest store 4 + UploadBin spec +2。
- **E-5 ⚠️** 22 AC 全部映射但 AC-http-cancel-then-upload-200（**关键 AC**）由三层组合保证而非端到端集成断言（R-3）。**不阻塞**但 QA 可补。
- **E-6 ✅** 无静默 skip / 删除测试，只升不降。

### F. 注释 / 文档

- **F-1 ✅** L42 双路径注释三处全覆盖。
- **F-2 ✅** 全部中文注释；术语引用清晰可追溯。
- **F-3 ✅** Cancel 方法 doc + 强写分支解释 3s 强写理由 FR-7 不变量。

### G. 红线 / Insight

- **G-1 ✅** `downloadClient.Timeout=0` 明确保留；NFR-6 / L38 兼容。
- **G-2 ✅** 无 secret 硬编码。
- **G-3 ✅** 无 schema / migration / persistence 改动。

### H. 破坏性 / 回归

- **H-1 ✅** verify_all PASS 22 / WARN 0 / FAIL 0；既有 happy path 未变。
- **H-2 ✅** cancels map 访问全在 Lock 区间；CancelFunc 多次调用 idempotent；guard 防御重入。
- **H-3 ✅** Windows + Linux 双平台覆盖；runtime.GOOS 注入 seam 生效；ctx cancel OS-agnostic。

---

## 给 PM / QA 的建议

1. **本次 review 无 P0 / P1，可直接放行 QA**。建议 QA 重点覆盖：
   - **AC-http-cancel-then-upload-200**（**关键 AC**）：手工 curl 一遍真起 cancel → upload 端到端拿 200，弥补 R-3 列出的"组合保证非端到端断言"缺口。
   - **NFR-5 race**：CI 跑一次 `go test -race ./internal/downloader/...`，弥补 R-5 本机 -race 缺位。
2. **PM 后续可考虑加 P3 issue**：R-2 显式 Content-Type / R-4 内联状态写入 / R-6 / R-7 真校验。
3. **L42 双路径注释**已三处落到具体行（F-10 闭合）。
4. **DRIFT-2 (kind 非法 422)** 与 01 §5.2 不一致，QA 06 测试报告应在 AC 表显式说明断言改 422。PM 写 07 时也可同步声明。
5. **insight 收割提醒**：未在 04 §Insight 记录的 dogfood E2E service 冲突陷阱（04 §8.2）应进 07 `## Insight` 段（裸标题、防 L43）。

---

## 评审签字

reviewer：code-reviewer sub-agent
verdict：**APPROVED**
date：2026-05-23
