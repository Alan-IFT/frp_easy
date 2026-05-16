#!/usr/bin/env bash
# start.sh — frp_easy 开发模式启动脚本（Linux/macOS）
#
# 开发模式：Go API (port 8080) + Vite dev (port 5173) 独立运行，
# Go 侧以 DevMode=true 开 CORS 允许 vite 代理。
#
# 用法：./scripts/start.sh
#       ./scripts/start.sh --prod   # 生产模式（单二进制，不启动 vite）

set -uo pipefail

PROD=false
for arg in "$@"; do
    case $arg in --prod) PROD=true ;; esac
done

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

if $PROD; then
    BIN="$ROOT/bin/frp-easy"
    if [[ ! -f "$BIN" ]]; then
        echo "未找到 bin/frp-easy，先编译..."
        bash "$ROOT/scripts/build.sh"
    fi
    echo "启动 frp_easy（生产）..."
    exec "$BIN"
fi

# 开发模式
PIDS=()

cleanup() {
    echo ""
    echo "正在停止..."
    for pid in "${PIDS[@]:-}"; do
        kill "$pid" 2>/dev/null || true
    done
    wait 2>/dev/null || true
}
trap cleanup INT TERM

# Go API
echo "启动 Go API (dev mode)..."
go run ./cmd/frp-easy &
PIDS+=($!)

# Vite dev（若 web/ 目录存在）
if [[ -f "$ROOT/web/package.json" ]]; then
    echo "启动 Vite dev server..."
    (cd "$ROOT/web" && npm run dev) &
    PIDS+=($!)
else
    echo "web/ 目录不存在，跳过 Vite（仅运行 Go API）"
fi

echo "开发服务已启动。Ctrl+C 停止。"
wait
