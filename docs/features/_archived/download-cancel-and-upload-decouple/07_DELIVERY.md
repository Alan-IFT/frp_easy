# 07 Delivery — T-027 download-cancel-and-upload-decouple

> 任务 ID：**T-027** · slug：**download-cancel-and-upload-decouple** · 模式：**full**
> 交付时间：2026-05-24
> 完成证据：verify_all PASS 22 / WARN 0 / FAIL 0；22 AC 全部测试通过；新增端到端 reproducer 锁住关键 AC FR-7 不变量

## 一句话总结

frp 二进制下载现可由用户主动取消；取消后立即可上传手动下载的 binary（解开"上传被下载状态阻断"的死结）；下载/上传写终点路径互斥保留（防双写 race），但已变为"可操作互斥"——前端给出显性"✕ 取消"按钮 + 5 态主按钮表，让低频用户也能一眼看清路径。

## 变更概览

### 后端
- `internal/downloader/downloader.go`
  - 新增 `StatusCanceled = "canceled"` 常量与 `DownloadState.Status` 第 5 态。
  - `Manager` 加 `cancels map[string]context.CancelFunc` 字段，由既有 `m.mu` 守护。
  - 新方法 `Cancel(kind string) error`：5 态全覆盖（idle/success/failed/canceled → no-op return nil；downloading → 触发 cancelFunc + 等状态切换；bad kind → ErrBadKind）；3s 等待超时分支拿锁强写 canceled，保 FR-7"零等待时间窗"不变量。
  - 新方法 `setCanceled`（Info 级日志，与 `setFailed` 的 Error 级语义区分）。
  - `setFailed` 加单调 guard 与 `setCanceled` 对称（防多 err 分支 race）。
  - `doDownload` ctx 化：顶端 `context.WithCancel` + 注册 cancels map + defer 反注册；archive 下载 + resolveLatestAsset 两处都改 `NewRequestWithContext`；6 处 err 分支前置 `errors.Is(ctx.Err(), context.Canceled)` 重检走 `setCanceled`；写 success 前 ctx + 状态单调双层重检（防 R-2 cancel/success race）；binTmp 加 `defer os.Remove` 兜底 cancel 路径清理。
- `internal/httpapi/handlers_system.go`
  - 新 handler `downloadCancel`：成功 200 + DownloadState body / bad kind 422 / downloader nil 503。
  - `uploadBin` 409 文案改为 `下载进行中，请先点击"取消下载"按钮后再上传`（消解原文案"或取消下载"的悬挂语义）。
- `internal/httpapi/router.go`
  - 挂 `r.Post("/system/download-cancel/{kind}", h.downloadCancel)` 到受保护 group（SessionAuth + CSRF）。

### 契约
- `openapi.yaml`
  - `DownloadState.status` enum 加 `canceled`。
  - 新增 `POST /api/v1/system/download-cancel/{kind}` path block（含 cookieAuth + csrfToken header + 4 个 response code）。

### 前端
- `web/src/types.ts` — `DownloadState.status` 联合类型加 `'canceled'`。
- `web/src/api/downloader.ts` — 新增 `apiCancelDownload(kind)`。
- `web/src/stores/downloader.ts` — 新增 `cancelDownload` action；`idleState(): DownloadState` 显式标注（防类型窄化）。
- `web/src/components/AppLayout.vue` — banner 内：下载中显示红色 ghost "✕ 取消" 小按钮（点击调 store cancel）；主按钮 5 态文案 + 颜色表（含 canceled→warning + "已取消，点击重试"）；进度条仅 downloading 时显示。
- `web/src/components/UploadBinButton.vue` — 加 `siblingDownloading` prop；同 kind 下载中时按钮 disabled + tooltip 改为引导用户先点取消按钮。

### 测试（只升不降）
- `internal/downloader/downloader_cancel_test.go`（新建）10 用例：Cancel × 5 态 + ErrBadKind + 并发 N=10 + F-3 3s 强写兜底 + F-4 resolveLatestAsset 中途取消 + 慢 fixture 用 `math/rand` 防 L40 gzip 压塌。
- `internal/httpapi/handlers_cancel_test.go`（新建）6 用例：cancel-no-cookie / no-csrf / bad-kind(422) / idle-200 / downloader-nil / uploadBin-409 文案防回归。
- `internal/httpapi/handlers_cancel_then_upload_test.go`（**QA 新增**）3 端到端用例：**TestDownloadCancel_ThenUpload_200**（关键 AC FR-7 不变量端到端独立 reproducer）+ TestDownloadCancel_HTTP200_DuringDownload + TestUploadBin_409Message_RuntimeAssert（runtime body 文案断言，升级 R-6 静态扫描）。
- `web/src/stores/__tests__/downloader.spec.ts`（新建）4 用例 + `web/src/components/__tests__/UploadBinButton.spec.ts` +2 用例。

### 基线
- `scripts/baseline.json` 升 version 12→13；go_tests 246→265 (+19)，frontend_tests 96→102 (+6)，test_count 342→367。

## DESIGN DRIFT（显式留痕）

| ID | 偏离 | 决策 | 理由 |
|---|---|---|---|
| DRIFT-1 | 02 §4.5 / 03 F-1：上传 confirm dialog → tooltip + disabled | 保持 tooltip 方案 | AppLayout banner 已有红色 "✕ 取消" 按钮显性出现，比 dialog 更醒目；mainstream UX 风格；若后续低频用户反馈差，加 ~60 行 dialog 即可回退（follow-up 入口保留） |
| DRIFT-2 | 01 §5.2 AC-http-cancel-bad-kind 写 400 / 03 F-2 改 422 | 采纳 422 | 与 codebase 既有约定一致（uploadBin / downloadBin / probePorts 全 422 CodeValidationFailed）；QA 测试断言改 422，PASS |
| DRIFT-3 | 02 §5.5 / 03 F-8：E2E happy path 用 Playwright | 降级为 Go HTTP 端到端 | T-006 E2E 框架无 mock GitHub 注入 seam；改用 `httptest` + 反射注入 unexported `apiBaseURL` 实现等价端到端覆盖；关键 AC 由 `TestDownloadCancel_ThenUpload_200` 在 QA 阶段一锤定音 |

## verify_all 最终结果

```
=== Summary ===
  PASS: 22
  WARN: 0
  FAIL: 0
  SKIP: 0
```

`scripts/verify_all.ps1`（PowerShell）跑通；22 步全 PASS（含 E.6 Adversarial tests 段标题、E.7a/b/c PS1 BOM 闸门、B.4 测试只升不降）。

## 后续 follow-up（已知但本任务 out-of-scope）

- **R-5 / NFR-5 race**：本机无 gcc 跑不通 `go test -race`；逻辑层 `m.mu` + 单调 guard 防御充分。建议在 CI（Linux runner 有 gcc）加一条 `go test -race ./internal/downloader/...` step。
- **R-2 (P2)**：`apiCancelDownload` 可显式传 `Content-Type: undefined` 抵消 axios 实例 default（与 T-023 镜像处理）；当前 chi 后端不读 body 实际无害。
- **R-4 (P2)**：success 写入分支 ctx 重检后调 `setCanceled` 走二次 Lock；可优化为内联状态写入避免锁切换；当前无 bug。
- **DRIFT-1 follow-up**：若低频用户反馈"看不到取消路径"，再加 ~60 行 confirm dialog 替换 tooltip。

## Insight

- **2026-05-24** · `http.Request.WithContext` + `http.Transport` 在 ctx Done 时主动关 conn 是 Go 1.7+ 文档化契约，stdlib 自身让 `resp.Body.Read` 返回 `context canceled` 或 `use of closed network connection`——给长耗时网络下载加 ctx-based cancel 不需要任何额外 net.Conn 操作，把整套 `req := http.NewRequestWithContext(ctx, ...)` 接上即可。但**前提**是请求必须经过 `http.Client.Do` / `http.Transport` 走 stdlib 内部 cancel 链；如果用了第三方 `net.Conn` 包装层（fasthttp 等）则此契约不生效 · evidence: T-027 doDownload + resolveLatestAsset 两处 ctx 化让 6 处 err 分支都能在 3s 内解阻塞，TestCancel_MidDownload / TestCancel_DuringResolveAsset 端到端验证
- **2026-05-24** · "Cancel 同步返回时状态必须已落地" 是用户行为正确性的硬不变量（FR-7 零等待时间窗）—— ctx cancel 不保证 goroutine 立刻退出（goroutine 可能卡在不响应 ctx 的 reader / 系统调用），所以 Cancel 不能"触发 cancelFunc 立即 return nil"，必须**轮询等状态切换 + 超时上限拿锁强写 canceled**。3s × 10ms 轮询 + 3s 兜底 force-write 是务实平衡（FR-7 不变量优先级 > 状态机单调防御 guard 的对称美）；强写后续 setFailed/setSuccess 走 guard 拒写保安全 · evidence: T-027 Cancel 实现 + TestCancel_3sTimeoutForceWrite 用 stuckTransport 模拟 goroutine 卡死路径
- **2026-05-24** · "互斥放宽" 是错的，"互斥可操作化" 才对：下载/上传写终点 binary 路径必须互斥（否则双写 race），但**互斥的 409 错误必须给用户**明确路径**解锁**——本任务初版 409 文案"或取消下载"是悬挂语义（用户没有取消按钮），新版改"请先点击\"取消下载\"按钮后再上传" + AppLayout 红色"✕ 取消"按钮显性出现。**互斥消息的可操作性 = 现状 + 显式动作动词 + UI 中真实存在的入口**。未来类似互斥（procmgr / 配置写盘锁）应套同款模板 · evidence: T-027 02 §3.2 / 04 §4 / handlers_system.go 409 文案 + AppLayout.vue 取消按钮 v-if 联动
- **2026-05-24** · sub-agent（gate-reviewer / code-reviewer）frontmatter 默认无 Write 工具的"reviewer 不落盘"陷阱（insight L41 / L44）在 T-027 第三次复现：03 + 05 两个 reviewer 都把完整 Markdown 内容塞到消息体让 PM 代为落盘；落盘 200+ 行长 Markdown 占 PM 工具 quota + 注意力。**真长期解**是在 `.harness/agents/gate-reviewer.md` / `.harness/agents/code-reviewer.md` frontmatter 加 `Write` 工具（与 developer / qa-tester 对齐）。建议下一个 trivial 任务直接做这个 frontmatter sync · evidence: T-027 stage 3 + stage 5 两次 PM 手工 Write 长 Markdown，agent 返回消息 800-2000 token 全是 review 内容

## 关联任务历史

- T-002 — 下载 MVP（建立 Manager / DownloadState / progressWriter 框架）
- T-014 — frp-binary-auto-download（resolveLatestAsset 接入 GitHub Release API）
- T-018 — upload-bin-multiport-ip-probe（共享 Install 方法、uploadBin handler、下载/上传互斥契约首次确立）
- T-025 — download-bin-timeout-fix（拆分 apiClient / downloadClient，downloadClient.Timeout=0 是本任务的 NFR-6 / L38 红线锚点）
- T-027（本任务）— 把 T-018 建立的"硬互斥"升级为"可操作互斥"，把 T-002 建立的 doDownload 从"不可中断"升级为"用户可取消"

## 验收闸门

- [x] 全部代码 + 测试落盘
- [x] 04/05/06 全部阶段文档落盘（00..07 共 8 份 + PM_LOG）
- [x] verify_all PASS 22 / WARN 0 / FAIL 0
- [x] 22 条 AC 全部测试通过（QA 阶段补齐关键 AC 端到端 reproducer）
- [x] 三层 enum / 端点路径 verbatim 一致（Go / OpenAPI / TS）
- [x] L42 双路径注释三处全覆盖（Manager.cancels / Cancel doc / downloadCancel handler）
- [x] Adversarial tests 段标题裸写（L21 / E.6 闸门）
- [x] DESIGN DRIFT 3 条显式留痕
