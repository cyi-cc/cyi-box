package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cyi-cc/fun"
	"golang.org/x/crypto/bcrypt"

	"github.com/cyi-cc/cyi-box/backend/internal/config"
	"github.com/cyi-cc/cyi-box/backend/internal/payx"
	"github.com/cyi-cc/cyi-box/backend/internal/service"
	"github.com/cyi-cc/cyi-box/backend/internal/store"
)

// mustWired 装配失败即退出：基础设施缺一不可，粘性错误由 fun 容器返回
func mustWired[T any](v *T, err error) *T {
	if err != nil {
		log.Fatalf("boot: wired %T failed: %v", v, err)
	}
	return v
}

func main() {
	cfg := mustWired(fun.Wired[config.Config]())
	st := mustWired(fun.Wired[store.Store]())

	// 首启种子：空库写 admin + 示例工具
	if hash, err := bcrypt.GenerateFromPassword([]byte(cfg.AdminPassword), bcrypt.DefaultCost); err == nil {
		if err := st.SeedAdmin(string(hash)); err != nil {
			log.Fatalf("boot: seed admin failed: %v", err)
		}
	}
	if err := st.SeedTools(); err != nil {
		log.Fatalf("boot: seed tools failed: %v", err)
	}

	f := fun.GetFun()
	if len(cfg.CorsOrigins) > 0 {
		f.CORS(cfg.CorsOrigins...)
	}
	if err := f.BindGuard(&service.AuthGuard{}); err != nil {
		log.Fatalf("boot: bind guard failed: %v", err)
	}
	for _, err := range []error{
		f.BindService(&service.AuthSvc{}),
		f.BindService(&service.UserSvc{}),
		f.BindService(&service.DashboardSvc{}),
		f.BindService(&service.ToolSvc{}),
		f.BindService(&service.VaultSvc{}),
		f.BindService(&service.BookmarkSvc{}),
		f.BindService(&service.ServerSvc{}),
		f.BindService(&service.DbMgrSvc{}),
		f.BindService(&service.WebSvc{}),
		f.BindService(&service.DiskSvc{}),
		f.BindService(&service.ProxySvc{}),
		f.BindService(&service.SettingSvc{}),
		f.BindService(&service.PaySvc{}),
		f.BindService(&service.LicenseSvc{}),
		f.BindService(&service.MemorySvc{}),
	} {
		if err != nil {
			log.Fatalf("boot: bind service failed: %v", err)
		}
	}

	// 支付：上游客户端 + 订单轮询 + 商户通知（单例）
	up := mustWired(fun.Wired[payx.Client]())
	mustWired(fun.Wired[service.PayNotifier]())
	mustWired(fun.Wired[service.PayPoller]())
	service.BootPay(st, up, cfg)

	// SFTP 上传放宽请求体上限（默认 4MB → 512MB）
	f.SetBodyLimit(512 << 20)
	for _, err := range []error{
		f.BindRoute("GET", "/files/download", service.SftpDownloadHandler(st, cfg)),
		f.BindRoute("POST", "/files/upload", service.SftpUploadHandler(st, cfg)),
		f.BindRoute("GET", "/ws/terminal", service.TerminalHandler(st, cfg)),
		f.BindRoute("POST", "/dbm/export", service.DbExportHandler(st, cfg)),
		f.BindRoute("POST", "/dbm/import", service.DbImportHandler(st, cfg)),
		f.BindRoute("POST", "/disk/upload", service.DiskUploadHandler(st, cfg)),
		f.BindRoute("GET", "/disk/download", service.DiskDownloadHandler(st, cfg)),
		f.BindRoute("GET", "/d/*", service.ShareDownloadHandler(st, cfg)),
		f.BindRoute("GET", "/v1/proxy", service.ProxyExtractHandler(st)),
		f.BindRoute("GET", "/v1/license", service.LicenseVerifyHandler(st)),
		f.BindRoute("POST", "/v1/license", service.LicenseVerifyHandler(st)),
		// 共享记忆库：MCP 协议端点 + 公开阅读链接
		f.BindRoute("POST", "/mcp", service.McpHandler(st)),
		f.BindRoute("GET", "/m", service.MemoryReadHandler(st)),
		f.BindRoute("GET", "/m/*", service.MemoryReadHandler(st)),
		// 易支付协议端点（GET/POST 同路由）
		f.BindRoute("GET", "/submit.php", service.PaySubmitHandler(st, up)),
		f.BindRoute("POST", "/submit.php", service.PaySubmitHandler(st, up)),
		f.BindRoute("GET", "/mapi.php", service.PayMapiHandler(st, up)),
		f.BindRoute("POST", "/mapi.php", service.PayMapiHandler(st, up)),
		f.BindRoute("GET", "/api.php", service.PayApiHandler(st)),
		f.BindRoute("POST", "/api.php", service.PayApiHandler(st)),
		f.BindRoute("GET", "/healthz", func(c *fun.RouteCtx) error {
			c.RequestCtx.WriteString("ok")
			return nil
		}),
	} {
		if err != nil {
			log.Fatalf("boot: bind route failed: %v", err)
		}
	}

	// 代理池后台调度：每小时测活、每日 00:00 全量同步
	schedCtx, stopSched := context.WithCancel(context.Background())
	service.StartProxyScheduler(schedCtx, st)

	// 优雅停机：SIGINT/SIGTERM → 等在途请求收尾
	go func() {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
		<-ch
		stopSched()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = f.Shutdown(ctx)
	}()

	log.Printf("cyi-box backend listening on :%d, db=%s", cfg.Port, cfg.DBPath)
	f.Start(cfg.Port)
}
