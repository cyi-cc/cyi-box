package store

import (
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

// ---- captcha_logs 调用流水 ----

// CaptchaLog 一次接入方调用：event ∈ widget|verify|check
type CaptchaLog struct {
	Id        int64
	KeyID     int64
	Sitekey   string
	Event     string
	Ok        bool
	Ip        string
	Msg       string
	CreatedAt int64
}

// LogCaptcha 写一条流水并顺带裁到最近 500 条（排障用，不做长期审计）
func (s *Store) LogCaptcha(l CaptchaLog) {
	l.CreatedAt = time.Now().Unix()
	_ = s.q.InsertCaptchaLog(storeCtx, db.InsertCaptchaLogParams{
		KeyID: l.KeyID, Sitekey: l.Sitekey, Event: l.Event,
		Ok: b2i(l.Ok), Ip: l.Ip, Msg: l.Msg, CreatedAt: l.CreatedAt,
	})
	_ = s.q.TrimCaptchaLogs(storeCtx)
}

func (s *Store) ListCaptchaLogs(limit int64) ([]CaptchaLog, error) {
	rows, err := s.q.ListCaptchaLogs(storeCtx, limit)
	if err != nil {
		return nil, err
	}
	out := make([]CaptchaLog, 0, len(rows))
	for _, r := range rows {
		out = append(out, CaptchaLog{
			Id: r.ID, KeyID: r.KeyID, Sitekey: r.Sitekey, Event: r.Event,
			Ok: r.Ok == 1, Ip: r.Ip, Msg: r.Msg, CreatedAt: r.CreatedAt,
		})
	}
	return out, nil
}
