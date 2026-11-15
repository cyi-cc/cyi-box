package service

import (
	"errors"

	"github.com/cyi-cc/fun"
	"github.com/valyala/fasthttp"

	"github.com/cyi-cc/cyi-box/backend/internal/store"
)

// 认证统一业务错误码：前端响应拦截器据此清理登录态并跳登录页
const authErrCode = 4011

type authPolicy uint8

const (
	policyDeny authPolicy = iota
	policyPublic
	policyAuthenticated
	policyAdmin
)

// endpointPolicies 显式端点策略表：缺省拒绝，新端点必须登记
var endpointPolicies = map[string]authPolicy{
	"AuthSvc.Login":          policyPublic,
	"AuthSvc.Me":             policyAuthenticated,
	"AuthSvc.Logout":         policyAuthenticated,
	"AuthSvc.ChangePassword": policyAuthenticated,
	"DashboardSvc.Stats":     policyAuthenticated,
	"ToolSvc.List":           policyAuthenticated,
	"ToolSvc.AdminList":      policyAdmin,
	"ToolSvc.SaveTool":       policyAdmin,
	"ToolSvc.DeleteTool":     policyAdmin,
	"VaultSvc.List":          policyAuthenticated,
	"VaultSvc.Save":          policyAuthenticated,
	"VaultSvc.Reveal":        policyAuthenticated,
	"VaultSvc.Totp":          policyAuthenticated,
	"VaultSvc.Delete":        policyAuthenticated,
	"BookmarkSvc.List":       policyAuthenticated,
	"BookmarkSvc.Save":       policyAuthenticated,
	"BookmarkSvc.Delete":     policyAuthenticated,
	"ServerSvc.List":         policyAdmin,
	"ServerSvc.Save":         policyAdmin,
	"ServerSvc.Delete":       policyAdmin,
	"ServerSvc.Metrics":      policyAdmin,
	"ServerSvc.SftpList":     policyAdmin,
	"ServerSvc.SftpMkdir":    policyAdmin,
	"ServerSvc.SftpRemove":   policyAdmin,
	"ServerSvc.SftpRename":   policyAdmin,
	"DbMgrSvc.List":          policyAdmin,
	"DbMgrSvc.Save":          policyAdmin,
	"DbMgrSvc.Delete":        policyAdmin,
	"DbMgrSvc.Test":          policyAdmin,
	"DbMgrSvc.Databases":     policyAdmin,
	"DbMgrSvc.Tables":        policyAdmin,
	"DbMgrSvc.Query":         policyAdmin,
	"WebSvc.Fetch":           policyAuthenticated,
	"DiskSvc.List":           policyAuthenticated,
	"DiskSvc.DeleteFile":     policyAuthenticated,
	"DiskSvc.CreateShare":    policyAuthenticated,
	"DiskSvc.ListShares":     policyAuthenticated,
	"DiskSvc.DeleteShare":    policyAuthenticated,
	"CaptchaSvc.ListKeys":    policyAdmin,
	"CaptchaSvc.SaveKey":     policyAdmin,
	"CaptchaSvc.DeleteKey":   policyAdmin,
	"SettingSvc.Get":         policyAuthenticated,
	"SettingSvc.Set":         policyAdmin,
	"PaySvc.List":            policyAdmin,
	"PaySvc.Stats":           policyAdmin,
	"PaySvc.UpstreamStatus":  policyAdmin,
	"PaySvc.SaveUpstream":    policyAdmin,
	"PaySvc.TestPay":         policyAdmin,
	"PaySvc.CashierInfo":     policyPublic,
	"PaySvc.CashierStatus":   policyPublic,
	"PaySvc.Watch":           policyPublic,
	"LicenseSvc.Apps":        policyAdmin,
	"LicenseSvc.SaveApp":     policyAdmin,
	"LicenseSvc.DeleteApp":   policyAdmin,
	"LicenseSvc.List":        policyAdmin,
	"LicenseSvc.Stats":       policyAdmin,
	"LicenseSvc.Generate":    policyAdmin,
	"LicenseSvc.Delete":      policyAdmin,
	"MemorySvc.Projects":     policyAdmin,
	"MemorySvc.List":         policyAdmin,
	"MemorySvc.Get":          policyAdmin,
	"MemorySvc.Save":         policyAdmin,
	"MemorySvc.Delete":       policyAdmin,
	"MemorySvc.McpInfo":      policyAdmin,
	"MemorySvc.ResetToken":   policyAdmin,
}

// AuthGuard 唯一 RPC 鉴权边界：token 由前端请求拦截器写入 ctx.State["token"]，
// SQLite 会话表校验有效性，命中后用户经 SetUserValue 传给服务层
type AuthGuard struct {
	Store *store.Store
}

func (g *AuthGuard) Guard(ctx fun.Ctx) error {
	policy := endpointPolicies[ctx.ServiceName+"."+ctx.MethodName]
	switch policy {
	case policyPublic:
		return nil
	case policyDeny:
		return fun.Error(4003, "接口未登记鉴权策略")
	}
	if ctx.RequestCtx == nil {
		return errors.New("auth guard: missing request context")
	}
	token := ctx.State["token"]
	if token == "" {
		return fun.Error(authErrCode, "请先登录")
	}
	if err := g.Store.CheckSession(token, sessionTTL, sessionRenewAfter); errors.Is(err, store.ErrNotFound) {
		return fun.Error(authErrCode, "登录已失效，请重新登录")
	} else if err != nil {
		return err
	}
	// 单用户模型：会话有效即管理员，policyAuthenticated 与 policyAdmin 同判
	ctx.RequestCtx.SetUserValue(sessionKey{}, token)
	return nil
}

type sessionKey struct{}

// sessionToken 取 Guard 校验过的会话 token；策略表保证进入服务层时必然已设置
func sessionToken(ctx *fasthttp.RequestCtx) (string, error) {
	if ctx == nil {
		return "", errors.New("auth session: missing request context")
	}
	token, ok := ctx.UserValue(sessionKey{}).(string)
	if !ok || token == "" {
		return "", errors.New("auth session: validated session missing")
	}
	return token, nil
}
