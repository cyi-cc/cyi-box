#!/bin/sh
set -e

# 后端跑在容器内 8890，nginx 对外 80（托管前端 + 反代 API/协议端点）
/app/cyibox &
API_PID=$!

trap 'kill -TERM $API_PID 2>/dev/null' TERM INT

nginx -g 'daemon off;' &
NGINX_PID=$!

wait $NGINX_PID
kill -TERM $API_PID 2>/dev/null || true
wait $API_PID 2>/dev/null || true
