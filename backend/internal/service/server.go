package service

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/cyi-cc/fun"
	"github.com/fasthttp/websocket"
	"github.com/pkg/sftp"
	"github.com/valyala/fasthttp"
	"golang.org/x/crypto/ssh"

	"github.com/cyi-cc/cyi-box/backend/internal/config"
	"github.com/cyi-cc/cyi-box/backend/internal/dto"
	"github.com/cyi-cc/cyi-box/backend/internal/sshx"
	"github.com/cyi-cc/cyi-box/backend/internal/store"
	"github.com/cyi-cc/cyi-box/backend/internal/vault"
)

// ServerSvc 服务器管理：连接密钥加密落库，所有 SSH/SFTP 端点仅管理员。
// 指标采集与文件操作每次新拨号——管理场景连接低频，免去连接池维护成本。
type ServerSvc struct {
	fun.Ctx
	Store *store.Store
	Cfg   *config.Config `fun:"auto"`
}

func (s *ServerSvc) List() ([]dto.ServerView, error) {
	items, err := s.Store.ListServers()
	if err != nil {
		return nil, fmt.Errorf("servers: list failed: %w", err)
	}
	views := make([]dto.ServerView, 0, len(items))
	for _, e := range items {
		views = append(views, serverView(e))
	}
	return views, nil
}

func (s *ServerSvc) Save(d dto.SaveServerDto) (dto.ServerView, error) {
	name := strings.TrimSpace(d.Name)
	host := strings.TrimSpace(d.Host)
	username := strings.TrimSpace(d.Username)
	if name == "" || len(name) > 64 {
		return dto.ServerView{}, fun.Error(4001, "名称必填且不超过 64 字符")
	}
	if host == "" || len(host) > 255 {
		return dto.ServerView{}, fun.Error(4001, "主机地址必填")
	}
	if username == "" {
		return dto.ServerView{}, fun.Error(4001, "用户名必填")
	}
	authType := "password"
	if d.AuthType == "key" {
		authType = "key"
	}
	port := int64(22)
	if d.Port != nil {
		if *d.Port < 1 || *d.Port > 65535 {
			return dto.ServerView{}, fun.Error(4001, "端口范围 1-65535")
		}
		port = *d.Port
	}

	e := store.Server{Name: name, Host: host, Port: port, Username: username, AuthType: authType}
	if d.Id != nil {
		old, err := s.Store.GetServer(*d.Id)
		if errors.Is(err, store.ErrNotFound) {
			return dto.ServerView{}, fun.Error(4001, "服务器不存在")
		}
		if err != nil {
			return dto.ServerView{}, err
		}
		e = old
		e.Name, e.Host, e.Port, e.Username, e.AuthType = name, host, port, username, authType
	}
	if d.Note != nil {
		e.Note = strings.TrimSpace(*d.Note)
	}
	if d.Secret != nil {
		secret := strings.TrimSpace(*d.Secret)
		if secret == "" {
			e.SecretEnc = ""
		} else {
			enc, err := vault.Encrypt(s.Cfg.VaultKey, secret)
			if err != nil {
				return dto.ServerView{}, err
			}
			e.SecretEnc = enc
		}
	}
	if e.SecretEnc == "" {
		return dto.ServerView{}, fun.Error(4001, "请填写密码或私钥")
	}
	if err := s.Store.SaveServer(&e); err != nil {
		return dto.ServerView{}, fmt.Errorf("servers: save failed: %w", err)
	}
	return serverView(e), nil
}

func (s *ServerSvc) Delete(d dto.ServerIdDto) (dto.OkView, error) {
	if err := s.Store.DeleteServer(d.Id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return dto.OkView{}, fun.Error(4001, "服务器不存在")
		}
		return dto.OkView{}, fmt.Errorf("servers: delete failed: %w", err)
	}
	return dto.OkView{Message: "已删除"}, nil
}

// Metrics SSH 拨号 + 采集；拨不通直接返回带因的错误，前端即「离线」
func (s *ServerSvc) Metrics(d dto.ServerIdDto) (dto.ServerMetricsView, error) {
	client, err := s.dial(d.Id)
	if err != nil {
		return dto.ServerMetricsView{}, err
	}
	defer client.Close()
	m, err := sshx.CollectMetrics(client)
	if err != nil {
		return dto.ServerMetricsView{}, err
	}
	v := dto.ServerMetricsView{
		Hostname:      m.Hostname,
		Os:            m.OS,
		Kernel:        m.Kernel,
		UptimeSeconds: m.UptimeSeconds,
		CpuPercent:    m.CpuPercent,
		Load1:         m.Load1,
		Load5:         m.Load5,
		Load15:        m.Load15,
		MemTotal:      m.MemTotal,
		MemUsed:       m.MemUsed,
		MemAvail:      m.MemAvail,
		SwapTotal:     m.SwapTotal,
		SwapUsed:      m.SwapUsed,
		DiskTotal:     m.DiskTotal,
		DiskUsed:      m.DiskUsed,
		DiskAvail:     m.DiskAvail,
		NetRxRate:     m.NetRxRate,
		NetTxRate:     m.NetTxRate,
		NetRxTotal:    m.NetRxTotal,
		NetTxTotal:    m.NetTxTotal,
		CollectedAt:   time.Now().Unix(),
	}
	for _, dsk := range m.Disks {
		v.Disks = append(v.Disks, dto.DiskMountView{
			Mount: dsk.Mount, Fs: dsk.Fs, Total: dsk.Total,
			Used: dsk.Used, Avail: dsk.Avail, Percent: dsk.Percent,
		})
	}
	return v, nil
}

func (s *ServerSvc) dial(id int64) (*ssh.Client, error) {
	e, err := s.Store.GetServer(id)
	if errors.Is(err, store.ErrNotFound) {
		return nil, fun.Error(4001, "服务器不存在")
	}
	if err != nil {
		return nil, err
	}
	return dialServer(e, s.Cfg.VaultKey)
}

// dialServer 解密 secret 并拨号；route 处理器与 RPC 共用
func dialServer(e store.Server, key []byte) (*ssh.Client, error) {
	if e.SecretEnc == "" {
		return nil, fun.Error(4001, "该服务器未配置密码/私钥")
	}
	secret, err := vault.Decrypt(key, e.SecretEnc)
	if err != nil {
		return nil, fmt.Errorf("servers: decrypt secret failed: %w", err)
	}
	return sshx.Dial(e.Host, e.Port, e.Username, e.AuthType, secret)
}

func serverView(e store.Server) dto.ServerView {
	v := dto.ServerView{
		Id:        e.Id,
		Name:      e.Name,
		Host:      e.Host,
		Port:      e.Port,
		Username:  e.Username,
		AuthType:  e.AuthType,
		HasSecret: e.SecretEnc != "",
		CreatedAt: e.CreatedAt,
	}
	if e.Note != "" {
		v.Note = &e.Note
	}
	return v
}

// ---- SFTP RPC ----

// cleanPath 规整远程路径：缺省 "/"，防穿越由 SSH 权限兜底
func cleanPath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return "/"
	}
	return path.Clean(p)
}

func (s *ServerSvc) sftpClient(id int64) (*sftp.Client, *ssh.Client, error) {
	c, err := s.dial(id)
	if err != nil {
		return nil, nil, err
	}
	fc, err := sftp.NewClient(c)
	if err != nil {
		c.Close()
		return nil, nil, fmt.Errorf("sftp: 初始化失败: %w", err)
	}
	return fc, c, nil
}

func (s *ServerSvc) SftpList(d dto.SftpPathDto) (dto.SftpListView, error) {
	fc, c, err := s.sftpClient(d.Id)
	if err != nil {
		return dto.SftpListView{}, err
	}
	defer fc.Close()
	defer c.Close()
	p := cleanPath(d.Path)
	fis, err := fc.ReadDir(p)
	if err != nil {
		return dto.SftpListView{}, fun.Error(4001, "无法读取目录："+err.Error())
	}
	v := dto.SftpListView{Path: p, Entries: []dto.SftpEntryView{}}
	for _, fi := range fis {
		v.Entries = append(v.Entries, dto.SftpEntryView{
			Name:    fi.Name(),
			Size:    fi.Size(),
			Mode:    fi.Mode().String(),
			IsDir:   fi.IsDir(),
			ModTime: fi.ModTime().Unix(),
		})
	}
	return v, nil
}

func (s *ServerSvc) SftpMkdir(d dto.SftpPathDto) (dto.OkView, error) {
	fc, c, err := s.sftpClient(d.Id)
	if err != nil {
		return dto.OkView{}, err
	}
	defer fc.Close()
	defer c.Close()
	if err := fc.Mkdir(cleanPath(d.Path)); err != nil {
		return dto.OkView{}, fun.Error(4001, "创建目录失败："+err.Error())
	}
	return dto.OkView{Message: "已创建"}, nil
}

// SftpRemove 删文件或空目录；非空目录拒绝递归删除，避免误删整棵目录树
func (s *ServerSvc) SftpRemove(d dto.SftpPathDto) (dto.OkView, error) {
	fc, c, err := s.sftpClient(d.Id)
	if err != nil {
		return dto.OkView{}, err
	}
	defer fc.Close()
	defer c.Close()
	p := cleanPath(d.Path)
	fi, err := fc.Stat(p)
	if err != nil {
		return dto.OkView{}, fun.Error(4001, "路径不存在")
	}
	if fi.IsDir() {
		err = fc.RemoveDirectory(p)
	} else {
		err = fc.Remove(p)
	}
	if err != nil {
		return dto.OkView{}, fun.Error(4001, "删除失败："+err.Error()+"（目录需为空）")
	}
	return dto.OkView{Message: "已删除"}, nil
}

func (s *ServerSvc) SftpRename(d dto.SftpRenameDto) (dto.OkView, error) {
	newName := strings.TrimSpace(d.NewName)
	if newName == "" || strings.ContainsAny(newName, "/\\") {
		return dto.OkView{}, fun.Error(4001, "新名字无效")
	}
	fc, c, err := s.sftpClient(d.Id)
	if err != nil {
		return dto.OkView{}, err
	}
	defer fc.Close()
	defer c.Close()
	old := cleanPath(d.Path)
	if err := fc.PosixRename(old, path.Join(path.Dir(old), newName)); err != nil {
		return dto.OkView{}, fun.Error(4001, "重命名失败："+err.Error())
	}
	return dto.OkView{Message: "已重命名"}, nil
}

// ---- 自定义路由：上传 / 下载 / WS 终端 ----

// routeAdmin 自定义路由不走 AuthGuard，手动校验 query token 且要求管理员
func routeAdmin(st *store.Store, ctx *fasthttp.RequestCtx) error {
	token := string(ctx.QueryArgs().Peek("token"))
	if token == "" {
		ctx.SetStatusCode(fasthttp.StatusUnauthorized)
		return errors.New("missing token")
	}
	su, err := st.GetSessionUser(token, sessionTTL, sessionRenewAfter)
	if err != nil {
		ctx.SetStatusCode(fasthttp.StatusUnauthorized)
		return errors.New("invalid token")
	}
	if su.User.Role != "admin" || su.User.Status != "active" {
		ctx.SetStatusCode(fasthttp.StatusForbidden)
		return errors.New("admin only")
	}
	return nil
}

func routeServer(st *store.Store, ctx *fasthttp.RequestCtx) (store.Server, error) {
	id, _ := strconv.ParseInt(string(ctx.QueryArgs().Peek("id")), 10, 64)
	e, err := st.GetServer(id)
	if err != nil {
		ctx.SetStatusCode(fasthttp.StatusNotFound)
		ctx.SetBodyString("server not found")
	}
	return e, err
}

// SftpDownloadHandler GET /files/download?id=&path=&token= 流式回传文件
func SftpDownloadHandler(st *store.Store, cfg *config.Config) fun.RouteHandler {
	return func(rc *fun.RouteCtx) error {
		ctx := rc.RequestCtx
		if err := routeAdmin(st, ctx); err != nil {
			return nil
		}
		e, err := routeServer(st, ctx)
		if err != nil {
			return nil
		}
		client, err := dialServer(e, cfg.VaultKey)
		if err != nil {
			ctx.SetStatusCode(502)
			ctx.SetBodyString("SSH 连接失败：" + err.Error())
			return nil
		}
		fc, err := sftp.NewClient(client)
		if err != nil {
			client.Close()
			ctx.SetStatusCode(502)
			ctx.SetBodyString("SFTP 初始化失败")
			return nil
		}
		p := cleanPath(string(ctx.QueryArgs().Peek("path")))
		f, err := fc.Open(p)
		if err != nil {
			fc.Close()
			client.Close()
			ctx.SetStatusCode(404)
			ctx.SetBodyString("文件不可读：" + err.Error())
			return nil
		}
		if fi, err := f.Stat(); err == nil {
			ctx.Response.Header.Set("Content-Length", strconv.FormatInt(fi.Size(), 10))
		}
		name := path.Base(p)
		ctx.Response.Header.Set("Content-Disposition", "attachment; filename*=UTF-8''"+urlPathEscape(name))
		ctx.SetContentType("application/octet-stream")
		// 流式回传：连接生命周期交给 StreamWriter 收尾
		ctx.SetBodyStreamWriter(func(w *bufio.Writer) {
			_, _ = io.Copy(w, f)
			f.Close()
			fc.Close()
			client.Close()
		})
		return nil
	}
}

func urlPathEscape(s string) string {
	var b strings.Builder
	for _, c := range []byte(s) {
		if c < 0x20 || c > 0x7e || strings.ContainsRune("\"%;+", rune(c)) {
			fmt.Fprintf(&b, "%%%02X", c)
		} else {
			b.WriteByte(c)
		}
	}
	return b.String()
}

// SftpUploadHandler POST /files/upload?id=&dir=&token= multipart 字段 file
func SftpUploadHandler(st *store.Store, cfg *config.Config) fun.RouteHandler {
	return func(rc *fun.RouteCtx) error {
		ctx := rc.RequestCtx
		if err := routeAdmin(st, ctx); err != nil {
			ctx.SetBodyString(`{"status":2,"msg":"未授权"}`)
			return nil
		}
		e, err := routeServer(st, ctx)
		if err != nil {
			return nil
		}
		fh, err := ctx.FormFile("file")
		if err != nil {
			ctx.SetStatusCode(400)
			ctx.SetBodyString(`{"status":2,"msg":"缺少文件"}`)
			return nil
		}
		src, err := fh.Open()
		if err != nil {
			ctx.SetStatusCode(400)
			ctx.SetBodyString(`{"status":2,"msg":"读取上传失败"}`)
			return nil
		}
		defer src.Close()

		client, err := dialServer(e, cfg.VaultKey)
		if err != nil {
			ctx.SetStatusCode(502)
			ctx.SetBodyString(`{"status":2,"msg":"SSH 连接失败"}`)
			return nil
		}
		defer client.Close()
		fc, err := sftp.NewClient(client)
		if err != nil {
			ctx.SetStatusCode(502)
			ctx.SetBodyString(`{"status":2,"msg":"SFTP 初始化失败"}`)
			return nil
		}
		defer fc.Close()

		dir := cleanPath(string(ctx.QueryArgs().Peek("dir")))
		dst := path.Join(dir, path.Base(fh.Filename))
		w, err := fc.Create(dst)
		if err != nil {
			ctx.SetStatusCode(500)
			ctx.SetBodyString(`{"status":2,"msg":"创建远端文件失败：` + err.Error() + `"}`)
			return nil
		}
		if _, err := io.Copy(w, src); err != nil {
			w.Close()
			ctx.SetStatusCode(500)
			ctx.SetBodyString(`{"status":2,"msg":"写入失败：` + err.Error() + `"}`)
			return nil
		}
		w.Close()
		ctx.SetContentType("application/json")
		ctx.SetBodyString(`{"status":0,"msg":"ok"}`)
		return nil
	}
}

// TerminalHandler GET /ws/terminal?id=&token= 升级 WebSocket → SSH PTY。
// 协议：客户端→服务端首字节 0x01 为 JSON 控制帧 {"cols":N,"rows":N}（resize），
// 其余原样写入 shell stdin；服务端→客户端全部原样回显。
func TerminalHandler(st *store.Store, cfg *config.Config) fun.RouteHandler {
	upgrader := websocket.FastHTTPUpgrader{
		CheckOrigin: func(*fasthttp.RequestCtx) bool { return true },
	}
	return func(rc *fun.RouteCtx) error {
		ctx := rc.RequestCtx
		if err := routeAdmin(st, ctx); err != nil {
			ctx.SetBodyString("unauthorized")
			return nil
		}
		e, err := routeServer(st, ctx)
		if err != nil {
			return nil
		}
		client, err := dialServer(e, cfg.VaultKey)
		if err != nil {
			ctx.SetStatusCode(502)
			ctx.SetBodyString("SSH 连接失败：" + err.Error())
			return nil
		}
		// Upgrade 内部 hijack 连接后异步跑回调；ssh.Client 生命周期归 runTerm
		if err := upgrader.Upgrade(ctx, func(ws *websocket.Conn) {
			runTerm(ws, client)
		}); err != nil {
			client.Close()
			return nil
		}
		return nil
	}
}

type termResize struct {
	Cols int `json:"cols"`
	Rows int `json:"rows"`
}

func runTerm(ws *websocket.Conn, client *ssh.Client) {
	defer ws.Close()
	defer client.Close()
	sess, err := client.NewSession()
	if err != nil {
		_ = ws.WriteMessage(websocket.TextMessage, []byte("\r\nopen session failed: "+err.Error()+"\r\n"))
		return
	}
	defer sess.Close()
	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}
	if err := sess.RequestPty("xterm-256color", 24, 80, modes); err != nil {
		_ = ws.WriteMessage(websocket.TextMessage, []byte("\r\npty failed: "+err.Error()+"\r\n"))
		return
	}
	stdin, err := sess.StdinPipe()
	if err != nil {
		return
	}
	stdout, err := sess.StdoutPipe()
	if err != nil {
		return
	}
	sess.Stderr = sess.Stdout
	if err := sess.Shell(); err != nil {
		_ = ws.WriteMessage(websocket.TextMessage, []byte("\r\nshell failed: "+err.Error()+"\r\n"))
		return
	}

	done := make(chan struct{})
	// ws → stdin
	go func() {
		defer close(done)
		for {
			_, data, err := ws.ReadMessage()
			if err != nil {
				_ = sess.Close()
				return
			}
			if len(data) > 0 && data[0] == 0x01 {
				var r termResize
				if json.Unmarshal(data[1:], &r) == nil && r.Cols > 0 && r.Rows > 0 {
					_ = sess.WindowChange(r.Rows, r.Cols)
				}
				continue
			}
			if _, err := stdin.Write(data); err != nil {
				return
			}
		}
	}()

	// stdout → ws
	buf := make([]byte, 32*1024)
	for {
		n, err := stdout.Read(buf)
		if n > 0 {
			if werr := ws.WriteMessage(websocket.BinaryMessage, buf[:n]); werr != nil {
				break
			}
		}
		if err != nil {
			break
		}
	}
	_ = sess.Close()
	<-done
}
