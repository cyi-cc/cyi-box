// Package dbx 通用数据库连接层：按 engine 生成 DSN、库/表/列 introspection、
// 受限执行 SQL（超时 + 行数上限）并把结果序列化为可 JSON 化结构。
// 支持 mysql / postgres / sqlite。
package dbx

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"
)

// Conn 已解密的连接信息（service 层组装）
type Conn struct {
	Engine   string
	Host     string
	Port     int64
	Username string
	Password string
	Database string // mysql/pg：默认库；sqlite：文件路径
	Params   string // 附加 DSN 参数，形如 "charset=utf8mb4&sslmode=disable"
}

func (c Conn) driverName() (string, error) {
	switch c.Engine {
	case "mysql":
		return "mysql", nil
	case "postgres", "postgresql", "pg":
		return "pgx", nil
	case "sqlite", "sqlite3":
		return "sqlite", nil
	}
	return "", fmt.Errorf("dbx: 不支持的引擎 %q", c.Engine)
}

// dsn 按引擎拼连接串；db 为空用连接默认库
func (c Conn) dsn(db string) string {
	if db == "" {
		db = c.Database
	}
	switch c.Engine {
	case "mysql":
		params := "parseTime=true&timeout=6s"
		if c.Params != "" {
			params = c.Params
			if !strings.Contains(params, "parseTime") {
				params += "&parseTime=true"
			}
			if !strings.Contains(params, "timeout") {
				params += "&timeout=6s"
			}
		}
		host := c.Host
		if c.Port > 0 {
			host += ":" + strconv.FormatInt(c.Port, 10)
		}
		return fmt.Sprintf("%s:%s@tcp(%s)/%s?%s", c.Username, c.Password, host, db, params)
	case "postgres", "postgresql", "pg":
		u := &url.URL{Scheme: "postgres", Host: c.Host, User: url.UserPassword(c.Username, c.Password), Path: "/" + db}
		if c.Port > 0 {
			u.Host += ":" + strconv.FormatInt(c.Port, 10)
		}
		q := u.Query()
		if c.Params != "" {
			if extra, err := url.ParseQuery(c.Params); err == nil {
				for k, vs := range extra {
					for _, v := range vs {
						q.Add(k, v)
					}
				}
			}
		}
		if q.Get("sslmode") == "" {
			q.Set("sslmode", "disable")
		}
		u.RawQuery = q.Encode()
		return u.String()
	default: // sqlite：host/port/user 不用，database 是文件路径
		path := db
		if path == "" {
			path = c.Database
		}
		return "file:" + path
	}
}

// Open 建连并 ping 验证
func Open(c Conn, db string) (*sql.DB, error) {
	driver, err := c.driverName()
	if err != nil {
		return nil, err
	}
	d, err := sql.Open(driver, c.dsn(db))
	if err != nil {
		return nil, err
	}
	d.SetMaxOpenConns(2)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	if err := d.PingContext(ctx); err != nil {
		d.Close()
		return nil, fmt.Errorf("dbx: 连接失败: %w", err)
	}
	return d, nil
}

// ---- introspection ----

// Databases 列出可访问的库；sqlite 恒为 main
func Databases(d *sql.DB, engine string) ([]string, error) {
	var q string
	switch engine {
	case "mysql":
		q = `SELECT schema_name FROM information_schema.schemata
		     WHERE schema_name NOT IN ('mysql','sys','information_schema','performance_schema') ORDER BY schema_name`
	case "postgres", "postgresql", "pg":
		q = `SELECT datname FROM pg_database WHERE NOT datistemplate ORDER BY datname`
	default: // sqlite
		return []string{"main"}, nil
	}
	return queryStrings(d, q)
}

type Table struct {
	Name string
	Type string // table | view
}

type Column struct {
	Table string
	Name  string
	Type  string
	Pk    bool
}

// TablesAndColumns 某库下的表/视图与列（sqlite 用 sqlite_master+pragma）
func TablesAndColumns(d *sql.DB, engine, database string) ([]Table, []Column, error) {
	if engine == "sqlite" || engine == "sqlite3" {
		return sqliteSchema(d)
	}
	var tq, cq string
	if engine == "mysql" {
		tq = `SELECT table_name, table_type FROM information_schema.tables
		      WHERE table_schema = ? ORDER BY table_name`
		cq = `SELECT table_name, column_name, column_type,
		             (column_key = 'PRI') AS pk FROM information_schema.columns
		      WHERE table_schema = ? ORDER BY table_name, ordinal_position`
	} else { // postgres：只取 public 及其余非系统 schema 的用户表
		tq = `SELECT table_name, table_type FROM information_schema.tables
		      WHERE table_schema NOT IN ('pg_catalog','information_schema')
		      ORDER BY table_name`
		cq = `SELECT c.table_name, c.column_name, c.data_type,
		             EXISTS (
		               SELECT 1 FROM pg_constraint con
		               JOIN pg_class rel ON rel.oid = con.conrelid
		               JOIN pg_namespace n ON n.oid = rel.relnamespace
		               JOIN pg_attribute att ON att.attrelid = con.conrelid
		                                AND att.attnum = ANY(con.conkey)
		               WHERE con.contype = 'p'
		                 AND n.nspname = c.table_schema
		                 AND rel.relname = c.table_name
		                 AND att.attname = c.column_name
		             ) AS pk
		      FROM information_schema.columns c
		      WHERE c.table_schema NOT IN ('pg_catalog','information_schema')
		      ORDER BY c.table_name, c.ordinal_position`
	}
	tables := []Table{}
	rows, err := d.Query(tq)
	if err != nil {
		return nil, nil, err
	}
	for rows.Next() {
		var t Table
		var ty string
		if err := rows.Scan(&t.Name, &ty); err != nil {
			rows.Close()
			return nil, nil, err
		}
		t.Type = "table"
		if strings.Contains(strings.ToLower(ty), "view") {
			t.Type = "view"
		}
		tables = append(tables, t)
	}
	rows.Close()

	cols := []Column{}
	rows2, err := d.Query(cq)
	if err != nil {
		return tables, cols, nil // 列拿不到不致命：补全降级
	}
	defer rows2.Close()
	for rows2.Next() {
		var c Column
		var pk any
		if err := rows2.Scan(&c.Table, &c.Name, &c.Type, &pk); err == nil {
			if b, ok := pk.(int64); ok {
				c.Pk = b != 0
			} else if b, ok := pk.(bool); ok {
				c.Pk = b
			}
			cols = append(cols, c)
		}
	}
	return tables, cols, nil
}

func sqliteSchema(d *sql.DB) ([]Table, []Column, error) {
	tables := []Table{}
	rows, err := d.Query(
		`SELECT name, type FROM sqlite_master WHERE type IN ('table','view')
		 AND name NOT LIKE 'sqlite_%' ORDER BY name`)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var t Table
		if err := rows.Scan(&t.Name, &t.Type); err != nil {
			return nil, nil, err
		}
		tables = append(tables, t)
	}
	rows.Close()

	cols := []Column{}
	for _, t := range tables {
		if t.Type != "table" {
			continue
		}
		pr, err := d.Query(`SELECT name, type, pk FROM pragma_table_info(?)`, t.Name)
		if err != nil {
			continue
		}
		for pr.Next() {
			var c Column
			var pk int64
			if err := pr.Scan(&c.Name, &c.Type, &pk); err == nil {
				c.Table = t.Name
				c.Pk = pk != 0
				cols = append(cols, c)
			}
		}
		pr.Close()
	}
	return tables, cols, nil
}

func queryStrings(d *sql.DB, q string, args ...any) ([]string, error) {
	rows, err := d.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// ---- 查询执行 ----

type Result struct {
	IsSelect     bool
	Columns      []string
	Rows         [][]any
	RowCount     int64
	AffectedRows int64
	Truncated    bool
	ElapsedMs    int64
}

const maxRowsCap = 10000

// returnsRows 按首个关键字预判语句是否产生结果集；
// 不能只靠 QueryContext 试探——sqlite 会把 INSERT 当空结果集执行掉，拿不到 affected
func returnsRows(sqlText string) bool {
	s := strings.TrimLeft(sqlText, " \t\r\n(")
	for strings.HasPrefix(s, "--") {
		if i := strings.IndexByte(s, '\n'); i >= 0 {
			s = strings.TrimLeft(s[i+1:], " \t\r\n(")
		} else {
			return true
		}
	}
	if i := strings.IndexAny(s, " \t\r\n("); i > 0 {
		s = s[:i]
	}
	switch strings.ToUpper(s) {
	case "SELECT", "WITH", "SHOW", "PRAGMA", "EXPLAIN", "VALUES", "TABLE", "DESC", "DESCRIBE":
		return true
	}
	return false
}

// Query 执行单条 SQL：有结果集走行收集（cap 截断），否则回 affected rows
func Query(ctx context.Context, d *sql.DB, sqlText string, maxRows int64) (*Result, error) {
	if maxRows <= 0 || maxRows > maxRowsCap {
		maxRows = 500
	}
	start := time.Now()
	if !returnsRows(sqlText) {
		res, err := d.ExecContext(ctx, sqlText)
		if err != nil {
			return nil, err
		}
		aff, _ := res.RowsAffected()
		return &Result{IsSelect: false, AffectedRows: aff,
			ElapsedMs: time.Since(start).Milliseconds(),
			Columns:   []string{}, Rows: [][]any{}}, nil
	}
	r := &Result{Columns: []string{}, Rows: [][]any{}}

	rows, err := d.QueryContext(ctx, sqlText)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	r.IsSelect = true
	r.Columns = cols
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		row := make([]any, len(cols))
		for i, v := range vals {
			row[i] = cellJSON(v)
		}
		r.Rows = append(r.Rows, row)
		if int64(len(r.Rows)) >= maxRows {
			r.Truncated = true
			break
		}
	}
	r.RowCount = int64(len(r.Rows))
	r.ElapsedMs = time.Since(start).Milliseconds()
	return r, nil
}

// cellJSON 把驱动返回值规整为 JSON 友好类型
func cellJSON(v any) any {
	switch t := v.(type) {
	case nil:
		return nil
	case []byte:
		if utf8.Valid(t) {
			return string(t)
		}
		return "0x" + base64.StdEncoding.EncodeToString(t)
	case time.Time:
		return t.Format("2006-01-02 15:04:05")
	case driver.Valuer: // 防御：部分驱动包一层
		vv, _ := t.Value()
		return cellJSON(vv)
	default:
		return v
	}
}

// Exec 明确写路径（删改/DDL），回受影响行数
func Exec(ctx context.Context, d *sql.DB, sqlText string) (*Result, error) {
	start := time.Now()
	res, err := d.ExecContext(ctx, sqlText)
	if err != nil {
		return nil, err
	}
	aff, _ := res.RowsAffected()
	return &Result{IsSelect: false, AffectedRows: aff,
		ElapsedMs: time.Since(start).Milliseconds(),
		Columns:   []string{}, Rows: [][]any{}}, nil
}

var ErrNoPK = errors.New("dbx: no primary key")
