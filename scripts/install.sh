#!/usr/bin/env bash
# install.sh — frp_easy 一键安装脚本（Linux / macOS）
#
# 用途：一条命令完成 frp_easy 的下载、安装与开机自启配置。自动探测 OS/架构，
#       调用 GitHub Releases API 取固定标签 `rolling` 的滚动发布（与 main 分支
#       同步、每次 push main 自动刷新），下载并校验发布包，解压到固定安装目录，
#       再调用解压包内的 install-service.sh 注册 systemd 服务。
# 用法：
#   推荐（curl | bash 形态，需 root/sudo）：
#     服务端（公网 VM，监听 0.0.0.0，需要公网 IP）：
#       curl -fsSL https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.sh | FRP_EASY_ROLE=server sudo -E bash
#     客户端（内网设备，仅监听 127.0.0.1，最安全）：
#       curl -fsSL https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.sh | FRP_EASY_ROLE=client sudo -E bash
#   谨慎用户（先下载审阅再执行）：
#     curl -fsSL https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.sh -o install.sh
#     FRP_EASY_ROLE=server sudo -E bash install.sh
# 参数：-h | --help              显示本帮助后退出（退出码 0）
#       环境变量 FRP_EASY_ROLE          server | client（必填；server 监听 0.0.0.0、client 监听 127.0.0.1）
#       环境变量 FRP_EASY_FORCE_ROLE    yes（升级期与已装 role 冲突时强制覆盖 .role 并重写配置；危险）
#       环境变量 FRP_EASY_PUBLIC_IP     合法 IPv4/IPv6（绕过公网 IP 自动探测，server 模式适用）
#       环境变量 FRP_EASY_INSTALL_DIR   覆盖安装目录（默认 /opt/frp-easy），高级用法
# 输出：stdout 中文进度行（每阶段一行）；stderr 仅错误。
# 退出码：0 成功（含 -h 帮助、macOS 降级收尾）
#         1 前置/环境失败（非 root / 缺依赖 / 非 amd64 / 网络或 API 不可用 / 下载解压失败）
#         2 服务注册阶段失败（透传 install-service.sh 的退出码）
#         3 FRP_EASY_ROLE 未指定 / 非法 / 升级期 role 冲突且无 FRP_EASY_FORCE_ROLE
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

必填环境变量：
  FRP_EASY_ROLE=server  —— 服务端（公网 VM；监听 0.0.0.0，对外提供 frps + Web UI）
  FRP_EASY_ROLE=client  —— 客户端（内网设备；仅监听 127.0.0.1，最安全）

推荐用法（curl | bash 形态，需 root / sudo 权限；sudo -E 透传环境变量）:
  服务端：curl -fsSL https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.sh | FRP_EASY_ROLE=server sudo -E bash
  客户端：curl -fsSL https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.sh | FRP_EASY_ROLE=client sudo -E bash

谨慎用户（先下载脚本审阅后再执行）:
  curl -fsSL https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.sh -o install.sh
  FRP_EASY_ROLE=server sudo -E bash install.sh

参数:
  -h, --help    显示本帮助后退出

环境变量:
  FRP_EASY_ROLE           server | client（必填）。server 监听 0.0.0.0；client 监听 127.0.0.1。
  FRP_EASY_FORCE_ROLE     yes（升级期与已装 .role 冲突时强制覆盖；将备份旧 frp_easy.toml 后重写）。
  FRP_EASY_PUBLIC_IP      合法 IPv4 / IPv6 字面量（绕过公网 IP 自动探测，server 模式适用；国内 VM 推荐）。
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
  3  FRP_EASY_ROLE 未指定 / 非法 / 升级期 role 冲突且无 FRP_EASY_FORCE_ROLE

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

# ---- 步骤 0.5：FRP_EASY_ROLE 解析与校验（T-017）----
# 设计依据：02 §5.1 + PM 决议 AMBIG-C 子问题 C1.a = (a) 拒绝静默默认。
# 必须设置 FRP_EASY_ROLE=server|client；其它取值（含未设置、含空串）一律 exit 3。
# 用户原话："服务端需要公网 IP，客户端监听 127.0.0.1 最安全" —— 静默默认值会让用户
# 错装客户端到公网 VM 或反之，安全风险高，因此宁可显式失败也不猜测。
ROLE="${FRP_EASY_ROLE:-}"
if [[ -z "$ROLE" || ( "$ROLE" != "server" && "$ROLE" != "client" ) ]]; then
    echo "错误：必须指定 FRP_EASY_ROLE=server|client（不允许静默默认）" >&2
    echo "  服务端（公网 VM）：curl -fsSL https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.sh | FRP_EASY_ROLE=server sudo -E bash" >&2
    echo "  客户端（内网设备）：curl -fsSL https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.sh | FRP_EASY_ROLE=client sudo -E bash" >&2
    echo "  说明：sudo 需 -E 才能透传环境变量；服务端默认监听 0.0.0.0，客户端默认监听 127.0.0.1。" >&2
    exit 3
fi
echo "    role=${ROLE}"

# ---- 函数：render_frp_easy_toml ----
# 意图：按 ROLE 渲染 frp_easy.toml 的字面文本到 stdout，供步骤 6.5 预生成配置。
# 入参：$1 = role，取值 "server" 或 "client"（调用前已由 §0.5 校验）。
# 出参：stdout 输出 TOML 文本（含 trailing newline）；无返回码语义。
# 字段名严格 = internal/appconf/config.go L36-39 struct tag（UIBindAddr/UIPort/
# DataDir/LogDir），go-toml/v2 大小写敏感，不可改。DataDir 写相对路径 "./.frp_easy"
# 配合 unit WorkingDirectory=/opt/frp-easy 解析为 /opt/frp-easy/.frp_easy（C-4）。
# heredoc 必须用 <<'EOF' 单引号封禁插值（insight L38 quote-removal 红线）。
render_frp_easy_toml() {
    local role="$1"
    if [[ "$role" == "server" ]]; then
        cat <<'EOF'
# frp_easy.toml — 由 install.sh 在角色为 server 的全新安装中生成（T-017）。
# UIBindAddr=0.0.0.0 表示监听所有网卡（公网 + LAN + 回环），便于 frpc 客户端通过公网 IP 访问 Web UI。
# 仅需本机访问时可手动改为 "127.0.0.1" 后 systemctl restart frp-easy。
UIBindAddr = "0.0.0.0"
UIPort = 7800
DataDir = "./.frp_easy"
LogDir = "./.frp_easy/logs"
EOF
    else
        cat <<'EOF'
# frp_easy.toml — 由 install.sh 在角色为 client 的全新安装中生成（T-017）。
# UIBindAddr=127.0.0.1 表示仅监听回环（最安全），管理 UI 不暴露到公网 / 局域网。
# 如需局域网内访问 UI，可手动改为 "0.0.0.0" 后 systemctl restart frp-easy。
UIBindAddr = "127.0.0.1"
UIPort = 7800
DataDir = "./.frp_easy"
LogDir = "./.frp_easy/logs"
EOF
    fi
}

# ---- 函数：detect_public_ip ----
# 意图：公网 IP 探测；仅 server 模式调用。先判 FRP_EASY_PUBLIC_IP 用户手动覆盖通道
# （C-2 / C-5：函数首行 short-circuit），否则按顺序尝试 3 个明文写死的候选 URL。
# 入参：无（读 env FRP_EASY_PUBLIC_IP + 函数内 const PUBLIC_IP_CANDIDATES）。
# 出参：stdout = 合法 IPv4 字面量（成功时；无尾换行）；失败时 stdout 空字符串。
# 返回码：0 成功 / 1 全部候选失败（含 short-circuit 校验失败）。
# 预算：单候选 curl --max-time 3 秒，最坏 3 × 3 = 9 秒；调用方在 server 横幅块同步等待。
# 验证：先判 HTTP 状态码 200（curl -f 让 4xx/5xx 自然 rc 非 0），再用 bash 正则
# ^([0-9]{1,3}\.){3}[0-9]{1,3}$ 强校验 IPv4 字面量（insight L37 红线：HTML 错误页
# 不当 IP 用）。
detect_public_ip() {
    # FRP_EASY_PUBLIC_IP short-circuit（C-2 / C-5；M-2 国内 VM 兜底通道）：
    # 用户预先知道公网 IP 时可手动指定，跳过 3 候选探测；仍需通过 IPv4 字面量校验
    # 防止用户把 hostname 或 URL 当 IP 传入。
    if [[ -n "${FRP_EASY_PUBLIC_IP:-}" ]]; then
        if [[ "$FRP_EASY_PUBLIC_IP" =~ ^([0-9]{1,3}\.){3}[0-9]{1,3}$ ]]; then
            printf '%s' "$FRP_EASY_PUBLIC_IP"
            return 0
        fi
        # 非 IPv4 字面量（可能是 IPv6 或 hostname）—— 也直接打印让用户自己拼 URL。
        # 仅在不含空格 / 换行 / 引号等危险字符时才信任。
        if [[ "$FRP_EASY_PUBLIC_IP" =~ ^[A-Za-z0-9.:_-]+$ ]]; then
            printf '%s' "$FRP_EASY_PUBLIC_IP"
            return 0
        fi
        return 1
    fi
    # NFR-2：明文写死候选 URL（用户可 grep 审计）；不允许从 env 动态拼接。
    local candidates=(
        "https://api.ipify.org"
        "https://ifconfig.me/ip"
        "https://icanhazip.com"
    )
    local url ip
    for url in "${candidates[@]}"; do
        # curl -f：HTTP 4xx/5xx 让 curl rc != 0（不读响应体）；--max-time 3 单次预算；
        # -sS 安静但保留错误；不带 -L（echo IP 服务无 redirect 场景）。
        ip="$(curl -fsS --max-time 3 "$url" 2>/dev/null || true)"
        # trim 尾部换行 / 空白
        ip="${ip%%[$' \t\r\n']*}"
        if [[ "$ip" =~ ^([0-9]{1,3}\.){3}[0-9]{1,3}$ ]]; then
            printf '%s' "$ip"
            return 0
        fi
    done
    return 1
}

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

# A 组（UX）：交互式终端下显示 curl 进度条；非交互（stderr 重定向到文件 / 管道）降级为静默
# 以避免 \r 覆盖序列污染日志（FR-A.3 / FR-A.6 / BC-A.5）。
# 仅去掉 -s（show progress），保留 -f（4xx/5xx 让 curl 退非 0，FR-A.5 错误分流不变）、
# -S（错误仍打印）、-L（跟随 302→CDN）。
CURL_PROGRESS_FLAG=""
if [[ -t 2 ]]; then
    CURL_PROGRESS_FLAG="--progress-bar"
fi
if ! curl -fSL $CURL_PROGRESS_FLAG -o "$TARBALL" "$ASSET_URL"; then
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
    # 白名单逐项覆盖：绝不触碰 frp_easy.toml、.frp_easy/，以及用户运行时下载的 frp_linux/。
    cp -a "$EXTRACTED/frp-easy" "$INSTALL_DIR/frp-easy"
    # T-014：升级不再覆盖/删除 frp_linux/ —— 发布包已不含 frp 二进制，
    # frp_linux/ 下的 frpc/frps 由用户经 UI 横幅按需下载，升级须原样保留。
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

# ---- 步骤 6.5：配置角色 + 修复运行时属主（T-017，仅 Linux）----
# macOS 不走 systemd 路径（步骤 7 darwin 分支 exit 0），其 role + chown 语义无意义；
# 仅 Linux 分支需要预生成 toml + 局部 chown。J 决议：darwin 仅同步公网 IP 探测，
# role 与 chown 不强制。
if [[ "$OS" == "linux" ]]; then
    echo "==> [6.5/8] 应用 role=${ROLE} 并修复运行时属主..."

    # 解析 RUN_USER —— C-1 红线：verbatim 复制 install-service.sh L69-75 两段式
    # if-then-else（与 install-service.sh L69-75 必须保持等价，更改时同步两处）。
    # 不能用 `${SUDO_USER:-$(id -un)}` 简写：M-1 要求严格按照 service 脚本同款形态，
    # 同源同据避免 install-service.sh 后续生成 unit 时 User= 与本块 chown 目标错位。
    RUN_USER=""
    if [[ -z "$RUN_USER" ]]; then
        if [[ -n "${SUDO_USER:-}" ]]; then
            RUN_USER="$SUDO_USER"
        else
            RUN_USER="$(id -un)"
        fi
    fi

    # 校验 RUN_USER 存在（install-service.sh L104-108 同款 getent 校验）；提前失败
    # 比让 chown 失败后 install-service.sh 重复报错更易诊断。
    if ! getent passwd "$RUN_USER" >/dev/null 2>&1; then
        echo "错误：用户 $RUN_USER 不存在，请先 useradd 或在 install-service.sh 时传 --user 参数。" >&2
        exit 1
    fi

    # role 一致性校验（升级路径）+ 持久化 .role 文件（FR-C.2）。
    # .role 内容：单行 "server\n" 或 "client\n"；权限 0644 root:root（运行时不读）。
    ROLE_FILE="${INSTALL_DIR}/.role"
    if [[ -f "$ROLE_FILE" ]]; then
        OLD_ROLE="$(head -n1 "$ROLE_FILE" 2>/dev/null | tr -d '[:space:]' || true)"
        if [[ "$OLD_ROLE" != "$ROLE" ]]; then
            if [[ "${FRP_EASY_FORCE_ROLE:-no}" != "yes" ]]; then
                echo "错误：已检测到 role=${OLD_ROLE}，本次指定 role=${ROLE} 冲突。" >&2
                echo "  如需切换 role，请先运行卸载脚本再重装：" >&2
                echo "    sudo ${INSTALL_DIR}/scripts/uninstall-service.sh" >&2
                echo "    sudo rm -f ${INSTALL_DIR}/.role ${INSTALL_DIR}/frp_easy.toml" >&2
                echo "  或显式覆盖（将备份旧 frp_easy.toml 后重写）：" >&2
                echo "    FRP_EASY_ROLE=${ROLE} FRP_EASY_FORCE_ROLE=yes sudo -E bash ..." >&2
                exit 3
            fi
            # 强制覆盖路径：备份旧 toml → 重写 toml → 重写 .role。
            if [[ -f "${INSTALL_DIR}/frp_easy.toml" ]]; then
                cp -a "${INSTALL_DIR}/frp_easy.toml" "${INSTALL_DIR}/frp_easy.toml.bak.$(date +%s)"
            fi
            render_frp_easy_toml "$ROLE" > "${INSTALL_DIR}/frp_easy.toml"
            printf '%s\n' "$ROLE" > "$ROLE_FILE"
            chmod 0644 "$ROLE_FILE" "${INSTALL_DIR}/frp_easy.toml"
            TOML_WROTE="yes"
        else
            TOML_WROTE="no"
        fi
    else
        # 首次安装 或 从 T-017 之前版本升上来（无 .role）。
        # D1 红线：升级期已有 frp_easy.toml 保留用户值优先，不覆盖；
        # 仅全新安装（frp_easy.toml 不存在）才预生成。
        printf '%s\n' "$ROLE" > "$ROLE_FILE"
        chmod 0644 "$ROLE_FILE"
        if [[ ! -f "${INSTALL_DIR}/frp_easy.toml" ]]; then
            render_frp_easy_toml "$ROLE" > "${INSTALL_DIR}/frp_easy.toml"
            chmod 0644 "${INSTALL_DIR}/frp_easy.toml"
            TOML_WROTE="yes"
        else
            TOML_WROTE="no"
        fi
    fi

    # 局部 chown（C-7：不允许 || true 静默吞失败）。
    # 仅 chown 运行时可写路径：frp_easy.toml（若刚创建或已存在）、.frp_easy/、
    # frp_linux/。binary 与 scripts 保持 root:root（最小权限审计）。
    mkdir -p "${INSTALL_DIR}/.frp_easy" "${INSTALL_DIR}/frp_linux"
    if ! chown "$RUN_USER:$RUN_USER" "${INSTALL_DIR}/frp_easy.toml"; then
        echo "错误：chown frp_easy.toml 给 $RUN_USER 失败（请检查文件是否存在与权限）。" >&2
        exit 1
    fi
    if ! chown -R "$RUN_USER:$RUN_USER" "${INSTALL_DIR}/.frp_easy"; then
        echo "错误：chown -R .frp_easy/ 给 $RUN_USER 失败。" >&2
        exit 1
    fi
    if ! chown -R "$RUN_USER:$RUN_USER" "${INSTALL_DIR}/frp_linux"; then
        echo "错误：chown -R frp_linux/ 给 $RUN_USER 失败。" >&2
        exit 1
    fi

    if [[ "$TOML_WROTE" == "yes" ]]; then
        echo "    已预生成 ${INSTALL_DIR}/frp_easy.toml（role=${ROLE}）"
    else
        echo "    保留已有 ${INSTALL_DIR}/frp_easy.toml（D1 用户值优先）"
    fi
    echo "    .role=${ROLE} 持久化；属主修复完成（${RUN_USER}:${RUN_USER}）"
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

# C 组（退出码透传，PM 默认 §8.4 (a)）：把 `if ! cmd; then rc=$?` 反模式拆为
# `set +e; cmd; rc=$?; set -e` 三行块，避免 bash `set -e` 在 `if` 条件上下文
# 不生效与 `!` 反转后 then 块内 `$?` 跨版本语义不一致两条隐患（02 §C.1）。
# diag 打印仅在失败时输出（GR §F-1 hint 4），便于 QA / 用户拿到真实退出码证据。
set +e
bash "$SERVICE_SCRIPT"
rc=$?
set -e
[[ $rc -ne 0 ]] && echo "    [diag] install-service.sh rc=$rc" >&2

if [[ $rc -ne 0 ]]; then
    echo "错误：服务注册失败（install-service.sh 退出码 ${rc}）。请查看上方 install-service.sh 的中文报错。" >&2
    exit "$rc"
fi

# ---- 步骤 8：打印安装结果（role-aware）----
echo "==> [8/8] 安装完成。"

# LAN_IPS 收集所有非空网卡 IPv4（hostname -I 输出全部），供 server 横幅打印 LAN 行。
# 旧实现只取第一块网卡 → 多网卡 VM 上信息缺失；现展开全部，取第一条作主显示。
LAN_IPS=()
if command -v hostname >/dev/null 2>&1; then
    # shellcheck disable=SC2207
    LAN_IPS=( $(hostname -I 2>/dev/null || true) )
fi
LOCAL_IP="${LAN_IPS[0]:-<本机IP>}"

if [[ "$ROLE" == "client" ]]; then
    # client 模式：仅打印本机访问一行（FR-B.3）。不发起公网 IP 探测，不打印 LAN
    # 与公网行；目的是避免误导用户去开放公网，呼应"客户端 127.0.0.1 最安全"。
    cat <<EOF

============================================================
frp_easy 一键安装完成！（角色：客户端）

安装目录：${INSTALL_DIR}
角色文件：${INSTALL_DIR}/.role

访问地址：
  本机访问：    http://127.0.0.1:7800
  （客户端模式仅监听回环；如需局域网访问 UI，请改 frp_easy.toml 的 UIBindAddr 为 0.0.0.0 后 systemctl restart frp-easy）

常用命令：
  systemctl status frp-easy        # 查看服务状态
  systemctl is-active frp-easy     # 仅查 active 状态
  journalctl -u frp-easy -f        # 实时日志

更新：
  重新运行同一条一键安装命令即可升级到最新版：
    curl -fsSL https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.sh | FRP_EASY_ROLE=client sudo -E bash
  升级会保留你的配置（frp_easy.toml）与数据（.frp_easy/），
  以及已下载的 frp 二进制（frp_linux/）。

卸载：
  sudo ${INSTALL_DIR}/scripts/uninstall-service.sh
============================================================
EOF
else
    # server 模式：探测公网 IP；探测函数首行含 FRP_EASY_PUBLIC_IP short-circuit
    # （C-2 / C-5）。`|| true` 防止 detect_public_ip rc=1 在 set -e 下中止脚本。
    PUBLIC_IP="$(detect_public_ip || true)"

    echo ""
    echo "============================================================"
    echo "frp_easy 一键安装完成！（角色：服务端）"
    echo ""
    echo "安装目录：${INSTALL_DIR}"
    echo "角色文件：${INSTALL_DIR}/.role"
    echo ""
    echo "访问地址："
    echo "  本机访问：    http://127.0.0.1:7800"
    echo "  局域网访问：  http://${LOCAL_IP}:7800"
    if [[ -n "$PUBLIC_IP" ]]; then
        # BC-3：IPv6 字面量必须用 [xxx]:port 包裹，否则浏览器无法解析。
        # 探测路径只返回 IPv4，仅当用户手动 FRP_EASY_PUBLIC_IP=<IPv6> 时会走到这里。
        if [[ "$PUBLIC_IP" == *:* ]]; then
            PUBLIC_URL="http://[${PUBLIC_IP}]:7800"
        else
            PUBLIC_URL="http://${PUBLIC_IP}:7800"
        fi
        if [[ "$PUBLIC_IP" == "$LOCAL_IP" ]]; then
            # AMBIG-F = F2：公网 = LAN 时仍打两行 + 标注（信息最完整）。
            echo "  公网访问：    ${PUBLIC_URL}   （与局域网 IP 相同 —— 本机直接在公网上）"
        else
            echo "  公网访问：    ${PUBLIC_URL}"
        fi
    else
        # M-2 / C-2 兜底文案：国内 VM 上 3/3 候选 URL 高概率失败；必须给用户明确的
        # 手动覆盖路径与"登云控制台复制出口 IP"提示。
        echo "  公网访问：    <公网 IP 探测失败，请手动确认服务器出口 IP>"
        echo ""
        echo "    国内 VM（腾讯云 / 阿里云 / 华为云）可登云控制台 → 实例详情复制公网 IP。"
        echo "    确认后重新运行（绕过探测）："
        echo "      curl -fsSL https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.sh | FRP_EASY_PUBLIC_IP=<your-ip> FRP_EASY_ROLE=server sudo -E bash"
    fi
    # FR-C.6：server 模式必须打印防火墙 / 安全组提示。
    echo ""
    echo "提示（仅 server 模式）：若外部 frpc 客户端无法连上 7800 / 7000 端口："
    echo "  ① 云厂商安全组（腾讯云 / 阿里云 / AWS）是否放行 7800/tcp 与 7000/tcp"
    echo "  ② 本机 ufw / firewalld 是否放行（ufw allow 7800/tcp、ufw allow 7000/tcp）"
    echo ""
    echo "常用命令："
    echo "  systemctl status frp-easy        # 查看服务状态"
    echo "  systemctl is-active frp-easy     # 仅查 active 状态"
    echo "  journalctl -u frp-easy -f        # 实时日志"
    echo ""
    echo "更新："
    echo "  重新运行同一条一键安装命令即可升级到最新版："
    echo "    curl -fsSL https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.sh | FRP_EASY_ROLE=server sudo -E bash"
    echo "  升级会保留你的配置（frp_easy.toml）与数据（.frp_easy/），"
    echo "  以及已下载的 frp 二进制（frp_linux/）。"
    echo ""
    echo "卸载："
    echo "  sudo ${INSTALL_DIR}/scripts/uninstall-service.sh"
    echo "============================================================"
fi

exit 0
