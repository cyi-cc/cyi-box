package service

import (
	"errors"
	"fmt"
	"strings"

	"github.com/cyi-cc/fun"
	"golang.org/x/crypto/bcrypt"

	"github.com/cyi-cc/cyi-box/backend/internal/dto"
	"github.com/cyi-cc/cyi-box/backend/internal/store"
)

// UserSvc 用户管理：全部 policyAdmin， Guard 已保证仅管理员可达
type UserSvc struct {
	fun.Ctx
	Store *store.Store
}

func (s *UserSvc) ListUsers(d dto.ListUsersDto) (dto.UserPageView, error) {
	page := max(d.Page, 1)
	pageSize := min(max(d.PageSize, 1), 100)
	keyword := ""
	if d.Keyword != nil {
		keyword = strings.TrimSpace(*d.Keyword)
	}
	users, total, err := s.Store.ListUsers(keyword, page, pageSize)
	if err != nil {
		return dto.UserPageView{}, fmt.Errorf("user: list failed: %w", err)
	}
	items := make([]dto.UserView, 0, len(users))
	for _, u := range users {
		items = append(items, userView(u))
	}
	return dto.UserPageView{Total: total, Items: items}, nil
}

// SaveUser Id 为空创建（密码必填），非空更新（密码留空不变）
func (s *UserSvc) SaveUser(d dto.SaveUserDto) (dto.UserView, error) {
	name := strings.TrimSpace(d.Name)
	if name == "" || len(name) > 32 {
		return dto.UserView{}, fun.Error(4001, "用户名必填且不超过 32 字符")
	}
	if d.Role != "admin" && d.Role != "user" {
		return dto.UserView{}, fun.Error(4001, "角色无效")
	}

	if d.Id == nil {
		if d.Password == nil || len(*d.Password) < 6 {
			return dto.UserView{}, fun.Error(4001, "密码至少 6 位")
		}
		if _, err := s.Store.GetUserByName(name); err == nil {
			return dto.UserView{}, fun.Error(4001, "用户名已存在")
		} else if !errors.Is(err, store.ErrNotFound) {
			return dto.UserView{}, err
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(*d.Password), bcrypt.DefaultCost)
		if err != nil {
			return dto.UserView{}, err
		}
		u, err := s.Store.CreateUser(name, string(hash), d.Role)
		if err != nil {
			return dto.UserView{}, fmt.Errorf("user: create failed: %w", err)
		}
		return userView(u), nil
	}

	u, err := s.Store.GetUserByID(*d.Id)
	if errors.Is(err, store.ErrNotFound) {
		return dto.UserView{}, fun.Error(4001, "用户不存在")
	}
	if err != nil {
		return dto.UserView{}, err
	}
	if existing, err := s.Store.GetUserByName(name); err == nil && existing.Id != u.Id {
		return dto.UserView{}, fun.Error(4001, "用户名已存在")
	}
	status := u.Status
	if d.Status != nil {
		if *d.Status != "active" && *d.Status != "disabled" {
			return dto.UserView{}, fun.Error(4001, "状态无效")
		}
		status = *d.Status
	}
	if err := s.guardLastAdmin(u, d.Role, status); err != nil {
		return dto.UserView{}, err
	}
	var hash *string
	if d.Password != nil && *d.Password != "" {
		if len(*d.Password) < 6 {
			return dto.UserView{}, fun.Error(4001, "密码至少 6 位")
		}
		h, err := bcrypt.GenerateFromPassword([]byte(*d.Password), bcrypt.DefaultCost)
		if err != nil {
			return dto.UserView{}, err
		}
		hs := string(h)
		hash = &hs
	}
	if err := s.Store.UpdateUser(u.Id, name, d.Role, status, hash); err != nil {
		return dto.UserView{}, fmt.Errorf("user: update failed: %w", err)
	}
	u, err = s.Store.GetUserByID(u.Id)
	if err != nil {
		return dto.UserView{}, err
	}
	return userView(u), nil
}

func (s *UserSvc) DeleteUser(d dto.DeleteUserDto) (dto.OkView, error) {
	su, err := sessionFrom(s.RequestCtx)
	if err != nil {
		return dto.OkView{}, err
	}
	if d.Id == su.User.Id {
		return dto.OkView{}, fun.Error(4001, "不能删除当前登录账号")
	}
	u, err := s.Store.GetUserByID(d.Id)
	if errors.Is(err, store.ErrNotFound) {
		return dto.OkView{}, fun.Error(4001, "用户不存在")
	}
	if err != nil {
		return dto.OkView{}, err
	}
	if err := s.guardLastAdmin(u, "user", "disabled"); err != nil {
		return dto.OkView{}, err
	}
	if err := s.Store.DeleteUser(d.Id); err != nil {
		return dto.OkView{}, fmt.Errorf("user: delete failed: %w", err)
	}
	return dto.OkView{Message: "已删除"}, nil
}

// guardLastAdmin 阻止把最后一个可用管理员降级/禁用/删除
func (s *UserSvc) guardLastAdmin(u store.User, newRole, newStatus string) error {
	if u.Role == "admin" && u.Status == "active" && (newRole != "admin" || newStatus != "active") {
		n, err := s.Store.CountAdmins()
		if err != nil {
			return err
		}
		if n <= 1 {
			return fun.Error(4001, "至少需要保留一个可用管理员")
		}
	}
	return nil
}
