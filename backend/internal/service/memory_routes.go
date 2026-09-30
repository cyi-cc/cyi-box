// memory_routes.go 记忆库对外协议端点：
//   POST /mcp      Streamable HTTP MCP（Bearer 令牌鉴权，令牌见 MemorySvc.McpInfo）
//   GET  /m        阅读链接索引（Bearer 鉴权——key 可枚举，不能裸奔）
//   GET  /m/*      记忆阅读链接（?sig= 签名鉴权，HMAC(key, 令牌) 签发，令牌重置即吊销）
package service

import (
	"context"
	"crypto/subtle"
	"net/http"

	"github.com/valyala/fasthttp"

	"github.com/cyi-cc/fun"
	"github.com/mark3labs/mcp-go/server"
	"github.com/valyala/fasthttp/fasthttpadaptor"

	"github.com/cyi-cc/cyi-box/backend/internal/memx"
	"github.com/cyi-cc/cyi-box/backend/internal/store"
)

// requestBase 按当前请求 Host + X-Forwarded-Proto 推导站点基址，
// 反代换域名后生成的 /m/ 链接与 MCP 接入地址始终可点。
func requestBase(ctx *fasthttp.RequestCtx) string {
	proto := string(ctx.Request.Header.Peek("X-Forwarded-Proto"))
	if proto == "" {
		proto = "http"
	}
	return proto + "://" + string(ctx.Host())
}

// McpHandler POST /mcp：Authorization Bearer 校验后交给 Streamable HTTP 服务。
// 令牌存 settings(mcp.token)，首次访问懒生成。
func McpHandler(st *store.Store) fun.RouteHandler {
	mcpSrv := server.NewStreamableHTTPServer(
		memx.NewMCPServer(st),
		server.WithStateLess(true),
		server.WithHTTPContextFunc(func(ctx context.Context, r *http.Request) context.Context {
			proto := r.Header.Get("X-Forwarded-Proto")
			if proto == "" {
				proto = "http"
				if r.TLS != nil {
					proto = "https"
				}
			}
			return memx.WithBase(ctx, proto+"://"+r.Host)
		}),
	)
	h := fasthttpadaptor.NewFastHTTPHandler(mcpSrv)
	return func(c *fun.RouteCtx) error {
		tok, err := st.McpToken()
		if err != nil {
			c.RequestCtx.SetStatusCode(500)
			c.RequestCtx.WriteString(err.Error())
			return nil
		}
		auth := string(c.RequestCtx.Request.Header.Peek("Authorization"))
		if subtle.ConstantTimeCompare([]byte(auth), []byte("Bearer "+tok)) != 1 {
			c.RequestCtx.SetStatusCode(401)
			c.RequestCtx.WriteString("unauthorized")
			return nil
		}
		h(c.RequestCtx)
		return nil
	}
}

// MemoryReadHandler GET /m/*：输出 markdown 原文；裸 /m/ 列出全部阅读链接。
func MemoryReadHandler(st *store.Store) fun.RouteHandler {
	return func(c *fun.RouteCtx) error {
		key := c.Wildcard
		base := requestBase(c.RequestCtx)
		if key == "" {
			// 索引收进 Bearer 鉴权：key 形如 project/topic 可枚举，裸索引等于全库暴露
			tok, err := st.McpToken()
			if err != nil {
				c.RequestCtx.SetStatusCode(500)
				c.RequestCtx.WriteString(err.Error())
				return nil
			}
			auth := string(c.RequestCtx.Request.Header.Peek("Authorization"))
			if subtle.ConstantTimeCompare([]byte(auth), []byte("Bearer "+tok)) != 1 {
				c.RequestCtx.SetStatusCode(401)
				c.RequestCtx.WriteString("unauthorized")
				return nil
			}
			ms, err := st.ListMemories("", "", 200)
			if err != nil {
				c.RequestCtx.SetStatusCode(500)
				c.RequestCtx.WriteString(err.Error())
				return nil
			}
			c.RequestCtx.SetContentType("text/plain; charset=utf-8")
			for _, m := range ms {
				readURL, _ := st.SignMemoryReadURL(base, m.Key)
				line := readURL
				if m.Project != "" {
					line += "\t[" + m.Project + "]"
				}
				c.RequestCtx.WriteString(line + "\n")
			}
			return nil
		}
		if !st.CheckMemoryReadSig(key, string(c.RequestCtx.QueryArgs().Peek("sig"))) {
			c.RequestCtx.SetStatusCode(403)
			c.RequestCtx.WriteString("bad or missing sig")
			return nil
		}
		m, err := st.GetMemory(key)
		if err != nil {
			c.RequestCtx.SetStatusCode(500)
			c.RequestCtx.WriteString(err.Error())
			return nil
		}
		if m == nil {
			c.RequestCtx.SetStatusCode(404)
			c.RequestCtx.WriteString("not found")
			return nil
		}
		c.RequestCtx.SetContentType("text/markdown; charset=utf-8")
		c.RequestCtx.WriteString(m.Content)
		return nil
	}
}
