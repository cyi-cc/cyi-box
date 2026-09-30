package service

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/cyi-cc/fun"

	"github.com/cyi-cc/cyi-box/backend/internal/config"
	"github.com/cyi-cc/cyi-box/backend/internal/dto"
	"github.com/cyi-cc/cyi-box/backend/internal/store"
	"github.com/cyi-cc/cyi-box/backend/internal/vault"
)

// VaultSvc 密码箱：密文只在服务端出现；列表只回元数据，
// 密码经 Reveal 显式取回，2FA 只回当前验证码不回密钥。
type VaultSvc struct {
	fun.Ctx
	Store *store.Store
	Cfg   *config.Config `fun:"auto"`
}

func (s *VaultSvc) owner() (store.SessionUser, error) {
	return sessionFrom(s.RequestCtx)
}

func (s *VaultSvc) List() ([]dto.VaultItemView, error) {
	su, err := s.owner()
	if err != nil {
		return nil, err
	}
	items, err := s.Store.ListVault(su.User.Id, "")
	if err != nil {
		return nil, fmt.Errorf("vault: list failed: %w", err)
	}
	views := make([]dto.VaultItemView, 0, len(items))
	for _, e := range items {
		views = append(views, vaultView(e))
	}
	return views, nil
}

func (s *VaultSvc) Save(d dto.SaveVaultDto) (dto.VaultItemView, error) {
	su, err := s.owner()
	if err != nil {
		return dto.VaultItemView{}, err
	}
	title := strings.TrimSpace(d.Title)
	if title == "" || len(title) > 64 {
		return dto.VaultItemView{}, fun.Error(4001, "标题必填且不超过 64 字符")
	}

	e := store.VaultEntry{UserID: su.User.Id, Title: title}
	if d.Id != nil {
		old, err := s.Store.GetVault(su.User.Id, *d.Id)
		if errors.Is(err, store.ErrNotFound) {
			return dto.VaultItemView{}, fun.Error(4001, "条目不存在")
		}
		if err != nil {
			return dto.VaultItemView{}, err
		}
		e = old
		e.Title = title
	}
	if d.Username != nil {
		e.Username = strings.TrimSpace(*d.Username)
	}
	if d.Url != nil {
		e.Url = strings.TrimSpace(*d.Url)
	}
	if d.Note != nil {
		e.Note = strings.TrimSpace(*d.Note)
	}

	if d.Password != nil {
		enc, err := vault.Encrypt(s.Cfg.VaultKey, *d.Password)
		if err != nil {
			return dto.VaultItemView{}, err
		}
		e.PasswordEnc = enc
	}
	if d.TotpSecret != nil {
		secret := strings.TrimSpace(*d.TotpSecret)
		if secret == "" {
			e.TotpEnc = "" // 空字符串 = 清除 2FA
		} else {
			parsed, err := vault.ParseSecret(secret)
			if err != nil {
				return dto.VaultItemView{}, fun.Error(4001, "2FA 密钥无效：支持 base32 或 otpauth:// 链接")
			}
			enc, err := vault.Encrypt(s.Cfg.VaultKey, parsed)
			if err != nil {
				return dto.VaultItemView{}, err
			}
			e.TotpEnc = enc
		}
	}
	if err := s.Store.SaveVault(&e); err != nil {
		return dto.VaultItemView{}, fmt.Errorf("vault: save failed: %w", err)
	}
	return vaultView(e), nil
}

// Reveal 显式取回明文密码（前端点「查看」才调用，列表永不携带）
func (s *VaultSvc) Reveal(d dto.VaultIdDto) (dto.SecretView, error) {
	su, err := s.owner()
	if err != nil {
		return dto.SecretView{}, err
	}
	e, err := s.Store.GetVault(su.User.Id, d.Id)
	if errors.Is(err, store.ErrNotFound) {
		return dto.SecretView{}, fun.Error(4001, "条目不存在")
	}
	if err != nil {
		return dto.SecretView{}, err
	}
	if e.PasswordEnc == "" {
		return dto.SecretView{}, fun.Error(4001, "该条目没有密码")
	}
	pt, err := vault.Decrypt(s.Cfg.VaultKey, e.PasswordEnc)
	if err != nil {
		return dto.SecretView{}, fmt.Errorf("vault: decrypt failed: %w", err)
	}
	return dto.SecretView{Value: pt}, nil
}

// Totp 返回当前 6 位验证码与剩余秒数；密钥不出服务端
func (s *VaultSvc) Totp(d dto.VaultIdDto) (dto.TotpView, error) {
	su, err := s.owner()
	if err != nil {
		return dto.TotpView{}, err
	}
	e, err := s.Store.GetVault(su.User.Id, d.Id)
	if errors.Is(err, store.ErrNotFound) {
		return dto.TotpView{}, fun.Error(4001, "条目不存在")
	}
	if err != nil {
		return dto.TotpView{}, err
	}
	if e.TotpEnc == "" {
		return dto.TotpView{}, fun.Error(4001, "该条目未设置 2FA")
	}
	secret, err := vault.Decrypt(s.Cfg.VaultKey, e.TotpEnc)
	if err != nil {
		return dto.TotpView{}, fmt.Errorf("vault: decrypt totp failed: %w", err)
	}
	code, left, err := vault.TOTP(secret, time.Now())
	if err != nil {
		return dto.TotpView{}, fun.Error(4001, "2FA 密钥无效")
	}
	return dto.TotpView{Code: code, SecondsLeft: left}, nil
}

func (s *VaultSvc) Delete(d dto.VaultIdDto) (dto.OkView, error) {
	su, err := s.owner()
	if err != nil {
		return dto.OkView{}, err
	}
	if err := s.Store.DeleteVault(su.User.Id, d.Id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return dto.OkView{}, fun.Error(4001, "条目不存在")
		}
		return dto.OkView{}, fmt.Errorf("vault: delete failed: %w", err)
	}
	return dto.OkView{Message: "已删除"}, nil
}

func vaultView(e store.VaultEntry) dto.VaultItemView {
	v := dto.VaultItemView{
		Id:          e.Id,
		Title:       e.Title,
		HasPassword: e.PasswordEnc != "",
		HasTotp:     e.TotpEnc != "",
		UpdatedAt:   e.UpdatedAt,
	}
	if e.Username != "" {
		v.Username = &e.Username
	}
	if e.Url != "" {
		v.Url = &e.Url
	}
	if e.Note != "" {
		v.Note = &e.Note
	}
	return v
}
