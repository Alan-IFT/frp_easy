# uninstall-service.ps1 — frp_easy Windows Service 卸载脚本
#
# 用途：停止并删除 Windows 服务（sc.exe stop + sc.exe delete），并对旧版（< T-019）
#       install-service.ps1 曾生成的 frp-easy-svc.cmd 包装脚本做防御性清理；
#       **不删除** frp_easy.toml 与 .frp_easy/ 数据目录。
#       T-019 起 install-service.ps1 不再生成 wrapper.cmd（cwd 由 frp-easy.exe 内
#       os.Chdir(filepath.Dir(os.Executable())) 锁定），但老安装升级再卸载时
#       磁盘上可能仍有残留 .cmd，本脚本顺手清掉避免用户困惑。
# 用法：以管理员身份运行 PowerShell，执行：.\scripts\uninstall-service.ps1 [-ServiceName "frp-easy"]
# 参数：-ServiceName  服务键名（默认 "frp-easy"）
# 输出：stdout 中文进度；卸载完成时打印数据目录保留提示
# 退出码：0 成功（含"服务从未安装"友好降级）/ 1 权限或 sc.exe delete 真正失败

[CmdletBinding()]
param(
    [string]$ServiceName = "frp-easy"
)

$ErrorActionPreference = "Stop"

# 轮询 sc.exe query 直到目标服务进入 STOPPED 或超时（MINOR-R3）。
# 与 install-service.ps1 同一封装；防止 stop 后立刻 delete 触发竞态。
function Wait-ServiceStopped {
    param(
        [Parameter(Mandatory)] [string] $Name,
        [int] $TimeoutSec = 5
    )
    $deadline = (Get-Date).AddSeconds($TimeoutSec)
    while ((Get-Date) -lt $deadline) {
        $out = & sc.exe query $Name 2>&1
        if ($LASTEXITCODE -ne 0) { return $true }
        if ($out -match 'STATE\s*:\s*\d+\s+STOPPED') { return $true }
        Start-Sleep -Milliseconds 200
    }
    return $false
}

# 前置：管理员权限检测
$isAdmin = ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
if (-not $isAdmin) {
    Write-Error "请以管理员身份运行本脚本（右键 PowerShell -> 以管理员身份运行）。"
    exit 1
}

$scCmd = Get-Command sc.exe -ErrorAction SilentlyContinue
if (-not $scCmd) {
    Write-Error "未检测到 sc.exe；本脚本依赖 Windows 自带 Service Controller。"
    exit 1
}

# 检测服务是否存在
$null = & sc.exe query $ServiceName 2>&1
if ($LASTEXITCODE -ne 0) {
    Write-Host "未检测到已安装的 $ServiceName 服务。"
    Write-Host "如果此前从未通过 install-service.ps1 安装，请忽略本提示。"
    exit 0
}

# 停止（忽略非运行错误）
& sc.exe stop $ServiceName 2>&1 | Out-Null
# 轮询等待 SCM 完成 stop（MINOR-R3）
if (-not (Wait-ServiceStopped -Name $ServiceName -TimeoutSec 5)) {
    Write-Host "==> 警告：等待 $ServiceName 进入 STOPPED 超时 5 秒，继续尝试 delete（sc.exe 自身会抛错则中止）"
}

# 删除服务
& sc.exe delete $ServiceName
if ($LASTEXITCODE -ne 0) {
    Write-Error "sc.exe delete 失败（退出码 $LASTEXITCODE）。请确认无其它进程持有句柄。"
    exit 1
}
Write-Host "==> 已删除服务：$ServiceName"

# 防御性清理：旧版（< T-019）install-service.ps1 曾生成 frp-easy-svc.cmd 包装脚本。
# 本任务移除生成逻辑后（cwd 由 frp-easy.exe 内 os.Chdir 锁定），老安装升级再卸载时
# 磁盘上可能仍有残留 .cmd；本段把它清掉，避免用户在卸载后看到一个"不知何用"的文件。
$ScriptDir  = $PSScriptRoot
$InstallDir = (Resolve-Path (Join-Path $ScriptDir "..")).Path
$WrapperPath = Join-Path $InstallDir "frp-easy-svc.cmd"
if (Test-Path -PathType Leaf $WrapperPath) {
    Remove-Item -Force $WrapperPath -ErrorAction SilentlyContinue
    Write-Host "==> 已清理旧版包装脚本残留：$WrapperPath"
}

@"

服务已卸载。

注意：数据目录与配置文件未删除：
  $InstallDir\frp_easy.toml
  $InstallDir\.frp_easy\

如需彻底清理，请手动执行：
  Remove-Item -Recurse -Force "$InstallDir\.frp_easy"
  Remove-Item -Force "$InstallDir\frp_easy.toml"
"@ | Write-Host

exit 0
