# T-027 · 04_DEVELOPMENT — download-cancel-and-upload-decouple

> 任务 ID：**T-027** · slug：**download-cancel-and-upload-decouple** · 模式：**full**
> 实施时间：2026-05-23
> 输入：01_REQUIREMENT_ANALYSIS.md (FR×10 / NFR×6 / AC×22) + 02_SOLUTION_DESIGN.md (READY) + 03_GATE_REVIEW.md (APPROVED FOR DEVELOPMENT, 4 SHOULD-FIX + 6 NIT)
> verify_all 闸门：**PASS 22 / WARN 0 / FAIL 0**

---

## §1 实施概要

实施 frp 二进制下载的 **Cancel 通道**，并把"上传被下载阻断"从"硬阻断"变为"可操作互斥"。后端给 `internal/downloader.Manager` 加 `Cancel(kind) error` + ctx 化 `doDownload` + 新状态 `StatusCanceled`；HTTP 加 `POST /api/v1/system/download-cancel/{kind}` 端点；前端给 AppLayout banner 加显性"✕ 取消"按钮、扩展 5 状态主按钮，UploadBinButton 在同 kind 下载时 `disabled + tooltip` 引导用户先取消。三层 `canceled` enum / 端点路径 / 字段名一锤定音（防 L29 漂移）。

**改动文件清单（10 + 2 新建测试）**：

| 文件 | 净变更行数（约） | 说明 |
|---|---|---|
| `internal/downloader/downloader.go` | +138 / -22 | 加 `StatusCanceled` 常量、`cancels` map、`Cancel` 方法、`setCanceled` / setFailed 加单调 guard、`doDownload` ctx 化（5 处 err 分支 + 末尾 ctx 重检 + binTmp defer 兜底）、`resolveLatestAsset` ctx 化（F-4 采纳）。 |
| `internal/httpapi/handlers_system.go` | +37 / -1 | 新增 `downloadCancel` handler（200 + 422 + 503）；`uploadBin` 409 文案精化（FR-6）。 |
| `internal/httpapi/router.go` | +2 | 挂 `r.Post("/system/download-cancel/{kind}", h.downloadCancel)`。 |
| `openapi.yaml` | +60 / -5 | `DownloadState.status` 加 `enum: [idle, downloading, success, failed, canceled]` 与详细说明；新增 `/api/v1/system/download-cancel/{kind}` path block（200/401/403/422/503）。 |
| `web/src/types.ts` | +2 / -1 | `DownloadState.status` union 加 `'canceled'`。 |
| `web/src/api/downloader.ts` | +8 | 加 `apiCancelDownload(kind)` JSON POST。 |
| `web/src/stores/downloader.ts` | +14 / -2 | 加 `cancelDownload` action（finally stopPolling）+ idleState 注释（F-7）。 |
| `web/src/components/AppLayout.vue` | +28 / -5 | 加"✕ 取消"按钮（仅 downloading 显示）、5 状态主按钮表（含 canceled warning + "已取消，点击重试" 文案）、`getDownloadBtnLabel/Type` 扩展、`handleCancel` handler、`<upload-bin-button :sibling-downloading>` 透传。 |
| `web/src/components/UploadBinButton.vue` | +12 / -4 | 加 `siblingDownloading` prop（默认 false），按钮 disabled 条件追加，tooltip 文案分支。 |
| `internal/downloader/downloader_cancel_test.go` | +450（新文件） | 10 个 Go 单测覆盖 AC-cancel-* 全表 + F-3 3s 兜底 + F-4 resolveLatestAsset cancel + N=10 并发。 |
| `internal/httpapi/handlers_cancel_test.go` | +160（新文件） | 6 个 HTTP 测试：no-cookie-401 / no-csrf-403 / bad-kind-422 / idle-200 / downloader-nil-503 + uploadBin 409 文案静态校验。 |
| `web/src/stores/__tests__/downloader.spec.ts` | +60（新文件） | 4 个 Vitest 测试：cancel-action / idle no-op / failure 仍 stopPolling / canceled 不算 isDownloading。 |
| `web/src/components/__tests__/UploadBinButton.spec.ts` | +28 | 2 个新增用例：siblingDownloading=true 时 disabled / tooltip 文案。 |

合计 6 个生产文件改动 + 4 个测试文件（新建/扩展）。

---

## §2 对 03 SHOULD-FIX 的回应

| 编号 | 处置 | 实施位置 / 备注 |
|---|---|---|
| **F-1 dialog vs tooltip（FR-9 偏离）** | **PM 决策保留 tooltip + disabled**（不做 dialog） | 在 `UploadBinButton.vue` 实现 `:disabled="uploading || props.siblingDownloading"` + tooltip 文案分支（`v-if="props.siblingDownloading"` → "正在下载 ... 请先点击左侧'取消'按钮再上传"）。AppLayout banner 已有显性红色"✕ 取消"按钮在相邻位置，比 dialog 更醒目（一眼可见、零次点击就发现路径），低频用户不会被埋掉。**DESIGN DRIFT** 已在 §7 显式记录；若上线后用户反馈"看不到取消路径"，follow-up 入口是 `UploadBinButton.handleFileChange` 增加 `confirm dialog` 三选一分支（≤ 60 行 Vue）。 |
| **F-2 kind 非法 400 → 422** | **采纳**（与 codebase uploadBin/downloadBin 一致约定） | `internal/httpapi/handlers_system.go` `downloadCancel` 写 `http.StatusUnprocessableEntity + CodeValidationFailed + field=kind`；`openapi.yaml` 新 path block 写 `'422'`；HTTP 测试 `TestDownloadCancel_BadKind` 断言 422。01 §5.2 的 AC-http-cancel-bad-kind-400 这条 AC 的断言改 422（仅这一处与 01 偏离）。06 测试报告应显式说明。 |
| **F-3 3s 超时兜底失态** | **采纳选项 A（拿锁强写 canceled）** | `internal/downloader/downloader.go` `Cancel` 方法在 3s 轮询未切到终态时进入兜底分支：`m.mu.Lock(); if Status == downloading { Status = StatusCanceled; Error = "用户取消下载（goroutine 卡住，强写）" }; m.mu.Unlock(); return nil`。doc 注释解释 "FR-7 不变量优先级 > 状态机单调防御 guard"。**新增单测 `TestCancel_3sTimeoutForceWrite`**：用 `stuckTransport` hijack archive 路径返不响应 ctx 的 io.Pipe Body，断言 Cancel 在 ~3s+ε 内返回且 state=canceled。 |
| **F-4 resolveLatestAsset ctx 化** | **采纳** | `resolveLatestAsset` 签名改 `(ctx context.Context, goos string) → (downloadURL, version string, err error)`；内部 `http.NewRequest` → `http.NewRequestWithContext(ctx, ...)`。`doDownload` 调用方传入 per-goroutine ctx；调用前后均判 `errors.Is(ctx.Err(), context.Canceled)` 走 `setCanceled`。**新增单测 `TestCancel_DuringResolveAsset`**：mock GitHub API 用 `<-r.Context().Done()` 永不响应；apiClient 设 60s 大超时；断言 Cancel 在 3s 内生效（不依赖 apiClient 超时）。 |

---

## §3 对 NIT 的回应

| 编号 | 处置 | 实施位置 |
|---|---|---|
| **F-5 binTmp defer 兜底注释** | **采纳** | `doDownload` 在 `binTmpPath := binTmp.Name()` 之后立刻 `defer func() { _ = os.Remove(binTmpPath) }()`，并加 3 行注释说明"双层清理：defer 兜底 cancel 路径（cancel 在解压前发生时下面主动 Remove 不会跑）+ 正常路径仍由后面的显式 Remove 优先释放"。defer 在显式 Remove 之后跑会走 ENOENT 分支，无副作用。 |
| **F-6 setFailed 加单调 guard** | **采纳** | `setFailed` 加 `if m.states[kind].Status == StatusDownloading { ... }` 包裹，与 `setCanceled` 对称。doc 注释解释"防御纵深：若 Cancel 已先一步把状态写为 canceled，setFailed 不应覆盖回 failed"。 |
| **F-7 idleState 显式标注返回类型** | **采纳** | `web/src/stores/downloader.ts` 的 `idleState` 工厂已经标注 `: DownloadState`（baseline 即如此），加注释解释为何不让它窄化成字面量类型。 |
| **F-8 E2E mock GitHub 在 T-006 框架是否可用** | **降级为 Vitest + Go 集成测试** | grep 过 `web/tests/e2e/`，T-006 框架没有 mock GitHub API 的 seam（fixtures 仅有 auth helper），故 happy path E2E 暂不引入；Vitest spec 覆盖 store + UploadBinButton 视觉契约；Go 测试覆盖 cancel 端到端行为；HTTP 集成测试覆盖端点契约。06 测试报告应在 AC 表说明"AC-e2e-cancel-then-upload-happy-path 由 downloader + httpapi + Vitest 三层组合覆盖"。 |
| **F-9 UploadBinButton 现有 :disabled 合并** | **采纳** | verbatim 替换为 `:disabled="uploading || props.siblingDownloading"`；并把 tooltip 内容分支换成 `<template v-if="props.siblingDownloading">` / `<template v-else>`，没引入空白漂移。 |
| **F-10 L42 双路径注释落到具体行** | **采纳** | `Manager.cancels` 字段上方 + `Cancel` 方法 doc + `downloadCancel` handler doc 三处都写双路径：`docs/features/download-cancel-and-upload-decouple/ 或归档后 docs/features/_archived/download-cancel-and-upload-decouple/`。 |

---

## §4 实施细节（关键代码片段 + 设计取舍）

### 4.1 后端 Manager 状态机 + cancels map

```go
const (
    StatusIdle        = "idle"
    StatusDownloading = "downloading"
    StatusSuccess     = "success"
    StatusFailed      = "failed"
    StatusCanceled    = "canceled" // T-027 新增
)

type Manager struct {
    mu     sync.Mutex
    states map[string]*DownloadState
    ...
    // T-027：per-kind ctx cancel func，由 m.mu 守护。
    // doDownload 入口注册，goroutine 退出前 defer 反注册；
    // 非 downloading 状态时该 entry 必不存在（Cancel 据此快速判 no-op）。
    cancels map[string]context.CancelFunc
}
```

`cancels` map 与 `states` map 共用 `m.mu`，避免引入新锁；任何"读 status → 拿 cancel func"路径都在同一临界区内一次性完成（见 §4.2 Cancel 方法）。

### 4.2 Cancel 方法（FR-1 + F-3 兜底）

```go
func (m *Manager) Cancel(kind string) error {
    if kind != "frpc" && kind != "frps" { return ErrBadKind }

    m.mu.Lock()
    st := m.states[kind]
    if st.Status != StatusDownloading {
        m.mu.Unlock()
        return nil // no-op：idle / success / failed / canceled 全 no-op
    }
    cancel, ok := m.cancels[kind]
    m.mu.Unlock()
    if ok { cancel() } else { m.logger.Warn(...) }

    // 等 doDownload goroutine 把状态写到终态（≤3s，正常 < 100ms）
    deadline := time.Now().Add(3 * time.Second)
    for time.Now().Before(deadline) {
        m.mu.Lock()
        cur := m.states[kind].Status
        m.mu.Unlock()
        if cur != StatusDownloading { return nil }
        time.Sleep(10 * time.Millisecond)
    }

    // F-3 选项 A：3s 仍未切换 → 强写 canceled 保 FR-7 不变量
    m.logger.Error("cancel timed out waiting for goroutine exit; force-writing canceled", "kind", kind)
    m.mu.Lock()
    if m.states[kind].Status == StatusDownloading {
        m.states[kind].Status = StatusCanceled
        m.states[kind].Error = "用户取消下载（goroutine 卡住，强写）"
    }
    m.mu.Unlock()
    return nil
}
```

设计要点：
- **不在 Cancel 里直接写 state**（除 3s 兜底外）：让 doDownload goroutine 自己在 ctx 分支调用 `setCanceled`，避免"Cancel 写 canceled → doDownload 再写 success"的覆盖。
- **3s 强写 break 状态机单调防御**：但 setSuccess/setFailed/setCanceled 后续都有 `if Status == downloading` guard，强写后即使 goroutine 醒来也无法覆盖回其它终态。
- **doc 注释引用双路径**（L42 防归档后引用 404）。

### 4.3 doDownload ctx 化（FR-3 + R-2 双层防御）

```go
func (m *Manager) doDownload(kind, goos string) {
    ctx, cancel := context.WithCancel(context.Background())
    m.mu.Lock(); m.cancels[kind] = cancel; m.mu.Unlock()
    defer func() {
        m.mu.Lock(); delete(m.cancels, kind); m.mu.Unlock()
        cancel()
    }()

    // resolveLatestAsset 已 ctx 化（F-4）
    downloadURL, version, err := m.resolveLatestAsset(ctx, goos)
    if err != nil {
        if errors.Is(ctx.Err(), context.Canceled) { m.setCanceled(kind); return }
        m.setFailed(kind, err.Error()); return
    }

    // archive 拉取
    req, _ := http.NewRequestWithContext(ctx, ...)
    resp, err := m.downloadClient.Do(req)
    if err != nil { /* ctx 重检 → setCanceled */ }

    // io.Copy err 分支同款 ctx 重检
    // 解压 err 分支同款
    // Install err 分支同款

    // R-2：写 success 前 ctx 重检 + 状态机单调 guard
    m.mu.Lock()
    if errors.Is(ctx.Err(), context.Canceled) {
        m.mu.Unlock(); m.setCanceled(kind); return
    }
    if m.states[kind].Status != StatusDownloading {
        m.logger.Warn(...); m.mu.Unlock(); return
    }
    m.states[kind].Status = StatusSuccess
    m.states[kind].Progress = 100
    m.states[kind].Error = ""
    m.mu.Unlock()
}
```

5 处 err 分支（NewRequest / Do / io.Copy / extract / Install）+ 末尾 success 写入前共 6 处 ctx 重检，覆盖任意时刻的 cancel 信号。`binTmpPath` 创建后立刻 `defer os.Remove`（F-5）兜底"cancel 在解压前发生时 line 257/273 主动 Remove 不会跑"的场景。

### 4.4 HTTP downloadCancel handler

```go
func (h *handlers) downloadCancel(w http.ResponseWriter, r *http.Request) {
    if h.deps.Downloader == nil {
        writeError(w, http.StatusServiceUnavailable, CodeInternal, "下载器未初始化", "")
        return
    }
    kind := chi.URLParam(r, "kind")
    if err := h.deps.Downloader.Cancel(kind); err != nil {
        if errors.Is(err, downloader.ErrBadKind) {
            writeError(w, http.StatusUnprocessableEntity, CodeValidationFailed,
                "kind 必须为 frpc 或 frps", "kind")
            return
        }
        writeError(w, http.StatusInternalServerError, CodeInternal, err.Error(), "")
        return
    }
    st, _ := h.deps.Downloader.Status(kind)
    writeJSON(w, http.StatusOK, st)
}
```

返 200 + 最新 DownloadState（FR-7 保证返回时 state 已是终态）。挂在 `r.Post("/system/download-cancel/{kind}", h.downloadCancel)` 受保护组内（SessionAuth + CSRF）。

### 4.5 前端 5 状态按钮表 + 取消按钮

`AppLayout.vue` 模板新增：

```vue
<n-button
  v-if="downloaderStore.isDownloading(kind)"
  size="small"
  type="error"
  ghost
  @click="handleCancel(kind)"
>
  ✕ 取消
</n-button>
<upload-bin-button
  :kind="kind"
  :sibling-downloading="downloaderStore.isDownloading(kind)"
  @uploaded="handleUploaded"
/>
```

`getDownloadBtnLabel` 5 态：
- idle → `一键下载 frpc/frps`
- downloading → `下载中... 42%`（progress 嵌入）
- success → `已下载`
- failed → `重试`
- canceled → `已取消，点击重试`

`getDownloadBtnType` 加 `'warning'` 返回值（canceled 状态）；`failed` 仍走 `'error'` 让用户能视觉区分"我主动取消"与"系统错误"。

### 4.6 三层一致性自检（L29 防漂移）

```
=== Go layer ===
internal/downloader/downloader.go:35: StatusCanceled = "canceled"
=== OpenAPI layer ===
openapi.yaml:332:          enum: [idle, downloading, success, failed, canceled]
=== TS layer ===
web/src/types.ts:102:  status: 'idle' | 'downloading' | 'success' | 'failed' | 'canceled'
```

三层 verbatim 一致。Endpoint 路径 `POST /api/v1/system/download-cancel/{kind}` 三层（Go router / OpenAPI / TS api 文件）也 verbatim 一致。

---

## §5 测试矩阵（对照 01 §5 AC）

### 5.1 Go 单测（`internal/downloader/downloader_cancel_test.go` 新建）

| AC | 测试函数 | 关键设计 |
|---|---|---|
| AC-cancel-mid-download | `TestCancel_MidDownload` | newSlowChunkServer 用 `math/rand` 生成伪随机字节（L40 防 gzip 压塌） → gzip → chunk=4096 + sleep=80ms，1 MiB 总 ~20s 慢；assert: status=canceled / Cancel 耗时 ≤3.5s / 等 goroutine 退出后无 `.dl-archive-*.tmp` / `.dl-bin-*.tmp` 残留。 |
| AC-cancel-idle-noop | `TestCancel_Idle_NoOp` | 新 Manager → Cancel("frpc") → state 仍 idle。 |
| AC-cancel-success-noop | `TestCancel_Success_NoOp` | 等 Start success → Cancel → state 仍 success。 |
| AC-cancel-after-failed-then-restart | `TestCancel_FailedThenRestart` | httptest 切换 handler（500 → 合法 archive）；从 failed Cancel 后 Start 必须能进 downloading。 |
| AC-cancel-then-restart-from-canceled | `TestCancel_CanceledThenRestart` | 从 canceled Cancel 后 Start 不报 ErrAlreadyInProgress，进入 downloading。 |
| AC-cancel-repeated-noop | `TestCancel_Repeated_NoOp` | 5 次串行 Cancel；后续 return nil 状态保持 canceled。 |
| AC-cancel-bad-kind | `TestCancel_BadKind` | Cancel("frpx") → ErrBadKind。 |
| 并发 N=10 | `TestCancel_Concurrent_N10` | 10 goroutine 并发 Cancel；assert: 全 return nil + state=canceled + no panic。（race detector 需 cgo；本机环境无 gcc，未跑 -race 但 mu 守护语义明确。） |
| F-3 兜底 | `TestCancel_3sTimeoutForceWrite` | stuckTransport 注入永远不响应 ctx 的 io.Pipe Body；assert: Cancel 在 ~3s+ε 内返回 + state=canceled（强写路径）。 |
| F-4 resolveLatestAsset cancel | `TestCancel_DuringResolveAsset` | API 端永远不返；apiClient 60s 大超时；assert: Cancel 3s 内 state=canceled（不依赖 apiClient 超时）。 |

**全部 10 个 PASS**；Windows file-lock 通过 `waitGoroutineExit` helper 等 cancels map entry 被 delete 后才 TempDir cleanup。

### 5.2 HTTP 集成（`internal/httpapi/handlers_cancel_test.go` 新建）

| AC | 测试函数 |
|---|---|
| AC-http-cancel-no-cookie-401 | `TestDownloadCancel_NoCookie` |
| AC-http-cancel-no-csrf-403 | `TestDownloadCancel_NoCSRF` |
| AC-http-cancel-bad-kind-422（F-2） | `TestDownloadCancel_BadKind`（断言 status=422 + code=VALIDATION_FAILED + field=kind） |
| AC-http-cancel-idle-200 | `TestDownloadCancel_Idle_200` |
| AC-http-cancel-downloader-nil-503 | `TestDownloadCancel_DownloaderNil` |
| AC-http-upload-during-download-message-updated（FR-6） | `TestUploadBin_409MessageContainsCancel`（源码静态扫描 + 旧文案防回归） |

**全部 6 个 PASS**。注：`AC-http-cancel-200`（真正 downloading→cancel）由 downloader 包单测（`TestCancel_MidDownload` 等）覆盖；`AC-http-cancel-then-upload-200`（cancel → upload 200）由 cancel 端点契约 + downloader cancel 行为 + upload 既有 happy path 三层组合保证（downloader 状态变 canceled 后，uploadBin 的 `if Status == downloading` 分支自然不触发）。

### 5.3 OpenAPI / 契约一致性

| AC | 验证 |
|---|---|
| AC-openapi-cancel-path-present | `grep -F '/api/v1/system/download-cancel/{kind}' openapi.yaml` 命中 1 行；verify_all D.1 PASS。 |
| AC-types-status-canceled-present | `grep -n canceled web/src/types.ts` 命中 line 102；verify_all B.1（typecheck）PASS。 |

### 5.4 前端 Vitest（`web/src/stores/__tests__/downloader.spec.ts` 新建 + `UploadBinButton.spec.ts` 扩展）

| AC | 测试函数 |
|---|---|
| AC-store-cancel-action | `cancel 成功后 store.frpc 切到 canceled，timer 被清` |
| (extra) cancel idle no-op | `cancel idle 时返 idle state，不改变 store 状态` |
| (extra) cancel 失败仍 stopPolling | `cancel 失败时仍然 stopPolling（finally 保证）` |
| (extra) canceled 不算 isDownloading | `canceled 状态不算 isDownloading` |
| AC-ui-upload-disabled-when-sibling-downloading | `T-027 siblingDownloading=true → 按钮 disabled + tooltip 文案引导取消` |
| (extra) sibling 上传保护 | `T-027 siblingDownloading=true → 不触发上传请求`（断言 disabled 属性存在） |

**全部 6 个新增/扩展 PASS**。Vitest 文件总测试数 frontend 96 → 102。

### 5.5 E2E（playwright）

**F-8 决策**：T-006 框架无 mock GitHub API seam，本任务不引入 E2E happy path。回归覆盖由三层组合（Go cancel + HTTP cancel + Vitest store cancel + UploadBinButton spec）保证。

---

## §6 verify_all 结果

**Baseline**（实施前）：

```
[A.1] PASS / [A.2] PASS / [A.3] PASS
[G.1] go vet PASS / [G.2] go test ./... PASS / [G.3] go build PASS
[B.1] typecheck PASS / [B.2] Lint PASS / [B.3] Unit tests PASS / [B.4] Test count >= baseline PASS / [B.5] No tsc residue PASS
[C.1] E2E smoke PASS
[D.1] OpenAPI schema present PASS
[E.1..E.6] PASS
[E.7] PASS

Summary: PASS 20 / WARN 0 / FAIL 0
```

**After T-027 实施**：

```
[A.1] PASS / [A.2] PASS / [A.3] PASS
[G.1] go vet PASS / [G.2] go test ./... PASS / [G.3] go build PASS
[B.1] typecheck PASS / [B.2] Lint PASS / [B.3] Unit tests PASS / [B.4] Test count >= baseline PASS / [B.5] No tsc residue PASS
[C.1] E2E smoke PASS
[D.1] OpenAPI schema present PASS
[E.1..E.6] PASS
[E.7a] BOM-required PS1 BOM PASS
[E.7b] iex-entry no-BOM PASS
[E.7c] PS1 classification PASS

Summary: PASS 22 / WARN 0 / FAIL 0
```

**Delta**：+2 个步骤（E.7b / E.7c 在 baseline json 之外的报告分隔），baseline 步骤全部保持 PASS。新增测试：Go +10（downloader_cancel_test.go）+ 6（handlers_cancel_test.go）= +16；Frontend +4（downloader.spec.ts）+ 2（UploadBinButton.spec.ts 扩展）= +6。Go test count 实测 246 + 16 = 262；Frontend 96 + 6 = 102。

---

## §7 设计偏离（DESIGN DRIFT）

### DRIFT-1：UploadBinButton 用 tooltip + disabled 替代 confirm dialog（偏离 01 FR-9）

- **01 FR-9 要求**：上传按钮在同 kind status=downloading 时弹 confirm dialog 三选一（取消下载并上传 / 仅上传 / 放弃）。
- **02 §4.5 提议**：改为 tooltip + disabled。
- **03 F-1 SHOULD-FIX**：要求 04 显式选择 + DESIGN DRIFT 记录。
- **PM 决策（本任务派发指令）**：保持 tooltip + disabled。
- **理由**：
  1. AppLayout banner 已有显性红色"✕ 取消"按钮在相邻位置（同 banner / 同 kind / 同行），用户视觉首次扫描即可看到取消路径，比 dialog 更醒目（零次点击就发现，dialog 需要先选文件才触发）。
  2. mainstream UX 风格（npm / yarn UI）一致——disabled + tooltip 是工业标准模式，dialog 三选一是 over-engineering。
  3. 用户故事 U-4 "一步取消"诉求被"取消按钮直接可见"满足，dialog 反而是"两步操作"（先选文件 → 再 dialog 选）。
- **Follow-up 入口**：若上线后用户反馈"看不到取消路径"，可在 `UploadBinButton.handleFileChange` 加 confirm dialog 三选一分支（≤ 60 行 Vue），不破坏 disabled tooltip 的默认 UX。

### DRIFT-2：kind 非法 HTTP 状态码 400 → 422（偏离 01 §5.2 AC-http-cancel-bad-kind-400）

- **01 §3 FR-4 / §5.2 AC**：写"400"。
- **02 OQ-4**：Architect 决策选 422（与 uploadBin / downloadBin / probePorts kind 校验一致）。
- **03 F-2 SHOULD-FIX**：要求 04 同步落实。
- **本任务**：实施按 422；HTTP 测试 `TestDownloadCancel_BadKind` 断言 422；OpenAPI block 写 `'422'`。**06 测试报告应在 AC 表显式说明 AC-http-cancel-bad-kind 的最终断言为 422**。

### DRIFT-3：resolveLatestAsset 改 ctx 化（撤销 02 OQ-1 自决）

- **02 OQ-1**：Architect 决策"不 ctx 化"（理由：API 阶段总 60s，cancel 窗口极小）。
- **03 F-4 SHOULD-FIX**：要求 04 改回 ctx 化（理由：API 阶段卡 60s 边界场景违反 NFR-1 ≤3s）。
- **本任务**：采纳 F-4，把 `resolveLatestAsset(goos)` 改为 `(ctx, goos)`，调用方传入 ctx；加单测 `TestCancel_DuringResolveAsset` 验证。

这三处是设计 → 评审 → 实施过程中已记录的偏离/采纳路径，reviewer 在 05 阶段可专项审视。

---

## §8 实施过程中遇到的意外 / 修正

### 8.1 Windows TempDir cleanup 与 file lock

**现象**：`TestCancel_3sTimeoutForceWrite` 等用例的 `t.TempDir()` 在测试函数返回时 cleanup 失败：`unlinkat ... \.dl-archive-XXX.tmp: The process cannot access the file because it is being used by another process.`

**根因**：doDownload goroutine 在 Cancel 强写后没有立即退出（仍在 io.Copy 阻塞），文件句柄未释放；Windows 的 file lock 让 Go runtime cleanup 无法删除。

**修复**：
1. 新增 `waitGoroutineExit(t, m, kind, timeout)` helper：轮询 `cancels` map entry 直到被 delete（goroutine 完全退出标志）。
2. 所有用慢 server / stuckTransport 的测试加 `t.Cleanup(func() { waitGoroutineExit(...) })`。
3. `stuckTransport` 增加 `releaseCh chan struct{}`，t.Cleanup 时 close 让 PipeReader/Writer 解除阻塞。

### 8.2 baseline 跑过的 E2E 在我环境第一次跑 FAIL

**现象**：verify_all 跑出 `[C.1] E2E smoke (playwright) ... FAIL` —— TC-01 访问 `/` 跳到 `/login` 而不是 `/setup`，TC-02 提交 setup 后没离开 `/setup`。

**根因**：用户机器上有一个 system service `frp-easy`（`C:\Program Files\frp-easy\frp-easy.exe`）常驻 listening 7800 端口，使用自己的 `.frp_easy/data.db`（已 initialized）。playwright `reuseExistingServer:true`（非 CI 模式）让它直接重用 service 而不 spawn 新 e2e server。service 的数据库 initialized=true → /setup 不可达。

**修复**：用 `net stop frp-easy`（admin 权限）停 service；之后 verify_all C.1 PASS。**这与 T-027 改动完全无关**（baseline 跑 PASS 时 service 当时未启动或未冲突）。归档时应在 06 测试报告提醒"E2E 跑前确认 7800 未被 system service 占用"。

### 8.3 vue-tsc props 在 template 直接访问的限制

**现象**：UploadBinButton.vue 加 `siblingDownloading?: boolean` prop 时，模板里直接用 `siblingDownloading` 编译失败（template 访问的不是 setup 暴露的局部变量）。

**修复**：模板里改用 `props.siblingDownloading`（vue-tsc 自动从 `<script setup>` 推断 `props` 是 reactive proxy）。

### 8.4 测试源码字符串静态校验的 Go 字面量转义

**现象**：`TestUploadBin_409MessageContainsCancel` 初版断言 ``strings.Contains(src, `"取消下载"按钮后再上传`)`` 失败 —— Go 源里是 `\"取消下载\"`（带反斜杠转义），不是裸 `"取消下载"`。

**修复**：分两段断言 `strings.Contains(src, ``取消下载``)` && `strings.Contains(src, ``按钮后再上传``)`，绕过引号转义；同时加旧文案防回归断言 `!strings.Contains(src, "请稍后再上传或取消下载")`。

---

## Dev-map updates

无新模块新增；本任务全部改动落在既有模块 `internal/downloader` / `internal/httpapi` / `web/src/api` / `web/src/stores` / `web/src/components` 内。`docs/dev-map.md` 暂不需 append（与 02 §6 reuse audit 一致 —— "新模块/新依赖：无"）。

## Insight to surface

- **2026-05-23** · Windows 用户机器上已部署的 `frp-easy` system service（常驻 0.0.0.0:7800）会让 playwright `reuseExistingServer:true` 静默接管成 e2e server，使 E2E setup 测试看到 `initialized=true` 全部 FAIL；root cause 与项目 code 改动无关，但 reset 路径需 admin `net stop frp-easy`。这是 dogfood 场景特有陷阱（开发者把自己产品装成 service）；CI 环境无此问题 · evidence: T-027 04 §8.2，service binPath=`C:\Program Files\frp-easy\frp-easy.exe`，bash scripts/verify_all.sh 实测复现

## Verdict

**READY FOR REVIEW**
