# build.ps1 — frp_easy 生产构建脚本（Windows）
#
# 输出：bin\frp-easy.exe（Windows x64）和（可选）bin\frp-easy-linux（Linux x64，交叉编译）
#
# 用法：.\scripts\build.ps1
#       .\scripts\build.ps1 -All    # 同时交叉编译 Linux 版本

[CmdletBinding()]
param([switch]$All)

$ErrorActionPreference = "Stop"
$root = (Get-Location).Path

# 找 Go
$goExe = "go"
if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    $candidate = "C:\Program Files\Go\bin\go.exe"
    if (Test-Path $candidate) { $goExe = $candidate }
    else { Write-Error "找不到 go 命令，请先安装 Go 1.22+"; exit 1 }
}

# 前端构建（若 web/ 存在且有 package.json）
if (Test-Path (Join-Path $root "web\package.json")) {
    Write-Host "构建前端（npm run build）..." -ForegroundColor Cyan
    Push-Location (Join-Path $root "web")
    npm install --frozen-lockfile 2>&1 | Out-Null
    npm run build
    Pop-Location
    Write-Host "前端构建完成" -ForegroundColor Green
}

$null = New-Item -ItemType Directory -Path (Join-Path $root "bin") -Force

$version = "0.1.0"
$ldflags = "-X main.Version=$version -s -w"

# Windows
Write-Host "编译 bin\frp-easy.exe ..." -ForegroundColor Cyan
$env:GOOS = "windows"; $env:GOARCH = "amd64"; $env:CGO_ENABLED = "0"
& $goExe build -ldflags $ldflags -o (Join-Path $root "bin\frp-easy.exe") ./cmd/frp-easy
Write-Host "bin\frp-easy.exe 构建完成" -ForegroundColor Green

if ($All) {
    Write-Host "交叉编译 bin\frp-easy-linux ..." -ForegroundColor Cyan
    $env:GOOS = "linux"; $env:GOARCH = "amd64"; $env:CGO_ENABLED = "0"
    & $goExe build -ldflags $ldflags -o (Join-Path $root "bin\frp-easy-linux") ./cmd/frp-easy
    Write-Host "bin\frp-easy-linux 构建完成" -ForegroundColor Green
}

Remove-Item Env:GOOS, Env:GOARCH, Env:CGO_ENABLED -ErrorAction SilentlyContinue
Write-Host "构建完成 ✓" -ForegroundColor Green
