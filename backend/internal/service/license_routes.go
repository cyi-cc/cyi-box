// license_routes.go 卡密公开校验端点：GET/POST /v1/license?appid=xx&card=xx&domain=xx
// 首次调用激活并绑定域名（开始计时），之后仅限同域名校验；供其他项目做网络验证。
package service

import (
	"context"
	"strings"
	"time"

	"github.com/cyi-cc/fun"

	"github.com/cyi-cc/cyi-box/backend/internal/db"
	"github.com/cyi-cc/cyi-box/backend/internal/store"
)

func LicenseVerifyHandler(st *store.Store) fun.RouteHandler {
	return func(c *fun.RouteCtx) error {
		appid := strings.TrimSpace(c.Param("appid"))
		card := strings.ToUpper(strings.TrimSpace(c.Param("card")))
		domain := normalizeDomain(c.Param("domain"))
		if appid == "" || card == "" || domain == "" {
			return licenseJSON(c, 0, "缺少参数：appid / card / domain", nil)
		}
		ctx := context.Background()
		app, err := st.Q().GetLicenseAppByAppid(ctx, appid)
		if err != nil || app.Enabled != 1 {
			return licenseJSON(c, 0, "项目不存在或已停用", nil)
		}
		lc, err := st.Q().GetLicenseCard(ctx, db.GetLicenseCardParams{AppID: app.ID, Card: card})
		if err != nil {
			return licenseJSON(c, 0, "卡密无效", nil)
		}
		now := time.Now().Unix()
		if lc.ActivatedAt == 0 {
			// 首激：绑域名 + 计时开始；hours=0 为永久卡（expires_at=0 永不过期）
			expires := int64(0)
			if lc.Hours > 0 {
				expires = now + lc.Hours*3600
			}
			if err := st.Q().ActivateLicenseCard(ctx, db.ActivateLicenseCardParams{
				Domain: domain, ActivatedAt: now, ExpiresAt: expires, ID: lc.ID,
			}); err != nil {
				return licenseJSON(c, 0, "激活失败", nil)
			}
			return licenseJSON(c, 1, "激活成功", map[string]any{
				"domain": domain, "hours": lc.Hours, "expires_at": expires,
			})
		}
		if lc.Domain != domain {
			return licenseJSON(c, 0, "卡密已绑定其他域名", nil)
		}
		if lc.ExpiresAt > 0 && lc.ExpiresAt <= now {
			return licenseJSON(c, 0, "卡密已过期", map[string]any{"expires_at": lc.ExpiresAt})
		}
		return licenseJSON(c, 1, "验证通过", map[string]any{
			"domain": lc.Domain, "hours": lc.Hours, "expires_at": lc.ExpiresAt,
		})
	}
}

// normalizeDomain 取纯主机名：去协议/路径/端口/末尾点，小写。
func normalizeDomain(raw string) string {
	d := strings.ToLower(strings.TrimSpace(raw))
	if i := strings.Index(d, "://"); i >= 0 {
		d = d[i+3:]
	}
	if i := strings.IndexAny(d, "/?#"); i >= 0 {
		d = d[:i]
	}
	if i := strings.Index(d, ":"); i >= 0 {
		d = d[:i]
	}
	return strings.TrimSuffix(d, ".")
}

func licenseJSON(c *fun.RouteCtx, code int, msg string, extra map[string]any) error {
	obj := map[string]any{"code": code, "msg": msg}
	for k, v := range extra {
		obj[k] = v
	}
	return epayJSON(c, obj)
}
