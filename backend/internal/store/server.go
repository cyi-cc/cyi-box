package store

import (
	"database/sql"
	"errors"
	"time"

	"github.com/cyi-cc/cyi-box/backend/internal/db"
)

// Server SSH 服务器条目：secret_enc 为 AES-GCM 密文，存密码或 PEM 私钥，
// 明文只在连接的瞬间存在于内存，不落盘不下发
type Server struct {
	Id        int64
	Name      string
	Host      string
	Port      int64
	Username  string
	AuthType  string // password | key
	SecretEnc string
	Note      string
	CreatedAt int64
	UpdatedAt int64
}

func serverFromRow(e db.Server) Server {
	return Server{
		Id: e.ID, Name: e.Name, Host: e.Host, Port: e.Port, Username: e.Username,
		AuthType: e.AuthType, SecretEnc: e.SecretEnc, Note: e.Note,
		CreatedAt: e.CreatedAt, UpdatedAt: e.UpdatedAt,
	}
}

func (s *Store) ListServers() ([]Server, error) {
	rows, err := s.q.ListServers(storeCtx)
	if err != nil {
		return nil, err
	}
	items := make([]Server, 0, len(rows))
	for _, r := range rows {
		items = append(items, serverFromRow(r))
	}
	return items, nil
}

func (s *Store) GetServer(id int64) (Server, error) {
	e, err := s.q.GetServer(storeCtx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return Server{}, ErrNotFound
	}
	return serverFromRow(e), err
}

func (s *Store) SaveServer(e *Server) error {
	now := time.Now().Unix()
	if e.Id > 0 {
		n, err := s.q.UpdateServer(storeCtx, db.UpdateServerParams{
			Name: e.Name, Host: e.Host, Port: e.Port, Username: e.Username,
			AuthType: e.AuthType, SecretEnc: e.SecretEnc, Note: e.Note,
			UpdatedAt: now, ID: e.Id,
		})
		if err != nil {
			return err
		}
		if n == 0 {
			return ErrNotFound
		}
		return nil
	}
	id, err := s.q.InsertServer(storeCtx, db.InsertServerParams{
		Name: e.Name, Host: e.Host, Port: e.Port, Username: e.Username,
		AuthType: e.AuthType, SecretEnc: e.SecretEnc, Note: e.Note,
		CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		return err
	}
	e.Id, e.CreatedAt, e.UpdatedAt = id, now, now
	return nil
}

func (s *Store) DeleteServer(id int64) error {
	n, err := s.q.DeleteServer(storeCtx, id)
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) CountServers() (int64, error) {
	return s.q.CountServers(storeCtx)
}
