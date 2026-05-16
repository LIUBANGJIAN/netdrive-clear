# NetDrive Clear Dockerfile
# 支持 x86_64 和 ARM 架构

# 多阶段构建：第一阶段用于编译
FROM --platform=${BUILDPLATFORM:-linux/amd64} golang:1.23-alpine AS builder

# 设置工作目录
WORKDIR /app

# 安装依赖工具
RUN apk add --no-cache git

# 复制 go 模块文件
COPY go.mod go.sum ./

# 下载依赖
RUN go mod download

# 复制源代码
COPY . .

# 编译应用
# 使用 CGO_ENABLED=0 禁用 CGO，生成静态二进制文件
# 使用 BUILDPLATFORM 和 TARGETPLATFORM 支持多架构
ARG TARGETOS
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -o netdrive-clear .

# 第二阶段：运行阶段
FROM alpine:3.19

# 安装 CA 证书（用于 HTTPS 连接）
RUN apk add --no-cache ca-certificates

# 创建工作目录
WORKDIR /app

# 创建数据目录（用于配置文件和日志）
RUN mkdir -p /app/data

# 从构建阶段复制二进制文件
COPY --from=builder /app/netdrive-clear .

# 复制 Web 静态文件
COPY --from=builder /app/web ./web

# 暴露端口
EXPOSE 8080

# 设置环境变量
ENV CONFIG_PATH=/app/data/config.json
ENV TZ=Asia/Shanghai

# 运行应用（通过环境变量自动检测配置文件，不使用命令行参数）
CMD ["./netdrive-clear"]
