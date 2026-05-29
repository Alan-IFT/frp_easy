#requires -Version 7
# start-e2e-server.ps1 — 为 Playwright E2E 测试启动 frp-easy 后端（Windows / PowerShell 7+）。
# 与 scripts/start-e2e-server.sh 行为对齐，专供 PowerShell 调用路径使用。
# 设计文档：docs/features/polish-pass/02_SOLUTION_DESIGN.md §2
#
# 使用独立临时数据目录（FRP_EASY_CONFIG 环境变量注入），保证测试数据隔离。
# Playwright webServer 通过子进程退出信号管理本进程生命周期。

$ErrorActionPreference = "Stop"

$ROOT     = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$BIN      = Join-Path $ROOT "bin\frp-easy.exe"
$DIST_DIR = Join-Path $ROOT "internal\assets\dist"

function Need-Rebuild {
    param([string]$Binary)
    if (-not (Test-Path $Binary)) { return $true }
    if (-not (Test-Path $DIST_DIR)) { return $false }
    $binMtime = (Get-Item $Binary).LastWriteTime
    $newer = Get-ChildItem -Path $DIST_DIR -Recurse -File -ErrorAction SilentlyContinue |
        Where-Object { $_.LastWriteTime -gt $binMtime } |
        Select-Object -First 1
    return [bool]$newer
}

if (Need-Rebuild -Binary $BIN) {
    [Console]::Error.WriteLine("[e2e-server] building binary (dist/ changed or binary missing)...")
    Push-Location $ROOT
    try {
        $env:CGO_ENABLED = "0"
        $goBin = "C:\Program Files\Go\bin\go.exe"
        if (-not (Test-Path $goBin)) {
            $goCmd = Get-Command go -ErrorAction SilentlyContinue
            if ($null -eq $goCmd) { throw "go not found in PATH and not at $goBin" }
            $goBin = $goCmd.Source
        }
        & $goBin build -o "bin\frp-easy.exe" ".\cmd\frp-easy"
        if ($LASTEXITCODE -ne 0) { throw "go build failed (exit $LASTEXITCODE)" }
    } finally {
        Pop-Location
    }
    [Console]::Error.WriteLine("[e2e-server] build done")
} else {
    [Console]::Error.WriteLine("[e2e-server] binary up-to-date, skipping build")
}

# E2E_PORT 默认 17800（playwright.config.ts 通过 webServer.env 注入同值）：刻意避开
# 产品默认 7800，与用户本机运行的 frp-easy 实例隔离，根治 C.1 假性失败（insight L25）。
$e2ePort = if ($env:E2E_PORT) { $env:E2E_PORT } else { "17800" }

# 临时数据目录与配置（每次 PID 唯一；Playwright 终止后留在 $env:TEMP，下次清理）
$tmpName = "frp-easy-e2e-" + ([Guid]::NewGuid().ToString("N"))
$E2E_TMP = Join-Path $env:TEMP $tmpName
New-Item -ItemType Directory -Force -Path $E2E_TMP | Out-Null
[Console]::Error.WriteLine("[e2e-server] using E2E_TMP: $E2E_TMP (port $e2ePort)")

# DataDir / LogDir 路径里有 Windows 反斜杠，TOML 字符串需转义（写双反斜杠或正斜杠）
$dataDir = ($E2E_TMP + "\data") -replace '\\', '/'
$logDir  = ($E2E_TMP + "\logs") -replace '\\', '/'
$tomlPath = Join-Path $E2E_TMP "frp_easy.toml"
$tomlContent = @"
UIBindAddr = "127.0.0.1"
UIPort     = $e2ePort
DataDir    = "$dataDir"
LogDir     = "$logDir"
"@
# 强制写无 BOM UTF-8（BurntSushi/toml 不接受 BOM）
[System.IO.File]::WriteAllText($tomlPath, $tomlContent, [System.Text.UTF8Encoding]::new($false))

$env:FRP_EASY_CONFIG = $tomlPath
# 直接 invoke：本脚本进程作为 frp-easy 的父；Playwright 关本进程即关 frp-easy
& $BIN
exit $LASTEXITCODE
