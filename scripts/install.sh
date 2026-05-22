#!/usr/bin/env bash
# install.sh — frp_easy 一键安装脚本（Linux / macOS）
#
# 用途：一条命令完成 frp_easy 的下载、安装与开机自启配置。自动探测 OS/架构，
#       调用 GitHub Releases API 取固定标签 `rolling` 的滚动发布（与 main 分支
#       同步、每次 push main 自动刷新），下载并校验发布包，解压到固定安装目录，
#       再调用解压包内的 install-service.sh 注册 systemd 服务。
# 用法：
#   推荐（curl | bash 形态，需 root/sudo）：
#     curl -fsSL https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.sh | sudo bash
#   谨慎用户（先下载审阅再执行）：
#     curl -fsSL https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.sh -o install.sh
#     sudo bash install.sh
# 参数：-h | --help              显示本帮助后退出（退出码 0）
#       环境变量 FRP_EASY_INSTALL_DIR  覆盖安装目录（默认 /opt/frp-easy），高级用法
# 输出：stdout 中文进度行（每阶段一行）；stderr 仅错误。
# 退出码：0 成功（含 -h 帮助、macOS 降级收尾）
#         1 前置/环境失败（非 root / 缺依赖 / 非 amd64 / 网络或 API 不可用 / 下载解压失败）
#         2 服务注册阶段失败（透传 install-service.sh 的退出码）
# 说明：本脚本不删除已存在安装中的 frp_easy.toml 与 .frp_easy/ 数据目录；
#       目标目录已存在 frp-easy 时按"升级"语义处理（覆盖二进制/脚本，保留配置与数据）。

set -euo pipefail

# ---- 全局常量 ----
API_URL="https://api.github.com/repos/Alan-IFT/frp_easy/releases/tags/rolling"
INSTALL_DIR="${FRP_EASY_INSTALL_DIR:-/opt/frp-easy}"
TMP_DIR=""

# ---- 步骤 0：参数解析与 -h/--help（必须在依赖检测之前）----
while [[ $# -gt 0 ]]; do
    case "$1" in
        -h|--help)
            cat <<'EOF'
用法: install.sh [-h|--help]

frp_easy 一键安装脚本（Linux / macOS）—— 下载滚动发布包（与 main 分支同步）、
安装到固定目录、注册 systemd 开机自启。

推荐用法（curl | bash 形态，需 root / sudo 权限）:
  curl -fsSL https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.sh | sudo bash

谨慎用户（先下载脚本审阅后再执行）:
  curl -fsSL https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.sh -o install.sh
  sudo bash install.sh

参数:
  -h, --help    显示本帮助后退出

环境变量:
  FRP_EASY_INSTALL_DIR    覆盖安装目录（默认 /opt/frp-easy），高级用法。

安装目录:
  默认 /opt/frp-easy（可由 FRP_EASY_INSTALL_DIR 覆盖）。

所需权限:
  root / sudo（安装到 /opt 与配置 systemd 均需 root 权限）。

所需依赖:
  curl、tar（缺失时脚本会给出安装提示）。

退出码:
  0  成功（含本帮助、macOS 降级收尾）
  1  前置/环境失败（非 root / 缺依赖 / 非 amd64 / 网络或 API 不可用 / 下载解压失败）
  2  服务注册阶段失败（透传 install-service.sh 退出码）

卸载:
  sudo /opt/frp-easy/scripts/uninstall-service.sh
EOF
            exit 0
            ;;
        *)
            echo "错误：未识别的参数 $1，运行 bash install.sh --help 查看用法。" >&2
            exit 1
            ;;
    esac
done

# ---- 步骤 1：前置依赖与权限检测 ----
echo "==> [1/8] 检查运行环境..."

if ! command -v curl >/dev/null 2>&1; then
    echo "错误：未检测到 curl，请先安装（Debian/Ubuntu: apt-get install -y curl；RHEL: yum install -y curl）。" >&2
    exit 1
fi

if ! command -v tar >/dev/null 2>&1; then
    echo "错误：未检测到 tar，请先安装（Debian/Ubuntu: apt-get install -y tar）。" >&2
    exit 1
fi

if [[ "$(id -u)" -ne 0 ]]; then
    echo "错误：请以 root / sudo 运行（安装到 /opt 与配置 systemd 需 root 权限）。" >&2
    echo "      用法：curl -fsSL <url> | sudo bash" >&2
    exit 1
fi

# ---- 步骤 2：探测 OS / 架构 ----
echo "==> [2/8] 探测操作系统与 CPU 架构..."
OS_RAW="$(uname -s)"
ARCH_RAW="$(uname -m)"

case "$OS_RAW" in
    Linux)  OS="linux" ;;
    Darwin) OS="darwin" ;;
    *)
        echo "错误：暂不支持的操作系统 $OS_RAW，请用源码构建（见 docs/DEPLOYMENT.md 路径 B）。" >&2
        exit 1
        ;;
esac

case "$ARCH_RAW" in
    x86_64|amd64) ARCH="amd64" ;;
    *)
        echo "错误：当前架构 $ARCH_RAW 暂无预编译发布包，请用源码构建（见 docs/DEPLOYMENT.md 路径 B）。" >&2
        exit 1
        ;;
esac

PLATFORM="${OS}-${ARCH}"
echo "    检测到平台：${PLATFORM}"

# ---- 步骤 3：查询 GitHub Releases API ----
echo "==> [3/8] 查询 GitHub 滚动发布..."
# 一次请求同时拿响应体与 HTTP 状态码：去掉 -f，让 curl 在 4xx/5xx 下仍返回 0，
# 仅网络层失败（DNS / 连接）才让 curl 非 0；据此分流 BC-1（网络）与 BC-2/BC-4（状态码）。
api_curl_ok=1
api_resp="$(curl -sSL -w $'\n%{http_code}' "$API_URL" 2>/dev/null)" || api_curl_ok=0

if [[ "$api_curl_ok" -eq 0 ]]; then
    echo "错误：无法访问 GitHub（请检查网络或代理）。" >&2
    exit 1
fi

http_code="$(printf '%s' "$api_resp" | tail -n1)"
body="$(printf '%s' "$api_resp" | sed '$d')"

# 先判 HTTP 状态码、后解析 JSON（限流 403 响应体也是合法 JSON）。
case "$http_code" in
    200)
        : # 正常，继续
        ;;
    403)
        echo "错误：GitHub API 触发限流（未认证请求 60 次/小时/IP），请稍后重试，或改用 docs/DEPLOYMENT.md 路径 A 手动下载。" >&2
        exit 1
        ;;
    404)
        echo "错误：滚动发布尚未生成（维护者尚未首次 push main），暂无法一键安装；请改用源码构建（docs/DEPLOYMENT.md 路径 B）或等待维护者首次 push。" >&2
        exit 1
        ;;
    *)
        echo "错误：GitHub API 返回异常状态 ${http_code}，请稍后重试。" >&2
        exit 1
        ;;
esac

# ---- 步骤 4：从 API 响应解析资产 URL（无 jq，用 grep/sed）----
echo "==> [4/8] 解析发布包下载地址..."
# 顺带提取版本号，仅用于进度打印。grep 无匹配在 set -e 下会中止，故加 || true。
VERSION="$(printf '%s' "$body" \
    | grep -oE '"tag_name":[[:space:]]*"[^"]+"' \
    | sed -E 's/.*"([^"]+)"[[:space:]]*$/\1/' \
    | head -n1 || true)"

ASSET_URL="$(printf '%s' "$body" \
    | grep -oE '"browser_download_url":[[:space:]]*"[^"]+"' \
    | sed -E 's/.*"(https[^"]+)"[[:space:]]*$/\1/' \
    | grep -E "frp-easy-.*-${PLATFORM}\.tar\.gz$" \
    | head -n1 || true)"

if [[ -z "$ASSET_URL" ]]; then
    if [[ "$OS" == "darwin" ]]; then
        # macOS 定制文案：release.yml 当前不产 darwin-amd64 资产，macOS 为次要平台。
        echo "提示：滚动发布未提供 macOS 专用包，frp_easy 的 macOS 支持为次要平台。" >&2
        echo "错误：滚动发布未包含当前平台（${PLATFORM}）的发布包，请用源码构建（见 docs/DEPLOYMENT.md 路径 B）。" >&2
        exit 1
    fi
    echo "错误：滚动发布未包含当前平台（${PLATFORM}）的发布包。" >&2
    exit 1
fi

if [[ -n "$VERSION" ]]; then
    echo "    滚动发布版本：${VERSION}"
fi

# ---- 步骤 5：下载、校验、解压 ----
echo "==> [5/8] 下载并校验发布包..."
TMP_DIR="$(mktemp -d)"
# trap 在 TMP_DIR 赋值之后设置，避免 trap 到空变量导致 rm -rf ""。
trap 'rm -rf "$TMP_DIR"' EXIT
TARBALL="$TMP_DIR/release.tar.gz"

if ! curl -fsSL -o "$TARBALL" "$ASSET_URL"; then
    echo "错误：发布包下载失败，请检查网络后重试。" >&2
    exit 1
fi

if [[ ! -s "$TARBALL" ]]; then
    echo "错误：下载的发布包为空（0 字节）。" >&2
    exit 1
fi

if ! tar tzf "$TARBALL" >/dev/null 2>&1; then
    echo "错误：发布包损坏，无法解压。" >&2
    exit 1
fi

if ! tar xzf "$TARBALL" -C "$TMP_DIR" 2>/dev/null; then
    echo "错误：发布包解压失败（磁盘空间不足或权限问题）。" >&2
    exit 1
fi

EXTRACTED="$(find "$TMP_DIR" -maxdepth 1 -type d -name 'frp-easy-*' | head -n1)"
if [[ -z "$EXTRACTED" || ! -d "$EXTRACTED" ]]; then
    echo "错误：发布包结构异常，未找到预期的顶层目录。" >&2
    exit 1
fi

# ---- 步骤 6：安装到固定目录（含升级语义）----
if [[ -e "$INSTALL_DIR/frp-easy" ]]; then
    echo "==> [6/8] 检测到已存在安装，执行升级（保留 frp_easy.toml 与 .frp_easy/）..."
    # 先停服（无 systemctl 或服务不存在均不报错）。
    if command -v systemctl >/dev/null 2>&1; then
        systemctl stop frp-easy >/dev/null 2>&1 || true
    fi
    # 白名单逐项覆盖：绝不触碰 frp_easy.toml 与 .frp_easy/。
    cp -a "$EXTRACTED/frp-easy" "$INSTALL_DIR/frp-easy"
    if [[ -d "$EXTRACTED/frp_linux" ]]; then
        rm -rf "$INSTALL_DIR/frp_linux"
        cp -a "$EXTRACTED/frp_linux" "$INSTALL_DIR/"
    fi
    if [[ -d "$EXTRACTED/scripts" ]]; then
        rm -rf "$INSTALL_DIR/scripts"
        cp -a "$EXTRACTED/scripts" "$INSTALL_DIR/"
    fi
    for f in README.txt VERSION LICENSE frp_easy.toml.example; do
        if [[ -e "$EXTRACTED/$f" ]]; then
            cp -a "$EXTRACTED/$f" "$INSTALL_DIR/$f"
        fi
    done
    chmod 0755 "$INSTALL_DIR/frp-easy" 2>/dev/null || true
else
    echo "==> [6/8] 安装到 ${INSTALL_DIR}..."
    mkdir -p "$INSTALL_DIR"
    cp -a "$EXTRACTED/." "$INSTALL_DIR/"
    chmod 0755 "$INSTALL_DIR/frp-easy" 2>/dev/null || true
fi

# ---- 步骤 7：注册服务 ----
SERVICE_SCRIPT="$INSTALL_DIR/scripts/install-service.sh"

if [[ "$OS" == "darwin" ]]; then
    # macOS 无 systemd：服务化降级为打印手动启动提示，以退出码 0 收尾。
    echo "==> [7/8] macOS 不支持 systemd 服务化，跳过服务注册（已下载安装完成）。"
    echo ""
    echo "frp_easy 已安装到：${INSTALL_DIR}"
    echo "macOS 下请手动启动："
    echo "  cd ${INSTALL_DIR} && ./frp-easy"
    echo ""
    echo "启动后浏览器访问：http://127.0.0.1:7800"
    exit 0
fi

echo "==> [7/8] 注册 systemd 开机自启服务..."
if [[ ! -f "$SERVICE_SCRIPT" ]]; then
    echo "错误：未找到 ${SERVICE_SCRIPT}，发布包结构异常。" >&2
    exit 1
fi

if ! bash "$SERVICE_SCRIPT"; then
    rc=$?
    echo "错误：服务注册失败（install-service.sh 退出码 ${rc}）。请查看上方 install-service.sh 的中文报错。" >&2
    exit "$rc"
fi

# ---- 步骤 8：打印安装结果 ----
echo "==> [8/8] 安装完成。"

LOCAL_IP="$(hostname -I 2>/dev/null | awk '{print $1}' || true)"
[[ -z "$LOCAL_IP" ]] && LOCAL_IP="<本机IP>"

cat <<EOF

============================================================
frp_easy 一键安装完成！

安装目录：${INSTALL_DIR}

访问地址：
  本机访问：    http://127.0.0.1:7800
  局域网访问：  http://${LOCAL_IP}:7800

常用命令：
  systemctl status frp-easy        # 查看服务状态
  systemctl is-active frp-easy     # 仅查 active 状态
  journalctl -u frp-easy -f        # 实时日志

卸载：
  sudo ${INSTALL_DIR}/scripts/uninstall-service.sh
============================================================
EOF

exit 0
