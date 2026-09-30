// Package memx 共享记忆库的对外协议面：Streamable HTTP MCP server（/mcp），
// 工具与 git.cyi.cc/chiyi/memory-mcp 完全同构，agent 改指 cyi-box 地址即可无缝迁移。
package memx

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/cyi-cc/cyi-box/backend/internal/store"
)

const maxContentLen = 64 * 1024

// 高置信度密钥格式拦截——记忆库绝不允许落凭证
var secretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`AKIA[0-9A-Z]{16}`),
	regexp.MustCompile(`-----BEGIN [A-Z0-9 ]*PRIVATE KEY`),
	regexp.MustCompile(`\b(ghp|gho|ghu|ghs|ghr)_[A-Za-z0-9]{20,}`),
	regexp.MustCompile(`github_pat_[A-Za-z0-9_]{20,}`),
	regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{20,}`),
	regexp.MustCompile(`\bxox[baprs]-[A-Za-z0-9-]{10,}`),
	regexp.MustCompile(`(?i)\b(password|passwd|api[_-]?key|secret)\s*[:=]\s*["']?[^\s"'$]{8,}`),
}

// LintContent 内容体检：超长与疑似凭证直接拒（管理页与 MCP 共用）
func LintContent(content string) error {
	if len(content) > maxContentLen {
		return fmt.Errorf("content too large (%d bytes, max %d)", len(content), maxContentLen)
	}
	for _, p := range secretPatterns {
		if loc := p.FindStringIndex(content); loc != nil {
			return fmt.Errorf("content rejected: looks like it contains a credential near offset %d; store secrets in a secrets manager, not here", loc[0])
		}
	}
	return nil
}

func toJSON(v any) *mcp.CallToolResult {
	b, _ := json.MarshalIndent(v, "", "  ")
	return mcp.NewToolResultText(string(b))
}

func errResult(err error) *mcp.CallToolResult {
	return mcp.NewToolResultError(err.Error())
}

type ctxKeyBase struct{}

// WithBase 把当前请求推导出的站点基址注入 ctx，memory_save 用它拼 /m/<key> 链接
func WithBase(ctx context.Context, base string) context.Context {
	return context.WithValue(ctx, ctxKeyBase{}, base)
}

func baseFrom(ctx context.Context) string {
	if v, ok := ctx.Value(ctxKeyBase{}).(string); ok {
		return v
	}
	return ""
}

// keyFrom 容忍传整段分享链接：带 "/m/" 时只取其后段
func keyFrom(ctx context.Context, key string) string {
	if base := baseFrom(ctx); base != "" {
		key = strings.TrimPrefix(key, base+"/m/")
	}
	if i := strings.LastIndex(key, "/m/"); i >= 0 {
		key = key[i+3:]
	}
	return key
}

// NewMCPServer 建 Streamable HTTP MCP 服务
func NewMCPServer(st *store.Store) *server.MCPServer {
	s := server.NewMCPServer("cyi-box-memory", "1.0.0",
		server.WithToolCapabilities(true),
		server.WithPromptCapabilities(true),
		server.WithResourceCapabilities(false, true),
	)

	s.AddTool(mcp.NewTool("memory_save",
		mcp.WithDescription(`Save a durable, reusable fact to the shared memory bank so future sessions (possibly other agents) can recall it.

WHEN TO CALL: when you learn non-obvious, persistent facts — project architecture, stack/framework, server addresses and how to reach them, build/test/deploy commands, repo conventions, recurring pitfalls, external service endpoints.

DO NOT SAVE: task progress or TODO state, anything derivable by reading the repo (file lists, function names), secrets/credentials/tokens (rejected server-side), one-off command output.

Content should be concise markdown. Re-saving the same key overwrites it (revision+1), so update facts in place rather than duplicating.`),
		mcp.WithString("key", mcp.Required(), mcp.Description(`Stable identifier like "vivid/deploy" — use "project/topic" naming.`)),
		mcp.WithString("content", mcp.Required(), mcp.Description("Markdown body of the memory")),
		mcp.WithString("project", mcp.Description("Project slug, e.g. repo name — groups memories for memory_list")),
		mcp.WithArray("tags", mcp.Items(map[string]any{"type": "string"}), mcp.Description(`e.g. ["server","stack","gotcha","command"]`)),
		mcp.WithString("written_by", mcp.Description("Which agent is writing: devin / codex / claude / human / ...")),
	), func(ctx context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		content, err := r.RequireString("content")
		if err != nil {
			return errResult(err), nil
		}
		key, err := r.RequireString("key")
		if err != nil {
			return errResult(err), nil
		}
		key = strings.TrimSpace(key)
		if key == "" {
			return errResult(fmt.Errorf("key 不能为空，形如 project/topic")), nil
		}
		if err := LintContent(content); err != nil {
			return errResult(err), nil
		}
		project := strings.TrimSpace(r.GetString("project", ""))
		m := &store.Memory{
			Key:       key,
			Content:   content,
			Project:   project,
			Tags:      r.GetStringSlice("tags", nil),
			WrittenBy: r.GetString("written_by", ""),
		}
		if err := st.SaveMemory(m); err != nil {
			return errResult(err), nil
		}
		url, _ := st.SignMemoryReadURL(baseFrom(ctx), m.Key)
		return toJSON(map[string]any{
			"ok": true, "key": m.Key, "revision": m.Revision,
			"url": url, "resource": "memory://" + m.Key,
		}), nil
	})

	s.AddTool(mcp.NewTool("memory_get",
		mcp.WithDescription("Read one memory by exact key. Use keys from memory_list/memory_search results or a shared URL like {base}/m/{key}."),
		mcp.WithString("key", mcp.Required(), mcp.Description(`e.g. "vivid/deploy"`)),
	), func(ctx context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		key, err := r.RequireString("key")
		if err != nil {
			return errResult(err), nil
		}
		key = keyFrom(ctx, key)
		m, err := st.GetMemory(key)
		if err != nil {
			return errResult(err), nil
		}
		if m == nil {
			return mcp.NewToolResultErrorf("no memory with key %q", key), nil
		}
		return toJSON(m), nil
	})

	s.AddTool(mcp.NewTool("memory_search",
		mcp.WithDescription("Keyword-search the memory bank. All terms must match (AND). Returns full memories ordered by recency."),
		mcp.WithString("query", mcp.Required(), mcp.Description("Space-separated keywords")),
		mcp.WithString("project", mcp.Description("Restrict to one project")),
		mcp.WithNumber("limit", mcp.Description("Max results, default 20")),
	), func(ctx context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query, err := r.RequireString("query")
		if err != nil {
			return errResult(err), nil
		}
		ms, err := st.SearchMemories(query, r.GetString("project", ""), int64(r.GetInt("limit", 20)))
		if err != nil {
			return errResult(err), nil
		}
		return toJSON(ms), nil
	})

	s.AddTool(mcp.NewTool("memory_list",
		mcp.WithDescription("List memory keys (metadata only). CALL THIS at the start of a session to see what is already known about the current project before doing exploration work."),
		mcp.WithString("project", mcp.Description("Restrict to one project")),
		mcp.WithString("tag", mcp.Description(`Restrict to one tag, e.g. "server"`)),
		mcp.WithNumber("limit", mcp.Description("Max results, default 20")),
	), func(ctx context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		ms, err := st.ListMemories(r.GetString("project", ""), r.GetString("tag", ""), int64(r.GetInt("limit", 20)))
		if err != nil {
			return errResult(err), nil
		}
		type summary struct {
			Key       string   `json:"key"`
			Project   string   `json:"project,omitempty"`
			Tags      []string `json:"tags,omitempty"`
			WrittenBy string   `json:"written_by,omitempty"`
			UpdatedAt int64    `json:"updated_at"`
		}
		out := make([]summary, 0, len(ms))
		for _, m := range ms {
			out = append(out, summary{m.Key, m.Project, m.Tags, m.WrittenBy, m.UpdatedAt})
		}
		return toJSON(out), nil
	})

	s.AddTool(mcp.NewTool("memory_projects",
		mcp.WithDescription("List project groups in the memory bank with memory counts. Use to discover which projects have memories."),
	), func(ctx context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		ps, err := st.MemoryProjects()
		if err != nil {
			return errResult(err), nil
		}
		type proj struct {
			Name  string `json:"name"`
			Count int64  `json:"count"`
		}
		out := make([]proj, 0, len(ps))
		for _, p := range ps {
			out = append(out, proj{p.Name, p.Count})
		}
		return toJSON(out), nil
	})

	s.AddTool(mcp.NewTool("memory_delete",
		mcp.WithDescription("Permanently delete a memory by key. Use when a fact is stale or was saved by mistake."),
		mcp.WithString("key", mcp.Required()),
	), func(ctx context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		key, err := r.RequireString("key")
		if err != nil {
			return errResult(err), nil
		}
		ok, err := st.DeleteMemory(keyFrom(ctx, key))
		if err != nil {
			return errResult(err), nil
		}
		return toJSON(map[string]any{"ok": ok, "key": key}), nil
	})

	s.AddPrompt(mcp.NewPrompt("save_context",
		mcp.WithPromptDescription("Collect this project's durable context (repo, stack, servers, commands, gotchas) and save it to the shared memory bank."),
	), func(ctx context.Context, r mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		return mcp.NewGetPromptResult("Save project context to shared memory", []mcp.PromptMessage{
			mcp.NewPromptMessage(mcp.RoleUser, mcp.NewTextContent(`Collect this project's durable context and save it with the memory_save tool:

1. Gather: git remote URL, default branch, current commit; language/framework/build system; external servers or endpoints the project talks to; non-obvious build/test/deploy commands; known pitfalls.
2. Do NOT include secrets, tokens, or task progress.
3. Call memory_save once per coherent topic using keys like "<project>/<topic>" (e.g. "vivid/deploy", "vivid/stack"), with project and tags set.
4. Reply with the list of saved keys and their URLs.`)),
		}), nil
	})

	// memory://<key> 资源：客户端可直接把记忆钉进上下文，省一轮 tools/call
	s.AddResourceTemplate(
		mcp.NewResourceTemplate("memory:/{/key*}", "memory",
			mcp.WithTemplateDescription("Read a memory by key (e.g. memory://vivid/deploy)"),
			mcp.WithTemplateMIMEType("text/markdown"),
		),
		func(ctx context.Context, r mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			var parts []string
			switch v := r.Params.Arguments["key"].(type) {
			case []string:
				parts = v
			case []any:
				for _, s := range v {
					if str, ok := s.(string); ok {
						parts = append(parts, str)
					}
				}
			case string:
				parts = []string{v}
			}
			key := strings.Join(parts, "/")
			m, err := st.GetMemory(key)
			if err != nil {
				return nil, err
			}
			if m == nil {
				return nil, fmt.Errorf("no memory with key %q", key)
			}
			return []mcp.ResourceContents{mcp.TextResourceContents{
				URI:      r.Params.URI,
				MIMEType: "text/markdown",
				Text:     m.Content,
			}}, nil
		},
	)

	return s
}
