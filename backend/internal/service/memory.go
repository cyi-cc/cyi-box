// memory.go 共享记忆库管理（仅管理员）：项目分组、搜索、读写删除、MCP 接入信息。
// 对外协议端点见 memory_routes.go（POST /mcp + GET /m/<key>）。
package service

import (
	"strings"

	"github.com/cyi-cc/fun"

	"github.com/cyi-cc/cyi-box/backend/internal/dto"
	"github.com/cyi-cc/cyi-box/backend/internal/memx"
	"github.com/cyi-cc/cyi-box/backend/internal/store"
)

// MemorySvc 记忆库管理（仅管理员）。
type MemorySvc struct {
	fun.Ctx
	Store *store.Store
}

func memoryMeta(m store.Memory) dto.MemoryMetaView {
	return dto.MemoryMetaView{
		Key: m.Key, Project: m.Project, Tags: m.Tags,
		WrittenBy: m.WrittenBy, Revision: m.Revision, UpdatedAt: m.UpdatedAt,
	}
}

func (s *MemorySvc) Projects() (dto.MemoryProjectsView, error) {
	rows, err := s.Store.MemoryProjects()
	if err != nil {
		return dto.MemoryProjectsView{}, err
	}
	items := make([]dto.MemoryProjectView, 0, len(rows))
	for _, r := range rows {
		items = append(items, dto.MemoryProjectView{Name: r.Name, Count: r.Count})
	}
	return dto.MemoryProjectsView{Items: items}, nil
}

// List query 非空走全文 AND 搜索；否则按 project/tag 过滤列元数据
func (s *MemorySvc) List(d dto.MemoryListDto) (dto.MemoryListView, error) {
	var ms []store.Memory
	var err error
	if q := strings.TrimSpace(d.Query); q != "" {
		ms, err = s.Store.SearchMemories(q, strings.TrimSpace(d.Project), d.Limit)
	} else {
		ms, err = s.Store.ListMemories(strings.TrimSpace(d.Project), strings.TrimSpace(d.Tag), d.Limit)
	}
	if err != nil {
		return dto.MemoryListView{}, err
	}
	items := make([]dto.MemoryMetaView, 0, len(ms))
	for _, m := range ms {
		items = append(items, memoryMeta(m))
	}
	return dto.MemoryListView{Items: items}, nil
}

func (s *MemorySvc) Get(d dto.MemoryKeyDto) (dto.MemoryView, error) {
	m, err := s.Store.GetMemory(strings.TrimSpace(d.Key))
	if err != nil {
		return dto.MemoryView{}, err
	}
	if m == nil {
		return dto.MemoryView{}, fun.Error(4004, "记忆不存在: "+d.Key)
	}
	readURL, _ := s.Store.SignMemoryReadURL(requestBase(s.Ctx.RequestCtx), m.Key)
	return dto.MemoryView{
		Key: m.Key, Url: readURL, Content: m.Content, Project: m.Project, Tags: m.Tags,
		WrittenBy: m.WrittenBy, Revision: m.Revision,
		CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt,
	}, nil
}

func (s *MemorySvc) Save(d dto.MemorySaveDto) (dto.MemorySaveView, error) {
	if err := memx.LintContent(d.Content); err != nil {
		return dto.MemorySaveView{}, fun.Error(4002, err.Error())
	}
	project := strings.TrimSpace(d.Project)
	key := strings.TrimSpace(d.Key)
	if key == "" {
		return dto.MemorySaveView{}, fun.Error(4001, "key 必填，形如 project/topic")
	}
	m := &store.Memory{
		Key: key, Content: d.Content, Project: project,
		Tags: d.Tags, WrittenBy: "admin",
	}
	if err := s.Store.SaveMemory(m); err != nil {
		return dto.MemorySaveView{}, err
	}
	readURL, _ := s.Store.SignMemoryReadURL(requestBase(s.Ctx.RequestCtx), m.Key)
	return dto.MemorySaveView{
		Key: m.Key, Revision: m.Revision, Url: readURL,
	}, nil
}

func (s *MemorySvc) Delete(d dto.MemoryKeyDto) (dto.OkView, error) {
	if _, err := s.Store.DeleteMemory(strings.TrimSpace(d.Key)); err != nil {
		return dto.OkView{}, err
	}
	return dto.OkView{Message: "已删除"}, nil
}

// McpInfo 返回对外 MCP 端点与 Bearer 令牌（缺省懒生成一个）
func (s *MemorySvc) McpInfo() (dto.MemoryMcpView, error) {
	tok, err := s.Store.McpToken()
	if err != nil {
		return dto.MemoryMcpView{}, err
	}
	return dto.MemoryMcpView{
		Url:   requestBase(s.Ctx.RequestCtx) + "/mcp",
		Token: tok,
	}, nil
}

func (s *MemorySvc) ResetToken() (dto.MemoryMcpView, error) {
	if _, err := s.Store.ResetMcpToken(); err != nil {
		return dto.MemoryMcpView{}, err
	}
	return s.McpInfo()
}

