package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/cyi-cc/fun"

	"github.com/cyi-cc/cyi-box/backend/internal/config"
	"github.com/cyi-cc/cyi-box/backend/internal/dbx"
	"github.com/cyi-cc/cyi-box/backend/internal/dto"
	"github.com/cyi-cc/cyi-box/backend/internal/store"
	"github.com/cyi-cc/cyi-box/backend/internal/vault"
)

// DbMgrSvc 通用数据库管理器：连接密码 AES-GCM 落库，全部端点仅管理员；
// 查询固定 20s 超时 + 行数封顶，防止一条烂 SQL 拖死进程
type DbMgrSvc struct {
	fun.Ctx
	Store *store.Store
	Cfg   *config.Config `fun:"auto"`
}

func (s *DbMgrSvc) List() ([]dto.DbconnView, error) {
	items, err := s.Store.ListDbconns()
	if err != nil {
		return nil, fmt.Errorf("dbm: list failed: %w", err)
	}
	views := make([]dto.DbconnView, 0, len(items))
	for _, e := range items {
		views = append(views, dbconnView(e))
	}
	return views, nil
}

func (s *DbMgrSvc) Save(d dto.SaveDbconnDto) (dto.DbconnView, error) {
	name := strings.TrimSpace(d.Name)
	engine := strings.TrimSpace(d.Engine)
	switch engine {
	case "mysql", "postgres", "sqlite":
	default:
		return dto.DbconnView{}, fun.Error(4001, "引擎只支持 mysql / postgres / sqlite")
	}
	if name == "" || len(name) > 64 {
		return dto.DbconnView{}, fun.Error(4001, "名称必填且不超过 64 字符")
	}

	e := store.Dbconn{Name: name, Engine: engine}
	if d.Id != nil {
		old, err := s.Store.GetDbconn(*d.Id)
		if errors.Is(err, store.ErrNotFound) {
			return dto.DbconnView{}, fun.Error(4001, "连接不存在")
		}
		if err != nil {
			return dto.DbconnView{}, err
		}
		e = old
		e.Name, e.Engine = name, engine
	}
	if d.Host != nil {
		e.Host = strings.TrimSpace(*d.Host)
	}
	if d.Port != nil {
		e.Port = *d.Port
	}
	if d.Username != nil {
		e.Username = strings.TrimSpace(*d.Username)
	}
	if d.Database != nil {
		e.Database = strings.TrimSpace(*d.Database)
	}
	if d.Params != nil {
		e.Params = strings.TrimSpace(*d.Params)
	}
	if d.Password != nil {
		pw := *d.Password
		if pw == "" {
			e.PasswordEnc = ""
		} else {
			enc, err := vault.Encrypt(s.Cfg.VaultKey, pw)
			if err != nil {
				return dto.DbconnView{}, err
			}
			e.PasswordEnc = enc
		}
	}
	if e.Engine != "sqlite" && e.Host == "" {
		return dto.DbconnView{}, fun.Error(4001, "主机地址必填")
	}
	if e.Engine == "sqlite" && e.Database == "" {
		return dto.DbconnView{}, fun.Error(4001, "sqlite 需在「数据库」填文件路径")
	}
	if err := s.Store.SaveDbconn(&e); err != nil {
		return dto.DbconnView{}, fmt.Errorf("dbm: save failed: %w", err)
	}
	return dbconnView(e), nil
}

func (s *DbMgrSvc) Delete(d dto.DbconnIdDto) (dto.OkView, error) {
	if err := s.Store.DeleteDbconn(d.Id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return dto.OkView{}, fun.Error(4001, "连接不存在")
		}
		return dto.OkView{}, fmt.Errorf("dbm: delete failed: %w", err)
	}
	return dto.OkView{Message: "已删除"}, nil
}

// openConn 解密码 + 建连 + ping
func openConn(st *store.Store, key []byte, id int64, database string) (*sql.DB, store.Dbconn, error) {
	e, err := st.GetDbconn(id)
	if errors.Is(err, store.ErrNotFound) {
		return nil, e, fun.Error(4001, "连接不存在")
	}
	if err != nil {
		return nil, e, err
	}
	pw := ""
	if e.PasswordEnc != "" {
		pw, err = vault.Decrypt(key, e.PasswordEnc)
		if err != nil {
			return nil, e, fmt.Errorf("dbm: decrypt password failed: %w", err)
		}
	}
	d, err := dbx.Open(dbx.Conn{
		Engine: e.Engine, Host: e.Host, Port: e.Port, Username: e.Username,
		Password: pw, Database: e.Database, Params: e.Params,
	}, database)
	return d, e, err
}

func (s *DbMgrSvc) open(id int64, database string) (*sql.DB, store.Dbconn, error) {
	return openConn(s.Store, s.Cfg.VaultKey, id, database)
}

// Test 连通性测试
func (s *DbMgrSvc) Test(d dto.DbconnIdDto) (dto.OkView, error) {
	c, _, err := s.open(d.Id, "")
	if err != nil {
		return dto.OkView{}, fun.Error(4001, err.Error())
	}
	defer c.Close()
	return dto.OkView{Message: "连接成功"}, nil
}

func (s *DbMgrSvc) Databases(d dto.DbScopeDto) (dto.DbDatabasesView, error) {
	c, e, err := s.open(d.Id, "")
	if err != nil {
		return dto.DbDatabasesView{}, err
	}
	defer c.Close()
	names, err := dbx.Databases(c, e.Engine)
	if err != nil {
		return dto.DbDatabasesView{}, fmt.Errorf("dbm: list databases failed: %w", err)
	}
	return dto.DbDatabasesView{Names: names}, nil
}

func (s *DbMgrSvc) Tables(d dto.DbScopeDto) (dto.DbTablesView, error) {
	db := ""
	if d.Database != nil {
		db = *d.Database
	}
	c, e, err := s.open(d.Id, db)
	if err != nil {
		return dto.DbTablesView{}, err
	}
	defer c.Close()
	tables, cols, err := dbx.TablesAndColumns(c, e.Engine, db)
	if err != nil {
		return dto.DbTablesView{}, fmt.Errorf("dbm: list tables failed: %w", err)
	}
	v := dto.DbTablesView{Tables: []dto.DbTableView{}, Columns: []dto.DbColumnView{}}
	for _, t := range tables {
		v.Tables = append(v.Tables, dto.DbTableView{Name: t.Name, Type: t.Type})
	}
	for _, cl := range cols {
		v.Columns = append(v.Columns, dto.DbColumnView{
			Table: cl.Table, Name: cl.Name, Type: cl.Type, Pk: cl.Pk,
		})
	}
	return v, nil
}

const dbQueryTimeout = 20 * time.Second

// Query 执行单条 SQL：SELECT 回结果集（JSON 行串），其余回 affected rows
func (s *DbMgrSvc) Query(d dto.DbQueryDto) (dto.DbQueryView, error) {
	sqlText := strings.TrimSpace(d.Sql)
	if sqlText == "" {
		return dto.DbQueryView{}, fun.Error(4001, "SQL 不能为空")
	}
	db := ""
	if d.Database != nil {
		db = *d.Database
	}
	c, _, err := s.open(d.Id, db)
	if err != nil {
		return dto.DbQueryView{}, err
	}
	defer c.Close()

	maxRows := int64(500)
	if d.MaxRows != nil && *d.MaxRows > 0 {
		maxRows = *d.MaxRows
	}
	ctx, cancel := context.WithTimeout(context.Background(), dbQueryTimeout)
	defer cancel()
	res, err := dbx.Query(ctx, c, sqlText, maxRows)
	if err != nil {
		return dto.DbQueryView{}, fun.Error(4002, "执行失败："+err.Error())
	}
	rowsJson, _ := json.Marshal(res.Rows)
	return dto.DbQueryView{
		IsSelect:     res.IsSelect,
		Columns:      res.Columns,
		RowsJson:     string(rowsJson),
		RowCount:     res.RowCount,
		AffectedRows: res.AffectedRows,
		Truncated:    res.Truncated,
		ElapsedMs:    res.ElapsedMs,
	}, nil
}

func dbconnView(e store.Dbconn) dto.DbconnView {
	return dto.DbconnView{
		Id:          e.Id,
		Name:        e.Name,
		Engine:      e.Engine,
		Host:        e.Host,
		Port:        e.Port,
		Username:    e.Username,
		Database:    e.Database,
		HasPassword: e.PasswordEnc != "",
		CreatedAt:   e.CreatedAt,
	}
}
