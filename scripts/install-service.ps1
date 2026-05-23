# install-service.ps1 — frp_easy Windows Service 安装脚本（T-019 修订）
#
# 用途：将解压后的 frp-easy.exe 注册为 Windows 服务（sc.exe create / config / failure / start）。
#       T-019 起 sc.exe binPath= 直接指向 frp-easy.exe，进程内的 Windows Service ABI
#       （cmd/frp-easy/service_windows.go）通过 os.Chdir(filepath.Dir(os.Executable()))
#       锁定 cwd 并实现完整 SetServiceStatus 状态机，根因层解决 sc.exe start 报错 1053。
#       旧版本生成的 frp-easy-svc.cmd 包装脚本本脚本会防御性清理（升级路径用）。
#       重复执行幂等（已存在则刷新 binPath + DisplayName + failure + restart）。
# 用法：以管理员身份运行 PowerShell（右键 → 以管理员身份运行），执行：
#       .\scripts\install-service.ps1 [-DisplayName "FRP Easy"] [-ServiceName "frp-easy"]
# 参数：-DisplayName  服务显示名（默认 "FRP Easy"；支持中文）
#       -ServiceName  服务键名（默认 "frp-easy"）
# 输出：stdout 中文进度；stderr 仅错误
# 退出码：0 成功 / 1 前置失败（非管理员 / 缺 sc.exe / 二进制缺失）/ 2 sc.exe 调用失败或服务启动超时
# 说明：
#   - 本脚本不删除 frp_easy.toml 与 .frp_easy/ 数据目录，卸载请走 uninstall-service.ps1。
#   - sc.exe 等号语法陷阱：binPath= "..." 等号后必须有空格，PowerShell 数组传参确保不被吞掉。
#   - 中文 / 空格路径（BC-3 / BC-4）由 frp-easy.exe 内 os.Executable() UTF-16 原生解析保证正确。

[CmdletBinding()]
param(
    [string]$DisplayName = "FRP Easy",
    [string]$ServiceName = "frp-easy"
)

$ErrorActionPreference = "Stop"

# 轮询 sc.exe query 直到目标服务进入 STOPPED 或超时（MINOR-R3）。
# 早期 Start-Sleep -Seconds 1 在服务停止较慢时会过早 config / start，触发
# ERROR_SERVICE_MARKED_FOR_DELETE 等竞态错误；这里改为轮询，超时 5 秒，
# 超时后仍继续往下走（让 sc.exe 自己抛错以保留原退出码语义）。
function Wait-ServiceStopped {
    param(
        [Parameter(Mandatory)] [string] $Name,
        [int] $TimeoutSec = 5
    )
    $deadline = (Get-Date).AddSeconds($TimeoutSec)
    while ((Get-Date) -lt $deadline) {
        $out = & sc.exe query $Name 2>&1
        if ($LASTEXITCODE -ne 0) { return $true }  # 服务已不存在（被删除）
        if ($out -match 'STATE\s*:\s*\d+\s+STOPPED') { return $true }
        Start-Sleep -Milliseconds 200
    }
    return $false
}

# T-019 Q8 决议：旧服务在 sc.exe stop / delete 后偶尔卡在 "marked for delete"
# 状态（SCM 句柄被 services.msc / Get-Service / 任务管理器服务页持有未释放）；
# 此时立刻 sc.exe config / start 会失败（ERROR_SERVICE_MARKED_FOR_DELETE / 1072）。
# 本函数轮询 sc.exe query 直到不再返回 "marked for delete" 或服务彻底消失。
# 超时返回 false 让调用方走中文诊断退出，不让用户面对 sc.exe 原始英文 1072 错误。
function Wait-ServiceMarkedDeleteCleared {
    param(
        [Parameter(Mandatory)] [string] $Name,
        [int] $TimeoutSec = 15
    )
    $deadline = (Get-Date).AddSeconds($TimeoutSec)
    while ((Get-Date) -lt $deadline) {
        $out = & sc.exe query $Name 2>&1
        if ($LASTEXITCODE -ne 0) { return $true }  # 服务已彻底消失
        if ($out -notmatch 'marked for delete') { return $true }
        Start-Sleep -Milliseconds 500
    }
    return $false
}

# T-019 Q9 决议：sc.exe start 命令本身是异步的，命令返回 0 不代表服务已进入
# RUNNING 状态（SCM 内部仍走 START_PENDING → RUNNING 状态机）。本函数轮询
# sc.exe query 直到 STATE 含 "4  RUNNING" 或超时 30 秒；超时返回 false 让调用方
# 走中文诊断退出。每 3 秒打印一行中文进度（AC-19），避免静默挂起 > 5 秒无输出。
function Wait-ServiceRunning {
    param(
        [Parameter(Mandatory)] [string] $Name,
        [int] $TimeoutSec = 30
    )
    $deadline = (Get-Date).AddSeconds($TimeoutSec)
    $lastProgressAt = Get-Date
    while ((Get-Date) -lt $deadline) {
        $out = & sc.exe query $Name 2>&1
        if ($LASTEXITCODE -eq 0 -and $out -match 'STATE\s*:\s*4\s+RUNNING') {
            return $true
        }
        # AC-19：每 3 秒打印一行中文进度，避免静默挂起 > 5 秒无任何输出。
        if (((Get-Date) - $lastProgressAt).TotalSeconds -ge 3) {
            Write-Host "==> 等待服务进入 RUNNING 状态..."
            $lastProgressAt = Get-Date
        }
        Start-Sleep -Milliseconds 500
    }
    return $false
}

# 前置 1：管理员权限检测（FR-2.2 硬约束）
$isAdmin = ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
if (-not $isAdmin) {
    Write-Error "请以管理员身份运行本脚本（右键 PowerShell -> 以管理员身份运行）。"
    exit 1
}

# 前置 2：sc.exe 存在
$scCmd = Get-Command sc.exe -ErrorAction SilentlyContinue
if (-not $scCmd) {
    Write-Error "未检测到 sc.exe；本脚本依赖 Windows 自带 Service Controller。"
    exit 1
}

# 前置 3：定位解压目录与二进制
$ScriptDir  = $PSScriptRoot
$InstallDir = (Resolve-Path (Join-Path $ScriptDir "..")).Path
$BinaryPath = Join-Path $InstallDir "frp-easy.exe"
if (-not (Test-Path -PathType Leaf $BinaryPath)) {
    Write-Error "未找到 $BinaryPath；请确认已解压发布包并保留目录结构。"
    exit 1
}

# T-019：升级路径防御性清理旧版（< T-019）install-service.ps1 生成的 wrapper.cmd。
# 本脚本不再生成它（cwd 由 frp-easy.exe 内 os.Chdir 锁定），但升级用户磁盘上仍可能
# 残留 frp-easy-svc.cmd —— 留着会让用户困惑"这个文件是干什么的"。
$LegacyWrapper = Join-Path $InstallDir "frp-easy-svc.cmd"
if (Test-Path -PathType Leaf $LegacyWrapper) {
    Remove-Item -Force $LegacyWrapper -ErrorAction SilentlyContinue
    if (-not (Test-Path -PathType Leaf $LegacyWrapper)) {
        Write-Host "==> 已清理旧版包装脚本残留：$LegacyWrapper"
    }
}

# 检测服务是否已存在（sc.exe query 返回非 0 表示不存在）
$null = & sc.exe query $ServiceName 2>&1
$existed = ($LASTEXITCODE -eq 0)

if ($existed) {
    Write-Host "==> 检测到已存在的服务：$ServiceName（将刷新 binPath / DisplayName / 失败动作 并重启）"
    # 先停止（忽略非运行错误）
    & sc.exe stop $ServiceName 2>&1 | Out-Null
    # 轮询等待 SCM 完成 stop 状态机（MINOR-R3）
    if (-not (Wait-ServiceStopped -Name $ServiceName -TimeoutSec 5)) {
        Write-Host "==> 警告：等待 $ServiceName 进入 STOPPED 超时 5 秒，继续尝试 config（sc.exe 自身会抛错则中止）"
    }
    # T-019 Q8 决议：探测 marked-for-delete 卡死状态（最长 15s），超时给中文诊断退出 2。
    if (-not (Wait-ServiceMarkedDeleteCleared -Name $ServiceName -TimeoutSec 15)) {
        Write-Error "检测到 $ServiceName 处于 marked for delete 状态超过 15 秒；请关闭所有 services.msc / Get-Service / 任务管理器服务页 后重试。"
        exit 2
    }
    & sc.exe config $ServiceName binPath= "`"$BinaryPath`"" start= auto DisplayName= "$DisplayName"
    if ($LASTEXITCODE -ne 0) {
        Write-Error "sc.exe config 失败（退出码 $LASTEXITCODE）。"
        exit 2
    }
} else {
    & sc.exe create $ServiceName binPath= "`"$BinaryPath`"" start= auto DisplayName= "$DisplayName"
    if ($LASTEXITCODE -ne 0) {
        Write-Error "sc.exe create 失败（退出码 $LASTEXITCODE）。"
        exit 2
    }
    Write-Host "==> 已新建服务：$ServiceName ($DisplayName)"
}

# 设置失败重启动作（Open Question 8 PM-resolved b：与 Linux Restart=on-failure 对齐）
# reset= 60 表示 60 秒计数窗口；崩溃后 5000 毫秒重启
& sc.exe failure $ServiceName reset= 60 actions= restart/5000 | Out-Null

# 服务描述（中文）
& sc.exe description $ServiceName "FRP 可视化管理 UI（frp_easy）" | Out-Null

# 启动服务
& sc.exe start $ServiceName
if ($LASTEXITCODE -ne 0 -and $LASTEXITCODE -ne 1056) {
    # 1056 = ERROR_SERVICE_ALREADY_RUNNING；幂等场景可接受
    Write-Error "sc.exe start 失败（退出码 $LASTEXITCODE）；请运行 sc query $ServiceName 查看状态。"
    exit 2
}

# T-019 Q9 决议：轮询等待 SCM 把服务状态推进到 RUNNING（最长 30s）；
# sc.exe start 命令本身是异步的，命令返回 0 不代表服务已 RUNNING。
# 现 frp-easy.exe 实现完整 SetServiceStatus 状态机后通常 < 5s 即 RUNNING；
# 30s 给极端慢盘 / 大量历史数据迁移留余量。超时走中文诊断退出 2。
if (-not (Wait-ServiceRunning -Name $ServiceName -TimeoutSec 30)) {
    Write-Error "$ServiceName 启动超时 30 秒未进入 RUNNING；请运行 sc query $ServiceName 与查看 $InstallDir\.frp_easy\logs\ui.log 排障。"
    exit 2
}

if ($existed) {
    Write-Host "==> 已刷新现有服务并重启"
} else {
    Write-Host "==> 服务已启动"
}

@"

服务已就绪。常用命令：
  sc query $ServiceName              # 查看服务状态
  sc stop  $ServiceName              # 手动停止
  sc start $ServiceName              # 手动启动
  事件查看器 -> Windows 日志 -> 系统  # 查看 SCM 日志
  Get-Content "$InstallDir\.frp_easy\logs\ui.log" -Tail 200 -Wait   # 实时日志

数据目录（保留）：
  $InstallDir\.frp_easy\
  $InstallDir\frp_easy.toml （首启后自动生成）

注意：服务 binPath 直接指向 $BinaryPath；安装服务后请勿移动 $InstallDir。
"@ | Write-Host

exit 0
