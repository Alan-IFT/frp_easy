# 02 — 解决方案设计 · T-019 windows-service-scm-1053-fix

> Harness 流水线 Stage 2（Solution Architect）。模式：**full**。
> 上游：`docs/features/windows-service-scm-1053-fix/01_REQUIREMENT_ANALYSIS.md`（verdict: READY；§8 全部 10 条 PM-resolved，直接据此设计）。
> 本设计是给 Developer 的"无歧义实现合同"——若 Developer 觉得任何一处需要二次设计决策，请回 PM 而非自行猜测。

---

## 1. Architecture summary

把 `frp-easy.exe` 在 Windows 上改造成**双入口（dual-entry）单二进制**：进程在 `main()` 顶端即调 `svc.IsWindowsService()` 自动判定运行宿主——被 Windows SCM 拉起时走 `svc.Run("frp-easy", &serviceHandler{})` 服务化分支（实现完整 `SetServiceStatus` 状态机：`START_PENDING` + 1 秒心跳 CheckPoint + WaitHint 5 秒 → `RUNNING` → 收到 `SERVICE_CONTROL_STOP` → 触发 ctx.Cancel → `STOP_PENDING` → 优雅关停 procmgr / HTTP / storage → `STOPPED`），手动 / TTY 启动时走现有 `run()` 控制台分支不变。`wrapper.cmd`（`frp-easy-svc.cmd`）整条中间壳被彻底移除：`sc.exe binPath=` 直接指向 `frp-easy.exe`，`os.Chdir(filepath.Dir(os.Executable()))` 在服务化分支首步锁 cwd 替代 wrapper.cmd 的 `cd /d`。`install-service.ps1` / `uninstall-service.ps1` 同步删除 wrapper 生成 / 清理逻辑并加上"残留 wrapper.cmd 防御性清理"；`install-service.ps1` 末尾新增 SCM RUNNING 轮询（最长 30s）让 install.ps1 步骤 7 退出即满足 IS-2。`go.mod` 把 `golang.org/x/sys` 从 indirect 提升为 direct——零新增第三方依赖（已是 v0.44.0 由 modernc.org/sqlite 间接引入）。

---

## 2. Affected modules

> 全部为绝对文件路径；前缀 `<repo>` = `C:\Programs\frp_easy`。

### 2.1 新增文件

| 文件 | 用途 | Build tag |
|---|---|---|
| `<repo>/cmd/frp-easy/service_windows.go` | Windows Service ABI 实现：`svc.Handler` + Execute 状态机 + 1 秒 CheckPoint 心跳 + Stop control code → ctx.Cancel | `//go:build windows` |
| `<repo>/cmd/frp-easy/service_other.go` | 非 Windows 平台空 stub（导出 `runService()` 占位返回 `errors.New("service mode not supported on this platform")`，并提供 `isWindowsService() bool { return false }`），保证 Linux/macOS 编译不引用 `x/sys/windows/svc` | `//go:build !windows` |
| `<repo>/cmd/frp-easy/service_windows_test.go` | Windows 单测：验证 install-service.ps1 不再生成 wrapper.cmd（通过 grep 脚本文本：不含 `frp-easy-svc.cmd` 生成块关键词 `Set-Content -Path $WrapperPath`）、验证 uninstall-service.ps1 仍保留 wrapper.cmd 防御性清理 | `//go:build windows` |

### 2.2 编辑文件

| 文件 | 改动摘要 |
|---|---|
| `<repo>/cmd/frp-easy/main.go` | ① `main()` 顶端（在 flag.Parse 之前）调 `isWindowsService()`（service_*.go 提供）；返回 true → 调 `runService()`（Windows 实现）→ 该函数内部最终调 `run(stopCh)`；返回 false → 走现有 `if err := run(nil); err != nil { ... }`。② `run()` 签名改为 `func run(stopCh <-chan struct{}) error`：select 中追加 `case <-stopCh:` 与 `case s := <-sigCh:` / `case e := <-serveErr:` 三路并存。stopCh 为 nil 时退化为现状（控制台分支不感知）。③ run() 内部 NFR-9 不破：appconf.Load → storage.Open → ... → http.ListenAndServe 启动顺序逐字节保留。 |
| `<repo>/scripts/install-service.ps1` | ① 删除 L68-82 wrapper.cmd 生成块（`$WrapperPath` 定义 + `Set-Content -Encoding Default`）。② `sc.exe create` / `sc.exe config` 的 `binPath=` 改为 `"`"$BinaryPath`""`（直接指向 `<InstallDir>\frp-easy.exe`，沿用 BC-4 验证过的 PowerShell 反引号双引号转义模式）。③ 在 sc.exe stop 后、config 前追加 `marked-for-delete` 探测与轮询（Q8 决议；最长 15s；超时打中文诊断后退出码 2）。④ sc.exe start 成功后追加 `Wait-ServiceRunning -Name $ServiceName -TimeoutSec 30`（Q9 决议），每 500 ms 轮询 `sc.exe query` 直到 `STATE : 4 RUNNING` 或超时；轮询期间每 3 秒打印一行中文进度（AC-19）。⑤ 在脚本第 60 行附近（`$InstallDir` 已定义后、新建服务前）追加防御性 `Remove-Item -Force (Join-Path $InstallDir "frp-easy-svc.cmd") -ErrorAction SilentlyContinue`（升级路径清理旧 wrapper 残留）。 |
| `<repo>/scripts/uninstall-service.ps1` | ① 保留 L70-77 wrapper.cmd 清理逻辑（语义从"删除自己生成的"降级为"清理可能残留的"），改注释为"防御性清理：旧版（< T-019）install-service.ps1 曾生成 wrapper.cmd；本任务移除生成逻辑后，老安装升级再卸载时可能仍有残留"。 |
| `<repo>/scripts/install.ps1` | ① 步骤 7 文案保持"==> [7/8] 注册 Windows 服务..."；移除任何隐含"sc.exe start 失败 1053"的中文（确认当前脚本无 1053 字样，不需改）。② 步骤 7 透传 `install-service.ps1` 退出码语义在修复后变成 0（不再透传 2）—— 现有 L285-290 `$LASTEXITCODE` 透传链条保留不动，仅依赖 install-service.ps1 行为变更间接生效。③ 不在 install.ps1 本身加 SCM 轮询（轮询在 install-service.ps1 内部完成；install.ps1 步骤 7 返回时 SCM 已 RUNNING）。 |
| `<repo>/go.mod` | `require` 块的 `golang.org/x/sys v0.44.0` 从 indirect 块移到 direct 块（与现有 `golang.org/x/crypto v0.51.0` / `golang.org/x/term v0.43.0` 同列）；版本号不变；不动 go.sum。 |

### 2.3 不动文件（重要锚定）

- `<repo>/internal/procmgr/manager.go` — `Manager.Shutdown()` 签名（`func (m *Manager) Shutdown()`）零字节改动；Q7 决议复用现有 graceful 路径。
- `<repo>/internal/procmgr/manager_windows.go` / `manager_unix.go` — 零字节改动。
- `<repo>/internal/appconf/`、`internal/storage/`、`internal/httpapi/`、`internal/logrotate/` — 全部零字节改动。
- `<repo>/scripts/install.sh`、`install-service.sh`、`uninstall-service.sh` — Linux 路径零字节改动（OOS-2）。
- `<repo>/scripts/verify_all.sh` / `verify_all.ps1` — 检查项数量不变（NFR-5）。

---

## 3. Module decomposition

### 3.1 `cmd/frp-easy/service_windows.go`（新增）

**职责**：实现 Windows Service ABI；把 SCM 控制信号映射为现有 `run()` 的 stopCh，让 NFR-9 不破。

**Public API**（包内导出给 main.go 用；非 Go-export 跨包级）：

```go
//go:build windows

package main

import (
    "context"
    "os"
    "path/filepath"
    "time"

    "golang.org/x/sys/windows/svc"
)

// isWindowsService 由 main.go 顶端调用；true 表示进程是被 SCM 拉起。
func isWindowsService() bool {
    inService, err := svc.IsWindowsService()
    if err != nil {
        return false // 安全降级到控制台分支
    }
    return inService
}

// runService 进入 Windows Service 主循环；阻塞直到 SCM Stop 完成。
// 内部锁 cwd → svc.Run → Execute → 通过 stopCh 唤起 run() 优雅退出。
func runService() error {
    // 锁 cwd 到 frp-easy.exe 所在目录，替代 wrapper.cmd 的 `cd /d`。
    // os.Executable() 在 Windows 上原生 UTF-16，不经 host codepage，
    // 对中文 / 空格路径（BC-3 / BC-4）天然正确。
    if exe, err := os.Executable(); err == nil {
        _ = os.Chdir(filepath.Dir(exe))
    }
    return svc.Run("frp-easy", &serviceHandler{})
}

// serviceHandler 实现 svc.Handler。
type serviceHandler struct{}

// Execute 是 SCM 主循环；返回前 status 通道必须最终发 STOPPED。
func (h *serviceHandler) Execute(args []string, r <-chan svc.ChangeRequest, s chan<- svc.Status) (ssec bool, errno uint32) {
    const accepted = svc.AcceptStop | svc.AcceptShutdown
    // 1. 立刻报 START_PENDING + CheckPoint 0 + WaitHint 5s（Q5 决议）。
    s <- svc.Status{State: svc.StartPending, CheckPoint: 0, WaitHint: 5000}

    // 2. 启动心跳 ticker：每 1s 累加 CheckPoint，保活 SCM 直到 run() 报 ready。
    stopCh := make(chan struct{})
    runErrCh := make(chan error, 1)
    readyCh := make(chan struct{})

    // 3. goroutine: run(stopCh) 内部启动 HTTP server 并阻塞；
    //    run() 在 HTTP listen 成功后立刻 close(readyCh)（见下方 main.go 修改）。
    go func() {
        runErrCh <- run(stopCh) // run 签名改为接收 stopCh + 可关 readyCh
    }()

    // 4. 心跳循环：直到 readyCh 关闭或 runErrCh 报错。
    cp := uint32(0)
    ticker := time.NewTicker(1 * time.Second)
    defer ticker.Stop()
HEARTBEAT:
    for {
        select {
        case <-readyCh:
            break HEARTBEAT
        case err := <-runErrCh:
            // run() 在 ready 前就退出 = 启动失败
            if err != nil {
                return false, 1 // Win32ExitCode=1 (一般错误)
            }
            return false, 0
        case <-ticker.C:
            cp++
            s <- svc.Status{State: svc.StartPending, CheckPoint: cp, WaitHint: 5000}
        }
    }

    // 5. 报 RUNNING。
    s <- svc.Status{State: svc.Running, Accepts: accepted}

    // 6. 主循环：等 SCM 控制信号 / run() 退出。
    for {
        select {
        case c := <-r:
            switch c.Cmd {
            case svc.Interrogate:
                s <- c.CurrentStatus
            case svc.Stop, svc.Shutdown:
                s <- svc.Status{State: svc.StopPending, WaitHint: 30000} // NFR-7 30s 上限
                close(stopCh)                                              // 触发 run() 优雅关停
                <-runErrCh                                                 // 等 run() 真正返回
                s <- svc.Status{State: svc.Stopped}
                return false, 0
            }
        case err := <-runErrCh:
            // run() 自发退出（如 HTTP server 致命错）
            if err != nil {
                s <- svc.Status{State: svc.Stopped}
                return false, 1
            }
            s <- svc.Status{State: svc.Stopped}
            return false, 0
        }
    }
}
```

**注**：上面 `run()` 的接口需要再多一个 `readyCh chan<- struct{}` 让 Execute 知道何时切 RUNNING。考虑到 main.go 改动最小化，**最终选实现 A**：

- `run(stopCh <-chan struct{}, readyCh chan<- struct{}) error`
- 控制台调用：`run(nil, nil)`（两个 nil 即退化到现有行为）；
- 服务化调用：`run(stopCh, readyCh)`；
- `run()` 内部在 `http.Server.Serve` 启动后、log "ready gate opened" 之前的位置 `if readyCh != nil { close(readyCh) }`。

### 3.2 `cmd/frp-easy/service_other.go`（新增）

**职责**：让 Linux / macOS 编译不依赖 `x/sys/windows/svc`。

```go
//go:build !windows

package main

// isWindowsService 在非 Windows 平台恒为 false。
func isWindowsService() bool { return false }

// runService 在非 Windows 平台不应被调用；安全网。
func runService() error {
    // 不暴露 svc 包；用纯字符串 error 避免引入额外依赖。
    return errInvalidPlatform
}

var errInvalidPlatform = stringError("service mode not supported on this platform")

type stringError string

func (e stringError) Error() string { return string(e) }
```

### 3.3 `cmd/frp-easy/main.go`（编辑）

`main()` 分流：

```go
func main() {
    if isWindowsService() {
        if err := runService(); err != nil {
            // 服务化分支错误已通过 SCM 上报；这里只兜底防 panic 后未退出。
            fmt.Fprintln(os.Stderr, err)
            os.Exit(1)
        }
        return
    }
    if err := run(nil, nil); err != nil {
        fmt.Fprintln(os.Stderr, err)
        os.Exit(1)
    }
}
```

`run()` 签名 + select 改造（仅在 main.go L88-93 + L279-290 两段共约 12 行）：

```go
func run(stopCh <-chan struct{}, readyCh chan<- struct{}) error {
    // ...（1-6 段全部不动；存储 / HTTP / procmgr / autoRestoreProcs 顺序保留 NFR-9）...

    // 在 srv.Serve(ln) goroutine 启动 + autoRestoreProcs + ready.Store(true) 之后：
    if readyCh != nil {
        close(readyCh) // 通知 Execute 切 RUNNING
    }

    // 7. 信号 / stopCh 三路 select
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
    select {
    case s := <-sigCh:
        logger.Info("signal received, shutting down", "signal", s.String())
    case <-stopCh:
        logger.Info("service stop requested, shutting down")
    case e := <-serveErr:
        if e != nil && !errors.Is(e, http.ErrServerClosed) {
            logger.Error("http server fatal", "err", e)
        }
    }

    // 优雅关停（既有逻辑零字节改）
    ready.Store(false)
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    pm.Shutdown()
    _ = srv.Shutdown(ctx)
    return nil
}
```

### 3.4 install-service.ps1 关键改动伪代码

```powershell
# (删除 L68-82 wrapper.cmd 生成块全部内容)

# L60 后追加防御性清理：
$LegacyWrapper = Join-Path $InstallDir "frp-easy-svc.cmd"
if (Test-Path -PathType Leaf $LegacyWrapper) {
    Remove-Item -Force $LegacyWrapper -ErrorAction SilentlyContinue
    Write-Host "==> 已清理旧版包装脚本：$LegacyWrapper"
}

# sc.exe create / config 的 binPath 改为直接指向 frp-easy.exe：
& sc.exe create $ServiceName binPath= "`"$BinaryPath`"" start= auto DisplayName= "$DisplayName"
& sc.exe config $ServiceName binPath= "`"$BinaryPath`"" start= auto DisplayName= "$DisplayName"

# (Q8) marked-for-delete 探测函数：
function Wait-ServiceMarkedDeleteCleared {
    param([string]$Name, [int]$TimeoutSec = 15)
    $deadline = (Get-Date).AddSeconds($TimeoutSec)
    while ((Get-Date) -lt $deadline) {
        $out = & sc.exe query $Name 2>&1
        if ($LASTEXITCODE -ne 0) { return $true } # 服务已彻底消失
        if ($out -notmatch 'marked for delete') { return $true }
        Start-Sleep -Milliseconds 500
    }
    return $false
}

# 在 sc.exe stop 后立刻调用：
if (-not (Wait-ServiceMarkedDeleteCleared -Name $ServiceName -TimeoutSec 15)) {
    Write-Error "检测到 $ServiceName 处于 marked for delete 状态超过 15 秒；请关闭所有 services.msc / Get-Service / 任务管理器服务页 后重试。"
    exit 2
}

# (Q9) SCM RUNNING 轮询函数：
function Wait-ServiceRunning {
    param([string]$Name, [int]$TimeoutSec = 30)
    $deadline = (Get-Date).AddSeconds($TimeoutSec)
    $lastProgressAt = Get-Date
    while ((Get-Date) -lt $deadline) {
        $out = & sc.exe query $Name 2>&1
        if ($LASTEXITCODE -eq 0 -and $out -match 'STATE\s*:\s*4\s+RUNNING') {
            return $true
        }
        # AC-19: 每 3 秒打印一行中文进度，避免静默 > 5s
        if (((Get-Date) - $lastProgressAt).TotalSeconds -ge 3) {
            Write-Host "==> 等待服务进入 RUNNING 状态..."
            $lastProgressAt = Get-Date
        }
        Start-Sleep -Milliseconds 500
    }
    return $false
}

# sc.exe start 后追加：
if (-not (Wait-ServiceRunning -Name $ServiceName -TimeoutSec 30)) {
    Write-Error "$ServiceName 启动超时 30 秒未进入 RUNNING；请运行 sc query $ServiceName 与查看 $InstallDir\.frp_easy\logs\ui.log 排障。"
    exit 2
}
Write-Host "==> 服务已启动"
```

---

## 4. Data model changes

**无**。本任务不涉及 SQLite schema / migration / kv 增删。Q4 决议（不写 Event Log）已排除注册表 Event Source 操作。

---

## 5. API contracts

**无 HTTP API 变更**。本任务仅修服务化握手，不改 `internal/httpapi/`。

**SCM ABI 契约**（新引入，记录在此供 QA 用 `sc query` 断言）：

| SCM state | frp-easy.exe 服务化分支何时报告 |
|---|---|
| `SERVICE_START_PENDING` (CheckPoint=0, WaitHint=5000) | Execute 入口立刻 |
| `SERVICE_START_PENDING` (CheckPoint=N, WaitHint=5000) | 每 1 秒，N 单调累加，直到 readyCh 关闭 |
| `SERVICE_RUNNING` (Accepts: Stop\|Shutdown) | run() goroutine 启动 HTTP server 成功后 |
| `SERVICE_STOP_PENDING` (WaitHint=30000) | 收到 SCM Stop / Shutdown 控制码后立刻 |
| `SERVICE_STOPPED` | run() goroutine 返回（procmgr.Shutdown + srv.Shutdown 完成）后 |
| `SERVICE_STOPPED` (Win32ExitCode=1) | run() 在 ready 之前返回 error（启动失败） |
| `SERVICE_STOPPED` (Win32ExitCode=2 via main.go os.Exit) | 端口占用（BC-12；既有 main.go L237-242 已 os.Exit(2)） |

---

## 6. Sequence / flow

```
┌─────────────────────────┐
│ Windows SCM (services.exe)│
└────────────┬────────────┘
             │ CreateProcessW("C:\Program Files\frp-easy\frp-easy.exe")
             ▼
   ┌──────────────────────┐
   │  frp-easy.exe main() │
   └─────────┬────────────┘
             │
             ▼
   ┌──────────────────────────────┐
   │ isWindowsService()           │
   │  = svc.IsWindowsService()    │
   └─────────┬────────────────────┘
             │ true
             ▼
   ┌──────────────────────────────────┐
   │ runService()                     │
   │ 1. os.Chdir(filepath.Dir(exe))   │  ← 替代 wrapper.cmd 的 cd /d
   │ 2. svc.Run("frp-easy", handler)  │
   └─────────┬────────────────────────┘
             │
             ▼
   ┌──────────────────────────────────────────────┐
   │ serviceHandler.Execute(args, r, s)           │
   │                                              │
   │ s <- {StartPending, CP=0, WaitHint=5s}       │
   │ go run(stopCh, readyCh)                      │───┐
   │                                              │   │
   │ ┌─────────────────────────────────────────┐  │   │  run() 内部：
   │ │ heartbeat ticker @ 1s:                  │  │   │  - appconf.Load
   │ │   s <- {StartPending, CP=N++, ...}      │  │   │  - storage.Open
   │ │ until <-readyCh                         │  │   │  - logrotate / binloc / procmgr / auth.RateLimiter
   │ └─────────────────────────────────────────┘  │   │  - net.Listen tcp + srv.Serve(ln) goroutine
   │                                              │   │  - autoRestoreProcs
   │ s <- {Running, Accepts: Stop|Shutdown}       │◀──┘  - ready.Store(true)
   │                                              │      - close(readyCh)  ★
   │ ┌─────────────────────────────────────────┐  │      - select { sigCh / stopCh / serveErr }
   │ │ main loop:                              │  │
   │ │   select c := <-r:                      │  │
   │ │     Interrogate → echo                  │  │
   │ │     Stop / Shutdown:                    │  │
   │ │       s <- {StopPending, WaitHint=30s}  │  │
   │ │       close(stopCh)        ──────────►  │──┼─► run() select 命中 <-stopCh
   │ │       <-runErrCh           ◀──────────  │◀─┤   pm.Shutdown() (→ procmgr Stop frpc/frps)
   │ │       s <- {Stopped}                    │  │   srv.Shutdown(ctx 10s)
   │ │       return false, 0                   │  │   return nil
   │ └─────────────────────────────────────────┘  │
   └──────────────────────────────────────────────┘
             │
             ▼
   ┌──────────────────────────┐
   │ SCM marks service STOPPED│
   └──────────────────────────┘
```

控制台路径（双击 .exe / dev 跑）：

```
main() → isWindowsService()=false → run(nil, nil)
       → 现有逻辑零变化（sigCh = SIGINT/SIGTERM 单一关停源）
```

---

## 7. Reuse audit

| 需求 | 既有代码 | 文件路径 | 决策 |
|---|---|---|---|
| 子进程优雅 stop（frpc/frps） | `procmgr.Manager.Shutdown()` 调用内部 Stop（每 kind 5s + 2s 兜底强杀） | `<repo>/internal/procmgr/manager.go` L294-344, L393-397 | **复用原样**。Q7 决议；NFR-9 不破。Execute 通过 stopCh → run() → pm.Shutdown() 串联。 |
| HTTP server 优雅关停 | `srv.Shutdown(ctx)` 10s 超时 | `<repo>/cmd/frp-easy/main.go` L294-297 | **复用原样**。 |
| signal.Notify SIGINT/SIGTERM 关停 | `signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)` + select | `<repo>/cmd/frp-easy/main.go` L280-290 | **扩展**：在同一 select 内追加 `case <-stopCh:` 第三路，控制台分支传 nil 退化。 |
| SCM 服务 stop 轮询模式 | `Wait-ServiceStopped` 函数（200ms 步进，最长 5s） | `<repo>/scripts/install-service.ps1` L26-43；`<repo>/scripts/uninstall-service.ps1` L17-32 | **复用模式，扩展为 `Wait-ServiceRunning` / `Wait-ServiceMarkedDeleteCleared`**：同款 (deadline, polling step, return bool) 风格。 |
| sc.exe binPath 双引号转义 | `binPath= "`"$WrapperPath`""` | `<repo>/scripts/install-service.ps1` L96, L102 | **复用模式，目标改为 `$BinaryPath`**。BC-4 已验证空格路径下该模式工作。 |
| 防御性清理已存在文件 | `Remove-Item -Force ... -ErrorAction SilentlyContinue` | `<repo>/scripts/uninstall-service.ps1` L75 | **复用**：install-service.ps1 顶端追加同款语法清理 `frp-easy-svc.cmd` 升级残留。 |
| os.Executable() 跨平台获取自身路径 | （Go stdlib，Caddy / Vault 已采用此 idiom） | Go stdlib | **新引入，但属标准库**。Windows 上原生 UTF-16，对中文路径（BC-3）天然正确。 |
| golang.org/x/sys/windows/svc 服务化 ABI | 已作为 modernc.org/sqlite 间接依赖在 go.sum（v0.44.0） | `<repo>/go.sum` L26-27 | **直接依赖提升**（NFR-1）。零下载、零版本差异。 |
| install.ps1 退出码透传 | `$LASTEXITCODE = 0; & $svc; if ($LASTEXITCODE -ne 0) {...}` | `<repo>/scripts/install.ps1` L283-290 | **复用原样**。修复后 install-service.ps1 退出 0 即可，无需脚本侧改动。 |

---

## 8. Risk analysis

| ID | 风险 | 影响 | 缓解 |
|---|---|---|---|
| **R-1** | run() 启动序列（appconf + storage 迁移 + autoRestoreProcs）在极端慢盘上 > 30s，SCM 仍报 1053 | AC-1 / AC-2 失败 | CheckPoint 1s 心跳（Q5 决议）+ WaitHint 5s；SCM 收到任何 CheckPoint 累加即视为"在干活"，重置 30s 死线；NFR-8 兜底。BC-13 已覆盖该路径。 |
| **R-2** | SCM Stop 控制码未正确触发 run() 关停 → 30s 后 SCM 强杀 → 触发 stop-side 等价错误 1061 | AC-7 失败、NFR-7 失败 | `close(stopCh)` → run() 内 `select case <-stopCh:` 直接走 pm.Shutdown + srv.Shutdown；Execute 阻塞 `<-runErrCh` 直到 run() 真正返回再报 STOPPED；run() 优雅关停预算 10s（srv.Shutdown ctx）+ procmgr 5s+2s × 2kind = 最坏 24s，留 6s 余量给 SCM 30s 上限。 |
| **R-3** | service_windows.go 单测在 Linux CI 上失败（svc 包仅 Windows 可编译） | verify_all 在 CI Linux runner FAIL | `service_windows_test.go` 加严格 `//go:build windows` build tag；service_other.go 不引用 svc 包。verify_all.sh 在 Linux 上 `go test ./...` 自动跳过 windows-only 文件（Go build tag 标准行为）。无需改 verify_all.sh 检查项数量（NFR-5）。 |
| **R-4** | 移除 wrapper.cmd 后中文路径 cwd 解析失败（host codepage 不再参与） | BC-3 失败、AC-12 失败 | `os.Executable()` 在 Windows 上调 `GetModuleFileNameW`（UTF-16）；`filepath.Dir()` + `os.Chdir()` 全程 UTF-16，不经 host codepage / GB18030。Caddy / Vault 已在生产证明该 idiom 对中文 / 非 ASCII Windows 路径正确。 |
| **R-5** | 升级路径残留旧 wrapper.cmd（T-018 用户重跑 install.ps1）让用户在 InstallDir 看到一个"不知何用"的 .cmd 文件 | 用户困惑（非 AC 失败但属 UX 退化） | install-service.ps1 顶端追加防御性 `Remove-Item -Force frp-easy-svc.cmd`（见 §3.4）；uninstall-service.ps1 L70-77 保留同款清理。两点同时清理保证 T-018 → T-019 升级或卸载链路任一走过都不留残留。 |
| **R-6** | sc.exe binPath 引号语义在 PowerShell 5.1 / 7.x 双 host 解析差异，直接指向 `C:\Program Files\frp-easy\frp-easy.exe`（含空格）时 sc.exe 错误解读为多个参数 | BC-4 失败、AC-13 失败 | 沿用 install-service.ps1 既有验证过的 `"`"$BinaryPath`""`（PowerShell 反引号转义内层双引号 + sc.exe `binPath=` 等号空格语法），与原 wrapper.cmd 路径同款；T-008 PowerShell 5.1 & 7.x 双 host 已验证过此模式对空格路径有效。 |
| **R-7** | Wait-ServiceMarkedDeleteCleared 15s / Wait-ServiceRunning 30s 轮询过程中用户 Ctrl+C 中止 → 服务半安装状态 | BC-10 已声明可接受，但需确保用户重跑能恢复 | BC-10 / IS-4 已声明"中止后不强求零残留，下次重跑走升级路径恢复"；install-service.ps1 重复执行幂等（IS-6）；现有 install-service.ps1 已用 `$ErrorActionPreference=Stop`，Ctrl+C 中止 PowerShell 抛 PipelineStoppedException 让脚本退出，下次重跑 Wait-ServiceStopped / Wait-ServiceRunning 都能从任意起点恢复。 |
| **R-8** | CI Windows runner 缺失或不在每次 PR 跑 → 双入口代码在合并后才在用户机暴露问题 | 发现晚、回归慢 | OOS-13 不让动 release.yml；本任务依赖 QA 在 Stage 7 真机 / VM 验证 AC-1 ~ AC-19；建议（非强制）在后续任务里给 verify_all 加一个 Windows runner job（属增强，不入本任务）。 |

---

## 9. Migration / rollout plan

1. **代码主线落地**：本任务的 Go / PowerShell 改动直接合并到 main 分支；release.yml（OOS-13）保持不动，下一个 push main 自动刷新 `rolling` tag、产出新版 `frp-easy-<sha>-windows-amd64.zip`。
2. **干净安装路径**：用户首次 `irm | iex` 拿到新 zip → install.ps1 步骤 5 解压新 frp-easy.exe → 步骤 6 安装到 `C:\Program Files\frp-easy\` → 步骤 7 调新 install-service.ps1（不再生成 wrapper.cmd，sc.exe binPath 直接指向 .exe，启动后 Wait-ServiceRunning）→ 步骤 8 成功横幅。**无需任何用户侧迁移动作**。
3. **升级路径（T-018 → T-019）**：用户重跑同一条 `irm | iex` → install.ps1 步骤 6 走"升级"分支（L243-266）覆盖 frp-easy.exe + scripts/ → 步骤 7 调新 install-service.ps1：
   - 顶端防御性清理删除旧 `frp-easy-svc.cmd`；
   - 检测到服务已存在（L88）→ sc.exe stop → Wait-ServiceStopped → Wait-ServiceMarkedDeleteCleared；
   - sc.exe config 把 binPath 从旧 wrapper.cmd 切换到 .exe；
   - sc.exe start + Wait-ServiceRunning。
   - 退出码 0 → install.ps1 步骤 8 成功横幅。
4. **回滚**（理论可行，实操不建议）：用户若需回退到 T-018 行为，可手动 `git checkout` 旧 install-service.ps1 重跑—但因主二进制已是双入口（控制台分支仍兼容旧 wrapper.cmd 链路），即使回退仅脚本不回退二进制也能正常工作。无需"协调式回滚"或灰度。
5. **失败诊断**：若用户 Win11 上仍报 1053（理论不应发生），按 install-service.ps1 新 `Wait-ServiceRunning` 失败时的中文文案 `请运行 sc query frp-easy 与查看 ...\.frp_easy\logs\ui.log 排障` 收集证据走 issue 流程。
6. **数据兼容**：零 schema 变更；用户的 `frp_easy.toml` / `.frp_easy\` / `frp_win\` 全部原样保留（install.ps1 L249-260 白名单覆盖保护）。

---

## 10. Out-of-scope clarifications

> 与 01 §3 OOS-1 ~ OOS-13 对齐但不重复；本节仅记录"设计层面"明确不覆盖的边界。

- **设计 NOT 覆盖**：Windows Job Object 绑定 frp-easy.exe + frpc.exe / frps.exe 子进程做"父亡则子全亡"硬清理（Q7 候选 b）。本设计依赖 `procmgr.Shutdown()` 现有 graceful 路径；若 QA 实测发现子进程残留再开新任务。
- **设计 NOT 覆盖**：自定义 Windows Event Log source 与消息资源 .dll 编译（Q4 候选 a/b）。错误诊断走 `.frp_easy\logs\ui.log` 单一通道（与 Linux journalctl 非对等但对用户排障一处即可）。
- **设计 NOT 覆盖**：`--service-debug` 调试 flag（Q10 候选 a）。开发者本地用 `sc.exe create frp-easy-dev binPath= "<path>\frp-easy.exe"` 临时装服务验证。
- **设计 NOT 覆盖**：Windows Defender / SmartScreen 自动加排除项（OOS-11）。
- **设计 NOT 覆盖**：服务账户从 LocalSystem 切到 NetworkService / LocalService（OOS-7 / Q6 决议 (a) 保留 LocalSystem）。
- **设计 NOT 覆盖**：CI Windows runner 增量（R-8 备注；超 OOS-13 release.yml 边界）。
- **设计 NOT 覆盖**：HTTP 200 端到端就绪轮询（Q9 决议仅加 SCM RUNNING 轮询）；HTTP 200 是 SCM RUNNING 的必然推论（in-process 模式下 main.go run() 内 net.Listen + Serve 在 close(readyCh) 之前已成功）。

---

## 11. Partition assignment

本项目 `.harness/agents/` 下虽然存在 `dev-frontend.md` / `dev-backend.md` / `dev-db.md`，但本任务的**全部改动集中在后端 Go 与 PowerShell 安装脚本**，无前端 `web/**` 变更、无数据库 `migrations/**` / `internal/storage/**` 变更。PM 派发 prompt 明确按 **"单 Developer 模式"** 处理；下表的"Partition"列即按单 developer 视角列出（若 PM 改派分区 agent，对应映射为括号中的 dev-* 标签）。

| 文件 | Partition | New / Edit | 依赖 |
|---|---|---|---|
| `<repo>/cmd/frp-easy/service_windows.go` | developer (dev-backend) | new | — |
| `<repo>/cmd/frp-easy/service_other.go` | developer (dev-backend) | new | — |
| `<repo>/cmd/frp-easy/service_windows_test.go` | developer (dev-backend) | new | 依赖 install-service.ps1 / uninstall-service.ps1 最终文本 |
| `<repo>/cmd/frp-easy/main.go` | developer (dev-backend) | edit（main() 分流 + run() 加 stopCh/readyCh 参数 + select 三路） | 依赖 service_windows.go / service_other.go |
| `<repo>/go.mod` | developer (dev-backend) | edit（x/sys 升 direct） | — |
| `<repo>/scripts/install-service.ps1` | developer (dev-backend) | edit（删 wrapper 生成 / 改 binPath / 加 Wait-ServiceMarkedDeleteCleared / Wait-ServiceRunning / 防御性清理） | 依赖 frp-easy.exe 已是双入口 |
| `<repo>/scripts/uninstall-service.ps1` | developer (dev-backend) | edit（注释从"删除自己生成的"降级为"防御性清理"；逻辑不动） | — |
| `<repo>/scripts/install.ps1` | developer (dev-backend) | edit（仅注释微调；逻辑不动） | — |
| `<repo>/docs/dev-map.md` | developer (dev-backend) | edit（在 cmd/frp-easy/ 行追加 service_windows.go / service_other.go 说明） | — |

## Dispatch order

单 Developer 模式：**单次派发，由 `.harness/agents/developer.md` 全部承担**。

派发顺序（developer 在自己 04_DEVELOPMENT.md 内部按此顺序实施）：

1. `go.mod` 升级（让后续 import 编译通过）；
2. `cmd/frp-easy/service_other.go`（先建 non-windows stub，让 main.go 引用不报错）；
3. `cmd/frp-easy/service_windows.go`（Windows 实现）；
4. `cmd/frp-easy/main.go`（run 签名扩展 + main 分流）；
5. `cmd/frp-easy/service_windows_test.go`（单测）；
6. `scripts/install-service.ps1`（删 wrapper + 加轮询 + 防御性清理）；
7. `scripts/uninstall-service.ps1`（注释降级）；
8. `scripts/install.ps1`（注释微调）；
9. `docs/dev-map.md`（同步）；
10. `scripts/verify_all`（Linux bash + Windows PowerShell 双跑 PASS:19）。

## Parallelism

无。同一 developer 顺序实施；上面 1 → 10 严格依赖（编译性依赖 + 单测依赖脚本最终文本 + 文档随结构更新）。

---

## 12. Verdict

**READY**

理由：
- 12 节全部填写完整；§3 / §6 已给出可逐行实施的 Go + PowerShell 伪代码与 ascii 序列图；
- §7 Reuse audit 8 项明确复用既有 procmgr.Shutdown / signal 链 / sc.exe 引号模式 / Wait-ServiceStopped 模式，避免重复造轮子；
- §8 Risk analysis 8 条全部带缓解（覆盖 1053 慢启 / SCM Stop 失败 / Linux CI 编译 / 中文 cwd / 升级残留 / sc.exe 引号 / Ctrl+C / CI 缺口）；
- §9 Migration 给出干净安装 + 升级 + 回滚 + 数据兼容四路径；
- §11 已明确 Partition assignment 与 Developer 实施顺序，无并行歧义；
- 所有 PM-resolved 决议（Q1 in-process / Q2 IsWindowsService / Q3 移除 wrapper / Q4 不写 EventLog / Q5 1s+5s / Q6 LocalSystem / Q7 procmgr.Shutdown / Q8 marked-for-delete 轮询 / Q9 SCM RUNNING 轮询 / Q10 不加 --service-debug）逐条在设计中落地引用；
- 无 NFR / OOS / BC 与设计冲突（NFR-1 零新依赖通过 x/sys 提升 direct 达成；NFR-2 体积增长极小；NFR-5 verify_all 检查项不变；NFR-7 stop 30s 上限由 srv.Shutdown 10s + procmgr 24s 留 6s 余量；NFR-8 心跳满足；NFR-9 run() 启动序列零字节改）；
- 设计无需新增第三方依赖、无需 schema 迁移、无需 API 变更——Gate Reviewer 可在 03 直接基于本设计判 APPROVED。

下游 Developer 按 §11 顺序实施即可；任何模糊点（如 run() readyCh 关闭时机、Execute 在 run() 报错时的 errno 映射）已在 §3.1 / §3.3 给出精确锚点，无须二次设计决策。
