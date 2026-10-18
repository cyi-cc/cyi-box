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
	extractBatches   = 8                // 最多取几批
	extractProbeTO   = 6 * time.Second  // 单代理探测超时（TLS+数据面验证，3s 会误杀慢但可用的）
	extractDeadline  = 30 * time.Second // 整个请求的最长耗时
)

type extractResult struct {
	addr string
	lat  int
	ok   bool
}

// ProxyExtractHandler GET /v1/proxy?key=xxx[&fmt=url][&host=example.com]
// 公开提取接口：key 落库校验，地区绑在 key 上（0=全部）。
// 并行探测一批候选，谁先探活返回谁；返回带握手实测协议的 "scheme://ip:port"（socks5h/http/socks4）。
// 两批全死返回 503。host 可选：探测目标改为主机名（CONNECT+TLS 验证到该站，
// 默认 gstatic）——消费方传真实目标，拿到的才是"对这个站可用"的代理。
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
		// host 参数：提取探测的 CONNECT+TLS 目标，默认 gstatic。
		// 只收合法主机名，去掉端口/路径等脏输入。
		target := ""
		if h := strings.TrimSpace(string(reqCtx.QueryArgs().Peek("host"))); h != "" {
			h = strings.SplitN(h, ":", 2)[0]
			valid := len(h) <= 253
			for _, r := range h {
				if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '.' || r == '-' || r == '_') {
					valid = false
					break
				}
			}
			if valid && strings.Contains(h, ".") {
				target = h
			}
		}

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
			if p, proto, _, ok := probeBatch(ctx, st, items, target); ok {
				st.TouchProxyApiKey(k.Id)
				reqCtx.SetContentType("text/plain; charset=utf-8")
				reqCtx.SetBodyString(fmt.Sprintf("%s://%s:%d\n", urlScheme(proto), p.Ip, p.Port))
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
// target 非空时把 CONNECT+TLS 探测指向该主机（消费方真实目标）。
// 返回胜出的代理及其握手协议名（socks5/socks4/http）。
func probeBatch(ctx context.Context, st *store.Store, items []store.RandomProxy, target string) (store.RandomProxy, string, int, bool) {
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
			proto, lat, err := proxyx.Check(ctx, p.Ip, int(p.Port), p.Protocols, extractProbeTO, target)
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
