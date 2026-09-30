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
	// proxies.source 是后加的列：老库探测缺失则 ALTER（COUNT 空表也返回行，不误判）
	var dummy int64
	if err := s.DB.QueryRow("SELECT COUNT(source) FROM proxies").Scan(&dummy); err != nil {
		if _, err := s.DB.Exec("ALTER TABLE proxies ADD COLUMN source TEXT NOT NULL DEFAULT ''"); err != nil {
			return fmt.Errorf("store: migrate proxies.source failed: %w", err)
		}
	}
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

var storeCtx = context.Background()

func optStr(s string) sql.NullString {
	return sql.NullString{String: s, Valid: true}
}

// SeedAdmin 空库时写入首个管理员；已有用户则跳过
func (s *Store) SeedAdmin(passwordHash string) error {
	n, err := s.q.CountAllUsers(storeCtx)
	if err != nil {
		return fmt.Errorf("store: count users failed: %w", err)
	}
	if n > 0 {
		return nil
	}
	now := time.Now().Unix()
	_, err = s.q.InsertUser(storeCtx, db.InsertUserParams{
		Name: s.Cfg.AdminName, PasswordHash: passwordHash, Role: "admin",
		CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		return fmt.Errorf("store: seed admin failed: %w", err)
	}
	log.Printf("store: seeded admin account %q", s.Cfg.AdminName)
	return nil
}

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

// ---- users ----

type User struct {
	Id           int64
	Name         string
	PasswordHash string
	Role         string
	Status       string
	CreatedAt    int64
	UpdatedAt    int64
}

var ErrNotFound = errors.New("record not found")

func userFromRow(u db.User) User {
	return User{
		Id: u.ID, Name: u.Name, PasswordHash: u.PasswordHash, Role: u.Role,
		Status: u.Status, CreatedAt: u.CreatedAt, UpdatedAt: u.UpdatedAt,
	}
}

func (s *Store) GetUserByName(name string) (User, error) {
	u, err := s.q.GetUserByName(storeCtx, name)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	return userFromRow(u), err
}

func (s *Store) GetUserByID(id int64) (User, error) {
	u, err := s.q.GetUserByID(storeCtx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	return userFromRow(u), err
}

func (s *Store) ListUsers(keyword string, page, pageSize int64) ([]User, int64, error) {
	total, err := s.q.CountUsers(storeCtx, optStr(keyword))
	if err != nil {
		return nil, 0, err
	}
	rows, err := s.q.ListUsers(storeCtx, db.ListUsersParams{
		Keyword: optStr(keyword),
		Limit:   pageSize,
		Offset:  (page - 1) * pageSize,
	})
	if err != nil {
		return nil, 0, err
	}
	users := make([]User, 0, len(rows))
	for _, r := range rows {
		users = append(users, userFromRow(r))
	}
	return users, total, nil
}

func (s *Store) CreateUser(name, passwordHash, role string) (User, error) {
	now := time.Now().Unix()
	id, err := s.q.InsertUser(storeCtx, db.InsertUserParams{
		Name: name, PasswordHash: passwordHash, Role: role, CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		return User{}, err
	}
	return s.GetUserByID(id)
}

func (s *Store) UpdateUser(id int64, name, role, status string, passwordHash *string) error {
	now := time.Now().Unix()
	if passwordHash != nil {
		return s.q.UpdateUserPassword(storeCtx, db.UpdateUserPasswordParams{
			Name: name, Role: role, Status: status, PasswordHash: *passwordHash,
			UpdatedAt: now, ID: id,
		})
	}
	return s.q.UpdateUser(storeCtx, db.UpdateUserParams{
		Name: name, Role: role, Status: status, UpdatedAt: now, ID: id,
	})
}

func (s *Store) DeleteUser(id int64) error {
	return s.q.DeleteUser(storeCtx, id)
}

func (s *Store) CountUsers() (int64, error) {
	return s.q.CountAllUsers(storeCtx)
}

func (s *Store) CountActiveUsers() (int64, error) {
	return s.q.CountActiveUsers(storeCtx)
}

func (s *Store) CountAdmins() (int64, error) {
	return s.q.CountAdmins(storeCtx)
}

// ---- sessions ----

func (s *Store) CreateSession(token string, userID int64, ttl time.Duration) error {
	now := time.Now()
	return s.q.InsertSession(storeCtx, db.InsertSessionParams{
		Token: token, UserID: userID,
		CreatedAt: now.Unix(), ExpiresAt: now.Add(ttl).Unix(),
	})
}

// SessionUser 会话命中后的用户视图：登录态校验只走这一处，过期/禁用统一判负
type SessionUser struct {
	User  User
	Token string
}

// GetSessionUser 校验 token 对应的会话与用户。
// 滑动续期：会话本身 ttl 过期即失效；存活但距签发/上次续期超过 renewAfter 时，
// 顺手把 expires_at 顺延回 now+ttl——活跃用户不掉线，闲置 ttl 时长强制下线。
func (s *Store) GetSessionUser(token string, ttl, renewAfter time.Duration) (SessionUser, error) {
	row, err := s.q.GetSessionUser(storeCtx, token)
	if errors.Is(err, sql.ErrNoRows) {
		return SessionUser{}, ErrNotFound
	}
	if err != nil {
		return SessionUser{}, err
	}
	su := SessionUser{
		Token: token,
		User: User{
			Id: row.ID, Name: row.Name, PasswordHash: row.PasswordHash,
			Role: row.Role, Status: row.Status,
			CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		},
	}
	now := time.Now()
	if now.Unix() > row.ExpiresAt {
		_ = s.q.DeleteSession(storeCtx, token)
		return SessionUser{}, ErrNotFound
	}
	lastRefresh := time.Unix(row.ExpiresAt, 0).Add(-ttl)
	if now.Sub(lastRefresh) >= renewAfter {
		_ = s.q.TouchSession(storeCtx, db.TouchSessionParams{
			ExpiresAt: now.Add(ttl).Unix(), Token: token,
		})
	}
	return su, nil
}

func (s *Store) DeleteSession(token string) error {
	return s.q.DeleteSession(storeCtx, token)
}

// DeleteOtherSessions 改密后吊销除当前外的全部会话
func (s *Store) DeleteOtherSessions(userID int64, keepToken string) error {
	return s.q.DeleteOtherSessions(storeCtx, db.DeleteOtherSessionsParams{
		UserID: userID, Token: keepToken,
	})
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
