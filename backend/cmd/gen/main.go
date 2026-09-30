// Command gen 生成前端 TS 客户端：BindServiceForGen 只反射方法签名，
// 不装配数据库等基础设施，离线可用。用法：cd backend && go run ./cmd/gen
package main

import (
	"log"

	"github.com/cyi-cc/fun"

	"github.com/cyi-cc/cyi-box/backend/internal/service"
)

func main() {
	f := fun.GetFun()
	f.BindServiceForGen(&service.AuthSvc{})
	f.BindServiceForGen(&service.UserSvc{})
	f.BindServiceForGen(&service.DashboardSvc{})
	f.BindServiceForGen(&service.ToolSvc{})
	f.BindServiceForGen(&service.VaultSvc{})
	f.BindServiceForGen(&service.BookmarkSvc{})
	f.BindServiceForGen(&service.ServerSvc{})
	f.BindServiceForGen(&service.DbMgrSvc{})
	f.BindServiceForGen(&service.WebSvc{})
	f.BindServiceForGen(&service.DiskSvc{})
	f.BindServiceForGen(&service.ProxySvc{})
	f.BindServiceForGen(&service.SettingSvc{})
	f.BindServiceForGen(&service.PaySvc{})
	f.BindServiceForGen(&service.LicenseSvc{})
	f.BindServiceForGen(&service.MemorySvc{})

	fun.SetOutput("../frontend/src/api")
	fun.GenCode(fun.GenTs{})
	log.Println("gen: TS client written to frontend/src/api/ts")
}
