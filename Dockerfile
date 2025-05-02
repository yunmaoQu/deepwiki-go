### Dockerfile

```dockerfile
# Dockerfile
FROM golang:1.20-alpine AS builder

WORKDIR /app

# 复制 Go 模块文件
COPY go.mod go.sum ./
RUN go mod download

# 复制源代码
COPY . .

# 构建应用程序
RUN CGO_ENABLED=0 GOOS=linux go build -o deepwiki-server ./cmd/server

# 使用小型基础镜像
FROM alpine:latest

RUN apk --no-cache add ca-certificates git

WORKDIR /app

# 从构建器阶段复制构建好的可执行文件
COPY --from=builder /app/deepwiki-server .

# 声明端口
EXPOSE 8080
CMD ["./deepwiki-server"]