// Package store 数据门面层：CRUD 委托给 sqlc 生成的 internal/db.Queries（类型安全），
// 只保留 sqlc 表达不了的逻辑——会话滑动续期、聚合映射、种子数据，以及对服务层稳定的
// 领域结构体（Id 命名、Enabled bool）与 ErrNotFound 语义。
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/cyi-cc/cyi-box/backend/internal/config"
	"github.com/cyi-cc/cyi-box/backend/internal/db"
)

type Store struct {
	Cfg *config.Config `fun:"auto"`
	DB  *sql.DB
	q   *db.Queries
}

// Q 暴露 sqlc 查询器：新代码可直接走类型安全 API，门面方法逐步收敛
func (s *Store) Q() *db.Queries { return s.q }

func (s *Store) New() error {
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)",
		s.Cfg.DBPath)
	conn, err := sql.Open("sqlite", dsn)
	if err != nil {
		return fmt.Errorf("store: open sqlite failed: %w", err)
	}
	// SQLite 单写者：串行化所有连接，避免 database is locked
	conn.SetMaxOpenConns(1)
	s.DB = conn
	s.q = db.New(conn)
	if err := s.migrate(); err != nil {
		return err
	}
	return nil
}

// migrate 表结构唯一来源是 internal/db/schema.sql（sqlc 与运行时共用）；
// 列级增量（老库补列）用探测式 ALTER，CREATE TABLE IF NOT EXISTS 覆盖不到。
func (s *Store) migrate() error {
	if _, err := s.DB.Exec(db.SchemaDDL); err != nil {
		return fmt.Errorf("store: migrate failed: %w", err)
	}
	if err := s.migrateSingleUser(); err != nil {
		return err
	}
	// 代理池功能已移除：清掉老库遗留的表
	if _, err := s.DB.Exec(`DROP TABLE IF EXISTS proxies;
DROP TABLE IF EXISTS proxy_regions;
DROP TABLE IF EXISTS proxy_syncs;
DROP TABLE IF EXISTS proxy_keys;`); err != nil {
		return fmt.Errorf("store: drop proxy tables failed: %w", err)
	}
	var dummy int64
	if err := s.DB.QueryRow("SELECT COUNT(app_id) FROM license_cards").Scan(&dummy); err != nil {
		if _, err := s.DB.Exec("ALTER TABLE license_cards ADD COLUMN app_id INTEGER NOT NULL DEFAULT 0"); err != nil {
			return fmt.Errorf("store: migrate license_cards.app_id failed: %w", err)
		}
	}
	if _, err := s.DB.Exec("CREATE INDEX IF NOT EXISTS idx_license_cards_app ON license_cards(app_id)"); err != nil {
		return fmt.Errorf("store: migrate license_cards index failed: %w", err)
	}
	// 记忆库全文索引：FTS5 外挂内容表 + 触发器同步（放这里而非 schema.sql，
	// sqlc 的 sqlite 解析器不认 CREATE VIRTUAL TABLE/TRIGGER）
	if _, err := s.DB.Exec(memFTSDDL); err != nil {
		return fmt.Errorf("store: migrate memories_fts failed: %w", err)
	}
	// rebuild 幂等：给触发器建成前已存在的行补索引
	if _, err := s.DB.Exec(`INSERT INTO memories_fts(memories_fts) VALUES('rebuild')`); err != nil {
		return fmt.Errorf("store: rebuild memories_fts failed: %w", err)
	}
	return nil
}

// migrateSingleUser 老库升级：users 表存在则把首个管理员密码哈希并入 settings，
// 重建 sessions/vault/bookmarks/files/shares 去掉 user_id 列，最后 DROP users。
// FK 临时关闭避免重建期父表删除冲突（单连接，PRAGMA 对后续语句生效）。
func (s *Store) migrateSingleUser() error {
	var n int64
	if err := s.DB.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='users'`).Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		return nil
	}
	var hash string
	_ = s.DB.QueryRow(`SELECT password_hash FROM users
		ORDER BY CASE WHEN role='admin' AND status='active' THEN 0 ELSE 1 END, id LIMIT 1`).Scan(&hash)
	if hash != "" {
		if _, err := s.DB.Exec(`INSERT INTO settings(k,v) VALUES('admin_password_hash',?) ON CONFLICT(k) DO NOTHING`, hash); err != nil {
			return fmt.Errorf("store: migrate admin password failed: %w", err)
		}
	}
	stmts := []string{
		"PRAGMA foreign_keys=OFF",
		`CREATE TABLE sessions_new(token TEXT PRIMARY KEY, created_at INTEGER NOT NULL, expires_at INTEGER NOT NULL)`,
		`INSERT INTO sessions_new SELECT token, created_at, expires_at FROM sessions`,
		`DROP TABLE sessions`,
		`ALTER TABLE sessions_new RENAME TO sessions`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_expiry ON sessions(expires_at)`,
		`CREATE TABLE vault_new(id INTEGER PRIMARY KEY AUTOINCREMENT, title TEXT NOT NULL, username TEXT NOT NULL DEFAULT '', password_enc TEXT NOT NULL DEFAULT '', url TEXT NOT NULL DEFAULT '', note TEXT NOT NULL DEFAULT '', totp_enc TEXT NOT NULL DEFAULT '', created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL)`,
		`INSERT INTO vault_new SELECT id, title, username, password_enc, url, note, totp_enc, created_at, updated_at FROM vault`,
		`DROP TABLE vault`,
		`ALTER TABLE vault_new RENAME TO vault`,
		`CREATE TABLE bookmarks_new(id INTEGER PRIMARY KEY AUTOINCREMENT, title TEXT NOT NULL, url TEXT NOT NULL, icon TEXT NOT NULL DEFAULT '', sort INTEGER NOT NULL DEFAULT 0, created_at INTEGER NOT NULL)`,
		`INSERT INTO bookmarks_new SELECT id, title, url, icon, sort, created_at FROM bookmarks`,
		`DROP TABLE bookmarks`,
		`ALTER TABLE bookmarks_new RENAME TO bookmarks`,
		`CREATE TABLE files_new(id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL, stored TEXT NOT NULL, size INTEGER NOT NULL DEFAULT 0, mime TEXT NOT NULL DEFAULT '', created_at INTEGER NOT NULL)`,
		`INSERT INTO files_new SELECT id, name, stored, size, mime, created_at FROM files`,
		`DROP TABLE files`,
		`ALTER TABLE files_new RENAME TO files`,
		`CREATE TABLE shares_new(id INTEGER PRIMARY KEY AUTOINCREMENT, code TEXT NOT NULL UNIQUE, file_id INTEGER NOT NULL REFERENCES files(id) ON DELETE CASCADE, file_name TEXT NOT NULL DEFAULT '', expires_at INTEGER NOT NULL DEFAULT 0, downloads INTEGER NOT NULL DEFAULT 0, created_at INTEGER NOT NULL)`,
		`INSERT INTO shares_new SELECT id, code, file_id, file_name, expires_at, downloads, created_at FROM shares`,
		`DROP TABLE shares`,
		`ALTER TABLE shares_new RENAME TO shares`,
		`CREATE INDEX IF NOT EXISTS idx_shares_file ON shares(file_id)`,
		`DROP TABLE users`,
		"PRAGMA foreign_keys=ON",
	}
	for _, q := range stmts {
		if _, err := s.DB.Exec(q); err != nil {
			return fmt.Errorf("store: single-user migration failed [%.40s]: %w", q, err)
		}
	}
	log.Print("store: migrated to single-user schema (users table dropped)")
	return nil
}

var storeCtx = context.Background()

func optStr(s string) sql.NullString {
	return sql.NullString{String: s, Valid: true}
}

// SeedPassword 无密码时把初始密码哈希写入 settings（单用户模型）
func (s *Store) SeedPassword(passwordHash string) error {
	v, err := s.GetSetting(PasswordHashKey)
	if err != nil {
		return err
	}
	if v != "" {
		return nil
	}
	if err := s.SetSetting(PasswordHashKey, passwordHash); err != nil {
		return fmt.Errorf("store: seed password failed: %w", err)
	}
	log.Print("store: seeded admin password")
	return nil
}

// PasswordHashKey settings 表里的登录密码 bcrypt 哈希键
const PasswordHashKey = "admin_password_hash"

func (s *Store) PasswordHash() (string, error) { return s.GetSetting(PasswordHashKey) }

func (s *Store) SetPasswordHash(hash string) error { return s.SetSetting(PasswordHashKey, hash) }

// SeedTools 空表时写入两条示例工具，演示工具箱菜单的数据驱动渲染
func (s *Store) SeedTools() error {
	tools, err := s.q.ListTools(storeCtx)
	if err != nil {
		return fmt.Errorf("store: count tools failed: %w", err)
	}
	if len(tools) > 0 {
		return nil
	}
	for _, t := range []db.InsertToolParams{
		{Name: "JSON 格式化", Icon: "code-outline", Url: "/tools/json", Description: "粘贴即格式化与校验 JSON", Sort: 1, Enabled: true},
		{Name: "时间戳转换", Icon: "time-outline", Url: "/tools/timestamp", Description: "Unix 时间戳与日期互转", Sort: 2, Enabled: true},
	} {
		if _, err := s.q.InsertTool(storeCtx, t); err != nil {
			return err
		}
	}
	return nil
}

var ErrNotFound = errors.New("record not found")

// ---- sessions ----

func (s *Store) CreateSession(token string, ttl time.Duration) error {
	now := time.Now()
	return s.q.InsertSession(storeCtx, db.InsertSessionParams{
		Token:     token,
		CreatedAt: now.Unix(), ExpiresAt: now.Add(ttl).Unix(),
	})
}

// CheckSession 校验会话 token。
// 滑动续期：会话本身 ttl 过期即失效；存活但距签发/上次续期超过 renewAfter 时，
// 顺手把 expires_at 顺延回 now+ttl——活跃用户不掉线，闲置 ttl 时长强制下线。
func (s *Store) CheckSession(token string, ttl, renewAfter time.Duration) error {
	exp, err := s.q.GetSessionExpiry(storeCtx, token)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	now := time.Now()
	if now.Unix() > exp {
		_ = s.q.DeleteSession(storeCtx, token)
		return ErrNotFound
	}
	lastRefresh := time.Unix(exp, 0).Add(-ttl)
	if now.Sub(lastRefresh) >= renewAfter {
		_ = s.q.TouchSession(storeCtx, db.TouchSessionParams{
			ExpiresAt: now.Add(ttl).Unix(), Token: token,
		})
	}
	return nil
}

func (s *Store) DeleteSession(token string) error {
	return s.q.DeleteSession(storeCtx, token)
}

// DeleteOtherSessions 改密后吊销除当前外的全部会话
func (s *Store) DeleteOtherSessions(keepToken string) error {
	return s.q.DeleteOtherSessions(storeCtx, keepToken)
}

func (s *Store) CountActiveSessions() (int64, error) {
	return s.q.CountActiveSessions(storeCtx, time.Now().Unix())
}

func (s *Store) CountTodayLogins() (int64, error) {
	return s.q.CountTodayLogins(storeCtx)
}

// LoginTrend 最近 days 天每日登录数（按会话创建数统计），缺口补零由调用方做
func (s *Store) LoginTrend(days int64) (map[string]int64, error) {
	rows, err := s.q.LoginTrend(storeCtx, time.Now().AddDate(0, 0, int(-days)).Unix())
	if err != nil {
		return nil, err
	}
	m := map[string]int64{}
	for _, r := range rows {
		if d, ok := r.D.(string); ok {
			m[d] = r.C
		}
	}
	return m, nil
}

// ---- tools ----

type Tool struct {
	Id          int64
	Name        string
	Icon        string
	Url         string
	Description string
	Sort        int64
	Enabled     bool
}

func toolFromRow(t db.Tool) Tool {
	return Tool{
		Id: t.ID, Name: t.Name, Icon: t.Icon, Url: t.Url,
		Description: t.Description, Sort: t.Sort, Enabled: t.Enabled,
	}
}

func (s *Store) ListTools(enabledOnly bool) ([]Tool, error) {
	var rows []db.Tool
	var err error
	if enabledOnly {
		rows, err = s.q.ListEnabledTools(storeCtx)
	} else {
		rows, err = s.q.ListTools(storeCtx)
	}
	if err != nil {
		return nil, err
	}
	tools := make([]Tool, 0, len(rows))
	for _, r := range rows {
		tools = append(tools, toolFromRow(r))
	}
	return tools, nil
}

func (s *Store) SaveTool(t *Tool) error {
	if t.Id > 0 {
		return s.q.UpdateTool(storeCtx, db.UpdateToolParams{
			Name: t.Name, Icon: t.Icon, Url: t.Url, Description: t.Description,
			Sort: t.Sort, Enabled: t.Enabled, ID: t.Id,
		})
	}
	id, err := s.q.InsertTool(storeCtx, db.InsertToolParams{
		Name: t.Name, Icon: t.Icon, Url: t.Url, Description: t.Description,
		Sort: t.Sort, Enabled: t.Enabled,
	})
	if err != nil {
		return err
	}
	t.Id = id
	return nil
}

func (s *Store) DeleteTool(id int64) error {
	return s.q.DeleteTool(storeCtx, id)
}

func (s *Store) CountTools() (int64, error) {
	return s.q.CountTools(storeCtx)
}

// NormalizeName 服务层共用的名字规整
func NormalizeName(s string) string { return strings.TrimSpace(s) }
