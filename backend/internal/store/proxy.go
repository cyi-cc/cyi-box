package store

import (
	"database/sql"
	"errors"
	"strconv"
	"time"

	"github.com/cyi-cc/cyi-box/backend/internal/db"
)

// ---- proxies ----

type ProxyRegion struct {
	Id         int64
	Code       string
	Name       string
	ZhName     string
	Region     string
	Slug       string
	AliveCount int64
}

type Proxy struct {
	Id            int64
	Ip            string
	Port          int64
	Protocols     string
	Anonymity     string
	Source        string
	RegionID      int64
	RegionCode    string
	RegionName    string
	RegionZhName  string
	Latency       int64
	Speed         int64
	Uptime        int64
	Alive         bool
	FailCount     int64
	LastCheckedAt int64
	LastAliveAt   int64
	LastSeenAt    int64
	CreatedAt     int64
}

type ProxySync struct {
	Id         int64
	StartedAt  int64
	FinishedAt int64
	Status     string
	Source     string
	Total      int64
	Added      int64
	Removed    int64
	Updated    int64
	Detail     string
	Error      string
}

type ProxyFilter struct {
	RegionID int64
	Protocol string
	Keyword  string
	Alive    int64
	Page     int64
	PageSize int64
}

// UpsertRegion returns the region id for code, creating/updating the row.
func (s *Store) UpsertRegion(code, name, zhName, region, slug string) (int64, error) {
	if err := s.q.UpsertProxyRegion(storeCtx, db.UpsertProxyRegionParams{
		Code: code, Name: name, ZhName: zhName, Region: region, Slug: slug,
	}); err != nil {
		return 0, err
	}
	r, err := s.q.GetProxyRegionByCode(storeCtx, code)
	if err != nil {
		return 0, err
	}
	return r.ID, nil
}

func (s *Store) ListRegions() ([]ProxyRegion, error) {
	rows, err := s.q.ListProxyRegions(storeCtx)
	if err != nil {
		return nil, err
	}
	out := make([]ProxyRegion, 0, len(rows))
	for _, r := range rows {
		out = append(out, ProxyRegion{
			Id: r.ID, Code: r.Code, Name: r.Name, ZhName: r.ZhName,
			Region: r.Region, Slug: r.Slug, AliveCount: r.AliveCount,
		})
	}
	return out, nil
}

// ProxyKey 同步 diff 用的键信息：来源决定「消失即删」的作用域
type ProxyKey struct {
	Id     int64
	Source string
}

// ProxyKeys maps "ip:port" to ProxyKey, used by sync diffing.
func (s *Store) ProxyKeys() (map[string]ProxyKey, error) {
	rows, err := s.q.ListProxyKeys(storeCtx)
	if err != nil {
		return nil, err
	}
	m := make(map[string]ProxyKey, len(rows))
	for _, r := range rows {
		m[r.Ip+":"+strconv.FormatInt(r.Port, 10)] = ProxyKey{Id: r.ID, Source: r.Source}
	}
	return m, nil
}

func (s *Store) InsertProxy(p *Proxy) error {
	id, err := s.q.InsertProxy(storeCtx, db.InsertProxyParams{
		Ip: p.Ip, Port: p.Port, Protocols: p.Protocols, Anonymity: p.Anonymity,
		Source: p.Source,
		RegionID: p.RegionID, Latency: p.Latency, Speed: p.Speed, Uptime: p.Uptime,
		LastSeenAt: p.LastSeenAt, CreatedAt: p.CreatedAt,
	})
	if err != nil {
		return err
	}
	p.Id = id
	return nil
}

func (s *Store) UpdateProxySeen(id int64, p *Proxy) error {
	return s.q.UpdateProxySeen(storeCtx, db.UpdateProxySeenParams{
		Protocols: p.Protocols, Anonymity: p.Anonymity, RegionID: p.RegionID,
		Latency: p.Latency, Speed: p.Speed, Uptime: p.Uptime,
		LastSeenAt: p.LastSeenAt, ID: id,
	})
}

func (s *Store) DeleteProxy(id int64) error {
	return s.q.DeleteProxy(storeCtx, id)
}

type DueProxy struct {
	Id        int64
	Ip        string
	Port      int64
	Protocols string
	FailCount int64
}

func (s *Store) DueProxies(checkedBefore int64) ([]DueProxy, error) {
	rows, err := s.q.ListProxiesDue(storeCtx, checkedBefore)
	if err != nil {
		return nil, err
	}
	out := make([]DueProxy, 0, len(rows))
	for _, r := range rows {
		out = append(out, DueProxy{Id: r.ID, Ip: r.Ip, Port: r.Port, Protocols: r.Protocols, FailCount: r.FailCount})
	}
	return out, nil
}

func (s *Store) MarkProxyAlive(id, checkedAt, aliveAt, latency int64) error {
	return s.q.MarkProxyAlive(storeCtx, db.MarkProxyAliveParams{
		LastCheckedAt: checkedAt, LastAliveAt: aliveAt, Latency: latency, ID: id,
	})
}

func (s *Store) MarkProxyFailed(id, checkedAt, failCount int64) error {
	return s.q.MarkProxyFailed(storeCtx, db.MarkProxyFailedParams{
		LastCheckedAt: checkedAt, FailCount: failCount, ID: id,
	})
}

func (s *Store) PurgeDeadProxies(before int64) (int64, error) {
	return s.q.DeleteDeadProxies(storeCtx, before)
}

func (s *Store) ListProxies(f ProxyFilter) ([]Proxy, int64, error) {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PageSize <= 0 || f.PageSize > 200 {
		f.PageSize = 50
	}
	total, err := s.q.CountProxiesFiltered(storeCtx, db.CountProxiesFilteredParams{
		Column1: f.RegionID, RegionID: f.RegionID,
		Column3: f.Protocol, Column4: optStr(f.Protocol),
		Column5: f.Keyword, Column6: optStr(f.Keyword),
		Column7: f.Alive, Alive: f.Alive,
	})
	if err != nil {
		return nil, 0, err
	}
	rows, err := s.q.ListProxiesFiltered(storeCtx, db.ListProxiesFilteredParams{
		Column1: f.RegionID, RegionID: f.RegionID,
		Column3: f.Protocol, Column4: optStr(f.Protocol),
		Column5: f.Keyword, Column6: optStr(f.Keyword),
		Column7: f.Alive, Alive: f.Alive,
		Limit: f.PageSize, Offset: (f.Page - 1) * f.PageSize,
	})
	if err != nil {
		return nil, 0, err
	}
	out := make([]Proxy, 0, len(rows))
	for _, r := range rows {
		out = append(out, Proxy{
			Id: r.ID, Ip: r.Ip, Port: r.Port, Protocols: r.Protocols, Anonymity: r.Anonymity,
			RegionID: r.RegionID, RegionCode: r.RegionCode, RegionName: r.RegionName,
			RegionZhName: r.RegionZhName, Latency: r.Latency, Speed: r.Speed, Uptime: r.Uptime,
			Alive: r.Alive == 1, FailCount: r.FailCount, LastCheckedAt: r.LastCheckedAt,
			LastAliveAt: r.LastAliveAt, LastSeenAt: r.LastSeenAt, CreatedAt: r.CreatedAt,
		})
	}
	return out, total, nil
}

func (s *Store) CountAliveProxies() (int64, error) {
	return s.q.CountProxies(storeCtx)
}

func (s *Store) LastProxyCheckAt() int64 {
	v, err := s.q.LastProxyCheckAt(storeCtx)
	if err != nil {
		return 0
	}
	if n, ok := v.(int64); ok {
		return n
	}
	return 0
}

func (s *Store) BeginProxySync(source string, startedAt int64) (int64, error) {
	return s.q.InsertProxySync(storeCtx, db.InsertProxySyncParams{
		StartedAt: startedAt, Source: source,
	})
}

func (s *Store) FinishProxySync(sync *ProxySync) error {
	return s.q.FinishProxySync(storeCtx, db.FinishProxySyncParams{
		FinishedAt: sync.FinishedAt, Status: sync.Status, Total: sync.Total,
		Added: sync.Added, Removed: sync.Removed, Updated: sync.Updated,
		Detail: sync.Detail, Error: sync.Error, ID: sync.Id,
	})
}

func (s *Store) ListProxySyncs(limit int64) ([]ProxySync, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.q.ListProxySyncs(storeCtx, limit)
	if err != nil {
		return nil, err
	}
	out := make([]ProxySync, 0, len(rows))
	for _, r := range rows {
		out = append(out, ProxySync{
			Id: r.ID, StartedAt: r.StartedAt, FinishedAt: r.FinishedAt, Status: r.Status,
			Source: r.Source, Total: r.Total, Added: r.Added, Removed: r.Removed,
			Updated: r.Updated, Detail: r.Detail, Error: r.Error,
		})
	}
	return out, nil
}

// ---- 提取密钥 ----

type ProxyApiKey struct {
	Id           int64
	Key          string
	Name         string
	RegionID     int64
	RegionCode   string
	RegionZhName string
	Enabled      bool
	UsedCount    int64
	LastUsedAt   int64
	CreatedAt    int64
}

func (s *Store) InsertProxyApiKey(key, name string, regionID int64) (int64, error) {
	return s.q.InsertProxyApiKey(storeCtx, db.InsertProxyApiKeyParams{
		Kkey: key, Name: name, RegionID: regionID, CreatedAt: nowUnix(),
	})
}

func (s *Store) ListProxyApiKeys() ([]ProxyApiKey, error) {
	rows, err := s.q.ListProxyApiKeys(storeCtx)
	if err != nil {
		return nil, err
	}
	out := make([]ProxyApiKey, 0, len(rows))
	for _, r := range rows {
		out = append(out, ProxyApiKey{
			Id: r.ID, Key: r.Kkey, Name: r.Name, RegionID: r.RegionID,
			RegionCode: r.RegionCode.String, RegionZhName: r.RegionZhName.String,
			Enabled: r.Enabled == 1, UsedCount: r.UsedCount,
			LastUsedAt: r.LastUsedAt, CreatedAt: r.CreatedAt,
		})
	}
	return out, nil
}

func (s *Store) GetProxyApiKey(key string) (ProxyApiKey, error) {
	r, err := s.q.GetProxyApiKey(storeCtx, key)
	if errors.Is(err, sql.ErrNoRows) {
		return ProxyApiKey{}, ErrNotFound
	}
	if err != nil {
		return ProxyApiKey{}, err
	}
	return ProxyApiKey{
		Id: r.ID, Key: r.Kkey, Name: r.Name, RegionID: r.RegionID,
		Enabled: r.Enabled == 1, UsedCount: r.UsedCount,
		LastUsedAt: r.LastUsedAt, CreatedAt: r.CreatedAt,
	}, nil
}

func (s *Store) TouchProxyApiKey(id int64) {
	_ = s.q.TouchProxyApiKey(storeCtx, db.TouchProxyApiKeyParams{
		LastUsedAt: nowUnix(), ID: id,
	})
}

func (s *Store) DeleteProxyApiKey(id int64) error {
	_, err := s.q.DeleteProxyApiKey(storeCtx, id)
	return err
}

func nowUnix() int64 { return time.Now().Unix() }

type RandomProxy struct {
	Id           int64
	Ip           string
	Port         int64
	Protocols    string
	FailCount    int64
	RegionCode   string
	RegionZhName string
}

// RandomProxies 随机取存活代理：regionID=0 全部地区，code 可按国家码二次过滤
func (s *Store) RandomProxies(regionID int64, protocol, code string, limit int64) ([]RandomProxy, error) {
	rows, err := s.q.RandomProxies(storeCtx, db.RandomProxiesParams{
		Column1: regionID, RegionID: regionID,
		Column3: protocol, Column4: optStr(protocol),
		Column5: code, Code: code,
		Limit: limit,
	})
	if err != nil {
		return nil, err
	}
	out := make([]RandomProxy, 0, len(rows))
	for _, r := range rows {
		out = append(out, RandomProxy{
			Id: r.ID, Ip: r.Ip, Port: r.Port, Protocols: r.Protocols,
			FailCount: r.FailCount,
			RegionCode: r.RegionCode, RegionZhName: r.RegionZhName,
		})
	}
	return out, nil
}
