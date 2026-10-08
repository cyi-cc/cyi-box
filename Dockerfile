# ---------- 前端构建 ----------
FROM node:22-alpine AS web
WORKDIR /app/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY frontend/ .
RUN npm run build

# ---------- 后端构建 ----------
FROM golang:1-alpine AS api
WORKDIR /app/backend
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ .
# modernc.org/sqlite 纯 Go 实现，零 CGO 静态二进制
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /cyibox ./cmd/server

# ---------- 运行时 ----------
FROM alpine:3.21
RUN apk add --no-cache nginx ca-certificates tzdata
WORKDIR /app
COPY --from=api /cyibox /app/cyibox
COPY --from=web /app/frontend/dist /app/web/dist
COPY docker/nginx.conf /etc/nginx/http.d/default.conf
COPY docker/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh && mkdir -p /app/data

ENV CYIBOX_PORT=8890 \
    CYIBOX_DB=/app/data/cyibox.db

EXPOSE 80
VOLUME ["/app/data"]
ENTRYPOINT ["/entrypoint.sh"]
