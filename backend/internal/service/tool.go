package service

import (
	"fmt"
	"strings"

	"github.com/cyi-cc/fun"

	"github.com/cyi-cc/cyi-box/backend/internal/dto"
	"github.com/cyi-cc/cyi-box/backend/internal/store"
)

// ToolSvc 工具箱注册表：List 登录用户可见（驱动侧边导航），管理端点为 admin
type ToolSvc struct {
	fun.Ctx
	Store *store.Store
}

func (s *ToolSvc) List() ([]dto.ToolView, error) {
	tools, err := s.Store.ListTools(true)
	if err != nil {
		return nil, fmt.Errorf("tool: list failed: %w", err)
	}
	return toolViews(tools), nil
}

func (s *ToolSvc) AdminList() ([]dto.ToolView, error) {
	tools, err := s.Store.ListTools(false)
	if err != nil {
		return nil, fmt.Errorf("tool: list failed: %w", err)
	}
	return toolViews(tools), nil
}

func (s *ToolSvc) SaveTool(d dto.SaveToolDto) (dto.ToolView, error) {
	name := strings.TrimSpace(d.Name)
	url := strings.TrimSpace(d.Url)
	if name == "" || len(name) > 32 {
		return dto.ToolView{}, fun.Error(4001, "工具名必填且不超过 32 字符")
	}
	t := store.Tool{
		Name: name,
		Url:  url,
		Sort: 0,
		Enabled: true,
	}
	if d.Id != nil {
		t.Id = *d.Id
	}
	if d.Icon != nil {
		t.Icon = strings.TrimSpace(*d.Icon)
	}
	if d.Description != nil {
		t.Description = strings.TrimSpace(*d.Description)
	}
	if d.Sort != nil {
		t.Sort = *d.Sort
	}
	if d.Enabled != nil {
		t.Enabled = *d.Enabled
	}
	if err := s.Store.SaveTool(&t); err != nil {
		return dto.ToolView{}, fmt.Errorf("tool: save failed: %w", err)
	}
	views := toolViews([]store.Tool{t})
	return views[0], nil
}

func (s *ToolSvc) DeleteTool(d dto.DeleteToolDto) (dto.OkView, error) {
	if err := s.Store.DeleteTool(d.Id); err != nil {
		return dto.OkView{}, fmt.Errorf("tool: delete failed: %w", err)
	}
	return dto.OkView{Message: "已删除"}, nil
}

func toolViews(tools []store.Tool) []dto.ToolView {
	views := make([]dto.ToolView, 0, len(tools))
	for _, t := range tools {
		views = append(views, dto.ToolView{
			Id:          t.Id,
			Name:        t.Name,
			Icon:        t.Icon,
			Url:         t.Url,
			Description: t.Description,
			Sort:        t.Sort,
			Enabled:     t.Enabled,
		})
	}
	return views
}
