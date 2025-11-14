# Dockerfile
FROM golang:1.24-alpine AS builder
RUN apk add --no-cache git build-base
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# 构建静态二进制
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o /app/v2v ./main.go

FROM alpine:3.18
RUN apk add --no-cache ca-certificates ffmpeg tzdata
# 创建运行目录
RUN mkdir -p /app/public/videos /app/public/pic /var/log/v2v
COPY --from=builder /app/v2v /app/v2v
COPY --from=builder /src/public /app/public
WORKDIR /app
EXPOSE 8080
ENV GIN_MODE=release
CMD ["/app/v2v"]