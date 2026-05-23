# T-027 · 01_REQUIREMENT_ANALYSIS — download-cancel-and-upload-decouple

> 任务 ID：**T-027** · slug：**download-cancel-and-upload-decouple** · 模式：**full**
> 报告时间：2026-05-23
> 上游 PM 决策方向：**候选 A — 加 Cancel API + UI 取消按钮**（见 `PM_LOG.md` §PM 决策结论，已 decision lock）

---

## §1 背景与问题陈述

### 1.1 用户原话（2026-05-23）

> frp 下载过程中，当前版本无法取消下载；就会出现网络很差时，下载很慢，人工去 git 下载好了，但又无法上传上去，因为上传完会提示下载进行中，导致上传失败；以用户体验好，符合软件工程标准，长期易使用易维护为原则来决策；你来决策就可以了，我只看结果是否符合需求。

### 1.2 问题链路（事实）

1. 后端 `internal/downloader/downloader.go` 的 `doDownload` 启动 goroutine 后**无 ctx**，archive 拉取卡在 `io.Copy(archiveTmp, teeReader)` 时无法外部中断 —— `Manager` 没有 `Cancel` 方法。
2. 后端 `internal/httpapi/handlers_system.go` line 466-469 上传链路硬阻断：`if st.Status == downloader.StatusDownloading → 409 PROC_BUSY "下载进行中，请稍后再上传或取消下载"`。
3. 前端 `web/src/components/AppLayout.vue` 与 `UploadBinButton.vue` 没有任何"取消下载"入口；提示语指向一个不存在的能力，用户陷死局。

### 1.3 PM-DECIDED 方向（一句话）

**补齐异步任务"启动 / 状态查询 / 取消"三件套；保留下载 / 上传互斥但变成"可操作互斥"——用户可一键取消下载然后立刻上传。**

---

## §2 用户故事

- **U-1**（取消正在跑的下载）：作为正在等待网络很差的 frp 下载完成的用户，我希望能一键中止该下载，以便释放下载锁、不必关闭整个 Web UI 或重启服务。
- **U-2**（取消后立刻上传）：作为已经手动从 GitHub 下载好 frpc/frps 二进制的用户，我希望取消后端下载后**立刻**上传本地文件成功，以便不再被"下载进行中"的提示阻断。
- **U-3**（取消后重试下载）：作为取消了一次失败下载的用户，我希望仍能在同一会话里再次发起一键下载（比如换了代理后重试），以便不必刷新页面就能继续工作。
- **U-4**（上传时一步取消下载）：作为点击"上传 frpc"的用户，如果同 kind 下载正在跑，我希望提示信息直接提供"取消下载"的操作入口（或先 cancel 再 retry upload），以便不必先去仪表盘另一处找按钮。
- **U-5**（取消后状态视觉明确）：作为取消下载的用户，我希望按钮 / 状态文案明确显示"已取消"（不是"失败"），以便我清楚区分"我主动取消"与"网络/解压出错"。

---

## §3 功能需求（FR）

### 3.1 后端核心契约

**FR-1 `Manager.Cancel(kind string) error` 新方法**

- 入参：`kind ∈ {"frpc", "frps"}`，其它值返回 `ErrBadKind`。
- 行为：
  - 当 `state.Status == StatusDownloading` → 触发该 kind 关联 ctx 的 cancel，`doDownload` goroutine 在 ≤3s 内（NFR-1）退出，状态机置为新值 `StatusCanceled`，progress 保留取消那一刻的值（不强制清零），error 字段写"用户取消下载"。**返回 `nil`**。
  - 当 `state.Status ∈ {StatusIdle, StatusSuccess, StatusFailed, StatusCanceled}` → **no-op**（不报错），返回 `nil`。语义："cancel 是一个**意图**，不是状态变更命令；对已停止的下载 cancel 等价于"确认未在跑""。
- 并发：
  - 重复 cancel（同 kind 多次连续调用）→ 第二次起均为 no-op，不报错，不抛 race。
  - 与 `Start` 并发：`Cancel` 不阻塞 `Start`，但 `Start` 在同 kind `StatusDownloading` 时仍返 `ErrAlreadyInProgress`（不变）。
  - 与 `doDownload` 内部 setStatus 并发：cancel 后 doDownload 的"成功路径"必须感知 ctx 已取消，**不能**把已经 canceled 的 state 再写回 success（见 R-2，由 Architect 决定具体策略 — 推荐 ctx 检查 + 状态机单调转换 guard）。

**FR-2 新状态值 `StatusCanceled = "canceled"`**

- 与 `StatusFailed` 区分语义：failed = 网络/解压/落盘失败；canceled = 用户主动 `Cancel` 触发。
- 与 `StatusIdle` 区分：canceled 表示"曾经发起过、被用户中止"，UI 可据此显示"已取消，可重试"按钮；idle 是"从未发起"。
- 重新调用 `Start(kind)` 时：从 `StatusCanceled` 进入 `StatusDownloading` 必须成功（**不**视作 ErrAlreadyInProgress）。

**FR-3 `doDownload` ctx 化**

- `doDownload` 必须接受 `context.Context`（或从 `Manager` 字段读取），并把 ctx 注入：
  - `http.NewRequestWithContext(ctx, ...)` 让 `m.downloadClient.Do(req)` 在 ctx Done 时主动关闭 conn / 解除 `io.Copy` 阻塞。
  - body 拷贝可走 ctx 感知的 reader（实现方式由 Architect 决定 — stdlib 的 ctx 已能让 `resp.Body.Read` 在连接 close 后返 err）。
  - GitHub release API 探测（`resolveLatestAsset`）也应 ctx 化以保证取消时 API 阶段也能立即解除。
- archive 临时文件（`.dl-archive-*.tmp`）已有 `defer os.Remove`，必须验证 cancel 路径不会绕过该 defer（goroutine 自然返回即可触发，但 Architect 必须设计单测覆盖）。

### 3.2 新 HTTP endpoint

**FR-4 `POST /api/v1/system/download-cancel/{kind}`**

- 方法：**POST**（写动作、需 CSRF；与 procStop / procRestart 风格一致）。
- 路径：`/api/v1/system/download-cancel/{kind}`，`kind ∈ {frpc, frps}`。
- 鉴权：cookieAuth + CSRF（与 download-bin 同分组，受保护）。
- 入参：仅 path 参数 `kind`；无 body。
- 返回码：
  - **200**：返回 `DownloadState` JSON（取消后的最新状态，便于前端立刻同步而不必再轮询一拍）。
  - **400**：kind 非法（不是 frpc/frps）。
  - **401**：未登录。
  - **403**：缺 CSRF token。
  - **503**：Downloader 未初始化。
- **不返回 404 / 409**：cancel idle 状态 = 200（与 FR-1 一致，no-op 也是成功）。

**FR-5 OpenAPI 契约一份**（L29 / L40 漂移防御）

- `openapi.yaml`：
  - `DownloadState.status` enum 扩展为 `idle | downloading | success | failed | canceled`。
  - 新增 `/api/v1/system/download-cancel/{kind}` path 条目，schema 复用 `DownloadState`。
- 前端 `web/src/types.ts`：`DownloadState.status` 类型扩展为同 5 个字面量。
- 前端 `web/src/api/downloader.ts`：新增 `apiDownloadCancel(kind)` 函数，路径与方法严格匹配。

### 3.3 上传互斥行为收紧

**FR-6 上传时下载状态检查的语义不变 + 错误消息精化**

- `uploadBin` 仍在 `st.Status == StatusDownloading` 时返 409 PROC_BUSY（不变 — 这是防"下载/上传写同一 binary 路径"的最后防线）。
- 但 **error.message 应明确指向解决路径**："下载进行中；请先点击'取消下载'按钮后再上传"。
- `StatusCanceled` / `StatusFailed` / `StatusSuccess` / `StatusIdle` 状态下上传必须成功（不应触发 409）。

**FR-7 Cancel 完成到 upload 解锁的时间窗 = 0**

- 设计层面要求：`Cancel(kind)` 返回时（HTTP 200 返回前），state.Status 必须已置为 `StatusCanceled`；不允许返回成功但 state 仍是 `downloading` 的中间态。
- 这保证前端"先调 cancel 等 200 → 再调 upload-bin"序列必然成功，**不依赖任何 sleep / retry 重试**。

### 3.4 前端按钮状态机

**FR-8 五状态按钮表（同一 kind 下，在 AppLayout banner 中）**

| state.status | 主按钮文案 | 主按钮 type | 主按钮 disabled | 取消按钮 | 进度条 |
|---|---|---|---|---|---|
| `idle` | `一键下载 {kind}` | primary | false | 隐藏 | 隐藏 |
| `downloading` | `下载中... {progress}%` | primary | true（loading 态） | **显示**："取消"（default type，size=small） | 显示，颜色 primary |
| `success` | `已下载` | success | true | 隐藏 | 隐藏 |
| `failed` | `重试` | error | false（点击重新 Start） | 隐藏 | 隐藏 |
| `canceled` | `重试` | warning | false（点击重新 Start） | 隐藏 | 隐藏 |

- 取消按钮点击 → 调 `apiDownloadCancel(kind)` → 成功后立即把本地 state.status 置 `canceled`（不等下一拍轮询），同时调 `stopPolling(kind)`。
- 进度条在 `canceled` 状态隐藏（避免歧义"取消了但条还在")。

**FR-9 UploadBinButton 行为微调**

- 上传按钮在同 kind `state.status == "downloading"` 时：
  - 允许点击文件选择器（不强禁用 — 用户可能想先选好文件再决定）。
  - 文件选好后真正 POST upload-bin 之前，前端**额外**展示一个 confirm dialog："同 kind 下载正在进行，是否先取消下载再上传？"（按钮：取消下载并上传 / 仅上传 / 放弃）。
  - "取消下载并上传"：先调 `apiDownloadCancel(kind)` → 等返回 200 → 再调 `apiUploadBin`。
  - "仅上传"：直接调 `apiUploadBin`，由后端返 409 + 精化后的消息（FR-6）。
  - "放弃"：关闭 dialog，不做任何 API 调用，清空选中的 file。
- 上传按钮在 `idle / success / failed / canceled` 状态下行为不变。

**FR-10 Downloader store 状态机扩展**

- `web/src/stores/downloader.ts`：
  - `idleState()` 已是 `{ status: 'idle', progress: 0 }`，无需改。
  - 新增 action `cancelDownload(kind)`：调 `apiDownloadCancel(kind)` → 成功后立即赋值 `this[kind] = { status: 'canceled', progress: 当前 progress, error: '用户取消下载' }`，并 `stopPolling(kind)`。
  - `isDownloading` getter 不变（canceled 不算 downloading）。

---

## §4 非功能需求（NFR）

**NFR-1 取消响应时延**

- 从前端点击"取消"按钮到 `apiDownloadStatus` 观测到 `status === 'canceled'`，端到端 **≤ 3 秒**（包含 HTTP 请求 + ctx 传播 + conn 关闭 + state 写入 + 下一拍轮询）。
- 后端 `Cancel()` 同步返回时 state 必须已是 canceled（FR-7），所以"轮询滞后"上限实际由前端 1s 轮询周期决定。

**NFR-2 双平台兼容**

- Windows 10/11 + Linux（amd64）下 ctx cancel 行为必须一致；不允许出现"Linux cancel 即时生效 / Windows cancel 卡死"或反之。
- 验证手段：`go test ./internal/downloader -run TestCancel` 在 CI 双平台 matrix（已有 verify_all）下均 PASS。

**NFR-3 service 模式兼容**

- Web UI 在 Windows service 模式 / Linux systemd service 模式（无 stdout）下，cancel 功能必须正常工作（参考 L26 / L27）。
- 不允许在 Cancel 路径里写任何 `fmt.Print*` / `os.Stdout` 交互；日志走 `m.logger`（已是 `*slog.Logger`）。

**NFR-4 已部分下载的临时文件清理**

- Cancel 触发后，`.dl-archive-*.tmp` / `.dl-bin-*.tmp` / `.install-*.tmp` 必须被清理（goroutine 退出时 defer 执行）。
- 不允许残留 tmp 占用磁盘 — 单测必须 assert `os.ReadDir(targetDir)` 不含 `.dl-*` 前缀文件。

**NFR-5 重入安全**

- 同 kind `Cancel` 在以下场景必须 race-free（go test -race PASS）：
  - 同时 N 个 cancel 调用（N=10）。
  - Cancel 与即将完成的 doDownload "刚好同时" 写状态。
  - Cancel 与新一次 Start 的快速序列（cancel → start → cancel → start，连续 5 轮）。

**NFR-6 与 T-025 archive client 决策的兼容**

- 不得回退到 `http.Client.Timeout > 0` 作为取消手段（L38）。取消信号必须**纯走 ctx**；client.Timeout=0 保持。

---

## §5 验收准则（AC）

### 5.1 后端 Go 单测

**AC-cancel-mid-download**
- 用 `httptest.NewServer` 起一个慢响应 server（response body 写 1 字节后 sleep 30s）。
- `Manager.Start("frpc")` → 等 status 变 downloading → 调 `Manager.Cancel("frpc")`。
- 在 3s 内 assert：
  1. `Status("frpc").Status == "canceled"`。
  2. `targetDir` 下不存在 `.dl-archive-*.tmp` / `.dl-bin-*.tmp` 残留。
  3. 慢 server 收到的 conn 已被 client 端关闭（server-side `r.Context().Done()` 触发）。
- `go test -race` PASS。

**AC-cancel-idle-noop**
- 全新 Manager，从未 Start → 调 `Manager.Cancel("frpc")` → 返回 `nil`，state 保持 `{Status: "idle", Progress: 0}`。

**AC-cancel-success-noop**
- Start → 等下载完成（mock 立即 200 + 合法 archive）→ status==success → 调 Cancel → 返回 nil，status 仍是 success（不被覆盖）。

**AC-cancel-after-failed-then-restart**
- Start → 模拟 failed（HTTP 500）→ Cancel（no-op）→ 再 Start → 必须能进入 downloading（不被 `ErrAlreadyInProgress` 阻断，因为 state 已 failed）。

**AC-cancel-then-restart-from-canceled**
- Start → Cancel → state==canceled → 再 Start → 必须能进入 downloading（state 转移 canceled→downloading 合法）。

**AC-cancel-repeated-noop**
- Start → Cancel × 5（连续）→ 仅第一次实际触发 ctx done；后续返回 nil，state 保持 canceled。`go test -race` PASS。

**AC-cancel-bad-kind**
- `Cancel("frpx")` → 返回 `ErrBadKind`。

### 5.2 HTTP / curl

**AC-http-cancel-200**
- 起 Start 在跑 → `curl -X POST .../system/download-cancel/frpc` 带 cookie+CSRF → 期望 HTTP 200，body 是合法 DownloadState JSON，`status == "canceled"`。

**AC-http-cancel-then-upload-200**
- Start 在跑 → POST download-cancel/frpc → 200 → 立刻 POST upload-bin（合法 binary）→ 期望 HTTP 200（不应再 409）。**关键 AC**。

**AC-http-cancel-idle-200**
- 从未 Start → POST download-cancel/frpc → 期望 HTTP 200，body 中 status==idle（state 未变）。

**AC-http-cancel-bad-kind-400**
- POST download-cancel/frpx → 400，error.code == VALIDATION_FAILED。

**AC-http-cancel-no-csrf-403**
- POST 不带 X-CSRF-Token → 403。

**AC-http-cancel-no-cookie-401**
- POST 不带 session cookie → 401。

**AC-http-cancel-downloader-nil-503**
- Downloader 注入为 nil（test deps）→ POST → 503。

**AC-http-upload-during-download-message-updated**
- Start 在跑 → POST upload-bin 不先 cancel → 期望 409，且 `error.message` 包含"请先点击'取消下载'按钮"或"先取消下载"字样（FR-6）。

### 5.3 OpenAPI / 前端契约一致性

**AC-openapi-cancel-path-present**
- `openapi.yaml` 必须包含 `/api/v1/system/download-cancel/{kind}` path 条目，且 `DownloadState.status` 描述含 "canceled"。
- verify_all 中已有的 OpenAPI 校验（如有）应 PASS。

**AC-types-status-canceled-present**
- `web/src/types.ts` 的 `DownloadState.status` 字面量类型必须包含 `'canceled'`。
- `npm run typecheck` PASS。

### 5.4 前端单测（Vitest）

**AC-store-cancel-action**
- mock `apiDownloadCancel` 返回 `{status: 'canceled', progress: 42}` → 调 `store.cancelDownload('frpc')` → assert：
  1. `store.frpc.status === 'canceled'`。
  2. `store._timers['frpc']` 已被清空（stopPolling 触发）。

**AC-ui-cancel-button-visible-when-downloading**
- mount AppLayout with `binMissing: ['frpc']` + `downloaderStore.frpc.status='downloading'` → 取消按钮存在、可点击；主按钮 disabled。

**AC-ui-cancel-button-hidden-when-not-downloading**
- 同上但 status 切到 `canceled` / `success` / `failed` / `idle` → 取消按钮不存在。

**AC-ui-upload-confirm-dialog-when-downloading**
- mount UploadBinButton with parent 注入"同 kind status=downloading" 上下文 → 选择文件 → 期望 dialog 出现，含三个按钮（取消下载并上传 / 仅上传 / 放弃）。

### 5.5 E2E（playwright，如已有 T-006 框架）

**AC-e2e-cancel-then-upload-happy-path**
- 用 mock GitHub API 让下载卡 30s → 点"一键下载 frpc" → 进度条出现 → 点"取消" → 取消按钮消失、按钮文案变 "重试"（warning 色）→ 选择本地合法 binary 上传 → 上传成功 toast。**关键 E2E**。

---

## §6 范围 / 非范围

### 6.1 In-scope

- 后端 `Manager.Cancel(kind)` + ctx 化 doDownload + `StatusCanceled` 新值。
- HTTP `POST /api/v1/system/download-cancel/{kind}`。
- OpenAPI + 前端 types + api client + store + UI 全链路 5 状态按钮 + cancel 按钮 + upload confirm dialog。
- 上传 409 错误消息精化。
- 单测 / HTTP 测试 / 前端 store 测试 / 至少 1 个 E2E happy path。
- T-025 archive client `Timeout=0` 不动；取消信号纯走 ctx。

### 6.2 Out-of-scope（不在本次迭代）

- **批量取消**：不实现"一键取消所有 kind 的下载"接口；调用方自己循环两次 cancel。
- **服务端推送 / WebSocket**：进度同步仍用现有 1s 轮询；不引入 SSE / WS。
- **多用户 / 多会话隔离**：当前 Manager 是 process-scoped 单例；不区分"用户 A 取消 vs 用户 B 取消"。session 级隔离不在本次。
- **下载暂停 / 续传**：cancel 是 hard cancel，不支持 pause/resume。
- **取消已 success / failed 的恢复**：cancel 不回滚已完成的安装（即使上一次 Start 已经原子 rename 完成，cancel 也不删 binary）。
- **超时自动取消**：不引入"下载超过 N 分钟自动 cancel"的策略 — 这是 T-025 已经回答过的方向（不要回退到 Client.Timeout）。
- **frpc admin / proxies / proc 子系统**：本任务只动 downloader + upload-bin + 前端 banner，不动其它模块。
- **配置项 / 设置页改动**：不新增 settings 项。

---

## §7 风险

**R-1：ctx cancel 后 `net.Conn.Read` 是否真正解除 `io.Copy` 阻塞**
- 风险描述：Go stdlib `http.Client` + `req.WithContext(ctx)` 在 ctx Done 时会调用 `Transport.CancelRequest` / 关闭底层 conn；`resp.Body.Read` 因 conn 关闭返 `read: use of closed network connection`，从而 `io.Copy` 解除。但若网络栈处于某些特殊状态（如 keep-alive idle 期），cancel 传播延迟可能 > 几百 ms。
- 提示 Architect：方案设计需明确：(a) `m.downloadClient.Transport` 已配置 `ResponseHeaderTimeout=60s`；(b) `req = req.WithContext(ctx)` 后 stdlib 在 ctx Done 时主动关 conn 是契约行为（Go 1.7+），cancel 传播时间在 ms 级，远小于 NFR-1 的 3s 上限；(c) 单测应使用 `httptest.Server` + 慢响应直接验证，不依赖真实网络。

**R-2：状态机竞态 — Cancel 与 doDownload 即将写 success 的 race**
- 风险描述：doDownload 末尾的 `m.states[kind].Status = StatusSuccess` 与外部 Cancel 的 `m.states[kind].Status = StatusCanceled` 都持 `m.mu.Lock()`，但**顺序**未定。最坏情况：cancel 拿锁置 canceled → doDownload 拿锁覆盖回 success，于是"取消"被悄悄吞掉。
- 提示 Architect：方案设计需明确：(a) doDownload 末尾在 lock 内**先检查 ctx.Err() != nil**，若是则 **不**写 success（因为 cancel 已发生）；(b) 或采用单调状态机：从 `canceled` 状态不允许回到 `success`（写 success 前 assert state == downloading）。推荐 (a)+(b) 兼用。

**R-3：前端轮询周期与状态转换的"短暂回弹"**
- 风险描述：用户点击取消 → 前端立刻把 store 置 canceled → 1s 内轮询返回旧 state（如果 cancel HTTP 200 与下一拍 status 200 顺序反了），可能短暂把 canceled 又"回弹"成 downloading。
- 提示 Architect：方案设计需明确：(a) FR-7 保证 cancel 返回时 state 已是 canceled，所以服务端不会再返 downloading；(b) 前端在收到 cancel 200 后立即 `stopPolling`，避免任何后续轮询覆盖；(c) 若仍担心 race，可在 store 侧引入 `cancelToken: number` 单调递增，轮询响应若 token 滞后则丢弃。

**R-4：上传 dialog 增加用户路径长度**
- 风险描述：FR-9 的 confirm dialog 增加一次点击，可能让"已经知道自己要先取消"的高级用户嫌烦。
- 缓解：dialog 默认聚焦"取消下载并上传"按钮，回车即可一键完成；不勉强用户。

**R-5：兼容性 — 旧前端读到带 `canceled` 状态的后端响应**
- 风险描述：用户刷新慢 / 多 tab 场景下，旧前端代码 `DownloadState.status` 联合类型不含 canceled，TypeScript 会跳过，但视觉上按钮可能"卡"在 downloading 文案。
- 缓解：本任务上线后用户刷新页面即可拿到新前端；不构成 release blocker。Architect 在 02 中明确不引入向后兼容层。

---

## §8 与历史任务关联

| 任务 | 关联点 | 必读路径 |
|---|---|---|
| **T-002** zero-config-quickstart | 下载 MVP 基础：Manager / DownloadState / progressWriter 形态由它定 | `docs/features/_archived/zero-config-quickstart/02_SOLUTION_DESIGN.md` |
| **T-014** frp-binary-auto-download | `resolveLatestAsset` GitHub release API；本任务 ctx 化时要一起改 | `docs/features/_archived/frp-binary-auto-download/02_SOLUTION_DESIGN.md` |
| **T-018** upload-bin-multiport-ip-probe | uploadBin / Install 共享路径；下载-上传互斥逻辑在 §A.2 定义 | `docs/features/_archived/upload-bin-multiport-ip-probe/02_SOLUTION_DESIGN.md` §A.2 |
| **T-025** download-bin-timeout-fix | `downloadClient.Timeout=0` 决策；本任务**不**回退该决策，cancel 走 ctx | `docs/features/_archived/download-bin-timeout-fix/02_SOLUTION_DESIGN.md` §6 R-1 |
| **T-023** upload-bin-content-type-fix | apiClient default Content-Type 污染 FormData 教训；本任务新增 `apiDownloadCancel` 走 JSON POST（无 body），不踩同坑 | trivial 修复，无 02 |

---

## §9 PM / Architect 自治决策项

> 用户明示"你来决策就可以了"，以下不再回问用户；标注归属。

1. **Cancel HTTP 方法选 POST 而非 DELETE**（decided by PM）：与 procStop / procRestart 风格统一；DELETE 在 chi 路由器中 CSRF 中间件配置一致，但 POST 更符合"动作"语义。
2. **新状态名选 `canceled` 而非 `cancelled` / `aborted` / `stopped`**（decided by PM）：与英文主流单词形态一致（美式拼写，与 Go ctx `context.Canceled` 同源），避免拼写漂移。
3. **取消按钮放在主按钮右侧 inline，而非合并为"下载中... 点击取消"单按钮**（decided by PM）：双按钮明确"主操作 vs 二次操作"，避免误点（点进度条想看详情结果取消了）。
4. **上传 confirm dialog 是否引入** —— 引入（decided by PM）：U-4 要求"一步取消"，若不引入 dialog，用户被 409 教训后还要找别处按钮，不符合 U-4。
5. **进度条在 canceled 状态隐藏** vs **保留在取消那一刻的位置**（decided by PM）：隐藏。"已取消"按钮文案已传达信息，进度条停在 42% 容易引起"是不是部分成功"的歧义。
6. **ctx 来源**（decided by Architect）：Manager 持有 per-kind cancelFunc map，还是每次 Start 时创建并存？由 Architect 在 02 设计具体数据结构，本文档不强制。
7. **canceled 状态的 progress 字段值**（decided by Architect）：保留 cancel 那一刻的值 / 强制清零 / 不写。RA 倾向"保留"以便调试，但若 Architect 有更好的理由可在 02 中改。
8. **GitHub API 阶段（resolveLatestAsset）是否也 ctx 化**（decided by Architect）：建议 ctx 化（FR-3 已要求），但若 Architect 评估其耗时 ≤ 60s 不构成"长操作"，可在 02 中说明理由后只 ctx 化 archive 下载阶段。

---

## §10 Verdict

**READY**

- 所有 FR / NFR / AC 均与 PM-DECIDED 方向一致；
- 无未决用户问题（用户明示自治）；
- 决策项均已在 §9 标注归属（PM / Architect），不阻塞 Architect 进入 02 设计阶段。
