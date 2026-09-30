// pay_routes.go 易支付兼容协议端点：/submit.php（跳收银台）、/mapi.php（JSON下单）、/api.php（查单）。
package service

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/cyi-cc/fun"

	"github.com/cyi-cc/cyi-box/backend/internal/payx"
	"github.com/cyi-cc/cyi-box/backend/internal/store"
)

// paySubmitHandler GET/POST /submit.php：验签建单后 302 到收银台。
func PaySubmitHandler(st *store.Store, up *payx.Client) fun.RouteHandler {
	return func(c *fun.RouteCtx) error {
		order, err := epayCreateOrder(c, st, up)
		if err != nil {
			return epayFail(c, err)
		}
		// 当码支付：优先跳二维码支付链接（qr.alipay.com），退化到本地收银台
		target := order.Qrcode
		if target == "" {
			target = "/cashier/" + order.TradeNo
		}
		c.RequestCtx.Redirect(target, 302)
		return nil
	}
}

// payMapiHandler GET/POST /mapi.php：验签建单，JSON 返回 payurl/qrcode。
func PayMapiHandler(st *store.Store, up *payx.Client) fun.RouteHandler {
	return func(c *fun.RouteCtx) error {
		order, err := epayCreateOrder(c, st, up)
		if err != nil {
			return epayFail(c, err)
		}
		payurl := order.Qrcode
		if payurl == "" {
			payurl = order.Payurl
		}
		return epayJSON(c, map[string]any{
			"code": 1, "msg": "success",
			"trade_no":     order.TradeNo,
			"out_trade_no": order.OutTradeNo,
			"type":         order.Channel,
			"name":         order.Subject,
			"money":        fenToYuan(order.Money),
			"payurl":       payurl,
			"qrcode":       order.Qrcode,
			"urlscheme":    order.Qrcode,
		})
	}
}

// PayApiHandler GET/POST /api.php：act=order 查单（pid+key 验签）。
func PayApiHandler(st *store.Store) fun.RouteHandler {
	return func(c *fun.RouteCtx) error {
		act := strings.TrimSpace(c.Param("act"))
		pidStr := strings.TrimSpace(c.Param("pid"))
		key := strings.TrimSpace(c.Param("key"))
		merPid, merKey, err := payMerchant(st)
		if err != nil || pidStr != merPid || key != merKey {
			return epayJSON(c, map[string]any{"code": -1, "msg": "商户校验失败"})
		}
		if act != "order" {
			return epayJSON(c, map[string]any{"code": -1, "msg": "不支持的 act"})
		}
		tradeNo := strings.TrimSpace(c.Param("trade_no"))
		outNo := strings.TrimSpace(c.Param("out_trade_no"))
		var order struct {
			TradeNo    string
			OutTradeNo string
			Subject    string
			Money      int64
			Status     int64
		}
		if tradeNo != "" {
			r, err := st.Q().GetPayOrderByTradeNo(context.Background(), tradeNo)
			if err != nil {
				return epayJSON(c, map[string]any{"code": -1, "msg": "订单不存在"})
			}
			order.TradeNo, order.OutTradeNo, order.Subject = r.TradeNo, r.OutTradeNo, r.Subject
			order.Money, order.Status = r.Money, r.Status
		} else if outNo != "" {
			r, err := st.Q().GetPayOrderByOutNo(context.Background(), outNo)
			if err != nil {
				return epayJSON(c, map[string]any{"code": -1, "msg": "订单不存在"})
			}
			order.TradeNo, order.OutTradeNo, order.Subject = r.TradeNo, r.OutTradeNo, r.Subject
			order.Money, order.Status = r.Money, r.Status
		} else {
			return epayJSON(c, map[string]any{"code": -1, "msg": "请传入 out_trade_no 或 trade_no"})
		}
		status := int64(0)
		if order.Status == payStatusPaid {
			status = 1
		}
		return epayJSON(c, map[string]any{
			"code": 1, "msg": "查询订单号成功",
			"trade_no":     order.TradeNo,
			"out_trade_no": order.OutTradeNo,
			"name":         order.Subject,
			"money":        fenToYuan(order.Money),
			"status":       status,
		})
	}
}

// epayOrderResult 建单结果（submit/mapi 共用）。
type epayOrderResult struct {
	TradeNo    string
	OutTradeNo string
	Channel    string
	Subject    string
	Money      int64
	Payurl     string
	Qrcode     string
}

// epayCreateOrder submit/mapi 共用下单逻辑：参数校验 → 验签 → 幂等建单。
func epayCreateOrder(c *fun.RouteCtx, st *store.Store, up *payx.Client) (epayOrderResult, error) {
	var out epayOrderResult
	pid := strings.TrimSpace(c.Param("pid"))
	merPid, merKey, err := payMerchant(st)
	if err != nil || merPid == "" {
		return out, errParam("商户未配置")
	}
	if pid == "" || pid != merPid {
		return out, errParam("PID 参数错误")
	}
	channel := strings.TrimSpace(c.Param("type"))
	if channel == "" {
		channel = "alipay"
	}
	if channel != "alipay" {
		return out, errParam("当前仅支持支付宝支付（alipay）")
	}
	outTradeNo := strings.TrimSpace(c.Param("out_trade_no"))
	if outTradeNo == "" || len(outTradeNo) > 64 {
		return out, errParam("商户订单号 out_trade_no 需为 1-64 位")
	}
	subject := strings.TrimSpace(c.Param("name"))
	if subject == "" || len(subject) > 100 {
		return out, errParam("商品名称 name 需为 1-100 个字符")
	}
	money, err := yuanToFen(c.Param("money"))
	if err != nil || money <= 0 || money > payMaxFen {
		return out, errParam("订单金额非法（0-2000 元）")
	}
	notifyURL := strings.TrimSpace(c.Param("notify_url"))
	returnURL := strings.TrimSpace(c.Param("return_url"))
	for _, u := range []string{notifyURL, returnURL} {
		if u != "" && !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
			return out, errParam("notify_url / return_url 须为合法 URL")
		}
	}
	if !epayVerify(c.Data, merKey, strings.TrimSpace(c.Param("sign"))) {
		return out, errParam("签名验证失败")
	}
	order, err := createPayOrder(st, up, createPayOrderIn{
		OutTradeNo: outTradeNo, Channel: channel, Subject: subject,
		Money: money, NotifyURL: notifyURL, ReturnURL: returnURL,
		ClientIP: c.RequestCtx.RemoteIP().String(),
	})
	if err != nil {
		return out, err
	}
	out.TradeNo, out.OutTradeNo, out.Channel = order.TradeNo, order.OutTradeNo, order.Channel
	out.Subject, out.Money, out.Payurl, out.Qrcode = order.Subject, order.Money, order.Payurl, order.Qrcode
	return out, nil
}

func epayJSON(c *fun.RouteCtx, obj map[string]any) error {
	raw, err := json.Marshal(obj)
	if err != nil {
		return err
	}
	c.RequestCtx.Response.Header.Set("Content-Type", "application/json; charset=utf-8")
	c.RequestCtx.Write(raw)
	return nil
}

func epayFail(c *fun.RouteCtx, err error) error {
	return epayJSON(c, map[string]any{"code": -1, "msg": err.Error()})
}

func errParam(msg string) error { return fun.Error(4001, msg) }
