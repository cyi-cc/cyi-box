package service

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/cyi-cc/fun"

	"github.com/cyi-cc/cyi-box/backend/internal/proxyx"
	"github.com/cyi-cc/cyi-box/backend/internal/store"
)

const (
	extractBatchSize = 15               // 每批并行探测的候选数
	extractBatches   = 4                // 最多取几批
	extractProbeTO   = 3 * time.Second  // 单代理探测超时
	extractDeadline  = 20 * time.Second // 整个请求的最长耗时
)

type extractResult struct {
	addr string
	lat  int
	ok   bool
}

// ProxyExtractHandler GET /v1/proxy?key=xxx[&fmt=url]
// 公开提取接口：key 落库校验，地区绑在 key 上（0=全部）。
// 并行探测一批候选，谁先探活返回谁；默认纯文本 "ip:port"，
// fmt=url 时带握手实测协议返回 "scheme://ip:port"（socks5h/http/socks4）。
// 两批全死返回 503。
func ProxyExtractHandler(st *store.Store) fun.RouteHandler {
	return func(rc *fun.RouteCtx) error {
		reqCtx := rc.RequestCtx
		key := strings.TrimSpace(string(reqCtx.QueryArgs().Peek("key")))
		if key == "" {
			routeJSONErr(reqCtx, 401, "missing key")
			return nil
		}
		k, err := st.GetProxyApiKey(key)
		if err != nil {
			routeJSONErr(reqCtx, 401, "invalid key")
			return nil
		}
		withScheme := strings.TrimSpace(string(reqCtx.QueryArgs().Peek("fmt"))) == "url"

		ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(extractDeadline))
		defer cancel()

		for b := 0; b < extractBatches; b++ {
			items, err := st.RandomProxies(k.RegionID, "", "", extractBatchSize)
			if err != nil {
				routeJSONErr(reqCtx, 500, "query failed")
				return nil
			}
			if len(items) == 0 {
				break
			}
			if p, proto, _, ok := probeBatch(ctx, st, items); ok {
				st.TouchProxyApiKey(k.Id)
				reqCtx.SetContentType("text/plain; charset=utf-8")
				if withScheme {
					reqCtx.SetBodyString(fmt.Sprintf("%s://%s:%d\n", urlScheme(proto), p.Ip, p.Port))
				} else {
					reqCtx.SetBodyString(fmt.Sprintf("%s:%d\n", p.Ip, p.Port))
				}
				return nil
			}
		}
		routeJSONErr(reqCtx, 503, "no alive proxy")
		return nil
	}
}

// urlScheme 握手协议名 → 代理 URL scheme：SOCKS5 用 socks5h（DNS 走代理端解析）。
func urlScheme(proto string) string {
	if proto == "socks5" {
		return "socks5h"
	}
	return proto
}

// probeBatch 并行探测一批候选：先活先赢，取消其余；死代理顺手标记移出池。
// 返回胜出的代理及其握手协议名（socks5/socks4/http）。
func probeBatch(ctx context.Context, st *store.Store, items []store.RandomProxy) (store.RandomProxy, string, int, bool) {
	type hit struct {
		p     store.RandomProxy
		proto string
		lat   int
	}
	hits := make(chan hit, 1)
	var wg sync.WaitGroup
	for _, p := range items {
		wg.Add(1)
		go func(p store.RandomProxy) {
			defer wg.Done()
			proto, lat, err := proxyx.Check(ctx, p.Ip, int(p.Port), p.Protocols, extractProbeTO)
			if err != nil {
				_ = st.MarkProxyFailed(p.Id, time.Now().Unix(), p.FailCount+1)
				return
			}
			select {
			case hits <- hit{p, proto, lat}:
			case <-ctx.Done():
			}
		}(p)
	}
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case h := <-hits:
		now := time.Now().Unix()
		_ = st.MarkProxyAlive(h.p.Id, now, now, int64(h.lat))
		return h.p, h.proto, h.lat, true
	case <-done:
		return store.RandomProxy{}, "", 0, false
	case <-ctx.Done():
		return store.RandomProxy{}, "", 0, false
	}
}
