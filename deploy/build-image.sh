#!/usr/bin/env bash
# ============================================================
# 集体台账 - WSL 本地打包 Docker 镜像脚本
#
# 用法（在 WSL 或 Windows 终端均可运行）：
#   chmod +x build-image.sh
#   ./build-image.sh
#
# 产物：jititaizhang-docker.tar
# 说明：本脚本请在项目根目录运行
# ============================================================

set -euo pipefail

# 镜像名
IMAGE="jititaizhang:latest"
# 输出 tar 文件名
OUTPUT="jititaizhang-docker.tar"

echo "1/2 构建镜像 ${IMAGE} ..."
docker build -f deploy/Dockerfile.local -t "${IMAGE}" .

echo "2/2 打包镜像到 ${OUTPUT} ..."
docker save "${IMAGE}" -o "${OUTPUT}"

echo "完成！镜像文件：$(pwd)/${OUTPUT}"
echo "部署：将 tar 拷到 NAS，执行 docker load -i ${OUTPUT} 后 docker compose -f deploy/docker-compose.nas.yml up -d"