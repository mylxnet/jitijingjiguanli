# 多阶段构建：前端 → Go 静态编译 → 最小 Alpine 运行
# 用法:
#   docker build -f deploy/Dockerfile -t jititaizhang:latest .
#   docker run -d -p 8080:8080 -v ./data:/data -v ./backups:/backups jititaizhang:latest

# ===== Stage 1: 前端构建 =====
FROM node:20-alpine AS frontend
WORKDIR /build/web
COPY web/package.json web/package-lock.json ./
RUN npm install --no-audit --no-fund
COPY web/ .
RUN npm run build

# ===== Stage 2: Go 静态编译 =====
FROM golang:alpine AS builder
WORKDIR /build
# 先拷依赖文件，利用 Docker 缓存
COPY server/go.mod server/go.sum ./server/
RUN cd server && go mod download
# 复制前端产物到 embed 目录
COPY --from=frontend /build/web/dist ./server/cmd/server/web/
# 复制 Go 源码
COPY server/ ./server/
# 纯 Go SQLite（modernc），CGO_ENABLED=0 可安全关闭
RUN cd server && CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /jititaizhang ./cmd/server

# ===== Stage 3: 运行 =====
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