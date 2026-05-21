#!/usr/bin/env bash
# start-e2e-server.sh — 为 Playwright E2E 测试启动 frp-easy 后端。
# 使用独立临时数据目录（FRP_EASY_CONFIG 环境变量注入），保证测试数据隔离。
# 通过 exec 替换 shell 进程，使 Playwright webServer 可直接管理 frp-easy 的生命周期。
#
# 注意：本脚本需要 bash 环境（Git Bash 或 WSL），不支持 Windows cmd / PowerShell 直接执行。

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
BIN_BASE="$ROOT/bin/frp-easy"
DIST_DIR="$ROOT/internal/assets/dist"

# 1. 判断是否需要重新构建：
#    - bin/frp-easy(.exe) 不存在；或
#    - dist/ 中有比二进制更新的文件（前端已重新构建）。
needs_rebuild() {
  local bin="$1"
  [[ ! -f "$bin" ]] && return 0
  # 若 dist/ 中有任何文件比 bin 新，则需要重建。
  [[ -d "$DIST_DIR" ]] && [[ -n "$(find "$DIST_DIR" -newer "$bin" -type f 2>/dev/null | head -1)" ]] && return 0
  return 1
}

# 确定二进制路径（跨平台）
BIN="$BIN_BASE"
if [[ -f "${BIN_BASE}.exe" ]]; then
  BIN="${BIN_BASE}.exe"
fi

if needs_rebuild "$BIN"; then
  echo "[e2e-server] building binary (dist/ changed or binary missing)..." >&2
  cd "$ROOT"
  CGO_ENABLED=0 go build -o "bin/frp-easy" ./cmd/frp-easy
  BIN="$BIN_BASE"
  # Windows: go build 产生 .exe
  if [[ ! -f "$BIN" && -f "${BIN}.exe" ]]; then
    BIN="${BIN}.exe"
  fi
  echo "[e2e-server] build done" >&2
else
  echo "[e2e-server] binary up-to-date, skipping build" >&2
fi

# 2. 创建临时数据目录和配置文件
# 使用 E2E_TMP 而非 TMPDIR，避免覆盖系统环境变量
E2E_TMP=$(mktemp -d)
echo "[e2e-server] using E2E_TMP: $E2E_TMP" >&2

cat > "$E2E_TMP/frp_easy.toml" <<EOF
UIBindAddr = "127.0.0.1"
UIPort     = 7800
DataDir    = "$E2E_TMP/data"
LogDir     = "$E2E_TMP/logs"
EOF

# 3. exec 替换 shell 进程，Playwright 通过 SIGTERM/SIGKILL 管理 frp-easy 生命周期
export FRP_EASY_CONFIG="$E2E_TMP/frp_easy.toml"
exec "$BIN"
