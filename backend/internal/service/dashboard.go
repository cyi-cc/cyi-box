package service

import (
	"context"
	"fmt"
	"time"

	"github.com/cyi-cc/fun"

	"github.com/cyi-cc/cyi-box/backend/internal/db"
	"github.com/cyi-cc/cyi-box/backend/internal/dto"
	"github.com/cyi-cc/cyi-box/backend/internal/store"
)

var bootTime = time.Now()

type DashboardSvc struct {
	fun.Ctx
	Store *store.Store
}

// Stats 仪表盘聚合：会话概况 + 各业务模块计数 + 收入趋势 + 最近动态
func (s *DashboardSvc) Stats() (dto.StatsView, error) {
	ctx := context.Background()
	q := s.Store.Q()
	now := time.Now()
	view := dto.StatsView{UptimeSeconds: int64(time.Since(bootTime).Seconds())}

	// 会话概况
	var err error
	if view.TotalUsers, err = s.Store.CountUsers(); err != nil {
		return view, fmt.Errorf("dashboard: count users failed: %w", err)
	}
	if view.ActiveSessions, err = s.Store.CountActiveSessions(); err != nil {
		return view, fmt.Errorf("dashboard: count sessions failed: %w", err)
	}
	if view.TodayLogins, err = s.Store.CountTodayLogins(); err != nil {
		return view, fmt.Errorf("dashboard: count today logins failed: %w", err)
	}
	const days = 14
	trend, err := s.Store.LoginTrend(days)
	if err != nil {
		return view, fmt.Errorf("dashboard: login trend failed: %w", err)
	}
	view.LoginTrend = make([]dto.TrendPoint, 0, days)
	for i := days - 1; i >= 0; i-- {
		day := now.AddDate(0, 0, -int(i)).Format("2006-01-02")
		view.LoginTrend = append(view.LoginTrend, dto.TrendPoint{Day: day, Count: trend[day]})
	}

	// 业务模块计数
	view.Servers, _ = s.Store.CountServers()
	view.Proxies, _ = q.CountProxiesAll(ctx)
	view.AliveProxies, _ = s.Store.CountAliveProxies()
	view.LicenseApps, _ = q.CountLicenseApps(ctx)
	view.Dbconns, _ = q.CountDbconns(ctx)
	view.Files, _ = q.CountFilesAll(ctx)
	view.Memories, _ = q.CountMemories(ctx)
	if ps, err := s.Store.MemoryProjects(); err == nil {
		view.MemoryProjects = int64(len(ps))
	}
	if ls, err := q.LicenseCardStatsAll(ctx, now.Unix()); err == nil {
		view.LicenseCards, view.LicenseActive = ls.Total, anyI64(ls.Active)
	}
	if su, err := sessionFrom(s.RequestCtx); err == nil {
		view.Bookmarks, _ = q.CountBookmarksByUser(ctx, su.User.Id)
		view.VaultItems, _ = q.CountVaultByUser(ctx, su.User.Id)
	}

	// 支付：今日 + 累计 + 14 天收入趋势 + 最近成交
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Unix()
	if st, err := q.PayOrderStats(ctx, dayStart); err == nil {
		view.TodayOrders, view.TodayMoney = st.Cnt, anyI64(st.Money)
	}
	if st, err := q.PayOrderStats(ctx, 0); err == nil {
		view.TotalMoney, view.PendingOrders = anyI64(st.Money), anyI64(st.Pending)
	}
	daily := map[string]db.PayDailyPaidRow{}
	if rows, err := q.PayDailyPaid(ctx, dayStart-int64((days-1)*86400)); err == nil {
		for _, r := range rows {
			daily[r.Day] = r
		}
	}
	view.PayTrend = make([]dto.PayTrendPoint, 0, days)
	for i := days - 1; i >= 0; i-- {
		t := now.AddDate(0, 0, -int(i))
		p := dto.PayTrendPoint{Day: t.Format("2006-01-02")}
		if r, ok := daily[p.Day]; ok {
			p.Money, p.Count = anyI64(r.Money), r.Cnt
		}
		view.PayTrend = append(view.PayTrend, p)
	}
	if rows, err := q.RecentPaidOrders(ctx, 6); err == nil {
		view.RecentOrders = make([]dto.RecentOrderView, 0, len(rows))
		for _, r := range rows {
			subject := r.Subject
			if subject == "" {
				subject = r.TradeNo
			}
			view.RecentOrders = append(view.RecentOrders, dto.RecentOrderView{
				TradeNo: r.TradeNo, Subject: subject, Money: r.Money, PaidAt: r.PaidAt,
			})
		}
	}
	if ms, err := s.Store.ListMemories("", "", 5); err == nil {
		view.RecentMemories = make([]dto.RecentMemoryView, 0, len(ms))
		for _, m := range ms {
			view.RecentMemories = append(view.RecentMemories, dto.RecentMemoryView{
				Key: m.Key, Project: m.Project, UpdatedAt: m.UpdatedAt,
			})
		}
	}
	return view, nil
}
