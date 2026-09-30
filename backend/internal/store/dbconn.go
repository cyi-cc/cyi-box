package store

import (
	"database/sql"
	"errors"
	"time"

	"github.com/cyi-cc/cyi-box/backend/internal/db"
)

// Dbconn 数据库连接条目：password_enc 为 AES-GCM 密文；sqlite 时 database 存文件路径
type Dbconn struct {
	Id          int64
	Name        string
	Engine      string // mysql | postgres | sqlite
	Host        string
	Port        int64
	Username    string
	PasswordEnc string
	Database    string
	Params      string // DSN 附加参数，如 mysql 的 charset、pg 的 sslmode
	CreatedAt   int64
	UpdatedAt   int64
}

func dbconnFromRow(e db.Dbconn) Dbconn {
	return Dbconn{
		Id: e.ID, Name: e.Name, Engine: e.Engine, Host: e.Host, Port: e.Port,
		Username: e.Username, PasswordEnc: e.PasswordEnc, Database: e.Database,
		Params: e.Params, CreatedAt: e.CreatedAt, UpdatedAt: e.UpdatedAt,
	}
}

func (s *Store) ListDbconns() ([]Dbconn, error) {
	rows, err := s.q.ListDbconns(storeCtx)
	if err != nil {
		return nil, err
	}
	items := make([]Dbconn, 0, len(rows))
	for _, r := range rows {
		items = append(items, dbconnFromRow(r))
	}
	return items, nil
}

func (s *Store) GetDbconn(id int64) (Dbconn, error) {
	e, err := s.q.GetDbconn(storeCtx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return Dbconn{}, ErrNotFound
	}
	return dbconnFromRow(e), err
}

func (s *Store) SaveDbconn(e *Dbconn) error {
	now := time.Now().Unix()
	if e.Id > 0 {
		n, err := s.q.UpdateDbconn(storeCtx, db.UpdateDbconnParams{
			Name: e.Name, Engine: e.Engine, Host: e.Host, Port: e.Port,
			Username: e.Username, PasswordEnc: e.PasswordEnc,
			Database: e.Database, Params: e.Params, UpdatedAt: now, ID: e.Id,
		})
		if err != nil {
			return err
		}
		if n == 0 {
			return ErrNotFound
		}
		return nil
	}
	id, err := s.q.InsertDbconn(storeCtx, db.InsertDbconnParams{
		Name: e.Name, Engine: e.Engine, Host: e.Host, Port: e.Port,
		Username: e.Username, PasswordEnc: e.PasswordEnc,
		Database: e.Database, Params: e.Params, CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		return err
	}
	e.Id, e.CreatedAt, e.UpdatedAt = id, now, now
	return nil
}

func (s *Store) DeleteDbconn(id int64) error {
	n, err := s.q.DeleteDbconn(storeCtx, id)
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
