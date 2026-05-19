#!/usr/bin/env bash
# install-service.sh — frp_easy Linux systemd 服务安装脚本
#
# 用途：将解压后的 frp-easy 二进制注册为 systemd 服务（unit 文件落在 /etc/systemd/system/frp-easy.service），
#       并 daemon-reload + enable --now 拉起服务；重复执行幂等（会刷新 unit 并重启服务）。
# 用法：sudo ./scripts/install-service.sh [--user <name>] [--name <unit-name>]
# 参数：--user  服务运行用户（默认 当前 root 调用者的 SUDO_USER 或 id -un）；getent 校验存在
#       --name  unit 基名（默认 frp-easy），高级用法（同主机多实例并行）
#       -h | --help  显示本帮助
# 输出：stdout 中文进度；stderr 仅错误；unit 文件 /etc/systemd/system/<name>.service 权限 0644
# 退出码：0 成功 / 1 前置失败（非 root / 缺 systemctl / 二进制缺失 / user 不存在）/ 2 systemctl 调用失败
# 说明：本脚本不删除 frp_easy.toml 与 .frp_easy/ 数据目录，卸载请走 uninstall-service.sh。

set -euo pipefail

UNIT_NAME="frp-easy"
RUN_USER=""

while [[ $# -gt 0 ]]; do
    case "$1" in
        --user)
            RUN_USER="${2:-}"
            shift 2
            ;;
        --name)
            UNIT_NAME="${2:-}"
            shift 2
            ;;
        -h|--help)
            cat <<'EOF'
用法: install-service.sh [--user <name>] [--name <unit-name>]

参数:
  --user <name>   systemd 服务运行用户（默认：SUDO_USER 或当前调用者）
  --name <name>   unit 基名（默认：frp-easy）
  -h, --help      显示本帮助

示例:
  sudo ./scripts/install-service.sh
  sudo ./scripts/install-service.sh --user nobody
EOF
            exit 0
            ;;
        *)
            echo "错误：未识别的参数 $1，运行 ./install-service.sh --help 查看用法" >&2
            exit 1
            ;;
    esac
done

# 默认运行用户：优先 SUDO_USER（sudo 调用时为真实用户），否则 id -un。
if [[ -z "$RUN_USER" ]]; then
    if [[ -n "${SUDO_USER:-}" ]]; then
        RUN_USER="$SUDO_USER"
    else
        RUN_USER="$(id -un)"
    fi
fi

# 前置 1：root 权限
if [[ "$(id -u)" -ne 0 ]]; then
    echo "错误：请以 root / sudo 运行本脚本（systemd unit 写入 /etc/systemd/system/ 需 root 权限）。" >&2
    exit 1
fi

# 前置 2：systemctl 存在
if ! command -v systemctl >/dev/null 2>&1; then
    echo "错误：未检测到 systemd（如 WSL1 / OpenRC / 极简容器），无法安装为系统服务。" >&2
    echo "      请改用前台运行：./frp-easy" >&2
    exit 1
fi

# 前置 3：解析解压目录绝对路径（脚本所在目录的上一级 = 解压目录顶层）
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
if command -v realpath >/dev/null 2>&1; then
    INSTALL_DIR="$(realpath "$SCRIPT_DIR/..")"
else
    INSTALL_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
fi

BINARY="$INSTALL_DIR/frp-easy"
if [[ ! -x "$BINARY" || ! -s "$BINARY" ]]; then
    echo "错误：未找到可执行的 $BINARY；请确认已解压发布包并保留目录结构。" >&2
    exit 1
fi

# 前置 4：运行用户存在性校验
if ! getent passwd "$RUN_USER" >/dev/null 2>&1; then
    echo "错误：用户 $RUN_USER 不存在，请先 useradd 或换一个 --user 参数。" >&2
    exit 1
fi

UNIT_PATH="/etc/systemd/system/${UNIT_NAME}.service"
TMP_UNIT="/etc/systemd/system/.${UNIT_NAME}.service.tmp"

# MINOR-R1：trap 必须在 TMP_UNIT 赋值之后才设置，避免 trap 到空变量。
# 正常路径会 mv 走该 tmp 文件，cleanup 时 rm -f 已不存在的路径不会失败。
trap 'rm -f "$TMP_UNIT"' EXIT

EXISTED="no"
if [[ -f "$UNIT_PATH" ]]; then
    EXISTED="yes"
    echo "==> 检测到已存在的 unit：$UNIT_PATH（将刷新并重启服务）"
    systemctl stop "$UNIT_NAME" >/dev/null 2>&1 || true
fi

# 原子写 unit 文件（参考 .harness/insight-index.md 2026-05-19 AtomicWrite 双重 chmod 模式）：
#   1) 先写 tmp 文件 → chmod 0644
#   2) mv -f 到目标路径（Linux POSIX rename 是原子的）
#   3) 再次 chmod 0644 防 umask 让最终权限变宽
cat > "$TMP_UNIT" <<EOF
[Unit]
Description=FRP Easy — frp 可视化管理 UI
After=network.target
Documentation=https://github.com/Alan-IFT/frp_easy

[Service]
Type=simple
ExecStart="${BINARY}"
WorkingDirectory="${INSTALL_DIR}"
User=${RUN_USER}
Restart=on-failure
RestartSec=5
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
EOF

chmod 0644 "$TMP_UNIT"
mv -f "$TMP_UNIT" "$UNIT_PATH"
chmod 0644 "$UNIT_PATH"

echo "==> 写入 unit 文件：$UNIT_PATH"

if ! systemctl daemon-reload; then
    echo "错误：systemctl daemon-reload 失败。" >&2
    exit 2
fi

if ! systemctl enable --now "$UNIT_NAME"; then
    echo "错误：systemctl enable --now $UNIT_NAME 失败；请查看 journalctl -u $UNIT_NAME。" >&2
    exit 2
fi

if [[ "$EXISTED" == "yes" ]]; then
    echo "==> 已刷新现有 unit 并重启服务"
else
    echo "==> 已新建 unit 并启动服务"
fi

cat <<EOF

服务已就绪。常用命令：
  systemctl status ${UNIT_NAME}        # 查看状态
  systemctl is-active ${UNIT_NAME}     # 仅查 active 状态
  journalctl -u ${UNIT_NAME} -f        # 实时日志（stderr）
  sudo "$SCRIPT_DIR/uninstall-service.sh" # 卸载

数据目录（保留）：
  ${INSTALL_DIR}/.frp_easy/
  ${INSTALL_DIR}/frp_easy.toml （首启后自动生成）

注意：unit 中的 ExecStart 与 WorkingDirectory 是绝对路径；安装服务后请勿移动 ${INSTALL_DIR}。
EOF

exit 0
