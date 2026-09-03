#!/usr/bin/env bash
# 一键构建单可执行：web 构建 -> 复制到 server/cmd/server/web -> go build（embed 前端）
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"

echo "== [1/3] 前端构建 =="
cd "$ROOT/web"
npm install --no-audit --no-fund
npm run build

echo "== [2/3] 复制 dist 到 embed 目录 =="
mkdir -p "$ROOT/server/cmd/server/web"
cp -r "$ROOT/web/dist/." "$ROOT/server/cmd/server/web/"

echo "== [3/3] Go 编译 =="
cd "$ROOT/server"
mkdir -p bin
go build -o bin/jititaizhang.exe ./cmd/server

echo "完成：$ROOT/server/bin/jititaizhang.exe"
