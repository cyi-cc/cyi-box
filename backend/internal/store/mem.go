// mem.go 共享记忆库门面：key 形态 "<project>/<topic>"，写覆盖即 revision+1。
// 同一套数据同时服务后台管理页（MemorySvc）与对外 MCP 协议端点（internal/memx）。
package store

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/cyi-cc/cyi-box/backend/internal/db"
)

// memFTSDDL 全文索引：外挂内容表（不复制数据）+ 增删改触发器同步。
// trigram 分词器：中英文都能做子串匹配（代价是 <3 字符的词无法索引，搜索侧兜底）
const memFTSDDL = `
CREATE VIRTUAL TABLE IF NOT EXISTS memories_fts USING fts5(
  k, content, project, tags,
  content='memories', content_rowid='rowid',
  tokenize='trigram'
);
CREATE TRIGGER IF NOT EXISTS memories_ai AFTER INSERT ON memories BEGIN
  INSERT INTO memories_fts(rowid, k, content, project, tags)
  VALUES (new.rowid, new.k, new.content, new.project, new.tags);
END;
CREATE TRIGGER IF NOT EXISTS memories_ad AFTER DELETE ON memories BEGIN
  INSERT INTO memories_fts(memories_fts, rowid, k, content, project, tags)
  VALUES ('delete', old.rowid, old.k, old.content, old.project, old.tags);
END;
CREATE TRIGGER IF NOT EXISTS memories_au AFTER UPDATE ON memories BEGIN
  INSERT INTO memories_fts(memories_fts, rowid, k, content, project, tags)
  VALUES ('delete', old.rowid, old.k, old.content, old.project, old.tags);
  INSERT INTO memories_fts(rowid, k, content, project, tags)
  VALUES (new.rowid, new.k, new.content, new.project, new.tags);
END;`

// Memory 一条持久记忆；Tags 经 JSON 数组落库
type Memory struct {
	Key       string
	Content   string
	Project   string
	Tags      []string
	WrittenBy string
	Revision  int64
	CreatedAt int64
	UpdatedAt int64
}

func memoryFromRow(r db.Memory) Memory {
	m := Memory{
		Key: r.K, Content: r.Content, Project: r.Project, WrittenBy: r.WrittenBy,
		Revision: r.Revision, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
	_ = json.Unmarshal([]byte(r.Tags), &m.Tags)
	if m.Tags == nil {
		m.Tags = []string{}
	}
	return m
}

// SaveMemory 覆盖式写入：同 key 保留 created_at、revision 自增
func (s *Store) SaveMemory(m *Memory) error {
	now := time.Now().Unix()
	tags, _ := json.Marshal(m.Tags)
	if err := s.q.UpsertMemory(storeCtx, db.UpsertMemoryParams{
		K: m.Key, Content: m.Content, Project: m.Project, Tags: string(tags),
		WrittenBy: m.WrittenBy, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		return err
	}
	row, err := s.q.GetMemory(storeCtx, m.Key)
	if err != nil {
		return err
	}
	m.Revision, m.CreatedAt, m.UpdatedAt = row.Revision, row.CreatedAt, row.UpdatedAt
	return nil
}

func (s *Store) GetMemory(key string) (*Memory, error) {
	row, err := s.q.GetMemory(storeCtx, key)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	m := memoryFromRow(row)
	return &m, nil
}

func (s *Store) DeleteMemory(key string) (bool, error) {
	n, err := s.q.DeleteMemory(storeCtx, key)
	return n > 0, err
}

// ListMemories project/tag 可空过滤，limit<=0 取默认 50，上限 500
func (s *Store) ListMemories(project, tag string, limit int64) ([]Memory, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}
	rows, err := s.q.ListMemories(storeCtx, db.ListMemoriesParams{
		Column1: project, Project: project,
		Column3: tag, Column4: optStr(tag), Limit: limit,
	})
	if err != nil {
		return nil, err
	}
	out := make([]Memory, 0, len(rows))
	for _, r := range rows {
		out = append(out, memoryFromRow(r))
	}
	return out, nil
}

// SearchMemories 每个空白分隔的词都要命中（AND 语义）。
// ≥3 字符的词走 FTS5 trigram 索引（按相关度排序）；出现短词或索引异常时退回内存 LIKE。
func (s *Store) SearchMemories(query, project string, limit int64) ([]Memory, error) {
	terms := strings.Fields(query)
	if len(terms) == 0 {
		return s.ListMemories(project, "", limit)
	}
	if limit <= 0 {
		limit = 50
	}
	canFTS := true
	for _, t := range terms {
		if len([]rune(t)) < 3 {
			canFTS = false
			break
		}
	}
	if canFTS {
		if ms, err := s.searchMemoriesFTS(terms, project, limit); err == nil {
			return ms, nil
		}
	}
	return s.searchMemoriesLike(terms, project, limit)
}

// searchMemoriesFTS 索引命中：词项加引号消除 MATCH 语法注入面
func (s *Store) searchMemoriesFTS(terms []string, project string, limit int64) ([]Memory, error) {
	var b strings.Builder
	for i, t := range terms {
		if i > 0 {
			b.WriteString(" AND ")
		}
		b.WriteString(`"` + strings.ReplaceAll(t, `"`, `""`) + `"`)
	}
	rows, err := s.DB.Query(`SELECT m.k, m.content, m.project, m.tags, m.written_by, m.revision, m.created_at, m.updated_at
		FROM memories_fts f JOIN memories m ON m.rowid = f.rowid
		WHERE f MATCH ? AND (? = '' OR m.project = ?)
		ORDER BY f.rank LIMIT ?`, b.String(), project, project, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Memory
	for rows.Next() {
		var m Memory
		var tags string
		if err := rows.Scan(&m.Key, &m.Content, &m.Project, &tags, &m.WrittenBy, &m.Revision, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(tags), &m.Tags)
		if m.Tags == nil {
			m.Tags = []string{}
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Store) searchMemoriesLike(terms []string, project string, limit int64) ([]Memory, error) {
	all, err := s.ListMemories(project, "", 500)
	if err != nil {
		return nil, err
	}
	var out []Memory
	for _, m := range all {
		hay := strings.ToLower(m.Key + "\n" + m.Project + "\n" + m.Content)
		ok := true
		for _, t := range terms {
			if !strings.Contains(hay, strings.ToLower(t)) {
				ok = false
				break
			}
		}
		if ok {
			out = append(out, m)
		}
		if int64(len(out)) >= limit {
			break
		}
	}
	return out, nil
}

type MemoryProject struct {
	Name  string
	Count int64
}

func (s *Store) MemoryProjects() ([]MemoryProject, error) {
	rows, err := s.q.MemoryProjects(storeCtx)
	if err != nil {
		return nil, err
	}
	out := make([]MemoryProject, 0, len(rows))
	for _, r := range rows {
		out = append(out, MemoryProject{Name: r.Name, Count: r.Cnt})
	}
	return out, nil
}

const mcpTokenKey = "mcp.token"

// McpToken 读取对外 MCP 端点的 Bearer 令牌；不存在则生成一个并落库
func (s *Store) McpToken() (string, error) {
	if v, err := s.GetSetting(mcpTokenKey); err == nil && v != "" {
		return v, nil
	}
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("store: gen mcp token failed: %w", err)
	}
	tok := hex.EncodeToString(b)
	if err := s.SetSetting(mcpTokenKey, tok); err != nil {
		return "", err
	}
	return tok, nil
}

// SignMemoryReadURL 签发 /m/<key> 阅读链接：?sig=HMAC(token, "read:"+key)[:24]
// 与 MCP 共用同一令牌——重置令牌即吊销全部已发链接
func (s *Store) SignMemoryReadURL(base, key string) (string, error) {
	tok, err := s.McpToken()
	if err != nil {
		return "", err
	}
	return base + "/m/" + key + "?sig=" + memReadSig(tok, key), nil
}

// CheckMemoryReadSig 校验阅读链接签名
func (s *Store) CheckMemoryReadSig(key, sig string) bool {
	tok, err := s.McpToken()
	if err != nil || sig == "" {
		return false
	}
	return hmac.Equal([]byte(sig), []byte(memReadSig(tok, key)))
}

func memReadSig(token, key string) string {
	h := hmac.New(sha256.New, []byte(token))
	h.Write([]byte("read:" + key))
	return hex.EncodeToString(h.Sum(nil))[:24]
}

func (s *Store) ResetMcpToken() (string, error) {
	if err := s.SetSetting(mcpTokenKey, ""); err != nil {
		return "", err
	}
	return s.McpToken()
}
