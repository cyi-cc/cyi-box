package store

import (
	"database/sql"
	"errors"

	"github.com/cyi-cc/cyi-box/backend/internal/db"
)

func (s *Store) GetSetting(key string) (string, error) {
	v, err := s.q.GetSetting(storeCtx, key)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return v, err
}

func (s *Store) SetSetting(key, value string) error {
	return s.q.UpsertSetting(storeCtx, db.UpsertSettingParams{K: key, V: value})
}
