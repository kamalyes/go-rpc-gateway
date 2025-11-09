# 多阶段构建 Dockerfile for Go RPC Gateway
# 优化镜像大小和安全性

# ===========================================
# 构建阶段
# ===========================================
FROM golang:1.21-alpine AS builder

# 设置工作目录
WORKDIR /app

# 安装构建依赖
RUN apk add --no-cache git ca-certificates tzdata

# 设置Go环境变量
ENV GO111MODULE=on \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

# 复制 go mod 文件
COPY go.mod go.sum ./

# 下载依赖
RUN go mod download

# 复制源代码
COPY . .

# 构建应用
RUN go build \
    -ldflags='-w -s -extldflags "-static"' \
    -a -installsuffix cgo \
    -o gateway cmd/gateway/main.go

# ===========================================
# 运行阶段
# ===========================================
FROM alpine:latest

# 安装必要的包
RUN apk --no-cache add ca-certificates tzdata curl

# 设置时区
ENV TZ=Asia/Shanghai

# 创建应用用户
RUN addgroup -g 1001 app && \
    adduser -u 1001 -G app -s /bin/sh -D app

# 设置工作目录
WORKDIR /app

# 从构建阶段复制二进制文件
COPY --from=builder /app/gateway .

# 复制配置文件
COPY --from=builder /app/examples/06-enterprise/config.yaml ./config.yaml

# 创建必要的目录
RUN mkdir -p logs data && \
    chown -R app:app /app

# 切换到应用用户
USER app

# 暴露端口
EXPOSE 8080 9090

# 健康检查
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:8080/health || exit 1

# 启动应用
CMD ["./gateway", "-config", "config.yaml"]