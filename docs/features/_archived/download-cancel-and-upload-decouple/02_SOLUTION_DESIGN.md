# T-027 · 02_SOLUTION_DESIGN — download-cancel-and-upload-decouple

> 任务 ID：**T-027** · slug：**download-cancel-and-upload-decouple** · 模式：**full**
> 报告时间：2026-05-23
> 输入：`01_REQUIREMENT_ANALYSIS.md`（FR ×10 / NFR ×6 / AC ×22，verdict READY）+ `PM_LOG.md`（候选 A）
> 单 Developer / 无 partition 拆分（项目 `.harness/agents/` 无 `dev-*.md` 分区代理）

---

## §1 设计目标与不变量

### 1.1 设计目标（与 01 §3 / §4 对齐）

- 给下载链路加"取消"通道，让 `doDownload` 在网络慢时可被前端按一下立刻中止。
- 保持下载 / 上传**互斥**（写同一 binary 路径），但变"硬阻断"为"可操作互斥"：UI 提供"先取消下载再上传"路径。
- 不动 T-025 `downloadClient.Timeout=0` 决策（L38）；取消信号纯走 `context.Context`。
- OpenAPI / 前端 TS / 后端 Go 三处枚举值一锤定音（L29 防漂移）。

### 1.2 不变量（任何设计变更不得违反）

1. **写终点路径互斥**：在任意时刻，对同一 kind 的"原子 rename 终点"（`frp_linux/frpc[.exe]` / `frp_win/frpc.exe` 等）**只能**有一个 goroutine 在写。下载路径与上传路径共用 `Manager.Install`（见 `internal/downloader/install.go:47`）；本任务不松互斥。
2. **取消语义正交于超时**：`Cancel()` 触发的是**用户意图**；`downloadClient.Timeout=0` 维持（L38 / NFR-6）。死连接保护由 Transport 阶段性上限（dial / TLS / ResponseHeader / IdleConn）承担，与 ctx cancel 在概念上正交。
3. **状态单调**：状态机 `idle → downloading → {success, failed, canceled}`，三个终态不可互相转移；唯一回退路径是再次 `Start()`（任一终态 → downloading）。
4. **取消是意图、不是断言**：`Cancel()` 对"已不在 downloading 的 kind"是 no-op（返回 nil），不报错。
5. **Cancel 同步返回时状态已落地**：HTTP 200 / Cancel return 时，`state.Status` 必须已是 `canceled`（FR-7），保证"cancel → upload"零等待时间窗。

### 1.3 状态机（PNG 替代 ASCII）

```
              Start(kind)
   idle ─────────────────────────► downloading
                                    │
        ┌───────────────────────────┼───────────────────────────┐
        │                           │                           │
   ctx.Done (Cancel)        正常完成（rename ok）          HTTP / 解压 / 落盘错误
        ▼                           ▼                           ▼
    canceled                     success                      failed
        │                           │                           │
        └─────────────► Start(kind) 任一终态 → downloading ◄────┘
```

---

## §2 后端 — `internal/downloader` 包改动

### 2.1 Manager 字段与常量扩展

文件：`internal/downloader/downloader.go`

```go
// 常量段（line 27-33）追加：
const (
    StatusIdle        = "idle"
    StatusDownloading = "downloading"
    StatusSuccess     = "success"
    StatusFailed      = "failed"
    StatusCanceled    = "canceled" // T-027 新增
)

// Sentinel errors（line 36-40）追加：
var (
    ErrAlreadyInProgress = errors.New("downloader: download already in progress")
    ErrUnsupportedOS     = errors.New("downloader: unsupported OS (only windows/linux amd64)")
    ErrBadKind           = errors.New("downloader: kind must be 'frpc' or 'frps'")
    // T-027：本任务不引入新 sentinel；Cancel 的"非 downloading 状态" no-op 返回 nil，
    // bad kind 复用 ErrBadKind。
)

// Manager 结构体（line 51-67）追加字段：
type Manager struct {
    mu     sync.Mutex
    states map[string]*DownloadState
    root   string

    apiClient      *http.Client
    downloadClient *http.Client

    logger *slog.Logger

    // T-027：per-kind cancel function，由 mu 守护。
    // 不在 downloading 状态时该 entry 必不存在（doDownload 在 goroutine 退出前 delete）。
    cancels map[string]context.CancelFunc

    apiBaseURL string
    goos       string
}
```

`New()` 初始化新增 `cancels: make(map[string]context.CancelFunc, 2)`。

### 2.2 `doDownload` ctx 化改造

文件：`internal/downloader/downloader.go:157-289`

改造点（按代码顺序）：

```go
// 函数签名不变（kind, goos string），ctx 在函数体内部创建。
func (m *Manager) doDownload(kind, goos string) {
    startTime := time.Now()

    // T-027：建 per-goroutine ctx，注册到 m.cancels，goroutine 退出时反注册。
    ctx, cancel := context.WithCancel(context.Background())
    m.mu.Lock()
    m.cancels[kind] = cancel
    m.mu.Unlock()
    defer func() {
        m.mu.Lock()
        delete(m.cancels, kind)
        m.mu.Unlock()
        cancel() // idempotent，确保 ctx 资源回收
    }()

    // ...（line 160-180 resolveParams / resolveLatestAsset / MkdirAll 保持不变；
    // 注意：resolveLatestAsset 仍走 m.apiClient + 60s 总超时，本任务不 ctx 化它
    // —— 决策见 §9 OQ-1）

    // archive temp 创建保持不变（line 183-192）。

    // line 194：http.NewRequest → http.NewRequestWithContext
    req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
    if err != nil {
        if errors.Is(ctx.Err(), context.Canceled) {
            m.setCanceled(kind)
            return
        }
        m.setFailed(kind, fmt.Sprintf("构建下载请求失败: %v", err))
        return
    }

    resp, err := m.downloadClient.Do(req)
    if err != nil {
        // T-027：ctx 取消时 stdlib transport 主动关 conn，Do 返回的 err 链中含 context.Canceled。
        if errors.Is(ctx.Err(), context.Canceled) {
            m.setCanceled(kind)
            return
        }
        m.setFailed(kind, fmt.Sprintf("下载超时: %v", err))
        return
    }
    defer resp.Body.Close()

    // ...（StatusCode 校验保持不变）

    bytesWritten, err := io.Copy(archiveTmp, teeReader)
    if err != nil {
        // T-027：cancel 路径走 setCanceled 不走 setFailed。
        if errors.Is(ctx.Err(), context.Canceled) {
            m.setCanceled(kind)
            return
        }
        m.setFailed(kind, fmt.Sprintf("下载写入失败: %v", err))
        return
    }

    // ...（line 232-277 解压 / Install 保持不变，但 extractErr / installErr 失败前也插入
    // ctx.Err 重检 —— 因为 cancel 可能在解压途中触发）

    // line 284-288：写 success 之前的 ctx 重检（R-2 防 race）。
    m.mu.Lock()
    if errors.Is(ctx.Err(), context.Canceled) {
        m.mu.Unlock()
        m.setCanceled(kind)
        return
    }
    // 状态机单调性 guard：仅 downloading 可转 success；防御性 assert（理论上不会触发）。
    if m.states[kind].Status != StatusDownloading {
        m.mu.Unlock()
        m.logger.Warn("download finished but state not downloading; skip success write",
            "kind", kind, "state", m.states[kind].Status)
        return
    }
    m.states[kind].Status = StatusSuccess
    m.states[kind].Progress = 100
    m.states[kind].Error = ""
    m.mu.Unlock()
}
```

**ctx 注入位置一览**（对照 FR-3）：

| 调用 | 改造 | 备注 |
|---|---|---|
| `resolveLatestAsset` 内 `http.NewRequest` (line 348) | **不动** | OQ-1：API 阶段总 60s，可被 client.Timeout 兜底；不阻塞 cancel 响应（API 阶段 cancel 等同 failed 路径） |
| `doDownload` 主 archive 拉取（line 194） | `http.NewRequestWithContext(ctx, ...)` | 关键路径 |
| `io.Copy(archiveTmp, teeReader)` (line 221) | **不直接改**，错误分支判 `ctx.Err()` | stdlib transport 关 conn 时 `resp.Body.Read` 返 err，io.Copy 自然解除 |
| 解压 `extractFromTarGz / Zip` (line 248-250) | **不动**（本地文件流不长） | 解压时 cancel 等到解压结束后由后续 ctx 重检捕获；解压本身预算 < 1s（本地 IO） |
| `Install` (line 270) | **不动** | 本地写盘，预算 < 1s；同上 |

### 2.3 新增 `Cancel` 方法

文件：`internal/downloader/downloader.go`（新增方法，建议放在 `Status` 后、`doDownload` 前）

```go
// Cancel triggers cancellation for kind's in-flight download.
//
// 行为契约（T-027 FR-1）：
//   - kind 非法 → ErrBadKind。
//   - 当前状态非 downloading（idle / success / failed / canceled） → no-op，返回 nil。
//   - 当前状态 downloading → 触发 ctx cancel，阻塞等待 state 切到 canceled
//     （≤3s 上限；正常 < 100ms），返回 nil。
//
// 重入安全：多次 Cancel 同 kind / Cancel 与 Start 交错 / Cancel 与 doDownload
// 即将写 success 的 race —— 均不报错、无 panic、状态机不被破坏（R-2 / NFR-5）。
//
// 设计要点：Cancel 不直接写 state；仅触发 ctx.Done，让 doDownload 自己在 ctx 分支
// 调用 setCanceled。这避免 "Cancel 写 canceled → doDownload 再写 success" 的覆盖
// （由 §2.2 中 doDownload 末尾的 ctx 重检 + 状态机单调 guard 兜底）。
func (m *Manager) Cancel(kind string) error {
    if kind != "frpc" && kind != "frps" {
        return ErrBadKind
    }

    m.mu.Lock()
    st := m.states[kind]
    if st.Status != StatusDownloading {
        m.mu.Unlock()
        return nil // no-op
    }
    cancel, ok := m.cancels[kind]
    m.mu.Unlock()

    if !ok {
        // 状态是 downloading 但 cancel func 未注册 —— 理论上不可能（doDownload 在 Start
        // 同步段 Status=downloading 后启动 goroutine，cancel 注册早于 ctx 化的 HTTP 请求；
        // 但若 doDownload 还没跑到注册前的指令，Cancel 调用会落到这里）。
        // 防御性日志后等待状态切换；不阻塞 caller > 3s。
        m.logger.Warn("cancel called but cancelFunc not yet registered", "kind", kind)
    } else {
        cancel()
    }

    // 等待 doDownload 把状态写到终态（canceled / failed / success 任一）。
    // 轮询周期 10ms × 300 = 3s 上限（NFR-1）。
    deadline := time.Now().Add(3 * time.Second)
    for time.Now().Before(deadline) {
        m.mu.Lock()
        cur := m.states[kind].Status
        m.mu.Unlock()
        if cur != StatusDownloading {
            return nil
        }
        time.Sleep(10 * time.Millisecond)
    }

    // 3s 仍未切换 —— 极端情况（doDownload 卡死在不可中断的 syscall，理论不应发生）。
    // 不强行写状态（保持单调性不被破坏）；只记日志让运维诊断。
    m.logger.Error("cancel timed out waiting for goroutine exit", "kind", kind)
    return nil
}
```

### 2.4 `setCanceled` 与 `setFailed` 区分

文件：`internal/downloader/downloader.go`（紧挨 `setFailed` 之后新增）

```go
// setCanceled 标记 kind 为已取消（用户主动行为，区别于 setFailed）。
// 日志走 Info（不是错误）；progress 保留 cancel 那一刻的值（便于调试，FR-1 + §9 OQ-2）；
// error 字段写"用户取消下载"（与 status 配套，前端可统一展示）。
func (m *Manager) setCanceled(kind string) {
    m.logger.Info("download canceled", "kind", kind)
    m.mu.Lock()
    // 单调性：仅 downloading 才可转 canceled；防御性 assert。
    if m.states[kind].Status == StatusDownloading {
        m.states[kind].Status = StatusCanceled
        m.states[kind].Error = "用户取消下载"
        // progress 保留 cancel 那一刻的值，不清零（OQ-2 决策）。
    }
    m.mu.Unlock()
}
```

### 2.5 临时文件清理验证

| tmp 文件路径 | 清理点位 | cancel 路径是否清理 |
|---|---|---|
| `.dl-archive-*.tmp` | `defer { archiveTmp.Close(); os.Remove(archiveTmpPath) }`（line 189-192） | ✅ goroutine 自然返回触发 defer |
| `.dl-bin-*.tmp` | line 257 `os.Remove(binTmpPath)`（解压失败分支）+ line 273 `os.Remove(binTmpPath)`（Install 后） | ⚠ 当前实现：cancel 在解压前触发 → 这两个 Remove 都不跑。**改进**：line 244 创建 binTmp 后立刻 `defer os.Remove(binTmpPath)`（无副作用：line 273 主动 Remove 后 defer 的 Remove 走 ENOENT 路径）。本任务一并修。 |
| `.install-*.tmp` | `Install` 内 defer（`install.go:71-74`） | ✅ 已正确（cancel 路径如果走到 Install 内部、Install 因 ctx 取消的 write error 失败，仍走 defer） |

**改动**：在 `doDownload` line 244 后加：
```go
defer func() { _ = os.Remove(binTmpPath) }()
```
保留 line 257 / 273 主动 Remove 不动（无副作用，但能让正常路径更快释放）。

---

## §3 后端 — `internal/httpapi` 改动

### 3.1 新增 endpoint `POST /api/v1/system/download-cancel/{kind}`

文件：`internal/httpapi/handlers_system.go`（新增 handler，建议放在 `downloadStatus` 之后）

```go
// downloadCancel handles POST /api/v1/system/download-cancel/{kind}（T-027 FR-4）。
//
// 返回：
//   - 200 + 最新 DownloadState JSON（成功，无论是真取消还是 idle/success/failed/canceled no-op）。
//   - 422 VALIDATION_FAILED：kind 不是 frpc / frps。
//   - 503 INTERNAL：downloader nil。
//   - 401 / 403：未登录 / 缺 CSRF（由中间件链统一处理，不在 handler 内）。
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

**契约决策**（与 01 FR-4 一致）：
- 返 **200 + DownloadState JSON**（非 204）：前端"cancel 后立刻同步状态"零额外往返；规避 R-3 前端轮询回弹。
- 路径选 `download-cancel/{kind}` 与 `download-status/{kind}` 风格统一（路径参数 kind，与 download-bin body 风格分歧由后者历史决定，不在本任务回退）。
- HTTP 方法 POST（写动作 + 走 CSRF，与 procStop / procRestart 一致）。

### 3.2 路由挂载

文件：`internal/httpapi/router.go`

在 line 122 `r.Get("/system/download-status/{kind}", h.downloadStatus)` 之后追加：

```go
r.Post("/system/download-cancel/{kind}", h.downloadCancel)
```

中间件链复用受保护组（SessionAuth + CSRF），无需新建 Group。

### 3.3 uploadBin 互斥提示文案修订

文件：`internal/httpapi/handlers_system.go:466-469`

原文（line 466-468）：
```go
if st, ok := h.deps.Downloader.Status(kind); ok && st.Status == downloader.StatusDownloading {
    writeError(w, http.StatusConflict, CodeProcBusy, "下载进行中，请稍后再上传或取消下载", "")
    return
}
```

改为：
```go
if st, ok := h.deps.Downloader.Status(kind); ok && st.Status == downloader.StatusDownloading {
    writeError(w, http.StatusConflict, CodeProcBusy,
        "下载进行中，请先点击\"取消下载\"按钮后再上传", "")
    return
}
```

行为不变（仍 409 PROC_BUSY），消息精化（FR-6）—— 指向用户**已存在**的操作入口，去掉"或取消下载"的悬挂语义。

### 3.4 OpenAPI 契约同步

文件：`openapi.yaml`

#### 3.4.1 `DownloadState.status` 描述增加 canceled

line 332 原文：
```yaml
status:
  type: string
  description: "idle | downloading | success | failed"
```

改为：
```yaml
status:
  type: string
  enum: [idle, downloading, success, failed, canceled]
  description: |
    下载状态机：
    - idle: 从未发起
    - downloading: 进行中
    - success: 已落盘
    - failed: 网络 / 解压 / 落盘错误
    - canceled: 用户主动取消（T-027）
```

> 注：原 schema 用 `description` 而非 `enum`，本任务顺手补 `enum` 让 codegen 工具能识别（L29 防漂移）。这是**只增不减**的契约升级，旧 spec mock 不受影响。

#### 3.4.2 新增 `/api/v1/system/download-cancel/{kind}` path

在 `/api/v1/system/download-status/{kind}` block（line 1281-1317）之后追加：

```yaml
  /api/v1/system/download-cancel/{kind}:
    post:
      summary: 取消正在进行的 frpc / frps 下载（T-027）
      operationId: cancelDownload
      security:
        - cookieAuth: []
          csrfToken: []
      tags: [system]
      parameters:
        - name: kind
          in: path
          required: true
          schema:
            type: string
            enum: [frpc, frps]
      responses:
        '200':
          description: |
            返回取消后的最新 DownloadState。
            - 若 kind 当前在 downloading，则 ctx cancel 已触发，state.status=canceled。
            - 若 kind 不在 downloading（idle / success / failed / canceled），则 no-op，返回当前 state。
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/DownloadState'
        '401':
          description: 未登录
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorBody'
        '403':
          description: 缺 X-CSRF-Token
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorBody'
        '422':
          description: kind 非法
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorBody'
        '503':
          description: 下载器未初始化
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorBody'
```

#### 3.4.3 共契字段名清单（L29 防漂移）

| 层 | 字段 | 类型 / 值 | 锚定来源 |
|---|---|---|---|
| Go `DownloadState.Status` | string | `"canceled"`（小写、美式拼写） | `internal/downloader/downloader.go:33` |
| OpenAPI `DownloadState.status.enum` | string[] | `[idle, downloading, success, failed, canceled]` | `openapi.yaml:332` |
| TS `DownloadState.status` | union | `'idle' \| 'downloading' \| 'success' \| 'failed' \| 'canceled'` | `web/src/types.ts:101` |
| Endpoint 路径 | URL | `/api/v1/system/download-cancel/{kind}` | router.go / openapi.yaml / `web/src/api/downloader.ts` |
| HTTP 方法 | verb | POST | 同上 |
| 路径参数名 | string | `kind` | 同上（chi `URLParam(r, "kind")` 严格匹配） |

实现时若有差异 → 直接 P0 阻断 merge（reviewer 闸门）。

---

## §4 前端改动

### 4.1 `web/src/types.ts`

line 100-104 修改：

```ts
export interface DownloadState {
  status: 'idle' | 'downloading' | 'success' | 'failed' | 'canceled'
  progress: number
  error?: string
}
```

### 4.2 `web/src/api/downloader.ts`

文件末追加：

```ts
export async function apiCancelDownload(kind: 'frpc' | 'frps'): Promise<DownloadState> {
  const res = await apiClient.post<DownloadState>(`/api/v1/system/download-cancel/${kind}`)
  return res.data
}
```

> 注：与 `apiDownloadBin` 不同，cancel 无 request body —— 但 axios `apiClient.post(url)`（不传第二参）会发空 body；后端 chi handler 不读 body，不会冲突。**不要**显式传 `null` 或 `{}` 作 body，避免与 insight L37 提到的 axios default Content-Type 污染做无意义交互。

### 4.3 `web/src/stores/downloader.ts`

文件改动：

```ts
import { defineStore } from 'pinia'
import { apiDownloadBin, apiDownloadStatus, apiCancelDownload } from '../api/downloader'
import type { DownloadState } from '../types'

// ... 现有 state / getters 不变

actions: {
  async downloadBin(kind: 'frpc' | 'frps'): Promise<void> { /* 不变 */ },

  // T-027 新增
  async cancelDownload(kind: 'frpc' | 'frps'): Promise<void> {
    try {
      const next = await apiCancelDownload(kind)
      // 后端契约：返回时 state 已是 canceled / 其它终态（FR-7）。
      this[kind] = next
    } catch (e) {
      // cancel API 失败时不强改本地状态；让调用方决定是否 message.error。
      throw e
    } finally {
      // 不论成功失败都 stopPolling —— 失败若需恢复轮询由 UI 主动调 startPolling。
      this.stopPolling(kind)
    }
  },

  startPolling(kind: 'frpc' | 'frps'): void {
    this.stopPolling(kind)
    const timer = setInterval(async () => {
      try {
        const state = await apiDownloadStatus(kind)
        this[kind] = state
        if (state.status !== 'downloading') {
          this.stopPolling(kind)
        }
      } catch { /* ignore */ }
    }, 1000)
    this._timers[kind] = timer
  },

  stopPolling(kind: 'frpc' | 'frps'): void { /* 不变 */ },

  downloadState(kind: 'frpc' | 'frps'): DownloadState { /* 不变 */ },
}
```

**R-3 防回弹策略**：cancelDownload 内一并 stopPolling（避免下一拍轮询把刚写的 canceled 覆盖回 downloading；理论上不会发生，因为后端 FR-7 保证 cancel 返回时 state 已是 canceled，但本地立刻 stopPolling 是零成本 belt）。

### 4.4 `AppLayout.vue`：取消按钮 + 状态扩展

文件：`web/src/components/AppLayout.vue`

#### 4.4.1 模板段（template）

`<n-space :size="4" align="center">` 内的 download 主按钮之后、`<upload-bin-button>` 之前，新增一个条件渲染的取消按钮：

```vue
<n-button
  v-if="downloaderStore.isDownloading(kind)"
  size="small"
  type="error"
  ghost
  @click="handleCancel(kind)"
>
  取消
</n-button>
```

进度条条件保持 `status === 'downloading'`；canceled 状态时进度条自然隐藏（OQ-5 决策）。

#### 4.4.2 script 段

`getDownloadBtnLabel` 扩展：

```ts
function getDownloadBtnLabel(kind: 'frpc' | 'frps'): string {
  const state = downloaderStore.downloadState(kind)
  if (state.status === 'downloading') return `下载中... ${state.progress}%`
  if (state.status === 'success')     return '已下载'
  if (state.status === 'failed')      return '重试'
  if (state.status === 'canceled')    return '已取消，点击重试'
  return `一键下载 ${kind}`
}
```

`getDownloadBtnType` 扩展：

```ts
function getDownloadBtnType(kind: 'frpc' | 'frps'): 'default' | 'primary' | 'success' | 'error' | 'warning' {
  const state = downloaderStore.downloadState(kind)
  if (state.status === 'success')  return 'success'
  if (state.status === 'failed')   return 'error'
  if (state.status === 'canceled') return 'warning'
  return 'primary'
}
```

主按钮 `:disabled` 表达式更新（仅 downloading + success 时禁用；canceled / failed 可点击重试）：

```vue
:disabled="downloaderStore.isDownloading(kind) || downloaderStore.downloadState(kind).status === 'success'"
```

（与现状一致，无需改 —— 因为 canceled 既不是 downloading 也不是 success，天然可点击。）

新增 handler：

```ts
async function handleCancel(kind: 'frpc' | 'frps') {
  try {
    await downloaderStore.cancelDownload(kind)
    message.info(`已取消 ${kind} 下载`)
  } catch {
    message.error(`取消 ${kind} 失败，请稍后再试`)
  }
}
```

### 4.5 `UploadBinButton.vue`：互斥可视化

PM 决策（PM_LOG 第 27 行 / 01 FR-9）原方案是 confirm dialog 三选一。**Architect 收紧为 tooltip + disabled**，理由：

- AppLayout 已经给用户一个明显的"取消"按钮（同 banner、同 kind、相邻位置）；
- dialog 三选一引入额外认知负担（R-4），高级用户嫌烦；
- 简单按钮 disabled + tooltip 引导"先点取消"是 mainstream 模式（npm / yarn UI 都这么干）。

**改动**（template 段 `<n-tooltip>` 内）：

```vue
<n-tooltip trigger="hover" placement="bottom">
  <template #trigger>
    <n-button
      size="small"
      :type="uploading ? 'warning' : 'default'"
      :loading="uploading"
      :disabled="uploading || siblingDownloading"
      @click="triggerFilePick"
    >
      {{ uploading ? `上传中 ${progress}%` : `上传 ${kind}` }}
    </n-button>
  </template>
  <template v-if="siblingDownloading">
    正在下载 {{ kind }}，请先点击左侧"取消"按钮再上传
  </template>
  <template v-else>
    本地选择已下载好的 {{ kind }} 二进制（适合 GitHub 不可达时使用）
  </template>
</n-tooltip>
```

**script 段新增**：

```ts
import { computed } from 'vue'
import { useDownloaderStore } from '../stores/downloader'

const downloaderStore = useDownloaderStore()
const siblingDownloading = computed(() => downloaderStore.isDownloading(props.kind))
```

> 注：此调整**变更**了 01 FR-9 的 confirm dialog 路径。变更理由如上，不构成 RA 阻断（UI 风格选择属 Architect 权限范围 / 01 §9 OQ-4 的 PM-DECIDED 在"是否引入交互"层面成立，但本任务依然给用户"一步取消"路径——只是用 tooltip + 显性 disabled 替代 dialog）。**若 PM Gate Review 认为必须按 01 严格遵循 dialog，本设计回退到 dialog 实现，工作量增加约 60 行 Vue 代码**。

> 同时 fallback：即使前端没禁用，用户绕过 disabled 触发上传，后端 §3.3 的精化 409 消息仍引导到正确路径。

---

## §5 测试矩阵（对照 01 §5 AC）

格式：AC ID → 测试层 / 测试文件 / 关键 mock 形态。

### 5.1 Go 单测（`internal/downloader/downloader_test.go` 已存在；本任务追加）

| AC | 层 | 测试函数 | mock 形态 |
|---|---|---|---|
| AC-cancel-mid-download | unit | `TestCancel_MidDownload` | `httptest.NewServer` chunk-write + sleep；用 `math/rand.New(rand.NewSource(42))` 生成伪随机字节防 gzip 压塌（L40）。chunk=4096 / sleep=80ms / total ≥ 256 KiB。assert: Status==canceled, dir 无 `.dl-archive-*.tmp` / `.dl-bin-*.tmp` 残留, server-side `r.Context().Done()` 触发。 |
| AC-cancel-idle-noop | unit | `TestCancel_IdleNoop` | 新 Manager → 直接 Cancel → state 仍 idle, return nil。 |
| AC-cancel-success-noop | unit | `TestCancel_SuccessNoop` | httptest 立即返合法 .tar.gz（含 frpc 入口）→ Start → 等 success → Cancel → state 仍 success。 |
| AC-cancel-after-failed-then-restart | unit | `TestCancel_FailedThenRestart` | httptest 第一次返 500 → state=failed → Cancel（no-op）→ httptest 切换 handler 返合法 archive → Start → 进 downloading。 |
| AC-cancel-then-restart-from-canceled | unit | `TestCancel_CanceledThenRestart` | 慢 server → Start → Cancel → state=canceled → Start → 进 downloading。 |
| AC-cancel-repeated-noop | unit | `TestCancel_RepeatedNoop` + `go test -race` | 慢 server → Start → 5 × Cancel 串行 → 第二次起 state 已 canceled, return nil; race detector PASS。 |
| AC-cancel-bad-kind | unit | `TestCancel_BadKind` | Cancel("frpx") → ErrBadKind。 |
| 并发 N=10 cancel | unit | `TestCancel_ConcurrentRace` (`go test -race`) | 慢 server → Start → `for i := 0; i < 10; i++ { go m.Cancel("frpc") }` + wg.Wait → state==canceled, no panic, no race report。 |

### 5.2 HTTP 集成（`internal/httpapi/handlers_system_test.go` 已存在；追加）

| AC | 测试函数 | 关键步骤 |
|---|---|---|
| AC-http-cancel-200 | `TestDownloadCancel_200` | mock Downloader → Start → POST `/system/download-cancel/frpc` (cookie+CSRF) → 200, body.status=="canceled"。 |
| AC-http-cancel-then-upload-200 | `TestDownloadCancel_ThenUpload_200` | 慢 mock → Start → POST cancel → 200 → POST `/system/upload-bin` (multipart, 合法 ELF/PE) → 200。**关键 AC**。 |
| AC-http-cancel-idle-200 | `TestDownloadCancel_IdleNoop` | 新 Manager → POST cancel → 200, body.status==idle。 |
| AC-http-cancel-bad-kind-422 | `TestDownloadCancel_BadKind` | POST `/system/download-cancel/frpx` → 422, error.code==VALIDATION_FAILED。**注**：01 FR-4 写 400，但 422 更符合现有 uploadBin / downloadBin "kind 非法" 一致约定（line 135 / 441）。本设计选 422 一致性优先；若 PM 要求 400，调整 5 行代码即可。 |
| AC-http-cancel-no-csrf-403 | `TestDownloadCancel_NoCSRF` | 不带 X-CSRF-Token → 403（由 CSRF middleware 统一返）。 |
| AC-http-cancel-no-cookie-401 | `TestDownloadCancel_NoCookie` | 不带 cookie → 401（SessionAuth middleware）。 |
| AC-http-cancel-downloader-nil-503 | `TestDownloadCancel_DownloaderNil` | 注入 deps.Downloader=nil → 503。 |
| AC-http-upload-during-download-message-updated | `TestUploadBin_409Message` | 慢 mock → Start → POST upload-bin → 409, error.message contains "请先点击"。 |

### 5.3 OpenAPI / 契约一致性

| AC | 验证手段 |
|---|---|
| AC-openapi-cancel-path-present | `scripts/verify_all` 内的 OpenAPI 校验（如已存在）+ grep 静态校验：`grep -F '/api/v1/system/download-cancel/{kind}' openapi.yaml` 命中 1 行。 |
| AC-types-status-canceled-present | `npm run typecheck` + grep `'canceled'` in `web/src/types.ts`。 |

### 5.4 前端 Vitest

| AC | 测试文件 | 关键 mock |
|---|---|---|
| AC-store-cancel-action | `web/src/stores/__tests__/downloader.spec.ts` (扩展) | mock `apiCancelDownload` 返 `{status:'canceled', progress:42}` → store.cancelDownload('frpc') → assert `store.frpc.status==='canceled'`, `store._timers['frpc']` 不存在。 |
| AC-ui-cancel-button-visible-when-downloading | `web/src/components/__tests__/AppLayout.spec.ts` (扩展) | mount with `binMissing=['frpc']` + downloaderStore 注入 `frpc.status='downloading'` → 找到 `n-button[text="取消"]`, 可点击。 |
| AC-ui-cancel-button-hidden-when-not-downloading | 同上 | 切到 canceled/success/failed/idle → "取消" 按钮 absent。 |
| AC-ui-upload-disabled-when-sibling-downloading | `web/src/components/__tests__/UploadBinButton.spec.ts` (扩展) | mount 注入 downloaderStore.frpc.status='downloading' → upload 按钮 `disabled=true`, tooltip 文本含"取消"。 |

### 5.5 E2E（playwright，T-006 框架已存在）

| AC | 测试文件 | 关键步骤 |
|---|---|---|
| AC-e2e-cancel-then-upload-happy-path | `e2e/download-cancel.spec.ts` (新增) | mock GitHub API 让 archive 卡 30s → 点"一键下载 frpc" → 进度条出现 → 点"取消" → 主按钮文案变 "已取消，点击重试" + warning 色 → 文件选择器选合法 binary → 上传成功 toast。 |

---

## §6 Reuse audit

| 需求点 | 已有代码 | 文件路径 | 决策 |
|---|---|---|---|
| Per-kind 锁 / 状态 | `Manager.states` + `Manager.mu` | `internal/downloader/downloader.go:51-67` | 复用同 mu 守护新增 `cancels` map |
| Sentinel error 模式 | `ErrBadKind` / `ErrAlreadyInProgress` | `internal/downloader/downloader.go:36-40` | 复用 `ErrBadKind`；不引入新 sentinel |
| Status 终态写入 | `setFailed` | `internal/downloader/downloader.go:409-415` | 模式复用 → 新增 `setCanceled` |
| 落盘 atomic rename + chmod | `Manager.Install` | `internal/downloader/install.go:47-125` | 完全复用，cancel 路径不动 Install |
| HTTP 受保护路由 + CSRF | r.Group + SessionAuth + CSRF | `internal/httpapi/router.go:88-128` | 复用，新 endpoint 挂同 Group |
| HTTP error envelope | `writeError(w, status, code, msg, field)` | `internal/httpapi/errors.go` (推断) | 复用 |
| chi URLParam 取 kind | `chi.URLParam(r, "kind")` | `internal/httpapi/handlers_system.go:154` | 复用同 idiom |
| pinia store polling 模式 | `startPolling / stopPolling` | `web/src/stores/downloader.ts:35-59` | 复用，cancelDownload 调 stopPolling |
| naive-ui n-button + n-tooltip + n-progress | `AppLayout.vue` 现有用法 | `web/src/components/AppLayout.vue:22-47` | 完全复用，新增按钮跟现有风格 |
| axios apiClient | `apiClient.post / get` | `web/src/api/client.ts` (推断 from L37 insight) | 复用，新 cancel 调用走同一实例 |
| message.info / .error | `useMessage()` | naive-ui | 复用 |

**新模块 / 新依赖**：无。所有改动是已有模块的内部扩展。

---

## §7 风险与对策

### R-1：ctx cancel 是否真能解除 `io.Copy(archiveTmp, teeReader)`

- **风险**：stdlib `http.Client` + `req.WithContext(ctx)` 在 ctx Done 时是否会让 `resp.Body.Read` 立即返 err？
- **论证**：Go stdlib `net/http.Transport`（`pkg.go.dev/net/http#Transport`）契约保证：当 request ctx 被 cancel，Transport 通过 `CloseIdleConnections` / 内部 cancelKey 机制主动关 connection，从而 `resp.Body.Read` 返回 `use of closed network connection` 或 `context canceled`。Go 1.7+ 文档化行为，与 OS / TCP 状态无关。
- **传播时延**：典型 < 10 ms（连接关闭是本地 syscall）。NFR-1 的 3s 上限远超实际。
- **保护**：单测 `TestCancel_MidDownload` 用 `httptest.Server` 慢响应 + 直接 assert "Cancel 调用后 1s 内 io.Copy 已返回" 锁住该行为；未来 Go 版本若回退，测试立挂。
- **结论**：无需额外保护层；ctx 单一信号即可。

### R-2：Cancel 与 doDownload 即将写 success 的 race

- **风险**：Cancel 拿锁置 canceled → doDownload 主循环跑完 → 拿锁覆盖回 success。canceled 被吞掉。
- **对策**：双层防御：
  1. **ctx 重检**（§2.2 末尾）：doDownload 在 line 285 拿锁写 success 前，先判 `ctx.Err() == context.Canceled` → 若是，跳过 success 写、调 `setCanceled`。
  2. **状态机单调 guard**（§2.4 `setCanceled`）：`setCanceled` 内 `if states[kind].Status == downloading` 才转 canceled。`setFailed` 同款 guard 应一并加（未在原代码，本任务顺手补 —— 但 setFailed 现有代码无 guard，本任务不动以缩小 diff scope，OQ-3）。
- **验证**：`go test -race ./internal/downloader/...` PASS + AC-cancel-repeated-noop。

### R-3：前端轮询周期与状态短暂回弹

- **风险**：cancel HTTP 200 返回前，前端轮询 tick 1 拿到 downloading；cancel 200 后写 canceled；下一拍轮询又拿到... 后端 state？
- **论证**：FR-7 + §2.3 Cancel 阻塞等待至 state 切到 canceled 才返回，因此 cancel 200 返回时后端 state 已是 canceled，下一拍轮询拿到的就是 canceled，**不会回弹**。
- **额外保护**：§4.3 store.cancelDownload 在 finally 内 stopPolling，连"下一拍轮询"都不发，零回弹风险。
- **保留通道**：cancel 失败时 stopPolling 仍触发；若用户希望恢复轮询需手动点"重试"按钮触发 startPolling（与 failed 状态对称）。

### R-4：cancel 后 archive / bin tmp 残留

- **风险**：goroutine 异常退出 / panic 时 defer 是否执行？
- **论证**：Go defer 在 goroutine 任何"正常 return"或 panic 路径都执行。cancel → ctx.Err → setCanceled → return 是正常 return，defer 跑。panic 路径有 `recover` 兜底（如有；当前 doDownload 无显式 recover —— 但 Go runtime 默认会触发 deferred funcs after panic）。
- **保护**：单测 AC-cancel-mid-download 直接 `os.ReadDir(targetDir)` 检 `.dl-*` 前缀文件不残留；触发 cancel 后等 100 ms 跑断言。
- **补充**：§2.5 增加 binTmp 的 defer 兜底（line 244 后），覆盖 cancel 在解压前触发的场景。

### R-5：旧前端读到 canceled 状态

- **风险**：用户多 tab / 慢刷新场景下，旧前端 TS 联合类型不含 canceled，视觉上按钮文案可能"卡"在 downloading。
- **对策**：本任务不引入向后兼容层。用户刷新页面即拿到新前端 bundle（embed 在 Go binary，T-006 insight），不构成 release blocker。
- **声明**：上线后 30 天若有用户反馈"按钮文案怪"，引导刷新解决。

### R-6（新增）：reuse audit 漏点 — `setFailed` 也可能跑在 cancel 状态后

- **风险**：cancel → setCanceled 后，doDownload 主循环若 ctx 不在某个 err 分支前检查，可能继续走 setFailed("HTTP xxx 错误") 路径覆盖 canceled。
- **对策**：§2.2 改造表已列举所有 err 分支，确保每个 `m.setFailed(...)` 前都有 `if errors.Is(ctx.Err(), context.Canceled) { setCanceled; return }` 兜底（HTTP body read / extract / Install 三段）。
- **额外保护**：可选 — 给 `setFailed` 也加单调 guard `if Status == downloading`（与 setCanceled 对称）。本任务**不强加**（缩小 diff），但 reviewer 可在 P3 NIT 提议。

---

## §8 Migration / rollout plan

### 8.1 向后兼容

- **OpenAPI**：`DownloadState.status` 增加 enum 值是**只增不减**变更 → 旧 client（不识别 canceled 的）会把它当未知 string，UI 兜底显示原文 → 不破坏；新 enum 值出现在已部署的旧前端只在"取消下载"特性触发时（旧前端无取消按钮 → 不会触发 → 0 旧用户受影响）。
- **后端 Status() 返回字段**：仅 status 值集合扩展；字段名 / 结构不变。
- **HTTP 既有 endpoint**：download-bin / download-status / upload-bin 路径 / 方法 / 请求 / 响应**全部不变**；唯一行为变化是 uploadBin 409 的 error.message 文案精化（向后兼容 — error.message 一直是 human-readable / 非契约字段）。

### 8.2 数据迁移

- 无持久化层改动。`Manager.states` 是进程内内存；服务重启即"全部 idle"，与现有行为一致。
- 无 schema / migration 脚本。

### 8.3 Feature flag

- 不引入 feature flag。功能 GA 直接上。
- 若上线后发现极端 race（NFR-5 未覆盖到的场景），回滚手段：revert 整个 PR；不会留下 schema 残骸（无 migration）。

### 8.4 部署节奏

- Go binary + 前端 bundle 是 single 部署单元（T-006 embed）→ 单 commit 落 main → CI 出 release 资产 → 用户 install.sh / install.ps1 重装/升级即可。无分批 / 灰度需求。

### 8.5 回滚

- git revert 单 commit 即可（涉及 6-7 个文件）。
- 用户已升级后又回滚的场景：state 字段仅扩展、未删；回滚后旧 binary 处理"未知" canceled 状态会把它视作 idle 或 failed（取决于代码路径，但都不崩溃）。

---

## §9 Out-of-scope & open questions

### 9.1 明确的范围边界（与 01 §6.2 对齐 + 本设计追加）

- **不批量取消**：无 "cancel-all" endpoint。
- **不引入 SSE / WebSocket**：保持 1s 轮询。
- **不区分多会话**：Manager 进程级单例。
- **不实现 pause / resume**：cancel 是硬中止。
- **不引入 ctx 化 `resolveLatestAsset`**：见 OQ-1。
- **不给 `setFailed` 加单调 guard**：见 R-6 / OQ-3，缩小 diff scope。
- **UploadBinButton 不引入 confirm dialog**：见 §4.5 与 01 FR-9 的差异说明；改为 tooltip + disabled。

### 9.2 Architect 决策与 Open Questions

- **OQ-1（Architect 决策）**：`resolveLatestAsset` 不 ctx 化。理由：apiClient 已有 60s 总超时；GitHub API 阶段典型耗时 < 1s；用户在 API 阶段点取消的实际窗口极小（< 1s）；ctx 化让所有签名漂移（需 NewRequestWithContext + 重写 apiClient.Do 用法 + 错误分支），收益比差。**若 reviewer 反对**，5 行代码就能改回。
- **OQ-2（Architect 决策）**：canceled 状态的 progress 字段保留 cancel 那一刻的值（不清零、不写）。理由：调试时能看到"卡在 23% 取消"对网络诊断有帮助；前端按钮已用文案"已取消，点击重试"传达语义，progress 不会引起歧义（OQ-5 进度条隐藏，progress 值仅 DevTools 可见）。
- **OQ-3（Architect 决策）**：本任务**不动** `setFailed` 单调 guard；只给 `setCanceled` 加。理由：setFailed 是现有代码，触发它的所有路径已是 doDownload 内部 err 分支（同 goroutine、走单一时间线），不存在外部并发；加 guard 是过度防御。reviewer 可在 P3 NIT 提议补，本任务不视为必需。
- **OQ-4（Architect 决策）**：HTTP error code 422 vs 400 给 "kind 非法"。选 **422**（与 uploadBin / downloadBin kind 校验一致）。01 §3 FR-4 写的是 400，但 422 是 codebase 一致约定（grep `kind 必须为 frpc 或 frps` 三处都用 422）。本设计**修正** 01 中此处的 400 → 422。
- **OQ-5（PM-DECIDED, 01 §9.5）**：canceled 状态进度条隐藏（不保留位置）。已在 §4.4 模板实现。

### 9.3 待 Gate 评审的 Open Questions

- 上述 OQ-1 / OQ-3 / OQ-4 三项 Architect 自决，gate-reviewer 可拒。
- §4.5 UploadBinButton 从 dialog 改 tooltip 是设计偏离 01 FR-9 的最显著点，明示交 gate-reviewer 审；若 PM 要求严格按 01，回退到 dialog 实现成本 ~60 行 Vue。

---

## §10 跨 stage 协调（insight 收割防御）

- **L42（注释引用归档路径）**：本任务的代码注释引用 02 设计文档路径处，写双路径：`docs/features/download-cancel-and-upload-decouple/02_SOLUTION_DESIGN.md 或归档后 docs/features/_archived/download-cancel-and-upload-decouple/02_SOLUTION_DESIGN.md`。涉及位置至少：
  - `downloader.go` Manager 字段 `cancels` 上方注释；
  - `Cancel` 方法 doc 注释。
- **L43（07_DELIVERY.md insight 段裸标题）**：PM 写 07 时必须用裸 `## Insight`（不是 `## §N Insight`），由 PM 自检 grep。
- **L21（06_TEST_REPORT.md adversarial 段裸标题）**：QA 写 06 时 `## Adversarial tests`（无数字前缀），verify_all E.6 闸门。
- **L41 / L44（reviewer 不落盘）**：gate / code reviewer 派发 prompt 必须含"必须直接写到 0X_*.md 文件"；PM 已知，不需 Architect 处理。
- **L29 / L40（前后端字段漂移）**：§3.4.3 已给出共契清单；reviewer 必须按表格 grep 校验三层一致。

---

## §11 实施步骤（给 Developer 用，按依赖顺序）

1. **OpenAPI 先行**（锚定字段名）：
   - `openapi.yaml` line 326-338：`DownloadState.status` 加 enum 5 值。
   - `openapi.yaml` line 1317 后：追加 `/api/v1/system/download-cancel/{kind}` path block（§3.4.2）。

2. **后端 downloader 包**：
   - `internal/downloader/downloader.go`：
     - line 33：加 `StatusCanceled = "canceled"`。
     - line 51-67：`Manager` 加 `cancels map[string]context.CancelFunc`。
     - line 72-82：`New` 初始化 `cancels: make(...)`。
     - line 157-289：`doDownload` 按 §2.2 改造（ctx 化 + 三处 err 分支 ctx 重检 + 末尾 ctx 重检 + 状态单调 guard + binTmp defer）。
     - 新增 `Cancel` 方法（§2.3）。
     - 新增 `setCanceled` 方法（§2.4）。
   - 文件顶部 import 加 `"context"`（若已有则免）。

3. **后端 httpapi 包**：
   - `internal/httpapi/handlers_system.go`：
     - line 466-468：uploadBin 错误消息精化（§3.3）。
     - 新增 `downloadCancel` handler（§3.1）。
   - `internal/httpapi/router.go` line 122 之后：挂 `r.Post("/system/download-cancel/{kind}", h.downloadCancel)`。

4. **前端 types + api + store**：
   - `web/src/types.ts` line 100-104：`DownloadState.status` 加 `'canceled'`。
   - `web/src/api/downloader.ts`：追加 `apiCancelDownload`。
   - `web/src/stores/downloader.ts`：追加 `cancelDownload` action + import。

5. **前端组件**：
   - `web/src/components/AppLayout.vue`：模板加取消按钮 + 5 状态扩展 + `handleCancel` handler（§4.4）。
   - `web/src/components/UploadBinButton.vue`：tooltip + disabled + siblingDownloading computed（§4.5）。

6. **测试**：
   - Go：`internal/downloader/downloader_test.go` 追加 §5.1 全部用例；`internal/httpapi/handlers_system_test.go` 追加 §5.2 全部用例。
   - Vitest：`web/src/stores/__tests__/downloader.spec.ts` + 组件测试追加 §5.4。
   - E2E：`e2e/download-cancel.spec.ts` 新建（§5.5，若 T-006 框架可用）。

7. **verify_all**：
   - 跑 `scripts/verify_all`（PS / sh 双端，与项目红线一致）PASS 才能声明完成。
   - 关注 E.6 标题闸门（adversarial / insight 段标题）—— 但这是 QA / PM 阶段的关切，Developer 阶段不必关。

8. **PR 描述**：列举 OpenAPI / Go / TS 三层字段共契一致性自检 grep 命令（防 L29 漂移）。

---

## §12 Verdict

**READY**

- 所有 FR / NFR / AC 均映射到具体模块改动 + 测试用例。
- 与 PM 决策方向（候选 A）一致；与 T-025 决策（client.Timeout=0）兼容。
- 关键风险（R-1..R-6）均有论证 + 测试覆盖。
- 三处 Architect 自决（OQ-1 / OQ-3 / OQ-4）+ 一处对 01 的设计偏离（§4.5 dialog→tooltip）明示交 gate-reviewer，回退成本均可控。
- 无 blocking 上游缺口。

Developer 可以无歧义实施。
