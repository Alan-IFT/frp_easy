# package.ps1 — frp_easy Windows 发布包打包脚本
#
# 用途：在 scripts/build.ps1 产物基础上组装 staging 目录并 Compress-Archive 出 bin\release\<...>.zip；
#       可选同时打 Linux tar.gz（需 Windows 10 22H2+ 自带 tar.exe）。
# 用法：.\scripts\package.ps1 [-Windows] [-Linux] [-Version <s>] [-SkipBuild]
# 参数：-Windows     打 windows-amd64 zip（默认开）
#       -Linux       同时打 linux-amd64 tar.gz（需 bin\frp-easy 与 tar.exe）
#       -Version     显式覆盖版本号
#       -SkipBuild   跳过 build.ps1 调用
# 输出：stdout 中文进度；产物落在 bin\release\frp-easy-<version>-<os>-amd64.<ext>
# 退出码：0 成功 / 1 前置缺失 / 2 build.ps1 调用失败

[CmdletBinding()]
param(
    [switch]$Linux,
    [switch]$Windows,
    [string]$Version,
    [switch]$SkipBuild
)

$ErrorActionPreference = "Stop"

# 默认开 Windows：仅当用户显式 -Linux 而不显式 -Windows 时才单独跑 Linux
if (-not $Linux -and -not $Windows) {
    $Windows = $true
}

$ROOT = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
Set-Location $ROOT

# 解析版本号
if (-not $Version) {
    $Version = & git describe --tags --always --dirty 2>$null
    if (-not $Version -or $LASTEXITCODE -ne 0) { $Version = "dev" }
}
Write-Host "==> 版本号：$Version"

# 调 build.ps1
if (-not $SkipBuild) {
    $buildArgs = @()
    if ($Linux) { $buildArgs += "-All" }
    Write-Host "==> 构建（scripts\build.ps1 $($buildArgs -join ' '))"
    & (Join-Path $ROOT "scripts\build.ps1") @buildArgs
    if ($LASTEXITCODE -ne 0) {
        Write-Error "scripts\build.ps1 调用失败（退出码 $LASTEXITCODE）。"
        exit 2
    }
}

# 前置校验
if ($Windows) {
    $exePath = Join-Path $ROOT "bin\frp-easy.exe"
    if (-not (Test-Path -PathType Leaf $exePath) -or (Get-Item $exePath).Length -eq 0) {
        Write-Error "bin\frp-easy.exe 不存在或为空；请先运行 scripts\build.ps1。"
        exit 1
    }
}
if ($Linux) {
    $linuxBin = Join-Path $ROOT "bin\frp-easy-linux"
    if (-not (Test-Path -PathType Leaf $linuxBin) -or (Get-Item $linuxBin).Length -eq 0) {
        Write-Error "bin\frp-easy-linux 不存在或为空；请先运行 scripts\build.ps1 -All。"
        exit 1
    }
}

# 前置 sanity check（03_GATE_REVIEW MINOR-5）：调 --version 捕获 ldflags 失效 / 二进制损坏
if ($Windows) {
    try {
        & (Join-Path $ROOT "bin\frp-easy.exe") --version | Out-Null
        if ($LASTEXITCODE -eq 0) {
            Write-Host "==> bin\frp-easy.exe --version sanity check 通过"
        } else {
            Write-Host "==> 警告：bin\frp-easy.exe --version 退出码 $LASTEXITCODE，可能 ldflags 失效" -ForegroundColor Yellow
        }
    } catch {
        Write-Host "==> 警告：bin\frp-easy.exe --version 调用失败：$_" -ForegroundColor Yellow
    }
}

# 校验 frp 子二进制完整性
if ($Windows) {
    foreach ($f in @("frp_win\frpc.exe", "frp_win\frps.exe")) {
        $p = Join-Path $ROOT $f
        if (-not (Test-Path -PathType Leaf $p) -or (Get-Item $p).Length -eq 0) {
            Write-Error "缺少 $f；请确认 frp_win\ 子目录完整。"
            exit 1
        }
    }
}
if ($Linux) {
    foreach ($f in @("frp_linux\frpc", "frp_linux\frps")) {
        $p = Join-Path $ROOT $f
        if (-not (Test-Path -PathType Leaf $p) -or (Get-Item $p).Length -eq 0) {
            Write-Error "缺少 $f；请确认 frp_linux\ 子目录完整。"
            exit 1
        }
    }
}

$ReleaseDir = Join-Path $ROOT "bin\release"
$null = New-Item -ItemType Directory -Path $ReleaseDir -Force

# frp_easy.toml.example 内联生成函数（FR-1.4 / MINOR-4：不写任何引号包裹 8+ 字符敏感串）
function Write-TomlExample {
    param([string]$Path)
    $content = @"
# frp_easy 自身的配置文件示例（与 frpc.toml / frps.toml 不同）
# 复制为 frp_easy.toml 后按需修改。首次启动若无此文件，frp-easy 会自动写入默认值。

UIBindAddr = "127.0.0.1"
UIPort     = 8080
DataDir    = "./.frp_easy"
LogDir     = "./.frp_easy/logs"
"@
    Set-Content -Path $Path -Value $content -Encoding UTF8
}

# README.txt — Windows 版
function Write-ReadmeWindows {
    param([string]$Path, [string]$Ver)
    $content = @"
frp_easy $Ver — 部署快速开始（Windows）
==========================================

三步起步：

  1. 解压（你已经完成本步）：右键 frp-easy-$Ver-windows-amd64.zip -> 全部解压。

  2. 进入解压目录：在该目录打开 PowerShell（按住 Shift 右键 -> 在此处打开 PowerShell）。

  3. 启动 UI 服务：
       .\frp-easy.exe
     看到 stderr 提示 "frp_easy UI 已启动：http://127.0.0.1:8080" 后，浏览器打开该地址。

可选（作为 Windows 服务运行）：
  以管理员身份打开 PowerShell，然后：
    .\scripts\install-service.ps1
    sc query frp-easy
    .\scripts\uninstall-service.ps1

常用命令：
  .\frp-easy.exe --version    # 显示版本号
  .\frp-easy.exe --help       # 显示帮助

文档：详见 docs/DEPLOYMENT.md（GitHub 仓库内）。
默认配置：UI 监听 127.0.0.1:8080；数据目录 .\.frp_easy；日志 .\.frp_easy\logs\。
"@
    Set-Content -Path $Path -Value $content -Encoding UTF8
}

# README.txt — Linux 版
function Write-ReadmeLinux {
    param([string]$Path, [string]$Ver)
    $content = @"
frp_easy $Ver — 部署快速开始（Linux）
========================================

三步起步：

  1. 解压（你已经完成本步）：
       tar xzf frp-easy-$Ver-linux-amd64.tar.gz

  2. 进入解压目录：
       cd frp-easy-$Ver-linux-amd64

  3. 启动 UI 服务：
       ./frp-easy
     看到 stderr 提示 "frp_easy UI 已启动：http://127.0.0.1:8080" 后，浏览器打开该地址。

可选（作为系统服务运行）：
  sudo ./scripts/install-service.sh
  systemctl status frp-easy
  journalctl -u frp-easy -f
  sudo ./scripts/uninstall-service.sh

常用命令：
  ./frp-easy --version    # 显示版本号
  ./frp-easy --help       # 显示帮助

文档：详见 docs/DEPLOYMENT.md（GitHub 仓库内）。
默认配置：UI 监听 127.0.0.1:8080；数据目录 ./.frp_easy；日志 ./.frp_easy/logs/。
"@
    Set-Content -Path $Path -Value $content -Encoding UTF8
}

# 组装并打包单平台
function Build-Package {
    param(
        [string]$Os,
        [string]$Arch,
        [string]$Ext
    )

    $pkgName = "frp-easy-$Version-$Os-$Arch"
    $staging = Join-Path $ROOT "bin\release\.staging-$Os"
    $top     = Join-Path $staging $pkgName

    Write-Host "==> 组装 staging: $top"
    if (Test-Path $staging) { Remove-Item -Recurse -Force $staging }
    $null = New-Item -ItemType Directory -Path (Join-Path $top "scripts") -Force

    if ($Os -eq "windows") {
        Copy-Item (Join-Path $ROOT "bin\frp-easy.exe") (Join-Path $top "frp-easy.exe")
        Copy-Item (Join-Path $ROOT "frp_win") (Join-Path $top "frp_win") -Recurse
        Copy-Item (Join-Path $ROOT "scripts\install-service.ps1") (Join-Path $top "scripts\install-service.ps1")
        Copy-Item (Join-Path $ROOT "scripts\uninstall-service.ps1") (Join-Path $top "scripts\uninstall-service.ps1")
        Write-ReadmeWindows -Path (Join-Path $top "README.txt") -Ver $Version
    } else {
        Copy-Item (Join-Path $ROOT "bin\frp-easy-linux") (Join-Path $top "frp-easy")
        Copy-Item (Join-Path $ROOT "frp_linux") (Join-Path $top "frp_linux") -Recurse
        Copy-Item (Join-Path $ROOT "scripts\install-service.sh") (Join-Path $top "scripts\install-service.sh")
        Copy-Item (Join-Path $ROOT "scripts\uninstall-service.sh") (Join-Path $top "scripts\uninstall-service.sh")
        Write-ReadmeLinux -Path (Join-Path $top "README.txt") -Ver $Version
    }

    Write-TomlExample -Path (Join-Path $top "frp_easy.toml.example")
    Set-Content -Path (Join-Path $top "VERSION") -Value $Version -Encoding ASCII

    # LICENSE：仓库根 LICENSE 若存在则带上，否则 WARN
    $licSrc = Join-Path $ROOT "LICENSE"
    if (Test-Path -PathType Leaf $licSrc) {
        Copy-Item $licSrc (Join-Path $top "LICENSE")
    } else {
        Write-Host "==> 警告：仓库根 LICENSE 不存在，发布包将不含 LICENSE 文件（建议后续补 LICENSE）" -ForegroundColor Yellow
    }

    # 健全性检查
    $fileCount = (Get-ChildItem -Recurse -File $top).Count
    if ($fileCount -lt 6) {
        Write-Error "staging 文件数量 $fileCount < 6，组装异常。"
        exit 1
    }

    $outPath = Join-Path $ReleaseDir "$pkgName.$Ext"
    if (Test-Path $outPath) { Remove-Item -Force $outPath }

    if ($Ext -eq "zip") {
        Compress-Archive -Path (Join-Path $staging "$pkgName") -DestinationPath $outPath -Force
    } else {
        # tar.gz：使用 Windows 10 22H2+ 自带的 tar.exe（bsdtar）
        $tarExe = Get-Command tar.exe -ErrorAction SilentlyContinue
        if (-not $tarExe) {
            Write-Error "未找到 tar.exe（Windows 10 22H2+ 自带）；请改在 Linux 上跑 package.sh。"
            exit 1
        }
        Push-Location $staging
        & tar.exe -czf $outPath $pkgName
        $tarExit = $LASTEXITCODE
        Pop-Location
        if ($tarExit -ne 0) {
            Write-Error "tar.exe 打包失败（退出码 $tarExit）。"
            exit 1
        }
    }

    Remove-Item -Recurse -Force $staging

    $sizeMB = [math]::Round((Get-Item $outPath).Length / 1MB, 1)
    if ($sizeMB -gt 25) {
        Write-Host "==> 警告：包体 $sizeMB MB 超出 25 MB 软上限（NFR-8.1）" -ForegroundColor Yellow
    }
    Write-Host "==> 完成：$outPath（$sizeMB MB）" -ForegroundColor Green
}

if ($Windows) {
    Build-Package -Os "windows" -Arch "amd64" -Ext "zip"
}
if ($Linux) {
    Build-Package -Os "linux" -Arch "amd64" -Ext "tar.gz"
}

Write-Host "==> 打包完成。产物目录：$ReleaseDir" -ForegroundColor Green
exit 0
