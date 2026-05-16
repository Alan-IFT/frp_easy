# start.ps1 — frp_easy 开发模式启动脚本（Windows）
#
# 开发模式：Go API (port 8080) + Vite dev (port 5173) 独立运行，
# Go 侧以 DevMode=true 开 CORS 允许 vite 代理。
#
# 用法：.\scripts\start.ps1
#       .\scripts\start.ps1 -Prod   # 生产模式（单二进制，不启动 vite）

[CmdletBinding()]
param([switch]$Prod)

$ErrorActionPreference = "Stop"
$root = (Get-Location).Path

# 找 Go 可执行文件
$goExe = "go"
if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    $candidate = "C:\Program Files\Go\bin\go.exe"
    if (Test-Path $candidate) { $goExe = $candidate }
    else { Write-Error "找不到 go 命令，请先安装 Go 1.22+"; exit 1 }
}

if ($Prod) {
    # 生产模式：直接运行已编译的二进制（若不存在则先编译）
    $bin = Join-Path $root "bin\frp-easy.exe"
    if (-not (Test-Path $bin)) {
        Write-Host "未找到 bin\frp-easy.exe，先编译..." -ForegroundColor Cyan
        & "$PSScriptRoot\build.ps1"
    }
    Write-Host "启动 frp_easy（生产）..." -ForegroundColor Green
    & $bin
} else {
    # 开发模式：同时启动 Go API + Vite
    $jobs = @()

    # Go API（dev 模式）
    Write-Host "启动 Go API (dev mode)..." -ForegroundColor Cyan
    $goJob = Start-Job -ScriptBlock {
        param($root, $goExe)
        Set-Location $root
        & $goExe run -v ./cmd/frp-easy 2>&1
    } -ArgumentList $root, $goExe
    $jobs += $goJob

    # Vite dev（若 web/ 目录存在）
    if (Test-Path (Join-Path $root "web\package.json")) {
        Write-Host "启动 Vite dev server..." -ForegroundColor Cyan
        $viteJob = Start-Job -ScriptBlock {
            param($root)
            Set-Location (Join-Path $root "web")
            npm run dev 2>&1
        } -ArgumentList $root
        $jobs += $viteJob
    } else {
        Write-Host "web/ 目录不存在，跳过 Vite（仅运行 Go API）" -ForegroundColor Yellow
    }

    Write-Host "开发服务已启动。Ctrl+C 停止。" -ForegroundColor Green
    try {
        while ($true) {
            Start-Sleep 1
            foreach ($j in $jobs) {
                $out = Receive-Job $j
                if ($out) { Write-Host $out }
            }
        }
    } finally {
        $jobs | Stop-Job
        $jobs | Remove-Job -Force
    }
}
