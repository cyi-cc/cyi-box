// cashier_page.go submit.php 直出的自包含收银台页（不 302、不依赖前端 SPA），
// 二维码由服务端生成 PNG data URL；状态经 /cashier-status 轮询。
package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"html/template"
	"strings"

	"github.com/cyi-cc/fun"
	"github.com/skip2/go-qrcode"

	"github.com/cyi-cc/cyi-box/backend/internal/db"
	"github.com/cyi-cc/cyi-box/backend/internal/store"
)

// cashierTmpl 收银台页面模板（新粗野主义风格，与前端 Cashier.vue 一致）。
var cashierTmpl = template.Must(template.New("cashier").Parse(`<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<meta name="referrer" content="never">
<title>收银台 · ¥{{.Money}}</title>
<style>
:root{--ink:#1c1b1a;--cream:#f7f4ec;--cream2:#e5e0d5;--ink3:#8a8577;--blue:#1677ff;--green:#18a058}
*{box-sizing:border-box;margin:0;padding:0}
body{min-height:100vh;display:flex;align-items:center;justify-content:center;background:var(--cream);padding:20px;font-family:-apple-system,"PingFang SC","Microsoft YaHei",sans-serif;color:var(--ink)}
.box{width:380px;max-width:100%;background:#fff;border:3px solid var(--ink);border-radius:20px;padding:28px;text-align:center;box-shadow:6px 6px 0 var(--ink)}
.head{border-bottom:2px dashed var(--cream2);padding-bottom:14px;margin-bottom:16px}
.logo{font-size:20px;font-weight:900}
.orderno{font-size:11px;color:var(--ink3);font-family:Consolas,monospace;margin-top:4px;word-break:break-all}
.subject{font-size:14px;font-weight:700}
.amount{font-size:40px;font-weight:900;font-family:Consolas,monospace;margin:8px 0 18px}
.amount .cny{font-size:22px;vertical-align:6px;margin-right:2px}
.qr-area{display:flex;flex-direction:column;align-items:center;gap:14px}
.qr-frame{position:relative;padding:14px;background:#fff;border:3px solid var(--ink);border-radius:16px;box-shadow:5px 5px 0 var(--ink)}
.qr-frame img{display:block;width:240px;height:240px;border-radius:8px}
.qr-empty{width:240px;height:240px;display:flex;align-items:center;justify-content:center;color:var(--ink3);font-size:13px}
.corner{position:absolute;width:18px;height:18px;border:3px solid var(--blue)}
.tl{top:-6px;left:-6px;border-right:0;border-bottom:0;border-radius:8px 0 0 0}
.tr{top:-6px;right:-6px;border-left:0;border-bottom:0;border-radius:0 8px 0 0}
.bl{bottom:-6px;left:-6px;border-right:0;border-top:0;border-radius:0 0 0 8px}
.br{bottom:-6px;right:-6px;border-left:0;border-top:0;border-radius:0 0 8px 0}
.tip{font-size:14px;font-weight:700;display:flex;align-items:center;gap:8px}
.badge{display:inline-flex;align-items:center;justify-content:center;width:24px;height:24px;background:var(--blue);color:#fff;border-radius:6px;font-size:14px;font-weight:900;border:2px solid var(--ink)}
.cd{font-size:12px;color:var(--ink3);display:flex;align-items:center;gap:6px;width:100%}
.cd .bar{flex:1;height:8px;background:var(--cream2);border:2px solid var(--ink);border-radius:6px;overflow:hidden}
.cd .bar i{display:block;height:100%;background:var(--blue);transition:width 1s linear}
.state{display:flex;flex-direction:column;align-items:center;gap:12px;padding:30px 0;font-size:15px;font-weight:700}
.ok{color:var(--green);font-size:18px}
.hint{font-size:12px;color:var(--ink3);font-weight:400}
svg{display:block}
</style>
</head>
<body>
<div class="box">
  <div class="head">
    <div class="logo">池易支付</div>
    <div class="orderno">订单号 {{.TradeNo}}</div>
  </div>
  <div class="subject">{{or .Subject "商品订单"}}</div>
  <div class="amount"><span class="cny">¥</span>{{.Money}}</div>

  <div id="wait" class="qr-area"{{if ne .Status 0}} style="display:none"{{end}}>
    <div class="qr-frame">
      {{if .QRDataURL}}<img src="{{.QRDataURL}}" alt="支付宝扫码">{{else}}<div class="qr-empty">二维码生成失败，请刷新重试</div>{{end}}
      <span class="corner tl"></span><span class="corner tr"></span><span class="corner bl"></span><span class="corner br"></span>
    </div>
    <div class="tip"><span class="badge">支</span>打开<b>支付宝</b>扫一扫完成付款</div>
    <div class="cd">&#9200;&nbsp;支付剩余&nbsp;<b id="cdtext">--:--</b><span class="bar"><i id="cdbar"></i></span></div>
  </div>

  <div id="paid" class="state"{{if ne .Status 1}} style="display:none"{{end}}>
    <svg width="56" height="56" viewBox="0 0 24 24" fill="none"><circle cx="12" cy="12" r="11" fill="#18a058"/><path d="M7 12.5l3.2 3.2L17 9" stroke="#fff" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round"/></svg>
    <div class="ok">支付成功</div>
    <div class="hint" id="redir" style="display:none">正在跳回商家页面…</div>
  </div>

  <div id="closed" class="state"{{if ne .Status 3}} style="display:none"{{end}}>
    <svg width="48" height="48" viewBox="0 0 24 24" fill="none"><circle cx="12" cy="12" r="11" fill="#909399"/><path d="M8 8l8 8M16 8l-8 8" stroke="#fff" stroke-width="2.4" stroke-linecap="round"/></svg>
    <div>订单已关闭</div>
  </div>
</div>
<script>
var TRADE_NO = {{.TradeNo}};
var EXPIRED = {{.ExpiredAt}};
var STATUS = {{.Status}};

function setState(s) {
  document.getElementById('wait').style.display = s === 0 ? 'flex' : 'none';
  document.getElementById('paid').style.display = s === 1 ? 'flex' : 'none';
  document.getElementById('closed').style.display = s === 3 ? 'flex' : 'none';
}
var timer = null;
function poll() {
  fetch('/api/cashier-status?trade_no=' + encodeURIComponent(TRADE_NO))
    .then(function (r) { return r.json() })
    .then(function (d) {
      var s = typeof d.status === 'number' ? d.status : 0;
      if (s === 1) {
        setState(1);
        if (d.return_url) {
          document.getElementById('redir').style.display = 'block';
          setTimeout(function () { location.href = d.return_url }, 1200);
        }
        clearInterval(timer);
      } else if (s === 3) {
        setState(3);
        clearInterval(timer);
      }
    })
    .catch(function () {});
}
if (STATUS === 0) {
  timer = setInterval(poll, 2500);
  poll();
}
function tick() {
  var left = Math.max(0, EXPIRED - Math.floor(Date.now() / 1000));
  var m = String(Math.floor(left / 60)).padStart(2, '0');
  var s = String(left % 60).padStart(2, '0');
  var el = document.getElementById('cdtext');
  if (el) el.textContent = m + ':' + s;
  var bar = document.getElementById('cdbar');
  if (bar) bar.style.width = Math.min(100, left / 18) + '%';
}
setInterval(tick, 1000);
tick();
</script>
</body>
</html>`))

type cashierPageData struct {
	TradeNo   string
	Subject   string
	Money     string
	QRDataURL string
	ExpiredAt int64
	Status    int64
}

// renderCashier 渲染收银台 HTML（二维码内容 → PNG data URL）。
func renderCashier(c *fun.RouteCtx, o db.PayOrder) error {
	d := cashierPageData{
		TradeNo:   o.TradeNo,
		Subject:   o.Subject,
		Money:     fenToYuan(o.Money),
		ExpiredAt: o.ExpiredAt,
		Status:    o.Status,
	}
	if o.Qrcode != "" {
		if png, err := qrcode.Encode(o.Qrcode, qrcode.Medium, 256); err == nil {
			d.QRDataURL = "data:image/png;base64," + base64.StdEncoding.EncodeToString(png)
		}
	}
	var buf bytes.Buffer
	if err := cashierTmpl.Execute(&buf, d); err != nil {
		return err
	}
	c.RequestCtx.Response.Header.Set("Content-Type", "text/html; charset=utf-8")
	c.RequestCtx.SetBody(buf.Bytes())
	return nil
}

// PayCashierPageHandler GET /cashier.php?trade_no=：收银台页渲染
// （submit.php 302 落点，trade_no 不可枚举即凭证；重复访问按订单当前状态渲染）。
func PayCashierPageHandler(st *store.Store) fun.RouteHandler {
	return func(c *fun.RouteCtx) error {
		o, err := st.Q().GetPayOrderByTradeNo(context.Background(), strings.TrimSpace(c.Param("trade_no")))
		if err != nil {
			return epayJSON(c, map[string]any{"code": -1, "msg": "订单不存在"})
		}
		lazyCloseExpired(st, &o)
		return renderCashier(c, o)
	}
}

// PayCashierStatusHandler GET /cashier-status?trade_no=：收银台页轮询
// （公开接口，trade_no 不可枚举即凭证；前端经 /api 前缀代理访问）。
func PayCashierStatusHandler(st *store.Store) fun.RouteHandler {
	return func(c *fun.RouteCtx) error {
		status, returnURL, err := cashierStatusReply(st, strings.TrimSpace(c.Param("trade_no")))
		if err != nil {
			return epayJSON(c, map[string]any{"status": -1})
		}
		return epayJSON(c, map[string]any{"status": status, "return_url": returnURL})
	}
}

// cashierStatusReply 收银台状态查询：订单态 + 已支付时的签名回跳地址（RPC 与页面轮询共用）。
func cashierStatusReply(st *store.Store, tradeNo string) (int64, string, error) {
	o, err := st.Q().GetPayOrderByTradeNo(context.Background(), tradeNo)
	if err != nil {
		return 0, "", err
	}
	lazyCloseExpired(st, &o)
	returnURL := ""
	if o.Status == payStatusPaid && o.ReturnUrl != "" {
		if pid, key, err := payMerchant(st); err == nil {
			returnURL = buildSignedURL(o.ReturnUrl, notifyParams(o, pid), key)
		}
	}
	return o.Status, returnURL, nil
}
