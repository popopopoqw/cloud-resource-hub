# 多阶段构建：前端已内嵌（go:embed），单二进制即可运行
FROM golang:1.26-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/resource-center .

FROM alpine:3.21
RUN adduser -D -u 10001 app && apk add --no-cache ca-certificates tzdata
RUN mkdir -p /data && chown app:app /data
WORKDIR /data
USER app
COPY --from=builder /out/resource-center /usr/local/bin/resource-center
# 数据统一落 /data（SQLite + 本地对象兜底 + 论坛会话），容器重建不丢需挂卷
ENV PORT=:8080 SQLITE_PATH=/data/resource-center.db DATA_DIR=/data/objects
VOLUME /data
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s \
  CMD wget -qO- http://127.0.0.1:8080/api/v1/health || exit 1
ENTRYPOINT ["resource-center"]
