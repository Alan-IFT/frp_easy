#!/usr/bin/env bash
# package.sh — frp_easy Linux/macOS 发布包打包脚本
#
# 用途：在 scripts/build.sh 产物基础上组装 staging 目录并 tar czf 出 bin/release/<...>.tar.gz；
#       可选同时打 Windows zip（需 tar.exe 或在 Windows 上跑 package.ps1）。
# 用法：./scripts/package.sh [--linux] [--windows] [--version <s>] [--skip-build] [-h]
# 参数：--linux       打 linux-amd64 tar.gz（默认开）
#       --windows     同时打 windows-amd64 zip（需 bin/frp-easy.exe；若 --skip-build 关，会触发 build.sh --all）
#       --version <s> 显式覆盖版本号（默认 git describe --tags --always --dirty || dev）
#       --skip-build  跳过 build.sh 调用（要求 bin/frp-easy[.exe] 已存在）
#       -h | --help   显示本帮助
# 输出：stdout 中文进度；产物落在 bin/release/frp-easy-<version>-<os>-amd64.<ext>
# 退出码：0 成功 / 1 前置缺失（bin/frp-easy / frp_linux/frpc / frp_linux/frps 等）/ 2 build.sh 调用失败

set -euo pipefail

DO_LINUX=true
DO_WINDOWS=false
SKIP_BUILD=false
OVERRIDE_VERSION=""

while [[ $# -gt 0 ]]; do
    case "$1" in
        --linux)
            DO_LINUX=true
            shift
            ;;
        --windows)
            DO_WINDOWS=true
            shift
            ;;
        --version)
            OVERRIDE_VERSION="${2:-}"
            shift 2
            ;;
        --skip-build)
            SKIP_BUILD=true
            shift
            ;;
        -h|--help)
            cat <<'EOF'
用法: package.sh [--linux] [--windows] [--version <s>] [--skip-build] [-h]

参数:
  --linux         打 linux-amd64 tar.gz（默认开）
  --windows       同时打 windows-amd64 zip
  --version <s>   覆盖版本号
  --skip-build    跳过 scripts/build.sh 调用
  -h, --help      显示本帮助

输出: bin/release/frp-easy-<version>-<os>-amd64.<ext>
EOF
            exit 0
            ;;
        *)
            echo "错误：未识别的参数 $1，运行 ./package.sh --help 查看用法" >&2
            exit 1
            ;;
    esac
done

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

# 解析版本号（继承 build.sh L19 兜底）
if [[ -n "$OVERRIDE_VERSION" ]]; then
    VERSION="$OVERRIDE_VERSION"
else
    VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "dev")
fi
echo "==> 版本号：$VERSION"

# 调 build.sh（除非 --skip-build）
if ! $SKIP_BUILD; then
    BUILD_ARGS=()
    if $DO_WINDOWS; then BUILD_ARGS+=("--all"); fi
    echo "==> 构建（scripts/build.sh ${BUILD_ARGS[*]:-}）"
    if ! bash "$ROOT/scripts/build.sh" "${BUILD_ARGS[@]:-}"; then
        echo "错误：scripts/build.sh 调用失败。" >&2
        exit 2
    fi
fi

# 前置校验：主二进制存在且非空
if $DO_LINUX; then
    if [[ ! -x "$ROOT/bin/frp-easy" || ! -s "$ROOT/bin/frp-easy" ]]; then
        echo "错误：bin/frp-easy 不存在或为空；请先运行 scripts/build.sh。" >&2
        exit 1
    fi
fi
if $DO_WINDOWS; then
    if [[ ! -s "$ROOT/bin/frp-easy.exe" ]]; then
        echo "错误：bin/frp-easy.exe 不存在或为空；请先运行 scripts/build.sh --all。" >&2
        exit 1
    fi
fi

# 前置 sanity check（03_GATE_REVIEW MINOR-5 / MINOR-R5）：调 --version 捕获 ldflags 失效 / 二进制损坏。
# Linux 主二进制：在 Linux 主机上 --version 失败必须 FAIL exit 1（产物损坏不应发布）；
# 在 Git Bash on Windows / macOS 等不能直接跑 ELF 的主机上降级为 WARN。
if $DO_LINUX && [[ -x "$ROOT/bin/frp-easy" ]]; then
    HOST_OS="$(uname -s 2>/dev/null || echo unknown)"
    if "$ROOT/bin/frp-easy" --version >/dev/null 2>&1; then
        echo "==> bin/frp-easy --version sanity check 通过"
    else
        if [[ "$HOST_OS" == "Linux" ]]; then
            echo "错误：bin/frp-easy --version 调用失败（主机为 Linux，二进制本应可执行）；请重新跑 scripts/build.sh。" >&2
            exit 1
        else
            echo "==> 警告：bin/frp-easy --version 调用失败或不可在当前主机执行（主机 $HOST_OS 非 Linux，无法直接跑 ELF），跳过 sanity check" >&2
        fi
    fi
fi

# 校验 frp 子二进制完整性
if $DO_LINUX; then
    for f in frp_linux/frpc frp_linux/frps; do
        if [[ ! -s "$ROOT/$f" ]]; then
            echo "错误：缺少 $f；请确认 frp_linux/ 子目录完整。" >&2
            exit 1
        fi
    done
fi
if $DO_WINDOWS; then
    for f in frp_win/frpc.exe frp_win/frps.exe; do
        if [[ ! -s "$ROOT/$f" ]]; then
            echo "错误：缺少 $f；请确认 frp_win/ 子目录完整。" >&2
            exit 1
        fi
    done
fi

RELEASE_DIR="$ROOT/bin/release"
mkdir -p "$RELEASE_DIR"

# 内联生成 frp_easy.toml.example（FR-1.4 / MINOR-4：不写任何引号包裹 8+ 字符敏感串）
# 仅写 README §配置说明 中的 4 个字段默认值。
make_toml_example() {
    local out="$1"
    cat > "$out" <<'EOF'
# frp_easy 自身的配置文件示例（与 frpc.toml / frps.toml 不同）
# 复制为 frp_easy.toml 后按需修改。首次启动若无此文件，frp-easy 会自动写入默认值。

UIBindAddr = "0.0.0.0"
UIPort     = 7800
DataDir    = "./.frp_easy"
LogDir     = "./.frp_easy/logs"
EOF
}

# 内联生成 README.txt — Linux 版
make_readme_linux() {
    local out="$1" ver="$2"
    cat > "$out" <<EOF
frp_easy ${ver} — 部署快速开始（Linux）
========================================

三步起步：

  1. 解压（你已经完成本步）：
       tar xzf frp-easy-${ver}-linux-amd64.tar.gz

  2. 进入解压目录：
       cd frp-easy-${ver}-linux-amd64

  3. 启动 UI 服务：
       ./frp-easy
     看到 stderr 提示 "frp_easy UI 已启动" 后，用浏览器打开 http://127.0.0.1:7800。

可选（作为系统服务运行）：
  sudo ./scripts/install-service.sh
  systemctl status frp-easy
  journalctl -u frp-easy -f
  sudo ./scripts/uninstall-service.sh

常用命令：
  ./frp-easy --version    # 显示版本号
  ./frp-easy --help       # 显示帮助

文档：详见 docs/DEPLOYMENT.md（GitHub 仓库内）。
默认配置：UI 监听 0.0.0.0:7800（局域网可访问，本机用 http://127.0.0.1:7800）；数据目录 ./.frp_easy；日志 ./.frp_easy/logs/。
EOF
}

# 内联生成 README.txt — Windows 版
make_readme_windows() {
    local out="$1" ver="$2"
    cat > "$out" <<EOF
frp_easy ${ver} — 部署快速开始（Windows）
==========================================

三步起步：

  1. 解压（你已经完成本步）：右键 frp-easy-${ver}-windows-amd64.zip -> 全部解压。

  2. 进入解压目录：在该目录打开 PowerShell（按住 Shift 右键 -> 在此处打开 PowerShell）。

  3. 启动 UI 服务：
       .\frp-easy.exe
     看到 stderr 提示 "frp_easy UI 已启动" 后，用浏览器打开 http://127.0.0.1:7800。

可选（作为 Windows 服务运行）：
  以管理员身份打开 PowerShell，然后：
    .\scripts\install-service.ps1
    sc query frp-easy
    .\scripts\uninstall-service.ps1

常用命令：
  .\frp-easy.exe --version    # 显示版本号
  .\frp-easy.exe --help       # 显示帮助

文档：详见 docs/DEPLOYMENT.md（GitHub 仓库内）。
默认配置：UI 监听 0.0.0.0:7800（局域网可访问，本机用 http://127.0.0.1:7800）；数据目录 .\.frp_easy；日志 .\.frp_easy\logs\。
EOF
}

# 组装并打包单平台
build_package() {
    local os="$1" arch="$2" ext="$3"
    local pkg_name="frp-easy-${VERSION}-${os}-${arch}"
    local staging="$ROOT/bin/release/.staging-${os}"
    local top="$staging/$pkg_name"

    echo "==> 组装 staging: $top"
    rm -rf "$staging"
    mkdir -p "$top/scripts"

    if [[ "$os" == "linux" ]]; then
        cp "$ROOT/bin/frp-easy" "$top/frp-easy"
        chmod 0755 "$top/frp-easy"
        mkdir -p "$top/frp_linux"
        cp -a "$ROOT/frp_linux/." "$top/frp_linux/"
        chmod 0755 "$top/frp_linux/frpc" "$top/frp_linux/frps"
        cp "$ROOT/scripts/install-service.sh" "$top/scripts/install-service.sh"
        cp "$ROOT/scripts/uninstall-service.sh" "$top/scripts/uninstall-service.sh"
        chmod 0755 "$top/scripts/install-service.sh" "$top/scripts/uninstall-service.sh"
        make_readme_linux "$top/README.txt" "$VERSION"
    else
        cp "$ROOT/bin/frp-easy.exe" "$top/frp-easy.exe"
        mkdir -p "$top/frp_win"
        cp -a "$ROOT/frp_win/." "$top/frp_win/"
        cp "$ROOT/scripts/install-service.ps1" "$top/scripts/install-service.ps1"
        cp "$ROOT/scripts/uninstall-service.ps1" "$top/scripts/uninstall-service.ps1"
        make_readme_windows "$top/README.txt" "$VERSION"
    fi

    make_toml_example "$top/frp_easy.toml.example"
    echo "$VERSION" > "$top/VERSION"

    # LICENSE：仓库根 LICENSE 若存在则带上，否则 WARN（Open Question 7 PM-resolved a）
    if [[ -f "$ROOT/LICENSE" ]]; then
        cp "$ROOT/LICENSE" "$top/LICENSE"
    else
        echo "==> 警告：仓库根 LICENSE 不存在，发布包将不含 LICENSE 文件（建议后续补 LICENSE）" >&2
    fi

    # 健全性检查（设计 §4 末尾断言）
    local file_count
    file_count=$(find "$top" -type f | wc -l)
    if [[ "$file_count" -lt 6 ]]; then
        echo "错误：staging 文件数量 $file_count < 6，组装异常。" >&2
        exit 1
    fi

    # 打包：tar.gz / zip
    local out="$RELEASE_DIR/${pkg_name}.${ext}"
    rm -f "$out"
    if [[ "$ext" == "tar.gz" ]]; then
        ( cd "$staging" && tar czf "$out" "$pkg_name" )
    else
        # Windows zip：优先用 zip 工具；不可用则用 tar 自带 zip 能力（bsdtar / Windows tar.exe 在 22H2+ 支持）
        if command -v zip >/dev/null 2>&1; then
            ( cd "$staging" && zip -r -q "$out" "$pkg_name" )
        elif command -v tar >/dev/null 2>&1 && tar --help 2>&1 | grep -q "format=zip" ; then
            ( cd "$staging" && tar -a -cf "$out" "$pkg_name" )
        else
            echo "错误：未找到 zip / 不支持 tar -a zip；请安装 zip 或在 Windows 上跑 package.ps1。" >&2
            exit 1
        fi
    fi

    # 清理 staging
    rm -rf "$staging"

    # 包体阈值 WARN（NFR-8.1）
    if command -v du >/dev/null 2>&1; then
        local size_mb
        size_mb=$(du -m "$out" | awk '{print $1}')
        if (( size_mb > 25 )); then
            echo "==> 警告：包体 ${size_mb} MB 超出 25 MB 软上限（NFR-8.1）" >&2
        fi
        echo "==> 完成：$out（${size_mb} MB）"
    else
        echo "==> 完成：$out"
    fi
}

if $DO_LINUX; then
    build_package "linux" "amd64" "tar.gz"
fi
if $DO_WINDOWS; then
    build_package "windows" "amd64" "zip"
fi

echo "==> 打包完成。产物目录：$RELEASE_DIR"
exit 0
