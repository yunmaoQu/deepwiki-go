FROM golang:1.18-alpine AS builder

# 设置工作目录
WORKDIR /app

# 安装必要的工具
RUN apk add --no-cache git

# 复制go.mod和go.sum
COPY go.mod go.sum ./

# 下载依赖
RUN go mod download

# 复制源代码
COPY . .

# 构建应用
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o deepwiki ./cmd/

# 第二阶段：创建最小运行镜像
FROM alpine:latest

# 安装运行时依赖
RUN apk --no-cache add ca-certificates git curl

# 工作目录
WORKDIR /root/

# 从构建器阶段复制二进制文件
COPY --from=builder /app/deepwiki .

# 创建必要的目录
RUN mkdir -p /data/repos /data/databases

# 设置环境变量
ENV GIN_MODE=release
ENV PORT=8001

# 暴露端口
EXPOSE 8001

# 启动命令
CMD ["./deepwiki"]