// captcha_routes.go 滑块拼图验证码公开端点（Turnstile 式 iframe 嵌入）：
//
//	GET  /captcha/widget?sitekey=pk_xxx  iframe 整页：出图内联、滑块交互、
//	                                    postMessage 回传 token；Referer 域名白名单
//	POST /v1/captcha/verify  {id, x}     widget 内部调用：±4px 判定 → 一次性 token
//	GET|POST /v1/captcha/check {secret, token}  对方后端核销：校验+销毁
package service

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/cyi-cc/fun"
	"github.com/valyala/fasthttp"

	"github.com/cyi-cc/cyi-box/backend/internal/captx"
	"github.com/cyi-cc/cyi-box/backend/internal/store"
)

// refererHost 取 Referer 的主机名（无 Referer 返回空串）
func refererHost(ctx *fasthttp.RequestCtx) string {
	ref := strings.TrimSpace(string(ctx.Request.Header.Referer()))
	if ref == "" {
		return ""
	}
	u, err := url.Parse(ref)
	if err != nil {
		return ""
	}
	return strings.ToLower(u.Hostname())
}

// domainAllowed host 命中白名单：精确匹配或 *.后缀（含父域本身）
func domainAllowed(host string, domains []string) bool {
	for _, d := range domains {
		if host == d {
			return true
		}
		if strings.HasPrefix(d, "*.") && (host == d[2:] || strings.HasSuffix(host, "."+d[2:])) {
			return true
		}
	}
	return false
}

func widgetErr(ctx *fasthttp.RequestCtx, code int, msg string) error {
	ctx.SetStatusCode(code)
	ctx.SetContentType("text/plain; charset=utf-8")
	ctx.SetBodyString("cyi-captcha: " + msg)
	return nil
}

// CaptchaWidgetHandler iframe 嵌入页：sitekey 校验 + Referer 域名白名单 +
// frame-ancestors 双重约束，题图内联进 HTML 单次返回。
func CaptchaWidgetHandler(st *store.Store) fun.RouteHandler {
	return func(c *fun.RouteCtx) error {
		ctx := c.RequestCtx
		sitekey := strings.TrimSpace(string(ctx.QueryArgs().Peek("sitekey")))
		key, err := st.CaptchaKeyBySitekey(sitekey)
		if err != nil {
			return widgetErr(ctx, 403, "invalid sitekey")
		}
		domains := splitCSV(key.Domains)
		refHost := refererHost(ctx)
		if len(domains) > 0 && (refHost == "" || !domainAllowed(refHost, domains)) {
			return widgetErr(ctx, 403, "domain not allowed")
		}
		ip := ctx.RemoteIP().String()
		if capBanned(ip) || !capSlotFree(ip) {
			return widgetErr(ctx, 429, "too many open puzzles")
		}
		puz, answerX, err := captx.New()
		if err != nil {
			return widgetErr(ctx, 500, "render failed")
		}
		id := acquirePuzzle(ip, &capPuzzle{
			answerX: answerX, keyID: key.Id,
			exp: time.Now().Add(capPuzzleTTL).Unix(),
		})
		if id == "" {
			return widgetErr(ctx, 429, "too many open puzzles")
		}
		// 域名白名单同步到 CSP frame-ancestors，浏览器层拦非授权嵌入；'self' 保留本站
		if len(domains) > 0 {
			var b strings.Builder
			b.WriteString("frame-ancestors 'self'")
			for _, d := range domains {
				b.WriteString(" https://" + d + " http://" + d)
			}
			ctx.Response.Header.Set("Content-Security-Policy", b.String())
		}
		ctx.Response.Header.Set("Cache-Control", "no-store")
		ctx.SetContentType("text/html; charset=utf-8")
		ctx.WriteString(renderWidget(id,
			base64.StdEncoding.EncodeToString(puz.BG),
			base64.StdEncoding.EncodeToString(puz.Piece), puz.Y))
		return nil
	}
}

// CaptchaVerifyHandler widget 内部调用：校验滑动位置 → 签发一次性 token。
func CaptchaVerifyHandler(st *store.Store) fun.RouteHandler {
	return func(c *fun.RouteCtx) error {
		ip := c.RequestCtx.RemoteIP().String()
		if capBanned(ip) {
			return capJSON(c, 0, "too many failures", nil)
		}
		id := strings.TrimSpace(c.Param("id"))
		x := int(parseI64(c.Param("x")))
		if id == "" {
			return capJSON(c, 0, "missing id", nil)
		}
		keyID, ok := tryPuzzle(ip, id, x)
		if !ok {
			return capJSON(c, 0, "verify failed", nil)
		}
		return capJSON(c, 1, "ok", map[string]any{"token": mintToken(keyID)})
	}
}

// CaptchaCheckHandler 对方后端核销：secret 鉴权 + token 一次性校验销毁。
func CaptchaCheckHandler(st *store.Store) fun.RouteHandler {
	return func(c *fun.RouteCtx) error {
		secret := strings.TrimSpace(c.Param("secret"))
		token := strings.TrimSpace(c.Param("token"))
		if secret == "" || token == "" {
			return capJSON(c, 0, "missing secret/token", nil)
		}
		key, err := st.CaptchaKeyBySecret(secret)
		if err != nil {
			return capJSON(c, 0, "invalid secret", nil)
		}
		if !consumeToken(token, key.Id) {
			return capJSON(c, 0, "token invalid or expired", nil)
		}
		st.TouchCaptchaKey(key.Id)
		return capJSON(c, 1, "ok", map[string]any{"sitekey": key.Sitekey})
	}
}

func capJSON(c *fun.RouteCtx, code int, msg string, extra map[string]any) error {
	obj := map[string]any{"code": code, "msg": msg}
	for k, v := range extra {
		obj[k] = v
	}
	return epayJSON(c, obj)
}

func parseI64(s string) int64 {
	var v int64
	fmt.Sscanf(strings.TrimSpace(s), "%d", &v)
	return v
}

func splitCSV(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// renderWidget 生成自包含 iframe 页：底图/拼图块内联 base64，滑块交互 +
// verify 后 postMessage 把 token 回传父页面。
func renderWidget(id, bg64, piece64 string, y int) string {
	var b strings.Builder
	b.WriteString(`<!doctype html><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<style>
*{margin:0;padding:0;box-sizing:border-box;-webkit-user-select:none;user-select:none}
body{width:300px;overflow:hidden;font-family:system-ui,sans-serif}
.stage{position:relative;width:300px;height:170px;border:2.5px solid #1c1b1a;border-radius:14px;overflow:hidden;background:#f7f4ec}
#bg{display:block;width:300px;height:170px}
#pc{position:absolute;left:0;top:0;cursor:grab;filter:drop-shadow(2px 2px 0 rgba(28,27,26,.35))}
.stage.ok #pc{filter:none}
.re{position:absolute;top:6px;right:6px;width:24px;height:24px;border:2px solid #1c1b1a;border-radius:7px;background:#f7f4ec;cursor:pointer;font-size:14px;line-height:18px;font-weight:900;color:#1c1b1a}
.track{position:relative;margin-top:10px;height:38px;border:2.5px solid #1c1b1a;border-radius:12px;background:#f1eee8;overflow:hidden}
.fill{position:absolute;left:0;top:0;bottom:0;width:0;background:#b7e4c7}
.track.err .fill{background:#f3c1bb}
.track.ok .fill{background:#7ed69f}
.tip{position:absolute;inset:0;display:flex;align-items:center;justify-content:center;font-size:12px;font-weight:700;color:#8a8577;letter-spacing:1px}
.handle{position:absolute;left:0;top:0;width:38px;height:33px;border-right:2.5px solid #1c1b1a;background:#fff;border-radius:9px 0 0 9px;cursor:grab;display:flex;align-items:center;justify-content:center;font-size:15px;color:#1c1b1a;touch-action:none}
.handle:active{cursor:grabbing}
@keyframes shake{0%,100%{transform:translateX(0)}25%{transform:translateX(-4px)}75%{transform:translateX(4px)}}
.shake{animation:shake .15s 3}
</style>
<div class="stage" id="st">
  <img id="bg" src="data:image/png;base64,`)
	b.WriteString(bg64)
	b.WriteString(`" draggable="false">
  <img id="pc" src="data:image/png;base64,`)
	b.WriteString(piece64)
	b.WriteString(`" draggable="false">
  <button class="re" id="re" title="换一张">&#8635;</button>
</div>
<div class="track" id="tr"><div class="fill" id="fl"></div><span class="tip" id="tip">按住滑块拖动完成拼图</span><div class="handle" id="hd">&#10148;</div></div>
<script>
(function(){
var ID=`)
	b.WriteString(fmt.Sprintf("%q", id))
	b.WriteString(`,PY=`)
	b.WriteString(fmt.Sprint(y))
	b.WriteString(`;
var pc=document.getElementById('pc'),hd=document.getElementById('hd'),
tr=document.getElementById('tr'),fl=document.getElementById('fl'),
tip=document.getElementById('tip'),st=document.getElementById('st');
pc.style.top=PY+'px';
var MAXP=300-68,MAXH=300-38,x=0,drag=false,x0=0,done=false;
function setX(v){x=Math.max(0,Math.min(MAXP,v));pc.style.left=x+'px';var hx=x/MAXP*MAXH;hd.style.left=hx+'px';fl.style.width=(hx+38)+'px'}
setX(0);
hd.addEventListener('pointerdown',function(e){if(done)return;drag=true;x0=e.clientX-x;hd.setPointerCapture(e.pointerId)});
hd.addEventListener('pointermove',function(e){if(!drag||done)return;setX(e.clientX-x0)});
hd.addEventListener('pointerup',function(){if(!drag||done)return;drag=false;send()});
document.getElementById('re').onclick=function(){location.reload()};
function parent(){try{return new URL(document.referrer).origin}catch(e){return'*'}}
function send(){
done=true;
fetch('/v1/captcha/verify',{method:'POST',headers:{'Content-Type':'application/x-www-form-urlencoded'},body:'id='+encodeURIComponent(ID)+'&x='+x})
.then(function(r){return r.json()}).then(function(d){
if(d&&d.code===1){tr.className='track ok';st.className='stage ok';tip.textContent='验证通过';
try{window.parent.postMessage({type:'cyi-captcha',token:d.token},parent())}catch(e){}}
else{tr.className='track err shake';tip.textContent='没对齐，再来一次';setTimeout(function(){location.reload()},700)}
}).catch(function(){done=false;setX(0)});
}
})();
</script>`)
	return b.String()
}
