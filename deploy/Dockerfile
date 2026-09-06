# 多阶段构建：从 Gitee 克隆 → 前端构建 → Go 静态编译 → 最小 Alpine 运行
# 私有仓库通过 Docker BuildKit 密钥传递凭据
#
# 前置条件：
#   1. 在 Gitee 创建 Personal Access Token（设置 → 私人令牌）
#   2. 将 token 写入 deploy/gitee_token.txt（不含换行）
#   3. 启用 Docker BuildKit：export DOCKER_BUILDKIT=1（新版默认已开启）
#
# 构建：
#   docker compose build
# 或：
#   DOCKER_BUILDKIT=1 docker compose build

# ===== 语法指示：启用 BuildKit（docker-compose 会自动使用） =====
# syntax=docker/dockerfile:1

# ===== Stage 1: 从 Gitee 克隆源码 =====
FROM alpine/git:latest AS clone
WORKDIR /repo
# 通过 BuildKit 密钥注入 Gitee Personal Access Token
# 使用方式：在 deploy/ 目录创建 gitee_token.txt 文件，写入 token
# 此密钥不会留在镜像层中
RUN --mount=type=secret,id=gitee_token \
    GITEE_TOKEN=$(cat /run/secrets/gitee_token) && \
    git clone https://0930w:${GITEE_TOKEN}@gitee.com/0930w/jititaizhang.git /repo && \
    rm -rf /repo/.git

# ===== Stage 2: 前端构建 =====
FROM node:20-alpine AS frontend
WORKDIR /build/web
COPY --from=clone /repo/web/package.json /repo/web/package-lock.json ./
RUN npm install --no-audit --no-fund
COPY --from=clone /repo/web/ .
RUN npm run build

# ===== Stage 3: Go 静态编译 =====
FROM golang:alpine AS builder
WORKDIR /build
COPY --from=clone /repo/server/go.mod /repo/server/go.sum ./server/
RUN cd server && go mod download
COPY --from=frontend /build/web/dist ./server/cmd/server/web/
COPY --from=clone /repo/server/ ./server/
RUN cd server && CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /jititaizhang ./cmd/server

# ===== Stage 4: 运行 =====
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /jititaizhang /jititaizhang
EXPOSE 8080
VOLUME ["/data", "/backups"]
ENV APP_PORT=8080 \
    DATA_DIR=/data \
    APP_BACKUP_DIR=/backups \
    APP_KEEP_BACKUP=30 \
    APP_AUTO_BACKUP=1
ENTRYPOINT ["/jititaizhang"]