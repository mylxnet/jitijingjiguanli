#!/usr/bin/env bash
# ============================================================
# 集体台账 - 飞牛 NAS 一键启动/管理脚本
#
# 适用：飞牛 NAS (fnOS) 自带 Docker
#
# 用法：
#   ./fnos.sh start       首次部署并启动（自动加载镜像）
#   ./fnos.sh start       镜像已加载时重新启动
#   ./fnos.sh stop        停止容器
#   ./fnos.sh restart     重启容器
#   ./fnos.sh status      查看运行状态
#   ./fnos.sh logs        查看日志
#   ./fnos.sh remove      卸载容器（保留数据）
#   ./fnos.sh help        显示帮助
#
# 访问地址：http://<NAS的IP>:8080
# ============================================================

set -euo pipefail

# ---------- 可配置项 ----------
# 镜像文件（与脚本放在同一目录）
IMAGE_FILE="$(dirname "$(readlink -f "$0")")/jititaizhang-docker.tar"
# 镜像名与标签
IMAGE="jititaizhang:latest"
# 容器名
CONTAINER="jititaizhang"
# 对外访问端口（如被占用可再换）
HOST_PORT="1133"
# 数据目录（宿主机路径，务必指向 NAS 存储池，避免容器重启数据丢失）
DATA_DIR="/vol1/docker/jititaizhang/data"
BACKUP_DIR="/vol1/docker/jititaizhang/backups"

# ---------- 配色 ----------
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'
ok()  { echo -e "${GREEN}[OK]${NC} $*"; }
warn(){ echo -e "${YELLOW}[!]${NC} $*"; }
err() { echo -e "${RED}[X]${NC} $*" >&2; }

# ---------- 工具函数 ----------
# 检查 Docker
check_docker() {
  if ! command -v docker >/dev/null 2>&1; then
    err "未检测到 Docker！请在飞牛 NAS 应用中安装并打开 Docker 后重试。"
    exit 1
  fi
  if ! docker info >/dev/null 2>&1; then
    err "Docker 守护进程未运行，请检查飞牛 Docker 服务。"
    exit 1
  fi
}

# 判断镜像是否已存在
image_exists() {
  docker image inspect "$IMAGE" >/dev/null 2>&1
}

# 加载镜像（若文件存在且镜像未加载）
load_image() {
  if image_exists; then
    ok "镜像 $IMAGE 已存在，跳过加载。"
    return 0
  fi
  if [ ! -f "$IMAGE_FILE" ]; then
    warn "未找到镜像文件：$IMAGE_FILE"
    warn "请把 jititaizhang-docker.tar 放到与脚本相同的目录后重试。"
    exit 1
  fi
  echo "正在加载镜像 $IMAGE_FILE ..."
  docker load -i "$IMAGE_FILE"
  ok "镜像加载完成。"
}

# 创建数据目录
prepare_dirs() {
  mkdir -p "$DATA_DIR" "$BACKUP_DIR"
  chmod 755 "$DATA_DIR" "$BACKUP_DIR"
  ok "数据目录就绪：$DATA_DIR"
  ok "备份目录就绪：$BACKUP_DIR"
}

# 启动容器
start_container() {
  if [ "$(docker ps -q -f name="^${CONTAINER}$")" ]; then
    warn "容器 $CONTAINER 已在运行。"
    return 0
  fi
  # 存在但已停止 → 直接启动
  if [ "$(docker ps -aq -f name="^${CONTAINER}$")" ]; then
    echo "启动已存在的容器 $CONTAINER ..."
    docker start "$CONTAINER"
    ok "已启动。"
    return 0
  fi

  echo "创建并启动容器 $CONTAINER ..."
  docker run -d \
    --name "$CONTAINER" \
    --restart unless-stopped \
    -p "${HOST_PORT}:8080" \
    -v "${DATA_DIR}:/data" \
    -v "${BACKUP_DIR}:/backups" \
    -e APP_PORT=8080 \
    -e DATA_DIR=/data \
    -e APP_BACKUP_DIR=/backups \
    -e APP_KEEP_BACKUP=30 \
    -e APP_AUTO_BACKUP=1 \
    "$IMAGE"
  ok "容器已启动。"
}

# 等待健康检查
wait_ready() {
  echo -n "等待服务就绪"
  for _ in $(seq 1 30); do
    if curl -sf "http://127.0.0.1:${HOST_PORT}/api/health" >/dev/null 2>&1; then
      echo ""
      ok "服务就绪！访问地址：http://<NAS的IP>:${HOST_PORT}"
      return 0
    fi
    echo -n "."
    sleep 2
  done
  echo ""
  warn "服务启动较慢，可通过 ${GREEN}./fnos.sh logs${NC} 查看日志。"
}

# ---------- 主命令 ----------
ACTION="${1:-help}"
check_docker

case "$ACTION" in
  start)
    load_image
    prepare_dirs
    start_container
    wait_ready
    ;;
  stop)
    docker stop "$CONTAINER" && ok "容器已停止。" || warn "容器未在运行。"
    ;;
  restart)
    docker restart "$CONTAINER"
    ok "已重启。"
    wait_ready
    ;;
  status)
    if [ "$(docker ps -q -f name="^${CONTAINER}$")" ]; then
      ok "容器运行中"
      docker ps --filter "name=^${CONTAINER}$" --format "  端口: {{.Ports}}  状态: {{.Status}}"
    elif [ "$(docker ps -aq -f name="^${CONTAINER}$")" ]; then
      warn "容器已创建但已停止（可用 ./fnos.sh start 启动）"
    else
      warn "容器尚未创建（用 ./fnos.sh start 首次部署）"
    fi
    ;;
  logs)
    docker logs -f "$CONTAINER"
    ;;
  remove)
    docker rm -f "$CONTAINER" 2>/dev/null || true
    ok "容器已移除（数据保留在 $DATA_DIR）。"
    ;;
  help|*)
    sed -n '2,24p' "$0" | sed 's/^# \{0,1\}//'
    ;;
esac