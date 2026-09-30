NODE_BIN := $(HOME)/.local/opt/node-v22.23.2-linux-x64/bin
export PATH := $(NODE_BIN):$(PATH)

.PHONY: dev backend frontend gen build

# 前后端一起起：后端 :8890，前端 :5173
dev:
	(cd backend && go run ./cmd/server) & (cd frontend && npm run dev)

backend:
	cd backend && go run ./cmd/server

frontend:
	cd frontend && npm run dev

# 改了 backend 服务/DTO 后跑这个：sqlc 重新生成 internal/db，再生成 frontend/src/api
gen:
	cd backend && go tool sqlc generate && go run ./cmd/gen
	# 用 cp 覆盖而非 mv：内容重写才能触发 vite watcher，mv(rename) 会让 dev server 继续喂过期模块
	cp -f frontend/src/api/ts/* frontend/src/api/ && rm -rf frontend/src/api/ts

build:
	cd backend && go build ./...
	cd frontend && npm run build
