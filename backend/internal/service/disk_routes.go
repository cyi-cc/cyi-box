package service

import (
	"errors"
	"fmt"
	"io"
	"mime"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/cyi-cc/fun"
	"github.com/valyala/fasthttp"

	"github.com/cyi-cc/cyi-box/backend/internal/config"
	"github.com/cyi-cc/cyi-box/backend/internal/store"
)

// routeUser 自定义路由的登录用户校验（区别于 routeAdmin：任何登录用户可用）
func routeUser(st *store.Store, ctx *fasthttp.RequestCtx) (store.SessionUser, error) {
	token := string(ctx.QueryArgs().Peek("token"))
	if token == "" {
		ctx.SetStatusCode(fasthttp.StatusUnauthorized)
		return store.SessionUser{}, errors.New("missing token")
	}
	su, err := st.GetSessionUser(token, sessionTTL, sessionRenewAfter)
	if err != nil || su.User.Status != "active" {
		ctx.SetStatusCode(fasthttp.StatusUnauthorized)
		return store.SessionUser{}, errors.New("invalid token")
	}
	return su, nil
}

func routeJSONErr(ctx *fasthttp.RequestCtx, code int, msg string) {
	ctx.SetStatusCode(code)
	ctx.SetContentType("application/json")
	ctx.SetBodyString(`{"status":2,"msg":"` + msg + `"}`)
}

// DiskUploadHandler POST /disk/upload?token= multipart 字段 file
func DiskUploadHandler(st *store.Store, cfg *config.Config) fun.RouteHandler {
	return func(rc *fun.RouteCtx) error {
		ctx := rc.RequestCtx
		su, err := routeUser(st, ctx)
		if err != nil {
			routeJSONErr(ctx, 401, "未登录或登录已失效")
			return nil
		}
		fh, err := ctx.FormFile("file")
		if err != nil {
			routeJSONErr(ctx, 400, "缺少文件")
			return nil
		}
		src, err := fh.Open()
		if err != nil {
			routeJSONErr(ctx, 400, "读取上传失败")
			return nil
		}
		defer src.Close()

		// 落盘名 = 时间戳+随机后缀（不沿用原名，天然免冲突与穿越）
		stored := fmt.Sprintf("%d_%s", time.Now().UnixNano(), safeStored(filepath.Base(fh.Filename)))
		dstPath := filepath.Join(cfg.DiskDir, stored)
		dst, err := os.Create(dstPath)
		if err != nil {
			routeJSONErr(ctx, 500, "写入失败")
			return nil
		}
		size, err := io.Copy(dst, src)
		dst.Close()
		if err != nil {
			_ = os.Remove(dstPath)
			routeJSONErr(ctx, 500, "保存失败")
			return nil
		}

		mimeType := fh.Header.Get("Content-Type")
		f := store.File{
			UserID: su.User.Id, Name: filepath.Base(fh.Filename),
			Stored: stored, Size: size, Mime: mimeType,
		}
		if err := st.InsertFile(&f); err != nil {
			_ = os.Remove(dstPath)
			routeJSONErr(ctx, 500, "记录失败")
			return nil
		}
		ctx.SetContentType("application/json")
		fmt.Fprintf(ctx, `{"status":0,"id":%d,"name":%q,"size":%d}`, f.Id, f.Name, f.Size)
		return nil
	}
}

// serveDiskFile 共用：按原始文件名附件回传
func serveDiskFile(ctx *fasthttp.RequestCtx, dir, stored, name, mimeType string, size int64) {
	path := filepath.Join(dir, stored)
	f, err := os.Open(path)
	if err != nil {
		ctx.SetStatusCode(404)
		ctx.SetBodyString("文件已不存在")
		return
	}
	defer f.Close()
	// RFC 5987：中文文件名走 filename*，再补一份 filename 兜底
	disp := "attachment; filename*=UTF-8''" + url.PathEscape(name)
	ctx.Response.Header.Set("Content-Disposition", disp)
	if mimeType != "" {
		ctx.SetContentType(mimeType)
	} else if mt := mime.TypeByExtension(filepath.Ext(name)); mt != "" {
		ctx.SetContentType(mt)
	} else {
		ctx.SetContentType("application/octet-stream")
	}
	ctx.Response.Header.Set("Content-Length", strconv.FormatInt(size, 10))
	_, _ = io.Copy(ctx, f)
}

// DiskDownloadHandler GET /disk/download?id=&token= 属主下载
func DiskDownloadHandler(st *store.Store, cfg *config.Config) fun.RouteHandler {
	return func(rc *fun.RouteCtx) error {
		ctx := rc.RequestCtx
		su, err := routeUser(st, ctx)
		if err != nil {
			routeJSONErr(ctx, 401, "未登录或登录已失效")
			return nil
		}
		id, _ := strconv.ParseInt(string(ctx.QueryArgs().Peek("id")), 10, 64)
		f, err := st.GetFile(id)
		if err != nil || f.UserID != su.User.Id {
			ctx.SetStatusCode(404)
			ctx.SetBodyString("文件不存在")
			return nil
		}
		serveDiskFile(ctx, cfg.DiskDir, f.Stored, f.Name, f.Mime, f.Size)
		return nil
	}
}

// ShareDownloadHandler GET /d/* 公开分享下载：code 即 wildcard 路径，过期返回 410
func ShareDownloadHandler(st *store.Store, cfg *config.Config) fun.RouteHandler {
	return func(rc *fun.RouteCtx) error {
		ctx := rc.RequestCtx
		code := strings.Trim(rc.Wildcard, "/")
		if code == "" {
			ctx.SetStatusCode(404)
			ctx.SetBodyString("链接无效")
			return nil
		}
		v, err := st.GetShareFile(code)
		if err != nil {
			ctx.SetStatusCode(404)
			ctx.SetBodyString("分享链接不存在或已删除")
			return nil
		}
		if v.ExpiresAt > 0 && time.Now().Unix() > v.ExpiresAt {
			ctx.SetStatusCode(410)
			ctx.SetContentType("text/html; charset=utf-8")
			ctx.SetBodyString(`<!doctype html><meta charset="utf-8"><body style="font-family:sans-serif;display:flex;height:100vh;align-items:center;justify-content:center;background:#f8f6f6"><div style="text-align:center"><h1 style="font-size:48px;margin:0">链接已过期</h1><p style="color:#8a857e">这个分享已经过了有效期，找文件的主人再要一个新链接吧</p></div></body>`)
			return nil
		}
		_ = st.IncShareDownloads(code)
		serveDiskFile(ctx, cfg.DiskDir, v.Stored, v.FileName, v.Mime, v.Size)
		return nil
	}
}
