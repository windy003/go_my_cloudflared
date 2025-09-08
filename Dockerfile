# 使用官方Go镜像
FROM golang:1.21-alpine

# 安装必要的包
RUN apk add --no-cache git wget

# 创建非root用户
RUN addgroup -g 1001 appgroup && \
    adduser -D -u 1001 -G appgroup appuser

# 设置工作目录
WORKDIR /app

# 复制go mod文件
COPY go/go.mod go/go.sum ./

# 下载依赖
RUN go mod download

# 复制源代码
COPY go/ ./


# 设置文件权限
RUN chown -R appuser:appgroup /app

# 切换到非root用户
USER appuser

# 暴露端口
EXPOSE 6000 6001 6443 6444

# 健康检查
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:6000/health || exit 1

# 直接运行Go代码，指定配置文件
CMD ["go", "run", "./cmd/server/go_my_cloudflared_server.go", "start", "--config", "./config/server.yml"]