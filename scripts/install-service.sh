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

# B.2.1（02 §B.2.1 + T-016 D-1 fix）：systemd.exec(5) 规定路径含特殊字符必须 C-style
# 转义；整体双引号 `WorkingDirectory="/foo bar"` 被 systemd 任意版本拒为 bad unit
# file setting（T-008 旧 insight 反例的真因）。OOS-2 限定仅支持 ASCII + 空格，所以
# 单字符替换即足够：把空格替换成字面 4 字符 `\x20`（反斜杠 + x + 2 + 0）。
#
# bash 5.x 双引号 + parameter expansion 的 quote-removal 陷阱：旧实现
# `printf '%s' "${p// /\\x20}"` 在 bash 5.2 实测产出 `frpx20easy`（反斜杠缺失）——
# 双引号内 `\\` 先被 quote-removal 还原为单个 `\`，再被 expansion 的 string 解析
# 吞掉，REPLACEMENT 退化为 `x20`。
# 修复：用单引号字面赋值 `\x20` 到变量 esc，参数扩展引用 `$esc` 时不再做反斜杠
# 脱壳，replacement 保留字面 4 字符 `\x20`（hex `5c 78 32 30`）。
systemd_escape_path() {
    local p="$1"
    local esc='\x20'
    printf '%s' "${p// /$esc}"
}

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

[boot-autostart-fix] 本脚本会注册 systemd system-level 服务（/etc/systemd/system/），
开机即起，不依赖任何用户登录。unit 含 Wants=network-online.target 让 frpc 等到
网络在线再启，配合 frp_easy.toml 渲染层的 loginFailExit=false 与 frp-easy 进程
的 autoRestoreProcs 指数 backoff，让"reboot 后远程连接立即恢复"成为硬保证。
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

# B 组（02 §B.2）：ExecStart= / WorkingDirectory= 改为裸 token + `\x20` 转义；
# 不再用整体双引号（systemd 拒收，T-008 旧 insight 已纠正）。
ESC_BINARY="$(systemd_escape_path "$BINARY")"
ESC_INSTALL_DIR="$(systemd_escape_path "$INSTALL_DIR")"

# 原子写 unit 文件（参考 .harness/insight-index.md 2026-05-19 AtomicWrite 双重 chmod 模式）：
#   1) 先写 tmp 文件 → chmod 0644
#   2) mv -f 到目标路径（Linux POSIX rename 是原子的）
#   3) 再次 chmod 0644 防 umask 让最终权限变宽
cat > "$TMP_UNIT" <<EOF
[Unit]
Description=FRP Easy — frp 可视化管理 UI
Documentation=https://github.com/Alan-IFT/frp_easy
# T-038 [boot-autostart-fix]：用 network-online.target 让 frp-easy 等到网络真正在线再启。
# 旧版用 network.target 仅表示"网络配置已下发"，不等于"路由可达"——会让 frpc 在 boot
# 时拿到 connect: network is unreachable 立即 exit（配合 loginFailExit=false 已无此问题，
# 但仍把 systemd 依赖修对作为多层防御）。NetworkManager-wait-online.service 或
# systemd-networkd-wait-online.service 任一 enabled 即可 gating。
Wants=network-online.target
After=network-online.target

[Service]
Type=simple
ExecStart=${ESC_BINARY}
WorkingDirectory=${ESC_INSTALL_DIR}
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

# B.3 + GR §F-3 降级路径：systemd-analyze verify 主动自检。
# 设计原方案对 verify 失败采取 "exit 2 + rm unit"，但 GR §F-3 提出 systemd-analyze
# 在 systemd 249-255 历史上存在误报 fatal 的情况（如对不可达 Documentation= URL）。
# 降级为 "warn + 继续" —— 让 daemon-reload / enable--now 作最终事实源；若它们也
# 失败将由下方 C.2 强诊断块详述根因。
if command -v systemd-analyze >/dev/null 2>&1; then
    verify_out="$(systemd-analyze verify "$UNIT_PATH" 2>&1)" && verify_rc=0 || verify_rc=$?
    if [[ "$verify_rc" -ne 0 ]]; then
        echo "警告：systemd-analyze verify 报告问题（退出码 $verify_rc）：" >&2
        printf '%s\n' "$verify_out" | sed 's/^/    /' >&2
        echo "    继续 daemon-reload 让 systemd 自己判定（若 reload/enable 失败将由下方诊断块详述）。" >&2
    fi
fi

# C 组（02 §C.2 + GR §F-1 diag 打印）：daemon-reload 改为 set +e/-e 包裹 + diag 打印。
set +e
systemctl daemon-reload
reload_rc=$?
set -e
[[ $reload_rc -ne 0 ]] && echo "    [diag] systemctl daemon-reload rc=$reload_rc" >&2
if [[ $reload_rc -ne 0 ]]; then
    echo "错误：systemctl daemon-reload 失败（退出码 $reload_rc）。" >&2
    echo "      unit 文件已写入：$UNIT_PATH（可 cat 审阅）。" >&2
    echo "      如需清理：sudo rm -f $UNIT_PATH && sudo systemctl daemon-reload" >&2
    exit 2
fi

# C 组（02 §C.2 + GR §F-1 / §F-2 字面前缀锚点）：enable--now 失败块扩展为
# 自动打印 status + journalctl 摘要 + unit 路径 + 清理提示。
# 字面前缀严格使用以下两条 AC-8 grep 锚点：
#   "==== 诊断信息：systemctl status $UNIT_NAME --no-pager ===="
#   "==== 诊断信息：journalctl -u $UNIT_NAME --no-pager -n 20 ===="
set +e
systemctl enable --now "$UNIT_NAME"
enable_rc=$?
set -e
[[ $enable_rc -ne 0 ]] && echo "    [diag] systemctl enable --now $UNIT_NAME rc=$enable_rc" >&2

if [[ $enable_rc -ne 0 ]]; then
    echo "错误：systemctl enable --now $UNIT_NAME 失败（退出码 $enable_rc）。" >&2
    echo "" >&2
    echo "==== 诊断信息：systemctl status $UNIT_NAME --no-pager ====" >&2
    systemctl status "$UNIT_NAME" --no-pager 2>&1 | sed 's/^/    /' >&2 || true
    echo "" >&2
    echo "==== 诊断信息：journalctl -u $UNIT_NAME --no-pager -n 20 ====" >&2
    journalctl -u "$UNIT_NAME" --no-pager -n 20 2>&1 | sed 's/^/    /' >&2 || true
    echo "" >&2
    echo "unit 文件已写入：$UNIT_PATH（可 cat 审阅）。" >&2
    echo "如需清理：sudo systemctl disable $UNIT_NAME && sudo rm -f $UNIT_PATH && sudo systemctl daemon-reload" >&2
    exit 2
fi

if [[ "$EXISTED" == "yes" ]]; then
    echo "==> 已刷新现有 unit 并重启服务"
else
    echo "==> 已新建 unit 并启动服务"
fi

# T-038 [boot-autostart-fix] 自检：确认 unit 已 active + enabled 才算装好。
# 失败时打印诊断 + exit 4（install.sh 透传同款码值；GR §5 C-1 / Q-4 决策）。
# 用 5 次 1s 轮询（与 install-service.ps1 Wait-ServiceRunning idiom 同款），
# 比裸 sleep 1 更稳：systemd active 状态推进有时 < 1s 有时 ~3s。
echo "==> [boot-autostart-fix] 自检：systemctl is-active + is-enabled..."
for i in 1 2 3 4 5; do
    if systemctl is-active --quiet "$UNIT_NAME"; then break; fi
    sleep 1
done
if ! systemctl is-active --quiet "$UNIT_NAME"; then
    echo "错误：[boot-autostart-fix self-check FAIL] $UNIT_NAME 未进入 active 状态。" >&2
    systemctl status "$UNIT_NAME" --no-pager -l 2>&1 | sed 's/^/    /' >&2
    exit 4
fi
if ! systemctl is-enabled --quiet "$UNIT_NAME"; then
    echo "错误：[boot-autostart-fix self-check FAIL] $UNIT_NAME 未 enabled（不会开机自启）。" >&2
    exit 4
fi
echo "==> [boot-autostart-fix] 自检通过：$UNIT_NAME 已 active + enabled"

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
