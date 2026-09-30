package store

import (
	"database/sql"
	"errors"
	"time"

	"github.com/cyi-cc/cyi-box/backend/internal/db"
)

// VaultEntry 密码箱条目：password_enc / totp_enc 为 AES-GCM 密文，离开 store 前必须解密
type VaultEntry struct {
	Id          int64
	UserID      int64
	Title       string
	Username    string
	PasswordEnc string
	Url         string
	Note        string
	TotpEnc     string
	CreatedAt   int64
	UpdatedAt   int64
}

func vaultFromRow(v db.Vault) VaultEntry {
	return VaultEntry{
		Id: v.ID, UserID: v.UserID, Title: v.Title, Username: v.Username,
		PasswordEnc: v.PasswordEnc, Url: v.Url, Note: v.Note, TotpEnc: v.TotpEnc,
		CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt,
	}
}

func (s *Store) ListVault(userID int64, keyword string) ([]VaultEntry, error) {
	rows, err := s.q.ListVault(storeCtx, db.ListVaultParams{
		UserID:  userID,
		Keyword: optStr(keyword),
	})
	if err != nil {
		return nil, err
	}
	items := make([]VaultEntry, 0, len(rows))
	for _, r := range rows {
		items = append(items, vaultFromRow(r))
	}
	return items, nil
}

func (s *Store) GetVault(userID, id int64) (VaultEntry, error) {
	v, err := s.q.GetVault(storeCtx, db.GetVaultParams{UserID: userID, ID: id})
	if errors.Is(err, sql.ErrNoRows) {
		return VaultEntry{}, ErrNotFound
	}
	return vaultFromRow(v), err
}

func (s *Store) SaveVault(e *VaultEntry) error {
	now := time.Now().Unix()
	if e.Id > 0 {
		n, err := s.q.UpdateVault(storeCtx, db.UpdateVaultParams{
			Title: e.Title, Username: e.Username, PasswordEnc: e.PasswordEnc,
			Url: e.Url, Note: e.Note, TotpEnc: e.TotpEnc,
			UpdatedAt: now, ID: e.Id, UserID: e.UserID,
		})
		if err != nil {
			return err
		}
		if n == 0 {
			return ErrNotFound
		}
		return nil
	}
	id, err := s.q.InsertVault(storeCtx, db.InsertVaultParams{
		UserID: e.UserID, Title: e.Title, Username: e.Username,
		PasswordEnc: e.PasswordEnc, Url: e.Url, Note: e.Note, TotpEnc: e.TotpEnc,
		CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		return err
	}
	e.Id, e.CreatedAt, e.UpdatedAt = id, now, now
	return nil
}

func (s *Store) DeleteVault(userID, id int64) error {
	n, err := s.q.DeleteVault(storeCtx, db.DeleteVaultParams{UserID: userID, ID: id})
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
