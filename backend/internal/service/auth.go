package service

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/cyi-cc/fun"
	"golang.org/x/crypto/bcrypt"

	"github.com/cyi-cc/cyi-box/backend/internal/dto"
	"github.com/cyi-cc/cyi-box/backend/internal/store"
)

// 登录态：24h 滑动过期；距上次续期超过 2h 的请求自动把有效期顺延回 24h
const sessionTTL = 24 * time.Hour
const sessionRenewAfter = 2 * time.Hour

type AuthSvc struct {
	fun.Ctx
	Store *store.Store
}

// Login 用户名+密码 → 签发会话 token（前端存 localStorage，随 state 透传）
func (s *AuthSvc) Login(d dto.LoginDto) (dto.SessionView, error) {
	name := strings.TrimSpace(d.Name)
	if name == "" || d.Password == "" {
		return dto.SessionView{}, fun.Error(4001, "请输入用户名和密码")
	}
	user, err := s.Store.GetUserByName(name)
	if errors.Is(err, store.ErrNotFound) {
		return dto.SessionView{}, fun.Error(4001, "用户名或密码错误")
	}
	if err != nil {
		return dto.SessionView{}, fmt.Errorf("auth: query user failed: %w", err)
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(d.Password)) != nil {
		return dto.SessionView{}, fun.Error(4001, "用户名或密码错误")
	}
	if user.Status != "active" {
		return dto.SessionView{}, fun.Error(4003, "账号已被禁用")
	}
	token, err := newToken()
	if err != nil {
		return dto.SessionView{}, err
	}
	if err := s.Store.CreateSession(token, user.Id, sessionTTL); err != nil {
		return dto.SessionView{}, fmt.Errorf("auth: create session failed: %w", err)
	}
	log.Printf("auth: user %q logged in from %s", user.Name, s.Ip)
	view := userView(user)
	return dto.SessionView{User: &view, Token: &token}, nil
}

// Me 返回当前登录用户（不重复签发 token）
func (s *AuthSvc) Me() (dto.SessionView, error) {
	su, err := sessionFrom(s.RequestCtx)
	if err != nil {
		return dto.SessionView{}, err
	}
	view := userView(su.User)
	return dto.SessionView{User: &view}, nil
}

// Logout 幂等：删掉当前会话即可
func (s *AuthSvc) Logout() (dto.OkView, error) {
	su, err := sessionFrom(s.RequestCtx)
	if err != nil {
		return dto.OkView{Message: "已退出"}, nil
	}
	if err := s.Store.DeleteSession(su.Token); err != nil {
		return dto.OkView{}, fmt.Errorf("auth: logout failed: %w", err)
	}
	return dto.OkView{Message: "已退出"}, nil
}

// ChangePassword 校验旧密码 → 改密 → 吊销其他会话（当前会话保留）
func (s *AuthSvc) ChangePassword(d dto.ChangePasswordDto) (dto.OkView, error) {
	su, err := sessionFrom(s.RequestCtx)
	if err != nil {
		return dto.OkView{}, err
	}
	if len(d.NewPassword) < 6 {
		return dto.OkView{}, fun.Error(4001, "新密码至少 6 位")
	}
	if bcrypt.CompareHashAndPassword([]byte(su.User.PasswordHash), []byte(d.OldPassword)) != nil {
		return dto.OkView{}, fun.Error(4001, "旧密码错误")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(d.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return dto.OkView{}, err
	}
	h := string(hash)
	if err := s.Store.UpdateUser(su.User.Id, su.User.Name, su.User.Role, su.User.Status, &h); err != nil {
		return dto.OkView{}, fmt.Errorf("auth: update password failed: %w", err)
	}
	if err := s.Store.DeleteOtherSessions(su.User.Id, su.Token); err != nil {
		return dto.OkView{}, fmt.Errorf("auth: revoke sessions failed: %w", err)
	}
	return dto.OkView{Message: "密码已修改"}, nil
}

func newToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func userView(u store.User) dto.UserView {
	return dto.UserView{
		Id:        u.Id,
		Name:      u.Name,
		Role:      u.Role,
		Status:    u.Status,
		CreatedAt: u.CreatedAt,
	}
}
