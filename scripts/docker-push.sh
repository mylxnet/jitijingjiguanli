#!/usr/bin/env bash
# 集体台账 · 构建 & 推送 Docker Hub 镜像
# 用法：
#   1. 先在 Docker Hub 创建仓库（如 jititaizhang）
#   2. 登录：docker login
#   3. 运行本脚本：bash scripts/docker-push.sh
#
# 前置条件：已安装 Docker，项目根目录下有 web/ 和 server/ 源码

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

# ===== 配置（改这里） =====
DOCKER_USER="你的用户名"
IMAGE_NAME="jititaizhang"
VERSION="0.10.0"

# ===== 构建 =====
echo "== [1/3] 构建镜像 =="
docker build -f deploy/Dockerfile.local \
  -t "${DOCKER_USER}/${IMAGE_NAME}:${VERSION}" \
  -t "${DOCKER_USER}/${IMAGE_NAME}:latest" \
  .

# ===== 推送 =====
echo "== [2/3] 推送镜像到 Docker Hub =="
docker push "${DOCKER_USER}/${IMAGE_NAME}:${VERSION}"
docker push "${DOCKER_USER}/${IMAGE_NAME}:latest"

echo "== [3/3] 完成 =="
echo "镜像: ${DOCKER_USER}/${IMAGE_NAME}:${VERSION}"
echo "在 NAS 上执行:"
echo "  docker compose -f deploy/docker-compose.hub.yml pull"
echo "  docker compose -f deploy/docker-compose.hub.yml up -d"