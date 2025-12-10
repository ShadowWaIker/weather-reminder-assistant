# 多阶段构建 Dockerfile
# 第一阶段：构建阶段
FROM golang:1.20-alpine AS builder

# 设置工作目录
WORKDIR /app

# 配置 apk 镜像源加速构建
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.ustc.edu.cn/g' /etc/apk/repositories && \
    echo "https://mirrors.ustc.edu.cn/alpine/v3.18/main" >> /etc/apk/repositories && \
    echo "https://mirrors.ustc.edu.cn/alpine/v3.18/community" >> /etc/apk/repositories

# 安装必要的工具
RUN apk add --no-cache git ca-certificates tzdata

# 复制 go mod 文件（如果存在）
COPY go.mod go.sum ./

# 复制 vendor 目录（vendor 模式）
COPY vendor/ ./vendor/

# 复制源代码
COPY main.go ./

# 设置 Go 环境变量
ENV CGO_ENABLED=0
ENV GOOS=linux
ENV GOARCH=amd64
ENV GOPROXY=https://goproxy.cn,direct

# 编译应用程序
# 使用 -mod=vendor 使用 vendor 目录中的依赖
RUN go build -mod=vendor -ldflags="-w -s" -o weather-reminder main.go

# 第二阶段：运行时阶段
FROM alpine:3.18

# 安装 CA 证书和时区数据
RUN apk add --no-cache ca-certificates tzdata

# 创建非 root 用户
RUN addgroup -g 1000 weather && \
    adduser -D -s /bin/sh -u 1000 -G weather weather

# 设置工作目录
WORKDIR /app

# 从构建阶段复制编译后的二进制文件
COPY --from=builder /app/weather-reminder .

# 复制配置文件模板
COPY config.yaml.example ./

# 更改文件所有权
RUN chown -R weather:weather /app

# 切换到非 root 用户
USER weather

# 暴露端口（如果应用需要）
# EXPOSE 8080

# 健康检查
HEALTHCHECK --interval=30s --timeout=15s --start-period=10s --retries=3 \
    CMD ./weather-reminder --healthcheck > /dev/null 2>&1 || exit 1

# 设置入口点
ENTRYPOINT ["./weather-reminder"]

# 默认参数
# CMD ["--help"]