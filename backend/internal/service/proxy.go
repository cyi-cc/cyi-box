package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cyi-cc/fun"

	"github.com/cyi-cc/cyi-box/backend/internal/dto"
	"github.com/cyi-cc/cyi-box/backend/internal/proxyx"
	"github.com/cyi-cc/cyi-box/backend/internal/store"
)

// 代理池后台任务参数
const (
	proxyCheckInterval = time.Hour          // 测活节奏：约每小时一轮
	proxyCheckStale    = 55 * time.Minute   // 距上次检测超过该时长才重测
	proxyCheckTimeout  = 4 * time.Second    // 单次 TCP 拨测超时
	proxyCheckWorkers  = 128                // 测活并发
	proxyPurgeAge      = 7 * 24 * time.Hour // 死代理记录保留时长
	syncDetailCap      = 200                // 同步详情里记录的地址上限
)

var (
	proxySyncing   atomic.Bool
	proxyChecking  atomic.Bool
	proxyManualSem = make(chan struct{}, 1) // 手动触发同步信号
)

// ProxySvc 代理池：地区分组浏览 + 同步记录；抓取/测活由后台调度器跑
type ProxySvc struct {
	fun.Ctx
	Store *store.Store
}

func proxyView(p store.Proxy) dto.ProxyView {
	return dto.ProxyView{
		Id: p.Id, Ip: p.Ip, Port: p.Port, Address: fmt.Sprintf("%s:%d", p.Ip, p.Port),
		Protocols: p.Protocols, Anonymity: p.Anonymity, RegionId: p.RegionID,
		RegionCode: p.RegionCode, RegionName: p.RegionName, RegionZhName: p.RegionZhName,
		Latency: p.Latency, Speed: p.Speed, Uptime: p.Uptime, Alive: p.Alive,
		FailCount: p.FailCount, LastCheckedAt: p.LastCheckedAt,
		LastAliveAt: p.LastAliveAt, LastSeenAt: p.LastSeenAt,
	}
}

func (s *ProxySvc) List(d dto.ProxyListDto) (dto.ProxyListView, error) {
	alive := d.Alive
	if alive != 0 && alive != 1 {
		alive = -1
	}
	items, total, err := s.Store.ListProxies(store.ProxyFilter{
		RegionID: d.RegionId, Protocol: strings.ToUpper(strings.TrimSpace(d.Protocol)),
		Keyword: strings.TrimSpace(d.Keyword), Alive: alive,
		Page: d.Page, PageSize: d.PageSize,
	})
	if err != nil {
		return dto.ProxyListView{}, err
	}
	out := dto.ProxyListView{Total: total, Items: make([]dto.ProxyView, 0, len(items))}
	for _, p := range items {
		out.Items = append(out.Items, proxyView(p))
	}
	return out, nil
}

func (s *ProxySvc) Regions() ([]dto.ProxyRegionView, error) {
	regions, err := s.Store.ListRegions()
	if err != nil {
		return nil, err
	}
	out := make([]dto.ProxyRegionView, 0, len(regions))
	for _, r := range regions {
		out = append(out, dto.ProxyRegionView{
			Id: r.Id, Code: r.Code, Name: r.Name, ZhName: r.ZhName,
			Region: r.Region, AliveCount: r.AliveCount,
		})
	}
	return out, nil
}

func (s *ProxySvc) Syncs() ([]dto.ProxySyncView, error) {
	syncs, err := s.Store.ListProxySyncs(30)
	if err != nil {
		return nil, err
	}
	out := make([]dto.ProxySyncView, 0, len(syncs))
	for _, v := range syncs {
		out = append(out, dto.ProxySyncView{
			Id: v.Id, StartedAt: v.StartedAt, FinishedAt: v.FinishedAt, Status: v.Status,
			Source: v.Source, Total: v.Total, Added: v.Added, Removed: v.Removed,
			Updated: v.Updated, DetailJson: v.Detail, Error: v.Error,
		})
	}
	return out, nil
}

func (s *ProxySvc) Stats() (dto.ProxyStatsView, error) {
	total, err := s.Store.CountAliveProxies()
	if err != nil {
		return dto.ProxyStatsView{}, err
	}
	regions, err := s.Store.ListRegions()
	if err != nil {
		return dto.ProxyStatsView{}, err
	}
	v := dto.ProxyStatsView{
		Total: total, Regions: int64(len(regions)),
		Syncing: proxySyncing.Load(), Checking: proxyChecking.Load(),
		LastCheckAt: s.Store.LastProxyCheckAt(),
	}
	if syncs, err := s.Store.ListProxySyncs(1); err == nil && len(syncs) > 0 {
		v.LastSyncAt = syncs[0].FinishedAt
		v.LastStatus = syncs[0].Status
	}
	return v, nil
}

// SyncNow 手动触发一次同步（后台执行，不等结果）
func (s *ProxySvc) SyncNow() (dto.OkView, error) {
	select {
	case proxyManualSem <- struct{}{}:
	default:
	}
	return dto.OkView{Message: "ok"}, nil
}

// CheckNow 手动触发一轮测活（后台执行）
func (s *ProxySvc) CheckNow() (dto.OkView, error) {
	go runProxyCheck(context.Background(), s.Store)
	return dto.OkView{Message: "ok"}, nil
}

// ---- 同步与测活 ----

type syncDetail struct {
	Added   []string `json:"added,omitempty"`
	Removed []string `json:"removed,omitempty"`
}

// runProxySync 全量同步：拉取源数据 → 区域 upsert → 代理按 ip:port diff。
// 源上消失的代理直接删除；已存在的刷新指标并复活。
func runProxySync(ctx context.Context, st *store.Store) {
	if !proxySyncing.CompareAndSwap(false, true) {
		return
	}
	defer proxySyncing.Store(false)

	now := time.Now()
	sync := &store.ProxySync{Source: "proxy5", StartedAt: now.Unix()}
	syncID, err := st.BeginProxySync(sync.Source, sync.StartedAt)
	if err != nil {
		log.Printf("proxy: begin sync failed: %v", err)
		return
	}
	sync.Id = syncID
	finish := func(status string, detail syncDetail, err error) {
		sync.FinishedAt = time.Now().Unix()
		sync.Status = status
		if b, e := json.Marshal(detail); e == nil {
			sync.Detail = string(b)
		}
		if err != nil {
			sync.Error = err.Error()
		}
		if e := st.FinishProxySync(sync); e != nil {
			log.Printf("proxy: finish sync failed: %v", e)
		}
	}

	// 多源抓取：任一源失败不致命，只影响该源名下代理的删除判定
	type srcData struct {
		name    string
		remotes []proxyx.Remote
	}
	sources := []srcData{}
	failed := map[string]error{}
	if p5, _, err := proxyx.Fetch(ctx); err != nil {
		failed["proxy5"] = err
	} else {
		sources = append(sources, srcData{"proxy5", p5})
	}
	if rl, err := proxyx.FetchRola(ctx); err != nil {
		failed["rola"] = err
	} else {
		sources = append(sources, srcData{"rola", rl})
	}
	if len(sources) == 0 {
		err := fmt.Errorf("all sources failed: proxy5=%v rola=%v", failed["proxy5"], failed["rola"])
		finish("error", syncDetail{}, err)
		log.Printf("proxy: %v", err)
		return
	}

	// 按 ip:port 合并各源记录：首个来源保留字段，协议取并集
	type merged struct {
		r      proxyx.Remote
		source string
	}
	pool := map[string]*merged{}
	order := []string{}
	for _, src := range sources {
		for _, r := range src.remotes {
			key := fmt.Sprintf("%s:%d", r.IP, r.Port)
			if m, ok := pool[key]; ok {
				seen := map[string]bool{}
				for _, p := range m.r.Protocols {
					seen[p] = true
				}
				for _, p := range r.Protocols {
					if !seen[p] {
						m.r.Protocols = append(m.r.Protocols, p)
					}
				}
				continue
			}
			pool[key] = &merged{r: r, source: src.name}
			order = append(order, key)
		}
	}
	sync.Total = int64(len(order))
	seenKeys := map[string]bool{}
	for k := range pool {
		seenKeys[k] = true
	}

	keys, err := st.ProxyKeys()
	if err != nil {
		finish("error", syncDetail{}, err)
		return
	}

	var detail syncDetail
	regionIDs := map[string]int64{}
	for _, key := range order {
		m := pool[key]
		r := m.r
		code := strings.ToUpper(strings.TrimSpace(r.CountryCode))
		if code == "" {
			code = "UN"
		}
		regionID, ok := regionIDs[code]
		if !ok {
			regionID, err = st.UpsertRegion(code, r.Country, proxyx.ZhName(code), r.Region, r.CountrySlug)
			if err != nil {
				continue
			}
			regionIDs[code] = regionID
		}
		p := &store.Proxy{
			Ip: r.IP, Port: int64(r.Port), Protocols: strings.Join(r.Protocols, ","),
			Anonymity: r.Anonymity, Source: m.source, RegionID: regionID,
			Latency: int64(r.Latency), Speed: int64(r.Speed), Uptime: int64(r.Uptime),
			LastSeenAt: now.Unix(), CreatedAt: now.Unix(),
		}
		if k, exists := keys[key]; exists {
			delete(keys, key)
			if err := st.UpdateProxySeen(k.Id, p); err == nil {
				sync.Updated++
			}
			continue
		}
		if err := st.InsertProxy(p); err == nil {
			sync.Added++
			if len(detail.Added) < syncDetailCap {
				detail.Added = append(detail.Added, key)
			}
		}
	}
	// 删除：出现在任一成功源里的都保留；只属于失败源的也保留（可能还在源上）
	for key, k := range keys {
		if seenKeys[key] {
			continue
		}
		if _, srcFailed := failed[k.Source]; srcFailed {
			continue
		}
		if err := st.DeleteProxy(k.Id); err == nil {
			sync.Removed++
			if len(detail.Removed) < syncDetailCap {
				detail.Removed = append(detail.Removed, key)
			}
		}
	}
	var syncErr error
	if len(failed) > 0 {
		syncErr = fmt.Errorf("partial: %v", failed)
	}
	finish("ok", detail, syncErr)
	log.Printf("proxy: sync done total=%d added=%d updated=%d removed=%d failed=%v",
		sync.Total, sync.Added, sync.Updated, sync.Removed, failed)
}

// runProxyCheck 测活一轮：取距上次检测超过阈值的存活代理，并发 TCP 拨测；
// 连不上即标记为死（alive=0），死记录超过保留期后物理清除。
func runProxyCheck(ctx context.Context, st *store.Store) {
	if !proxyChecking.CompareAndSwap(false, true) {
		return
	}
	defer proxyChecking.Store(false)

	due, err := st.DueProxies(time.Now().Add(-proxyCheckStale).Unix())
	if err != nil {
		log.Printf("proxy: due list failed: %v", err)
		return
	}
	if len(due) == 0 {
		return
	}
	now := time.Now().Unix()
	sem := make(chan struct{}, proxyCheckWorkers)
	var wg sync.WaitGroup
	for _, p := range due {
		wg.Add(1)
		go func(p store.DueProxy) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			latency, err := proxyx.Check(ctx, p.Ip, int(p.Port), p.Protocols, proxyCheckTimeout)
			if err != nil {
				_ = st.MarkProxyFailed(p.Id, now, p.FailCount+1)
				return
			}
			_ = st.MarkProxyAlive(p.Id, now, now, int64(latency))
		}(p)
	}
	wg.Wait()
	if n, err := st.PurgeDeadProxies(time.Now().Add(-proxyPurgeAge).Unix()); err == nil && n > 0 {
		log.Printf("proxy: purged %d dead records", n)
	}
	log.Printf("proxy: checked %d proxies", len(due))
}

// StartProxyScheduler 调度器：启动补一次同步（数据为空时）→ 每小时测活 → 每日 00:00 同步。
func StartProxyScheduler(ctx context.Context, st *store.Store) {
	if syncs, err := st.ListProxySyncs(1); err == nil {
		if len(syncs) == 0 || time.Since(time.Unix(syncs[0].StartedAt, 0)) > 20*time.Hour {
			go runProxySync(ctx, st)
		}
	}
	go func() {
		checkTick := time.NewTicker(proxyCheckInterval)
		defer checkTick.Stop()
		for {
			next := nextMidnight()
			syncTimer := time.NewTimer(time.Until(next))
			select {
			case <-ctx.Done():
				syncTimer.Stop()
				return
			case <-checkTick.C:
				runProxyCheck(ctx, st)
			case <-proxyManualSem:
				go runProxySync(ctx, st)
			case <-syncTimer.C:
				runProxySync(ctx, st)
			}
		}
	}()
	log.Printf("proxy: scheduler started (check hourly, sync daily 00:00)")
}

func nextMidnight() time.Time {
	now := time.Now()
	return time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
}

// ---- 提取密钥管理（admin） ----

func proxyKeyView(k store.ProxyApiKey) dto.ProxyKeyView {
	return dto.ProxyKeyView{
		Id: k.Id, Key: k.Key, Name: k.Name, RegionId: k.RegionID,
		RegionCode: k.RegionCode, RegionZhName: k.RegionZhName,
		Enabled: k.Enabled, UsedCount: k.UsedCount,
		LastUsedAt: k.LastUsedAt, CreatedAt: k.CreatedAt,
	}
}

func (s *ProxySvc) ListKeys() ([]dto.ProxyKeyView, error) {
	keys, err := s.Store.ListProxyApiKeys()
	if err != nil {
		return nil, err
	}
	out := make([]dto.ProxyKeyView, 0, len(keys))
	for _, k := range keys {
		out = append(out, proxyKeyView(k))
	}
	return out, nil
}

// CreateKey 生成一个提取密钥：绑定地区（0=全部），key 随机生成落库
func (s *ProxySvc) CreateKey(d dto.ProxyKeySaveDto) (dto.ProxyKeyView, error) {
	key, err := shareCode()
	if err != nil {
		return dto.ProxyKeyView{}, err
	}
	key = "pk_" + key + key // 20 位随机串，够长防穷举
	id, err := s.Store.InsertProxyApiKey(key, strings.TrimSpace(d.Name), d.RegionId)
	if err != nil {
		return dto.ProxyKeyView{}, err
	}
	return dto.ProxyKeyView{Id: id, Key: key, Name: d.Name, RegionId: d.RegionId, Enabled: true}, nil
}

func (s *ProxySvc) DeleteKey(d dto.IdDto) (dto.OkView, error) {
	if err := s.Store.DeleteProxyApiKey(d.Id); err != nil {
		return dto.OkView{}, err
	}
	return dto.OkView{Message: "ok"}, nil
}
