# install-service.ps1 — frp_easy Windows Service 安装脚本
#
# 用途：将解压后的 frp-easy.exe 注册为 Windows 服务（sc.exe create / config / failure / start）。
#       为锁定 cwd，安装时同时生成 frp-easy-svc.cmd 包装脚本，sc.exe binPath 指向该 .cmd；
#       重复执行幂等（已存在则刷新 binPath + DisplayName + failure + restart）。
# 用法：以管理员身份运行 PowerShell（右键 → 以管理员身份运行），执行：
#       .\scripts\install-service.ps1 [-DisplayName "FRP Easy"] [-ServiceName "frp-easy"]
# 参数：-DisplayName  服务显示名（默认 "FRP Easy"；支持中文）
#       -ServiceName  服务键名（默认 "frp-easy"）
# 输出：stdout 中文进度；stderr 仅错误
# 退出码：0 成功 / 1 前置失败（非管理员 / 缺 sc.exe / 二进制缺失）/ 2 sc.exe 调用失败
# 说明：
#   - 本脚本不删除 frp_easy.toml 与 .frp_easy/ 数据目录，卸载请走 uninstall-service.ps1。
#   - 已知风险（03_GATE_REVIEW MINOR-3）：sc.exe stop 可能无法优雅传播到 frp-easy.exe 子进程，
#     QA 待验证；若实测失败将走 DESIGN DRIFT 回退 --config flag 方案。
#   - sc.exe 等号语法陷阱：binPath= "..." 等号后必须有空格，PowerShell 数组传参确保不被吞掉。

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

# 生成 cwd 包装脚本：sc.exe 启动服务时默认 cwd 为 %WinDir%\System32，
# 而 frp-easy 读取 frp_easy.toml 是相对路径；用 cmd /d 锁定 cwd 到解压目录。
$WrapperPath = Join-Path $InstallDir "frp-easy-svc.cmd"
$WrapperContent = @"
@echo off
REM frp-easy-svc.cmd — 由 install-service.ps1 自动生成的服务包装脚本。
REM 作用：cd 到解压目录后启动 frp-easy.exe，让 frp_easy.toml 与 .frp_easy/ 相对路径可用。
REM 删除本文件请同步走 uninstall-service.ps1。
cd /d "$InstallDir"
"$BinaryPath"
"@
# MINOR-R2：用 Default（host codepage，简中环境为 GB18030/936）而非 ASCII，
# 否则 InstallDir 含中文 / 非 ASCII 字符时 cmd.exe 读到的 cd /d 路径会乱码 → 找不到目录。
Set-Content -Path $WrapperPath -Value $WrapperContent -Encoding Default
Write-Host "==> 已生成服务包装脚本：$WrapperPath"

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
    & sc.exe config $ServiceName binPath= "`"$WrapperPath`"" start= auto DisplayName= "$DisplayName"
    if ($LASTEXITCODE -ne 0) {
        Write-Error "sc.exe config 失败（退出码 $LASTEXITCODE）。"
        exit 2
    }
} else {
    & sc.exe create $ServiceName binPath= "`"$WrapperPath`"" start= auto DisplayName= "$DisplayName"
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

注意：服务 binPath 与包装脚本均使用绝对路径；安装服务后请勿移动 $InstallDir。
"@ | Write-Host

exit 0
