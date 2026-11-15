package service

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"time"

	"github.com/cyi-cc/fun"
	"golang.org/x/crypto/bcrypt"

	"github.com/cyi-cc/cyi-box/backend/internal/config"
	"github.com/cyi-cc/cyi-box/backend/internal/dto"
	"github.com/cyi-cc/cyi-box/backend/internal/store"
)

// 登录态：24h 滑动过期；距上次续期超过 2h 的请求自动把有效期顺延回 24h
const sessionTTL = 24 * time.Hour
const sessionRenewAfter = 2 * time.Hour

type AuthSvc struct {
	fun.Ctx
	Store *store.Store
	Cfg   *config.Config `fun:"auto"`
}

// adminView 单用户模型：登录即管理员，显示名取 CYIBOX_ADMIN_NAME
func (s *AuthSvc) adminView() dto.UserView {
	name := "admin"
	if s.Cfg != nil && s.Cfg.AdminName != "" {
		name = s.Cfg.AdminName
	}
	return dto.UserView{Id: 1, Name: name, Role: "admin", Status: "active"}
}

// Login 密码（settings 存 bcrypt 哈希）→ 签发会话 token（前端存 localStorage，随 state 透传）
func (s *AuthSvc) Login(d dto.LoginDto) (dto.SessionView, error) {
	if d.Password == "" {
		return dto.SessionView{}, fun.Error(4001, "请输入密码")
	}
	hash, err := s.Store.PasswordHash()
	if err != nil {
		return dto.SessionView{}, fmt.Errorf("auth: read password failed: %w", err)
	}
	if hash == "" || bcrypt.CompareHashAndPassword([]byte(hash), []byte(d.Password)) != nil {
		return dto.SessionView{}, fun.Error(4001, "密码错误")
	}
	token, err := newToken()
	if err != nil {
		return dto.SessionView{}, err
	}
	if err := s.Store.CreateSession(token, sessionTTL); err != nil {
		return dto.SessionView{}, fmt.Errorf("auth: create session failed: %w", err)
	}
	log.Printf("auth: logged in from %s", s.Ip)
	view := s.adminView()
	return dto.SessionView{User: &view, Token: &token}, nil
}

// Me 返回当前登录用户（不重复签发 token）
func (s *AuthSvc) Me() (dto.SessionView, error) {
	if _, err := sessionToken(s.RequestCtx); err != nil {
		return dto.SessionView{}, err
	}
	view := s.adminView()
	return dto.SessionView{User: &view}, nil
}

// Logout 幂等：删掉当前会话即可
func (s *AuthSvc) Logout() (dto.OkView, error) {
	token, err := sessionToken(s.RequestCtx)
	if err != nil {
		return dto.OkView{Message: "已退出"}, nil
	}
	if err := s.Store.DeleteSession(token); err != nil {
		return dto.OkView{}, fmt.Errorf("auth: logout failed: %w", err)
	}
	return dto.OkView{Message: "已退出"}, nil
}

// ChangePassword 校验旧密码 → 改密 → 吊销其他会话（当前会话保留）
func (s *AuthSvc) ChangePassword(d dto.ChangePasswordDto) (dto.OkView, error) {
	token, err := sessionToken(s.RequestCtx)
	if err != nil {
		return dto.OkView{}, err
	}
	if len(d.NewPassword) < 6 {
		return dto.OkView{}, fun.Error(4001, "新密码至少 6 位")
	}
	old, err := s.Store.PasswordHash()
	if err != nil {
		return dto.OkView{}, fmt.Errorf("auth: read password failed: %w", err)
	}
	if bcrypt.CompareHashAndPassword([]byte(old), []byte(d.OldPassword)) != nil {
		return dto.OkView{}, fun.Error(4001, "旧密码错误")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(d.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return dto.OkView{}, err
	}
	if err := s.Store.SetPasswordHash(string(hash)); err != nil {
		return dto.OkView{}, fmt.Errorf("auth: update password failed: %w", err)
	}
	if err := s.Store.DeleteOtherSessions(token); err != nil {
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
