# install.ps1 — frp_easy 一键安装脚本（Windows）
#
# 用途：一条命令完成 frp_easy 的下载、安装与 Windows 服务注册。自动探测架构，
#       调用 GitHub Releases API 取固定标签 `rolling` 的滚动发布（与 main 分支
#       同步、每次 push main 自动刷新），下载并校验 zip 发布包，解压到固定安装
#       目录，再调用解压包内的 install-service.ps1 注册 Windows 服务。
# 用法：
#   推荐（irm | iex 形态，需以管理员身份运行 PowerShell）：
#     irm https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.ps1 | iex
#   谨慎用户（先下载审阅再执行）：
#     irm https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.ps1 -OutFile install.ps1
#     # 审阅 install.ps1 内容后（管理员 PowerShell）
#     .\install.ps1
# 参数：-Help                          显示本帮助后退出（退出码 0）
#       环境变量 FRP_EASY_INSTALL_DIR  覆盖安装目录（默认 C:\Program Files\frp-easy），高级用法
# 输出：stdout 中文进度行（每阶段一行）；stderr 仅错误。
# 退出码：0 成功（含 -Help 帮助）
#         1 前置/环境失败（非管理员 / 非 amd64 / 网络或 API 不可用 / 下载解压失败）
#         2 服务注册阶段失败（透传 install-service.ps1 的退出码）
# 说明：本脚本不删除已存在安装中的 frp_easy.toml 与 .frp_easy\ 数据目录；
#       目标目录已存在 frp-easy.exe 时按"升级"语义处理（覆盖二进制/脚本，保留配置与数据）。

[CmdletBinding()]
param(
    [switch]$Help
)

$ErrorActionPreference = "Stop"

# ---- 全局常量 ----
$ApiUrl = "https://api.github.com/repos/Alan-IFT/frp_easy/releases/tags/rolling"
$InstallDir = if ($env:FRP_EASY_INSTALL_DIR) { $env:FRP_EASY_INSTALL_DIR } else { "C:\Program Files\frp-easy" }

# ---- 函数：Get-PublicIPv4 ----
# 意图：Windows 路径同步 Linux detect_public_ip 的公网 IP 探测（T-017 FR-B / G-8）。
# 入参：无（读 env FRP_EASY_PUBLIC_IP + 函数内常量 $PublicIPCandidates）。
# 出参：成功返回 IPv4 字符串；失败返回 $null。
# 预算：单候选 Invoke-WebRequest -TimeoutSec 3 秒，最坏 3 × 3 = 9 秒。
# 验证：HTTP 200 + Trim() + [ipaddress]::TryParse 校验 IPv4 字面量；HTML 错误页
# 不当 IP 用（insight L37 红线）。
# Windows 路径暂无 role 区分（OOS-2），公网 IP 探测一律执行（C-6）。
function Get-PublicIPv4 {
    # FRP_EASY_PUBLIC_IP short-circuit：用户预先指定时跳过外网探测。
    if ($env:FRP_EASY_PUBLIC_IP) {
        $parsed = $null
        if ([ipaddress]::TryParse($env:FRP_EASY_PUBLIC_IP, [ref]$parsed)) {
            return $env:FRP_EASY_PUBLIC_IP
        }
        # 非 IP 字面量但格式安全（hostname / IPv6）—— 也直接返回，由用户自负。
        if ($env:FRP_EASY_PUBLIC_IP -match '^[A-Za-z0-9.:_-]+$') {
            return $env:FRP_EASY_PUBLIC_IP
        }
        return $null
    }
    # NFR-2：明文写死候选 URL（用户可 grep 审计），与 install.sh 同款 3 条。
    $PublicIPCandidates = @(
        'https://api.ipify.org',
        'https://ifconfig.me/ip',
        'https://icanhazip.com'
    )
    $prevProgress = $ProgressPreference
    $ProgressPreference = 'SilentlyContinue'
    try {
        foreach ($url in $PublicIPCandidates) {
            try {
                $resp = Invoke-WebRequest -Uri $url -TimeoutSec 3 -UseBasicParsing -ErrorAction Stop
                if ($resp.StatusCode -eq 200) {
                    $ip = ($resp.Content).Trim()
                    $parsed = $null
                    if ([ipaddress]::TryParse($ip, [ref]$parsed) -and $parsed.AddressFamily -eq 'InterNetwork') {
                        return $ip
                    }
                }
            } catch {
                # 单候选失败 → 继续下一个
                continue
            }
        }
    } finally {
        $ProgressPreference = $prevProgress
    }
    return $null
}

# ---- 步骤 0：-Help（必须在依赖检测之前）----
if ($Help) {
    @"
用法: install.ps1 [-Help]

frp_easy 一键安装脚本（Windows）—— 下载滚动发布包（与 main 分支同步）、
安装到固定目录、注册 Windows 服务实现开机自启。

推荐用法（irm | iex 形态，需以管理员身份运行 PowerShell）:
  irm https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.ps1 | iex

谨慎用户（先下载脚本审阅后再执行）:
  irm https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.ps1 -OutFile install.ps1
  # 审阅 install.ps1 内容后（管理员 PowerShell）
  .\install.ps1

参数:
  -Help    显示本帮助后退出

环境变量:
  FRP_EASY_INSTALL_DIR    覆盖安装目录（默认 C:\Program Files\frp-easy），高级用法。

安装目录:
  默认 C:\Program Files\frp-easy（可由 FRP_EASY_INSTALL_DIR 覆盖）。

所需权限:
  管理员（写入 Program Files 与注册 Windows 服务均需管理员权限）。

退出码:
  0  成功（含本帮助）
  1  前置/环境失败（非管理员 / 非 amd64 / 网络或 API 不可用 / 下载解压失败）
  2  服务注册阶段失败（透传 install-service.ps1 退出码）

卸载:
  以管理员身份运行 PowerShell 执行 C:\Program Files\frp-easy\scripts\uninstall-service.ps1
"@ | Write-Host
    exit 0
}

# ---- 步骤 1：前置检测 ----
Write-Host "==> [1/8] 检查运行环境..."

$isAdmin = ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
if (-not $isAdmin) {
    Write-Error "请以管理员身份运行 PowerShell（右键 -> 以管理员身份运行）后再执行一键安装。"
    exit 1
}

if (-not (Get-Command sc.exe -ErrorAction SilentlyContinue)) {
    Write-Error "未检测到 sc.exe；本脚本依赖 Windows 自带的 Service Controller。"
    exit 1
}

# ---- 步骤 2：架构探测 ----
Write-Host "==> [2/8] 探测 CPU 架构..."
$archRaw = $env:PROCESSOR_ARCHITECTURE
if ($archRaw -ne "AMD64") {
    Write-Error "当前架构 $archRaw 暂无预编译发布包，请用源码构建（见 docs/DEPLOYMENT.md 路径 B）。"
    exit 1
}
$platform = "windows-amd64"
Write-Host "    检测到平台：$platform"

# ---- 步骤 3：查询 GitHub Releases API ----
Write-Host "==> [3/8] 查询 GitHub 滚动发布..."
# Invoke-WebRequest 对 4xx/5xx 默认抛异常（$ErrorActionPreference=Stop）。
# 在 catch 里区分网络层失败（无 Response）与 HTTP 错误状态码。
$apiContent = $null
try {
    $resp = Invoke-WebRequest -Uri $ApiUrl -UseBasicParsing -ErrorAction Stop
    $apiContent = $resp.Content
} catch {
    $statusCode = $null
    if ($_.Exception.Response) {
        try { $statusCode = [int]$_.Exception.Response.StatusCode } catch { $statusCode = $null }
    }
    if ($null -eq $statusCode) {
        Write-Error "无法访问 GitHub（请检查网络或代理）。"
        exit 1
    } elseif ($statusCode -eq 403) {
        Write-Error "GitHub API 触发限流（未认证请求 60 次/小时/IP），请稍后重试，或改用 docs/DEPLOYMENT.md 路径 A 手动下载。"
        exit 1
    } elseif ($statusCode -eq 404) {
        Write-Error "滚动发布尚未生成（维护者尚未首次 push main），暂无法一键安装；请改用源码构建（docs/DEPLOYMENT.md 路径 B）或等待维护者首次 push。"
        exit 1
    } else {
        Write-Error "GitHub API 返回异常状态 $statusCode，请稍后重试。"
        exit 1
    }
}

# ---- 步骤 4：解析资产 URL ----
Write-Host "==> [4/8] 解析发布包下载地址..."
try {
    $json = $apiContent | ConvertFrom-Json
} catch {
    Write-Error "GitHub API 返回的内容无法解析，请稍后重试。"
    exit 1
}

$version = $json.tag_name
$asset = $json.assets | Where-Object { $_.name -match "frp-easy-.*-windows-amd64\.zip$" } | Select-Object -First 1
if (-not $asset) {
    Write-Error "滚动发布未包含 Windows 平台（$platform）的发布包。"
    exit 1
}
$assetUrl = $asset.browser_download_url
if ($version) {
    Write-Host "    滚动发布版本：$version"
}

# ---- 步骤 5：下载、校验、解压 ----
Write-Host "==> [5/8] 下载并校验发布包..."
$tmpDir = New-Item -ItemType Directory -Path (Join-Path ([System.IO.Path]::GetTempPath()) ("frp-easy-" + [guid]::NewGuid().ToString("N")))
try {
    $zipPath = Join-Path $tmpDir.FullName "release.zip"
    # A 组（UX，PM 默认 §8.1 (d) 组合 + 非交互降级）：
    #  - 去掉 -UseBasicParsing：PS 5.x 下该 flag 显式抑制 Write-Progress；
    #    PS 7+ 下该 flag 已是 no-op，去掉无副作用（02 §A.3）。
    #  - $ProgressPreference 显式控制：交互式终端 Continue（显示进度），
    #    重定向 / CI / 无 UserInteractive 时 SilentlyContinue（避免日志膨胀，FR-A.6）。
    #  - try/finally 恢复原 $ProgressPreference 不污染调用环境。
    $prevProgress = $ProgressPreference
    $isInteractive = [Environment]::UserInteractive -and -not [Console]::IsErrorRedirected
    if ($isInteractive) {
        $ProgressPreference = 'Continue'
    } else {
        $ProgressPreference = 'SilentlyContinue'
    }
    try {
        try {
            Invoke-WebRequest -Uri $assetUrl -OutFile $zipPath -ErrorAction Stop
        } catch {
            Write-Error "发布包下载失败，请检查网络后重试。"
            exit 1
        }
    } finally {
        $ProgressPreference = $prevProgress
    }

    if (-not (Test-Path -PathType Leaf $zipPath) -or (Get-Item $zipPath).Length -le 0) {
        Write-Error "下载的发布包为空（0 字节）。"
        exit 1
    }

    try {
        Expand-Archive -Path $zipPath -DestinationPath $tmpDir.FullName -Force -ErrorAction Stop
    } catch {
        Write-Error "发布包损坏或解压失败（磁盘空间不足或权限问题）。"
        exit 1
    }

    $extracted = Get-ChildItem -Path $tmpDir.FullName -Directory -Filter 'frp-easy-*' | Select-Object -First 1
    if (-not $extracted) {
        Write-Error "发布包结构异常，未找到预期的顶层目录。"
        exit 1
    }

    # ---- 步骤 6：安装到固定目录（含升级语义）----
    if (Test-Path -PathType Leaf (Join-Path $InstallDir "frp-easy.exe")) {
        Write-Host "==> [6/8] 检测到已存在安装，执行升级（保留 frp_easy.toml 与 .frp_easy\）..."
        # 先停服（服务不存在不影响）。
        & sc.exe stop frp-easy 2>&1 | Out-Null
        Start-Sleep -Milliseconds 500
        # 白名单逐项覆盖：绝不触碰 frp_easy.toml、.frp_easy\，以及用户运行时下载的 frp_win\。
        Copy-Item -Force (Join-Path $extracted.FullName "frp-easy.exe") (Join-Path $InstallDir "frp-easy.exe")
        # T-014：升级不再覆盖/删除 frp_win\ —— 发布包已不含 frp 二进制，
        # frp_win\ 下的 frpc.exe/frps.exe 由用户经 UI 横幅按需下载，升级须原样保留。
        foreach ($sub in @("scripts")) {
            $src = Join-Path $extracted.FullName $sub
            if (Test-Path $src) {
                $dst = Join-Path $InstallDir $sub
                if (Test-Path $dst) { Remove-Item -Recurse -Force $dst }
                Copy-Item -Recurse -Force $src $dst
            }
        }
        foreach ($f in @("README.txt", "VERSION", "LICENSE", "frp_easy.toml.example")) {
            $src = Join-Path $extracted.FullName $f
            if (Test-Path $src) {
                Copy-Item -Force $src (Join-Path $InstallDir $f)
            }
        }
    } else {
        Write-Host "==> [6/8] 安装到 $InstallDir..."
        if (-not (Test-Path $InstallDir)) {
            New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
        }
        Copy-Item -Recurse -Force (Join-Path $extracted.FullName "*") $InstallDir
    }

    # ---- 步骤 7：注册 Windows 服务 ----
    Write-Host "==> [7/8] 注册 Windows 服务..."
    $svc = Join-Path $InstallDir "scripts\install-service.ps1"
    if (-not (Test-Path -PathType Leaf $svc)) {
        Write-Error "未找到 $svc，发布包结构异常。"
        exit 1
    }
    # C 组（退出码透传，02 §C.3）：显式重置 $LASTEXITCODE 防止
    # install-service.ps1 因 terminating error 未走到 `exit N` 时
    # $LASTEXITCODE 保留上一条命令陈旧值，导致透传错误码。
    $LASTEXITCODE = 0
    & $svc
    if ($LASTEXITCODE -ne 0) {
        Write-Error "服务注册失败（install-service.ps1 退出码 $LASTEXITCODE）。请查看上方中文报错。"
        exit $LASTEXITCODE
    }
} finally {
    Remove-Item -Recurse -Force $tmpDir.FullName -ErrorAction SilentlyContinue
}

# ---- 步骤 8：打印安装结果 ----
# Windows 路径暂无 role 区分（OOS-2），公网 IP 探测一律执行（C-6）。
# Service 默认以 LocalSystem 跑，没有 Linux unit User= 那个根因；UI 监听地址沿用
# appconf.Default() = 0.0.0.0，main.go 启动时会打印安全提示。
Write-Host "==> [8/8] 安装完成。"

$localIp = "<本机IP>"
try {
    $ip = Get-NetIPAddress -AddressFamily IPv4 -ErrorAction Stop |
        Where-Object { $_.IPAddress -notmatch '^127\.' -and $_.IPAddress -notmatch '^169\.254\.' } |
        Select-Object -First 1
    if ($ip) { $localIp = $ip.IPAddress }
} catch {
    $localIp = "<本机IP>"
}

# 公网 IP 探测（T-017 G-8 / FR-B 同步）。失败时给出明确中文降级文案 + 手动覆盖样例。
$publicIp = Get-PublicIPv4

$publicLine = ""
$publicHint = ""
if ($publicIp) {
    # BC-3：IPv6 字面量必须用 [xxx]:port 包裹，否则浏览器无法解析。
    # 探测路径只返回 IPv4，仅当用户手动 $env:FRP_EASY_PUBLIC_IP=<IPv6> 时会走到这里。
    if ($publicIp -match ':') {
        $publicUrl = "http://[${publicIp}]:7800"
    } else {
        $publicUrl = "http://${publicIp}:7800"
    }
    if ($publicIp -eq $localIp) {
        # AMBIG-F = F2：公网 = LAN 时仍打两行 + 标注。
        $publicLine = "  公网访问：    ${publicUrl}   （与局域网 IP 相同 —— 本机直接在公网上）"
    } else {
        $publicLine = "  公网访问：    ${publicUrl}"
    }
} else {
    $publicLine = "  公网访问：    <公网 IP 探测失败，请手动确认服务器出口 IP>"
    $publicHint = @"

    国内 VM（腾讯云 / 阿里云 / 华为云）可登云控制台 → 实例详情复制公网 IP。
    确认后可设 `$env:FRP_EASY_PUBLIC_IP=<your-ip>` 后重新运行本脚本（绕过探测）。
"@
}

@"

============================================================
frp_easy 一键安装完成！

安装目录：$InstallDir

访问地址：
  本机访问：    http://127.0.0.1:7800
  局域网访问：  http://${localIp}:7800
$publicLine$publicHint

常用命令：
  sc query frp-easy        # 查看服务状态
  sc stop  frp-easy        # 手动停止
  sc start frp-easy        # 手动启动

更新：
  重新运行同一条一键安装命令即可升级到最新版：
    irm https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.ps1 | iex
  升级会保留你的配置（frp_easy.toml）与数据（.frp_easy\），
  以及已下载的 frp 二进制（frp_win\）。

卸载：
  以管理员身份运行 PowerShell 执行：
  $InstallDir\scripts\uninstall-service.ps1
============================================================
"@ | Write-Host

exit 0
