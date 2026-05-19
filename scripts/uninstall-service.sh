#!/usr/bin/env bash
# uninstall-service.sh — frp_easy Linux systemd 服务卸载脚本
#
# 用途：停止并移除 systemd unit（/etc/systemd/system/frp-easy.service），daemon-reload。
#       **不删除** frp_easy.toml 与 .frp_easy/ 数据目录（NFR-7、AC-9 要求）。
# 用法：sudo ./scripts/uninstall-service.sh [--name <unit-name>]
# 参数：--name  unit 基名（默认 frp-easy）
#       -h | --help  显示本帮助
# 输出：stdout 中文进度；卸载完成时打印数据目录保留提示
# 退出码：0 成功（含"服务从未安装"友好降级）/ 1 权限或文件操作失败

set -euo pipefail

UNIT_NAME="frp-easy"

while [[ $# -gt 0 ]]; do
    case "$1" in
        --name)
            UNIT_NAME="${2:-}"
            shift 2
            ;;
        -h|--help)
            cat <<'EOF'
用法: uninstall-service.sh [--name <unit-name>]

参数:
  --name <name>   unit 基名（默认：frp-easy）
  -h, --help      显示本帮助

说明:
  本脚本只停止并移除 systemd unit；不会删除 frp_easy.toml 与 .frp_easy/ 数据目录。
EOF
            exit 0
            ;;
        *)
            echo "错误：未识别的参数 $1，运行 ./uninstall-service.sh --help 查看用法" >&2
            exit 1
            ;;
    esac
done

# 前置：root 权限
if [[ "$(id -u)" -ne 0 ]]; then
    echo "错误：请以 root / sudo 运行本脚本（移除 /etc/systemd/system/ 下 unit 需 root 权限）。" >&2
    exit 1
fi

UNIT_PATH="/etc/systemd/system/${UNIT_NAME}.service"

if [[ ! -f "$UNIT_PATH" ]]; then
    echo "未检测到已安装的 ${UNIT_NAME} 服务（${UNIT_PATH} 不存在）。"
    echo "如果此前从未通过 install-service.sh 安装，请忽略本提示。"
    exit 0
fi

if command -v systemctl >/dev/null 2>&1; then
    systemctl disable --now "$UNIT_NAME" >/dev/null 2>&1 || true
fi

if ! rm -f "$UNIT_PATH"; then
    echo "错误：无法删除 $UNIT_PATH（权限不足？）。" >&2
    exit 1
fi
echo "==> 已移除 unit 文件：$UNIT_PATH"

if command -v systemctl >/dev/null 2>&1; then
    systemctl daemon-reload || true
    systemctl reset-failed "$UNIT_NAME" >/dev/null 2>&1 || true
fi
echo "==> systemctl daemon-reload 完成"

# 解析解压目录路径，给用户一个明确的"如需彻底清理"指引
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
INSTALL_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

cat <<EOF

服务已卸载。

注意：数据目录与配置文件未删除：
  ${INSTALL_DIR}/frp_easy.toml
  ${INSTALL_DIR}/.frp_easy/

如需彻底清理，请手动执行：
  rm -rf "${INSTALL_DIR}/.frp_easy" "${INSTALL_DIR}/frp_easy.toml"
EOF

exit 0
