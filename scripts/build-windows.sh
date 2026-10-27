#!/usr/bin/env bash
# 构建 Windows 单文件可执行（内嵌前端 + SQLite，目标机零依赖）。
# 用法（WSL 内，项目根目录）：bash scripts/build-windows.sh
# 产物：dist/windows/jititaizhang.exe
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
OUT="$ROOT/dist/windows"
VERSION="$(sed -n "s/.*APP_VERSION = '\(.*\)'.*/\1/p" "$ROOT/web/src/version.ts")"

echo "== [1/4] 前端构建（版本 ${VERSION}）=="
cd "$ROOT/web"
npm install --no-audit --no-fund
npx vite build

echo "== [2/4] 复制 dist 到 embed 目录（先清空，避免旧 chunk 堆积）=="
rm -rf "$ROOT/server/cmd/server/web"
mkdir -p "$ROOT/server/cmd/server/web"
cp -r "$ROOT/web/dist/." "$ROOT/server/cmd/server/web/"

echo "== [3/4] 生成 Windows 资源（应用图标 + 版本信息）=="
cd "$ROOT/server"
go run github.com/tc-hib/go-winres@latest make --in cmd/server/winres/winres.json --arch amd64 --out cmd/server/rsrc

echo "== [4/4] 交叉编译 Windows amd64 =="
mkdir -p "$OUT"
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 \
  go build -trimpath -ldflags="-s -w" -o "$OUT/jititaizhang.exe" ./cmd/server

ls -lh "$OUT/jititaizhang.exe"
echo "完成：$OUT/jititaizhang.exe"
