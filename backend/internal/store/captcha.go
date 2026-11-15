package store

import (
	"crypto/rand"
	"encoding/hex"
	"database/sql"
	"errors"
	"time"

	"github.com/cyi-cc/cyi-box/backend/internal/db"
)

// CaptchaKey 滑块验证码接入方密钥对：sitekey(pk_) 嵌页面出题用，
// skey(sk_) 对方后端核销 token 用；domains 逗号分隔白名单（空=不限）。
type CaptchaKey struct {
	Id        int64
	Sitekey   string
	Skey      string
	Name      string
	Domains   string
	Enabled   bool
	UsedCount int64
	CreatedAt int64
}

func b2i(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

func captchaKeyFromRow(k db.CaptchaKey) CaptchaKey {
	return CaptchaKey{
		Id: k.ID, Sitekey: k.Sitekey, Skey: k.Skey, Name: k.Name,
		Domains: k.Domains, Enabled: k.Enabled == 1, UsedCount: k.UsedCount,
		CreatedAt: k.CreatedAt,
	}
}

func (s *Store) CaptchaKeyBySitekey(sitekey string) (CaptchaKey, error) {
	k, err := s.q.GetCaptchaKeyBySitekey(storeCtx, sitekey)
	if errors.Is(err, sql.ErrNoRows) {
		return CaptchaKey{}, ErrNotFound
	}
	if err != nil {
		return CaptchaKey{}, err
	}
	return captchaKeyFromRow(k), nil
}

func (s *Store) CaptchaKeyBySecret(skey string) (CaptchaKey, error) {
	k, err := s.q.GetCaptchaKeyBySecret(storeCtx, skey)
	if errors.Is(err, sql.ErrNoRows) {
		return CaptchaKey{}, ErrNotFound
	}
	if err != nil {
		return CaptchaKey{}, err
	}
	return captchaKeyFromRow(k), nil
}

func (s *Store) ListCaptchaKeys() ([]CaptchaKey, error) {
	rows, err := s.q.ListCaptchaKeys(storeCtx)
	if err != nil {
		return nil, err
	}
	out := make([]CaptchaKey, 0, len(rows))
	for _, r := range rows {
		out = append(out, captchaKeyFromRow(r))
	}
	return out, nil
}

func (s *Store) InsertCaptchaKey(k *CaptchaKey) error {
	id, err := s.q.InsertCaptchaKey(storeCtx, db.InsertCaptchaKeyParams{
		Sitekey: k.Sitekey, Skey: k.Skey, Name: k.Name, Domains: k.Domains,
		Enabled: b2i(k.Enabled), CreatedAt: time.Now().Unix(),
	})
	if err != nil {
		return err
	}
	k.Id = id
	return nil
}

func (s *Store) UpdateCaptchaKey(k *CaptchaKey) error {
	return s.q.UpdateCaptchaKey(storeCtx, db.UpdateCaptchaKeyParams{
		Name: k.Name, Domains: k.Domains, Enabled: b2i(k.Enabled), ID: k.Id,
	})
}

func (s *Store) DeleteCaptchaKey(id int64) error {
	_, err := s.q.DeleteCaptchaKey(storeCtx, id)
	return err
}

func (s *Store) TouchCaptchaKey(id int64) {
	_ = s.q.TouchCaptchaKey(storeCtx, id)
}

// SeedCaptchaKey 种子内置密钥：本站登录页的滑块用固定 sitekey，缺失才补
func (s *Store) SeedCaptchaKey() error {
	keys, err := s.ListCaptchaKeys()
	if err != nil {
		return err
	}
	for _, k := range keys {
		if k.Sitekey == InternalCaptchaSitekey {
			return nil
		}
	}
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return err
	}
	return s.InsertCaptchaKey(&CaptchaKey{
		Sitekey: InternalCaptchaSitekey, Skey: "sk_" + hex.EncodeToString(b),
		Name: "内置（本站登录）", Enabled: true,
	})
}

// InternalCaptchaSitekey 本站登录页滑块的固定 sitekey
const InternalCaptchaSitekey = "pk_internal"
