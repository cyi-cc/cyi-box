package service

import (
	"strings"

	"github.com/cyi-cc/fun"

	"github.com/cyi-cc/cyi-box/backend/internal/dto"
	"github.com/cyi-cc/cyi-box/backend/internal/store"
)

// SettingSvc 站点级 KV 设置：读取对所有登录用户开放，写入仅管理员
type SettingSvc struct {
	fun.Ctx
	Store *store.Store
}

func (s *SettingSvc) Get(d dto.SettingDto) (dto.SettingView, error) {
	key := strings.TrimSpace(d.Key)
	if key == "" || len(key) > 64 {
		return dto.SettingView{}, fun.Error(4001, "key 必填且不超过 64 字符")
	}
	v, err := s.Store.GetSetting(key)
	if err != nil {
		return dto.SettingView{}, err
	}
	return dto.SettingView{Value: v}, nil
}

func (s *SettingSvc) Set(d dto.SettingDto) (dto.OkView, error) {
	key := strings.TrimSpace(d.Key)
	if key == "" || len(key) > 64 || len(d.Value) > 2000 {
		return dto.OkView{}, fun.Error(4001, "key 必填且不超过 64 字符，value 不超过 2000 字符")
	}
	if err := s.Store.SetSetting(key, strings.TrimSpace(d.Value)); err != nil {
		return dto.OkView{}, err
	}
	return dto.OkView{Message: "已保存"}, nil
}
