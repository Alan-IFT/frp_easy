#!/usr/bin/env bash
# build.sh — frp_easy 生产构建脚本（Linux/macOS）
#
# 输出：bin/frp-easy（Linux x64）和（可选）bin/frp-easy.exe（Windows x64，交叉编译）
#
# 用法：./scripts/build.sh
#       ./scripts/build.sh --all    # 同时交叉编译 Windows 版本

set -uo pipefail

ALL=false
for arg in "$@"; do
    case $arg in --all) ALL=true ;; esac
done

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS="-X main.Version=${VERSION} -s -w"

# 前端构建（若 web/ 存在且有 package.json）
if [[ -f "$ROOT/web/package.json" ]]; then
    echo "构建前端（npm run build）..."
    # npm ci：CI 专用、严格按 package-lock.json 安装、绝不改写 lockfile。
    # 不用 `npm install --frozen-lockfile`——`--frozen-lockfile` 是 Yarn 的 flag，
    # npm 不识别会静默忽略、退化成普通 npm install，在 CI 冷环境从零安装时
    # 可能改写 package-lock.json，使 `git describe --dirty` 把发布产物标成 -dirty。
    (cd "$ROOT/web" && npm ci >/dev/null && npm run build)
    echo "前端构建完成"
fi

mkdir -p "$ROOT/bin"

# Linux
echo "编译 bin/frp-easy ..."
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "$LDFLAGS" -o "$ROOT/bin/frp-easy" ./cmd/frp-easy
echo "bin/frp-easy 构建完成"

if $ALL; then
    echo "交叉编译 bin/frp-easy.exe ..."
    CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags "$LDFLAGS" -o "$ROOT/bin/frp-easy.exe" ./cmd/frp-easy
    echo "bin/frp-easy.exe 构建完成"
fi

echo "构建完成 ✓"
