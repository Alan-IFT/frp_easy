# 02 — 方案设计（T-038 boot-autostart-hardening）

> 由 Solution Architect（PM 上下文角色化）产出。模式：`full`。
> 上游：[01_REQUIREMENT_ANALYSIS.md](./01_REQUIREMENT_ANALYSIS.md)（Verdict=READY）。

## 1. Architecture summary

在 4 层同时加固"开机即可用"硬保证，每层独立修复一个实测主因：

1. **systemd 层**：unit 文件 `After=network.target` → `After=network-online.target + Wants=network-online.target`，让 frp-easy 等到网络真正在线。
2. **FRP 上游层**：`internal/frpconf/render.go` 渲染 frpc.toml 时显式加 `loginFailExit = false`，让 frpc 在登录失败时无限重连（这是 frp 自己的 retry 机制，本质 idempotent）。
3. **frp-easy application 层**：`autoRestoreProcs` 从一次性调用改为 per-kind 异步 retry goroutine，5 次指数 backoff（5s / 15s / 45s / 120s / 300s），与用户主动 UI 操作竞态安全；最后状态写 kv `system.autorestore.last`。
4. **UX 层**：新增 `internal/svcprobe/` 跨平台探测包 + `GET /api/v1/system/service-status` 端点 + Dashboard 顶部"服务化状态"卡片，让用户 0 click 看到 supervised / boot-autostart / 上次自动恢复结果。

新增一个 install.sh / install.ps1 末尾的"自检"步骤（systemctl is-active + is-enabled 双 PASS / sc.exe query + qc 双 PASS）防止安装"看起来成功但服务没起来"的沉默失败。新增 README 与 install-service.sh `--help` 的字面警告锚字串 `[boot-autostart-fix]` 让 verify_all 静态闸门守门未来回退。

## 2. Affected modules

### 新增

- `internal/svcprobe/probe.go` —— 跨平台服务化状态探测接口
- `internal/svcprobe/probe_linux.go` —— Linux/systemd 实现（build tag `linux`）
- `internal/svcprobe/probe_windows.go` —— Windows/SCM 实现（build tag `windows`）
- `internal/svcprobe/probe_other.go` —— darwin / 其它 OS 兜底（build tag `darwin,!linux,!windows`）
- `internal/svcprobe/probe_test.go` —— 单测
- `web/src/components/ServiceStatusCard.vue` —— UI 卡片组件
- `web/src/composables/useServiceStatus.ts` —— 状态获取 composable

### 编辑

- `cmd/frp-easy/main.go` —— `autoRestoreProcs` 重构 + 裸跑警告横幅
- `internal/frpconf/render.go` —— 加 `LoginFailExit` 字段并恒设 `false`
- `internal/frpconf/render_test.go` —— 单测确认新字段输出
- `internal/httpapi/router.go` —— 挂 `/api/v1/system/service-status` 路由
- `internal/httpapi/handlers_system.go` —— 实现 `systemServiceStatus` handler
- `scripts/install-service.sh` —— 改 `After=` + 加自检
- `scripts/install-service.ps1` —— 加 `depend= Tcpip/Dnscache` best-effort + 加自检
- `scripts/install.sh` —— 步骤 7.5 自检 + exit code 4
- `scripts/install.ps1` —— 同上
- `web/src/pages/Dashboard.vue` —— mount ServiceStatusCard
- `README.md` —— "或：手动下载发布包" 段加 `[boot-autostart-fix]` 警告块
- `docs/DEPLOYMENT.md` —— 同款警告
- `scripts/verify_all.sh` —— 新增 4 个 grep 闸门
- `scripts/verify_all.ps1` —— 同上（insight L26 双实现对账）
- `docs/dev-map.md` —— 项目导航（dev 阶段更新）

## 3. Module decomposition

### 3.1 `internal/svcprobe/`

```go
package svcprobe

// Status 是探测结果。
type Status struct {
    Supervised    bool   `json:"supervised"`     // 当前进程被 systemd / SCM 拉起
    Supervisor    string `json:"supervisor"`     // "systemd" | "windows-service" | "none"
    BootAutostart bool   `json:"boot_autostart"` // 开机自启已就位
    RunAs         string `json:"run_as"`         // 进程实际 owner（user-visible 信息）
    ProbeError    string `json:"probe_error,omitempty"` // 探测失败时的原因
}

// Probe 返回当前进程的服务化状态。任何探测失败必须 best-effort 降级到 supervised=false / BootAutostart=false，不能 panic。
// 5s 总时长预算（适用于 Windows sc.exe qc / Linux systemctl is-enabled 等可能阻塞的外部调用）。
func Probe(ctx context.Context) Status
```

**probe_linux.go**:
- `supervised`: `os.Getenv("INVOCATION_ID") != ""`（systemd 自动注入，参 systemd 232+ 文档）
- `supervisor`: 若 supervised 则 `"systemd"`，否则 `"none"`
- `bootAutostart`: 跑 `systemctl is-enabled frp-easy.service`，stdout trim 后 `== "enabled"`
- `runAs`: `os.Getenv("USER")` 或 `os.Getuid()` 反查 passwd

**probe_windows.go**:
- `supervised`: `svc.IsWindowsService()`（golang.org/x/sys/windows/svc 已有依赖）
- `supervisor`: 若 supervised 则 `"windows-service"`，否则 `"none"`
- `bootAutostart`: 跑 `sc.exe qc frp-easy`，stdout 含正则 `START_TYPE\s*:\s*2\s+AUTO_START`
- `runAs`: 若 supervised → "LocalSystem"（服务模式默认）；否则当前 user via `os/user.Current()`

**probe_other.go** (darwin):
- 全部返回 `Status{Supervisor: "none"}`

### 3.2 `cmd/frp-easy/main.go` — autoRestoreProcs 重构

```go
// autoRestoreProcs 在启动尾巴跑：根据 kv.mode.{kind}.enabled 决定是否拉起子进程。
// 失败时启动 per-kind retry goroutine，5 次指数 backoff，全失败后写 kv.system.autorestore.last。
// 与用户主动操作竞态安全：retry 在每轮 sleep 后检查 state，若已 running/starting/stopping 即退出。
func autoRestoreProcs(rootCtx context.Context, store *storage.Store, pm *procmgr.Manager, loc binloc.Locator, logger *slog.Logger, configPaths map[string]string) {
    // first attempt（同步，与现状对齐，保证 ready gate 之前至少试过一次）
    // ...同 §3.2 原逻辑读 kv.mode.{kind}.enabled / loc.Missing / configPaths...

    // 失败 / 二进制缺失 → 持久化 attempt 0
    // 二进制缺失 = "永久失败"，不 retry
    // first attempt 失败 = 启 retry goroutine

    for _, kind := range []string{"frpc", "frps"} {
        // ...
        go retryRestoreLoop(rootCtx, store, pm, kind, logger)
    }
}

// retryRestoreLoop 在 goroutine 内执行 5 次指数 backoff retry。
// 退出条件：成功 / 5 次失败 / state ∈ {running, starting, stopping}（用户介入）/ rootCtx 取消。
const retryBackoff = []time.Duration{
    5 * time.Second,
    15 * time.Second,
    45 * time.Second,
    120 * time.Second,
    300 * time.Second,
}

func retryRestoreLoop(ctx context.Context, store *storage.Store, pm *procmgr.Manager, kind string, logger *slog.Logger) {
    attempts := []AutoRestoreAttempt{}  // 用于 kv 持久化
    for i, d := range retryBackoff {
        select {
        case <-ctx.Done():
            persistAutoRestoreLast(store, kind, attempts, "canceled")
            return
        case <-time.After(d):
        }
        // 用户介入检测：若 state 已变 → 退出
        if st := pm.Status(kind).State; st != procmgr.StateStopped && st != procmgr.StateError {
            logger.Info("auto-restore retry aborted: user-initiated state", "kind", kind, "state", st)
            persistAutoRestoreLast(store, kind, attempts, "user-initiated")
            return
        }
        logger.Info("auto-restore retry", "kind", kind, "attempt", i+1, "of", len(retryBackoff))
        _, err := pm.Start(kind)
        attempts = append(attempts, AutoRestoreAttempt{Index: i + 1, OK: err == nil, Reason: errString(err), At: time.Now().UTC()})
        if err == nil {
            persistAutoRestoreLast(store, kind, attempts, "ok")
            return
        }
    }
    // 5 次全失败
    logger.Error("auto-restore exhausted", "kind", kind)
    persistAutoRestoreLast(store, kind, attempts, "exhausted")
}
```

**关键不变量**:
- `autoRestoreProcs` 同步 first attempt 与现状字节对齐（NFR-9 启动序列零字节改）。
- retry goroutine 必须用 `rootCtx`（从 `run()` 顶层传入），让 SIGTERM 能取消。
- `persistAutoRestoreLast` 用 `store.KVSet(ctx, "system.autorestore.last", json.Marshal(...))`，5s timeout context。

### 3.3 `internal/frpconf/render.go` — loginFailExit 强制 false

```go
type frpcRoot struct {
    ServerAddr    string          `toml:"serverAddr,omitempty"`
    ServerPort    int             `toml:"serverPort,omitempty"`
    LoginFailExit *bool           `toml:"loginFailExit,omitempty"` // T-038: 必填 false 让 frpc 在登录失败时无限重连
    // ... 其它字段不变
}

// RenderFrpc 内部恒设 LoginFailExit = false（指针 + omitempty 让 nil 与显式 false 区分清晰）
func RenderFrpc(in FrpcRenderInput) ([]byte, error) {
    // ...校验段...
    no := false
    root := frpcRoot{
        ServerAddr:    in.ServerAddr,
        ServerPort:    in.ServerPort,
        LoginFailExit: &no, // T-038: 让 frpc 自己 retry，配合 frp-easy autoRestoreProcs 双层防御
    }
    // ...其它段不变
}
```

**TOML 输出示例**（参考真实测试机渲染）：
```toml
serverAddr = '43.136.30.208'
serverPort = 7001
loginFailExit = false  # ← 新增
[log]
to = '/opt/frp-easy/.frp_easy/logs/frpc.log'
# ...
```

frp 上游对此字段语义：`true`（默认）= 登录失败立即 exit；`false` = 进入重连循环（dial timeout / heartbeat 监控）。这是项目从首日就该有的配置（frp 文档明示）。

### 3.4 `internal/httpapi/handlers_system.go` — systemServiceStatus

```go
// SystemServiceStatusResponse 是 GET /api/v1/system/service-status 的响应体。
type SystemServiceStatusResponse struct {
    Supervised    bool                  `json:"supervised"`
    Supervisor    string                `json:"supervisor"`
    BootAutostart bool                  `json:"boot_autostart"`
    RunAs         string                `json:"run_as"`
    AutoRestore   AutoRestoreSection    `json:"auto_restore"`
}

type AutoRestoreSection struct {
    EnabledKinds []string             `json:"enabled_kinds"`
    LastRun      *AutoRestoreLastRun  `json:"last_run,omitempty"`
}

type AutoRestoreLastRun struct {
    Timestamp string                  `json:"timestamp"`  // RFC3339 UTC
    Outcome   string                  `json:"outcome"`    // "ok" | "exhausted" | "user-initiated" | "canceled"
    Attempts  []AutoRestoreAttempt    `json:"attempts"`
}

type AutoRestoreAttempt struct {
    Index  int    `json:"index"`
    OK     bool   `json:"ok"`
    Reason string `json:"reason,omitempty"`
    At     string `json:"at"` // RFC3339 UTC
}

// systemServiceStatus handles GET /api/v1/system/service-status.
// 实现：probe + 读 kv mode.* + 读 kv system.autorestore.last（JSON 反序列化）。
// 总预算 5s context；任一探测失败降级为 supervised=false / boot_autostart=false / lastRun=nil。
func (h *handlers) systemServiceStatus(w http.ResponseWriter, r *http.Request) {
    ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
    defer cancel()
    
    status := svcprobe.Probe(ctx)
    
    enabledKinds := []string{}
    for _, k := range []string{"frpc", "frps"} {
        if readBoolKV(ctx, h, "mode."+k+".enabled") {
            enabledKinds = append(enabledKinds, k)
        }
    }
    
    var lastRun *AutoRestoreLastRun
    if v, ok, _ := h.deps.Store.KVGet(ctx, "system.autorestore.last"); ok {
        var lr AutoRestoreLastRun
        if json.Unmarshal([]byte(v), &lr) == nil {
            lastRun = &lr
        }
    }
    
    writeJSON(w, http.StatusOK, SystemServiceStatusResponse{
        Supervised:    status.Supervised,
        Supervisor:    status.Supervisor,
        BootAutostart: status.BootAutostart,
        RunAs:         status.RunAs,
        AutoRestore: AutoRestoreSection{
            EnabledKinds: enabledKinds,
            LastRun:      lastRun,
        },
    })
}
```

### 3.5 `web/src/composables/useServiceStatus.ts`

```ts
import { ref, onMounted } from 'vue'
import { apiClient } from '@/api/client'

export interface ServiceStatus {
  supervised: boolean
  supervisor: 'systemd' | 'windows-service' | 'none'
  boot_autostart: boolean
  run_as: string
  auto_restore: {
    enabled_kinds: string[]
    last_run?: {
      timestamp: string
      outcome: 'ok' | 'exhausted' | 'user-initiated' | 'canceled'
      attempts: Array<{ index: number; ok: boolean; reason?: string; at: string }>
    }
  }
}

export function useServiceStatus() {
  const status = ref<ServiceStatus | null>(null)
  const loading = ref(false)
  const error = ref<string | null>(null)

  async function refresh() {
    loading.value = true
    error.value = null
    try {
      const resp = await apiClient.get<ServiceStatus>('/system/service-status')
      status.value = resp.data
    } catch (e: any) {
      error.value = e?.message ?? '加载失败'
    } finally {
      loading.value = false
    }
  }

  onMounted(refresh)
  return { status, loading, error, refresh }
}
```

### 3.6 `web/src/components/ServiceStatusCard.vue`

模板要点（伪代码 / 高层结构）：
```vue
<template>
  <!-- 锚字串 [boot-autostart-fix] 让 verify_all 闸门可 grep -->
  <n-card title="服务化状态" :bordered="true" :class="cardClass">
    <n-descriptions :column="2" size="small">
      <n-descriptions-item label="监管方式">
        <n-tag :type="supervisedTagType">{{ supervisorLabel }}</n-tag>
      </n-descriptions-item>
      <n-descriptions-item label="开机自启">
        <n-tag :type="bootAutostartTagType">{{ bootAutostartLabel }}</n-tag>
      </n-descriptions-item>
      <n-descriptions-item label="运行用户">{{ status.run_as || '—' }}</n-descriptions-item>
      <n-descriptions-item label="重启后自动恢复">
        <span v-if="status.auto_restore.enabled_kinds.length === 0">无</span>
        <n-tag v-for="k in status.auto_restore.enabled_kinds" :key="k" size="small">{{ k }}</n-tag>
      </n-descriptions-item>
    </n-descriptions>

    <!-- last_run 展示 -->
    <n-collapse v-if="status.auto_restore.last_run">...上次自动恢复详情...</n-collapse>

    <!-- 如何修复折叠区（supervised=false 或 boot_autostart=false 时显示） -->
    <n-collapse v-if="needsFix">
      <n-collapse-item title="如何修复">
        <!-- [boot-autostart-fix] 锚字串：与 README / install-service.sh --help 三处一致 -->
        <pre>{{ fixCommandForPlatform }}</pre>
      </n-collapse-item>
    </n-collapse>
  </n-card>
</template>
```

样式：`needsFix=true` 时 `:class="cardClass"` 加 warning 边框色；正常情况边框 default。

### 3.7 systemd unit 文件（install-service.sh 渲染）

```ini
[Unit]
Description=FRP Easy — frp 可视化管理 UI
Documentation=https://github.com/Alan-IFT/frp_easy
Wants=network-online.target
After=network-online.target

[Service]
Type=simple
ExecStart=${ESC_BINARY}
WorkingDirectory=${ESC_INSTALL_DIR}
User=${RUN_USER}
Restart=on-failure
RestartSec=5
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
```

变化：`After=network.target` → `Wants=network-online.target` + `After=network-online.target`（2 行替 1 行）。其余字段保持不变。

### 3.8 install-service.sh / .ps1 自检块

**install-service.sh**（末尾追加，在 `exit 0` 之前）：
```bash
# [boot-autostart-fix] 自检：确认 unit 已 active + enabled 才算装好。
# 失败时打印诊断 + exit 4（与 install.sh 透传同款码值）。
echo "==> 自检：systemctl is-active + is-enabled..."
sleep 1  # 给 systemd 一点点状态推进时间
if ! systemctl is-active --quiet "$UNIT_NAME"; then
    echo "错误：自检失败 —— $UNIT_NAME 未进入 active 状态。" >&2
    systemctl status "$UNIT_NAME" --no-pager -l 2>&1 | sed 's/^/    /' >&2
    exit 4
fi
if ! systemctl is-enabled --quiet "$UNIT_NAME"; then
    echo "错误：自检失败 —— $UNIT_NAME 未 enabled（不会开机自启）。" >&2
    exit 4
fi
echo "==> 自检通过：$UNIT_NAME 已 active + enabled"
```

**install-service.ps1**（类似，在 Wait-ServiceRunning 通过之后、`exit 0` 之前）：
```powershell
# [boot-autostart-fix] 自检
Write-Host "==> 自检：sc.exe query + qc..."
& sc.exe query $ServiceName | Out-Null
$qcOut = & sc.exe qc $ServiceName 2>&1
if ($qcOut -notmatch 'START_TYPE\s*:\s*2\s+AUTO_START') {
    Write-Error "自检失败 —— $ServiceName 未配置为 AUTO_START。"
    exit 4
}
Write-Host "==> 自检通过：$ServiceName AUTO_START + RUNNING"

# 最佳努力声明 TCP/IP + DNS 依赖（D-3：失败仅 warn）
$null = & sc.exe config $ServiceName depend= "Tcpip/Dnscache" 2>&1
if ($LASTEXITCODE -ne 0) {
    Write-Host "提示：sc.exe config depend= 配置失败（rc=$LASTEXITCODE），不影响本次安装；服务化下 SCM 通常已隐式等待网络栈。"
}
```

### 3.9 verify_all 新闸门

`scripts/verify_all.sh` 增加 4 行 step（按现有 step helper 格式）：
```bash
# G.6 install-service.sh unit 模板含 network-online.target
step "G.6 install-service.sh network-online dep" "grep -q 'network-online.target' scripts/install-service.sh"

# G.7 render.go 渲染 loginFailExit 字段
step "G.7 render.go has loginFailExit" "grep -q 'LoginFailExit' internal/frpconf/render.go"

# G.8 README 含 [boot-autostart-fix] 锚字串
step "G.8 README [boot-autostart-fix] anchor" "grep -q '\\[boot-autostart-fix\\]' README.md"

# G.9 ServiceStatusCard.vue 含同锚字串
step "G.9 ServiceStatusCard.vue [boot-autostart-fix] anchor" "grep -q '\\[boot-autostart-fix\\]' web/src/components/ServiceStatusCard.vue"
```

`.ps1` 同款（insight L26：双实现对账）：必须用 `Get-Content` 按行读 + `-cmatch` 严格行内匹配，**避免** insight L26 的 Raw 单字符串 + `-match` 假阳性。

## 4. Data model changes

新增 kv key（不动 SQL schema）：

| Key | Value 类型 | 写入者 | 读取者 |
|---|---|---|---|
| `system.autorestore.last` | JSON（AutoRestoreLastRun struct）| `cmd/frp-easy/main.go::persistAutoRestoreLast` | `internal/httpapi/handlers_system.go::systemServiceStatus` |

JSON schema 示例：
```json
{
  "timestamp": "2026-05-25T00:10:20Z",
  "outcome": "exhausted",
  "attempts": [
    {"index": 1, "ok": false, "reason": "process exited within 3s", "at": "2026-05-25T00:10:25Z"},
    {"index": 2, "ok": false, "reason": "process exited within 3s", "at": "2026-05-25T00:10:40Z"},
    ...
  ]
}
```

向后兼容性：reader 必须容忍未来加新字段（json.Unmarshal 默认丢弃未知字段）。schema 版本化无必要（单值 kv，向后兼容由 JSON 包默认行为保证）。

## 5. API contracts

### GET /api/v1/system/service-status

**Auth**: 受保护（SessionAuth + ReadyGate）。
**Method**: GET only。
**Request**: 无 body。
**Response 200**: `SystemServiceStatusResponse`（见 §3.4 struct）。
**Response 401**: 未登录（中间件统一）。
**Response 503**: ReadyGate 拦截（ready=false 时返 Retry-After: 2）。
**Timeout**: 5s（probe 兜底，超时返回 supervised=false + probe_error="probe timeout"）。
**Cacheable**: 否（每次调用都 re-probe，因为外部环境（systemd 状态、sc.exe 配置）可能在两次调用间被运维改）。

## 6. Sequence / flow

### 6.1 启动序列（修订）

```
systemd 拉起 frp-easy
  ↓ (Wants/After=network-online.target 保证网络已在线)
main.go run() 启动
  ↓
appconf.Load → storage.Open → binloc / procmgr / 等初始化
  ↓
HTTP server.Serve goroutine
  ↓
autoRestoreProcs(rootCtx, ...)
  ├── for kind in [frpc, frps]:
  │     read kv.mode.{kind}.enabled
  │     if !enabled: skip
  │     if binary missing: persistLast(reason="binary missing"); skip
  │     pm.Start(kind)  ← 同步 first attempt
  │     if err != nil:
  │         go retryRestoreLoop(rootCtx, ..., kind)  ← 启 retry goroutine
  ↓
ready.Store(true)  ← 不被 retry 阻塞
  ↓
select { sigCh | stopCh | serveErr }  ← 主循环
```

### 6.2 retry goroutine 生命周期

```
retryRestoreLoop(ctx, kind):
  attempts := []
  for i, d in [5s, 15s, 45s, 120s, 300s]:
    select {
      case <-ctx.Done(): persistLast(reason="canceled"); return
      case <-time.After(d):
    }
    if pm.Status(kind).State != stopped|error:
      // 用户介入
      persistLast(reason="user-initiated"); return
    err := pm.Start(kind)
    attempts = append(attempts, ...)
    if err == nil:
      persistLast(reason="ok"); return
  // 5 次全失败
  persistLast(reason="exhausted"); return
```

### 6.3 service-status 请求流

```
GET /api/v1/system/service-status
  ↓ (middleware: ReadyGate → SessionAuth)
handler.systemServiceStatus(ctx, 5s timeout):
  svcprobe.Probe(ctx)  ← 跨平台
    Linux: getenv("INVOCATION_ID") + spawn systemctl is-enabled
    Win:   svc.IsWindowsService() + spawn sc.exe qc
    Other: Status{Supervisor:"none"}
  readBoolKV(mode.frpc.enabled) + readBoolKV(mode.frps.enabled)
  store.KVGet("system.autorestore.last") → json.Unmarshal
  writeJSON(200, SystemServiceStatusResponse{...})
```

## 7. Reuse audit

| Need | Existing code | File path | Decision |
|---|---|---|---|
| Windows service detection | `svc.IsWindowsService()` | `cmd/frp-easy/service_windows.go` | Reuse — extract to svcprobe |
| systemd INVOCATION_ID | (none used yet) | — | New（标准 systemd contract，无新依赖） |
| kv 读写 | `store.KVGet/KVSet` | `internal/storage/kv.go` | Reuse as-is |
| autoRestoreProcs first-attempt | `cmd/frp-easy/main.go::autoRestoreProcs` L433-467 | 同 | 重构（保留 first attempt 逻辑） |
| procmgr.Start 启动子进程 | `pm.Start(kind)` | `internal/procmgr/manager.go` | Reuse as-is，不动 |
| logger 双轨 stderr + ui.log | T-022 idiom | `cmd/frp-easy/main.go::run` L186-200 | Reuse |
| HTTP handler 模式 | `handlers_system.go::systemReady` 等 | 同 | Reuse pattern |
| Vue composable + Naive UI 卡片 | `useThemeVars` / `n-card` 等 | T-036 idiom | Reuse |
| `LoginFailExit` TOML 字段 | (none) | — | 新增字段到 frpcRoot struct，无新依赖 |
| `Wants=network-online.target` systemd 语义 | (none used) | — | 新增 unit 模板字段，无新依赖（systemd 标准） |

无新依赖项。所有新代码使用 stdlib + 项目已用的 chi / slog / go-toml/v2 / naive-ui。

## 8. Risk analysis

| Risk | 概率 | 影响 | Mitigation |
|---|---|---|---|
| **R-1**: `systemctl is-enabled frp-easy` 在 frp-easy 进程内调用产生 fork 噪声 | 高 | 性能微影响（< 50ms / call）；service-status API 每次调都 spawn | 接受。NFR-2 < 100ms 预算足。频次低（仅当用户打开 Dashboard / 刷新页面）。 |
| **R-2**: retry goroutine 在 5min sleep 期间收到 SIGTERM，主循环优雅关停被它阻 30s+ | 中 | 重启 frp-easy 慢 | rootCtx 在 SIGTERM 时立即取消，retry goroutine select case <-ctx.Done() 退出。pm.Shutdown 不等 retry goroutine（fire-and-forget）。 |
| **R-3**: `Wants=network-online.target` 让 systemd boot 慢（等 NetworkManager-wait-online 超时 ~30s） | 中 | 用户 reboot 后稍慢看到服务 | 已 verified：测试机 `NetworkManager-wait-online.service = enabled`，正常网络环境下该 unit 5s 内即 active。慢网络是用户预期（"等网络好了才能用 frp"）。 |
| **R-4**: `loginFailExit = false` 让 frpc 永远不退出，掩盖配置错误（如 token 错） | 低 | 用户 token 配错时 UI 看到 running 但 frpc 一直连不上 | frpc 自身日志会写 `auth failed` 等明确错误。Dashboard frpc 卡片已显示 lastErr。可观测性不丢。 |
| **R-5**: Windows `sc.exe qc` 在某些 Windows 11 N 版本上输出格式不同 | 低 | boot_autostart 探测假阴性 | 探测失败统一降级为 boot_autostart=false（不 panic）。用户看到"未自启"卡片 + 修复命令，是 fail-safe 路径。 |
| **R-6**: frpc.toml 用户手写 loginFailExit=true 后被 UI 改 server 配置触发 render 重写 → 覆盖为 false | 中 | 用户对此行为不知情 | 决策（D-8）：当前路径就是 UI 渲染。render.go 强 false 是"项目认为正确"的默认（frp 默认 true 是 frp 上游设计缺陷的表现）。文档化在 README 配置段："frp_easy 渲染的 frpc.toml 含 `loginFailExit=false`，让 frpc 自动重连。" |
| **R-7**: 多任务并行的旧 frp-easy 进程占 7800 端口让 verify_all C.1 FAIL | 高 | dev/verify_all 假阴性 | insight L30 / L38 已有模式：git stash 隔离对照。本任务延续。 |
| **R-8**: install-service.sh `sleep 1` 不够 systemd 推进到 active | 低 | 自检假阴性 | 改用 `systemctl is-active --quiet` 轮询 ≤ 5s（同款 T-019 Wait-ServiceRunning idiom），失败才报错。dev 阶段细化。 |
| **R-9**: kv `system.autorestore.last` JSON value 在升级期被旧 reader 读到不认识的字段 | 低 | 反序列化失败 | json.Unmarshal 默认丢弃未知字段；本次 schema 是首次引入，无 backward concern。 |
| **R-10**: e2e Playwright fixture（T-033）已经会在 dev 跑 verify_all 时启 frp-easy 后端，与 retry goroutine 写 kv 互相打架 | 中 | spec 假阳/假阴 | 单测覆盖足够；e2e 不直接触发 retry（用户在 ready gate 之后没装/没启 frpc 才会触发）。 |

## 9. Migration / rollout plan

- **零停机 / 零配置迁移**：本任务对用户已部署环境的影响：
  - 升级跑 `install.sh ... | sudo bash -s -- --role <X>` 一键安装 → install-service.sh 自动覆盖旧 unit（已有 EXISTED 分支） → daemon-reload + restart frp-easy。新 unit 立即生效。
  - 重启后第一次 autoRestoreProcs 会用新 retry 逻辑。
  - 已存在的 `runtime/frpc.toml` 不会被升级脚本动；下次用户在 UI 改 server / proxies → handlers_proxies / handlers_server 触发 `applyConfigBestEffort` → 重新渲染 → 新 frpc.toml 含 loginFailExit=false。
- **强制即时生效（可选 hotfix 命令）**：dev 阶段可让 README 引导用户在 UI 上"重启 frpc" → handlers_proc.procRestart → procmgr.Restart → 子进程重启加载新 loginFailExit。
- **Rollback**：纯 forward 路径，不需要 rollback；若新 retry 逻辑产生意外副作用，git revert + 重新跑 install.sh 即可。

## 10. Out-of-scope clarifications

- 本设计不引入 systemd timer / cron 类外部调度——retry 在 frp-easy 进程内 goroutine 跑，与服务进程生命周期绑定。
- 不修改 procmgr `waitUntilStable(3s)` 行为——它是单次启动语义的合理时长（让 frp 走完自身初始化）；retry 在外层兜底。
- 不动 storage SQL schema——kv 已有，足够。
- 不动 frp_easy.toml（appconf）—— 用户配置 UI 监听地址等，与本任务正交。
- UI 卡片不显示 retry 进行中的实时进度（如"retry attempt 2/5"）——`last_run` 是上次完成快照，足够。实时进度增加 SSE/polling 复杂度，本期不要。

## 11. Partition assignment

项目用 single Developer 模式（`.harness/agents/dev-*.md` 派分区文件均存在，但 insight L27 说明 SDK 上下文会全部 collapse 到 PM 角色化）。仍保留分区表清晰责任：

| File | Partition | New / Edit |
|---|---|---|
| `internal/svcprobe/probe.go` | dev-backend | new |
| `internal/svcprobe/probe_linux.go` | dev-backend | new |
| `internal/svcprobe/probe_windows.go` | dev-backend | new |
| `internal/svcprobe/probe_other.go` | dev-backend | new |
| `internal/svcprobe/probe_test.go` | dev-backend | new |
| `cmd/frp-easy/main.go` | dev-backend | edit |
| `internal/frpconf/render.go` | dev-backend | edit |
| `internal/frpconf/render_test.go` | dev-backend | edit |
| `internal/httpapi/router.go` | dev-backend | edit |
| `internal/httpapi/handlers_system.go` | dev-backend | edit |
| `scripts/install-service.sh` | dev-services | edit |
| `scripts/install-service.ps1` | dev-services | edit |
| `scripts/install.sh` | dev-services | edit |
| `scripts/install.ps1` | dev-services | edit |
| `scripts/verify_all.sh` | dev-services | edit |
| `scripts/verify_all.ps1` | dev-services | edit |
| `web/src/components/ServiceStatusCard.vue` | dev-frontend | new |
| `web/src/composables/useServiceStatus.ts` | dev-frontend | new |
| `web/src/pages/Dashboard.vue` | dev-frontend | edit |
| `README.md` | dev-services | edit |
| `docs/DEPLOYMENT.md` | dev-services | edit |
| `docs/dev-map.md` | dev-services | edit (last) |

### Dispatch order

1. dev-backend（svcprobe / autoRestoreProcs / render.go / handlers）
2. dev-services（install-service.sh / .ps1 + verify_all）
3. dev-frontend（ServiceStatusCard + Dashboard mount）
4. dev-services (再次)（README / DEPLOYMENT / dev-map 收尾）

实际由 PM 单 context 顺序跑（角色 collapse）。

### Parallelism

dev-backend 与 dev-services 第一波（install-service.sh）可并行；UI 与 verify_all 静态闸门同款。但为简化 PM 单 context 顺序跑，按上面 4 步串行执行更易归因。

## 12. Verdict

**READY** — 设计完整可实施，无设计层 blocker。

## 13. SA self-check（dev 实施前最后一次梳理）

- [x] 每个新模块都有 file path + public API 签名
- [x] 每个 risk 都配 mitigation
- [x] reuse audit 表非空，列了 10 项 reuse 决策
- [x] 不引入新依赖（stdlib + 项目已有 svc / chi / toml / naive-ui）
- [x] 双实现对账：verify_all 新 step 必须 .sh + .ps1 双实现（insight L26）
- [x] D-1...D-8 决策与 01 §8 对齐
- [x] AC-4 真机测试机 ssh alan@192.168.100.90 可达 + 实测路径明确
- [x] 修改后的 systemd unit 在测试机上可幂等覆盖（已 verified `EXISTED` 分支）
- [x] retry goroutine 与用户 UI 操作竞态安全（select state check + ctx.Done）
- [x] 升级路径不破坏用户已有 frpc.toml（仅 UI 改配置触发 re-render 才覆盖）

完毕。
