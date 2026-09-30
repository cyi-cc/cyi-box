// pay.go 支付核心：上游 catfk 账号托管、分类/商品自检、卡密库存保障（全程带锁）、
// 订单 RPC、支付轮询与商户异步通知。易支付协议端点在 pay_routes.go。
package service

import (
	"context"
	"crypto/md5"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/cyi-cc/fun"

	"github.com/cyi-cc/cyi-box/backend/internal/config"
	"github.com/cyi-cc/cyi-box/backend/internal/db"
	"github.com/cyi-cc/cyi-box/backend/internal/dto"
	"github.com/cyi-cc/cyi-box/backend/internal/payx"
	"github.com/cyi-cc/cyi-box/backend/internal/store"
	"github.com/cyi-cc/cyi-box/backend/internal/vault"
)

// ---- 常量与 settings 键 ----

const (
	payStatusPending = int64(0)
	payStatusPaid    = int64(1)
	payStatusClosed  = int64(3)

	payUnitFen   = int64(1)      // 上游商品单价 0.01 元 = 1 分
	payTargetFen = int64(100000) // 库存目标 1000 元 = 100000 分 = 10w 张卡
	payCatName   = "CDK"
	payGoodsName = "CDK0.01元"
	payMaxFen    = int64(200000) // 单笔上限 2000 元
)

const (
	keyPayPid      = "pay_pid"
	keyPayKey      = "pay_key"
	keyUpUsername  = "up_username"
	keyUpPassword  = "up_password" // AES 加密存储
	keyUpNickname  = "up_nickname"
	keyUpShop      = "up_shop"
	keyUpGoodsID   = "up_goods_id"
	keyUpGoodsKey  = "up_goods_key"
	keyUpGoodsName = "up_goods_name"
	keyUpLastErr   = "up_last_error"
)

// ---- 易支付签名与金额工具 ----

// epaySign 标准 MD5 签名：参数名 ASCII 升序拼 k=v&…（忽略 sign/sign_type 与空值），末尾直接拼接商户密钥。
func epaySign(params map[string]string, key string) string {
	keys := make([]string, 0, len(params))
	for k, v := range params {
		if k == "sign" || k == "sign_type" || v == "" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		if b.Len() > 0 {
			b.WriteByte('&')
		}
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(params[k])
	}
	b.WriteString(key)
	sum := md5.Sum([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}

func epayVerify(params map[string]string, key, sign string) bool {
	if params["sign_type"] != "MD5" {
		return false
	}
	return sign != "" && epaySign(params, key) == sign
}

// fenToYuan 分转元字符串，如 199 -> "1.99"。
func fenToYuan(fen int64) string {
	sign := ""
	if fen < 0 {
		sign, fen = "-", -fen
	}
	return fmt.Sprintf("%s%d.%02d", sign, fen/100, fen%100)
}

// yuanToFen 元字符串转分（最多两位小数）。
func yuanToFen(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("金额不能为空")
	}
	if strings.HasPrefix(s, "-") {
		return 0, fmt.Errorf("金额不能为负")
	}
	parts := strings.SplitN(s, ".", 2)
	whole, err := strconv.ParseInt(strings.TrimSpace(parts[0]), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("金额格式错误")
	}
	fen := whole * 100
	if len(parts) == 2 {
		frac := parts[1]
		if len(frac) > 2 {
			frac = frac[:2]
		}
		frac += strings.Repeat("0", 2-len(frac))
		f, err := strconv.ParseInt(frac, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("金额格式错误")
		}
		fen += f
	}
	return fen, nil
}

// genTradeNo 平台订单号：P + 时间戳 + 16 位随机 hex（不可枚举）。
func genTradeNo() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return "P" + strconv.FormatInt(time.Now().Unix(), 10) + hex.EncodeToString(b)
}

// ---- 商户凭证 ----

func payMerchant(st *store.Store) (pid, key string, err error) {
	if pid, err = st.GetSetting(keyPayPid); err != nil {
		return "", "", err
	}
	if key, err = st.GetSetting(keyPayKey); err != nil {
		return "", "", err
	}
	return pid, key, nil
}

func strSetting(st *store.Store, key string) string {
	v, _ := st.GetSetting(key)
	return v
}

func intSetting(st *store.Store, key string) int64 {
	v, _ := strconv.ParseInt(strSetting(st, key), 10, 64)
	return v
}

func anyI64(v any) int64 {
	switch t := v.(type) {
	case int64:
		return t
	case float64:
		return int64(t)
	case []byte:
		n, _ := strconv.ParseInt(string(t), 10, 64)
		return n
	case string:
		n, _ := strconv.ParseInt(t, 10, 64)
		return n
	}
	return 0
}

// ---- PaySvc 管理端 RPC ----

type PaySvc struct {
	fun.Ctx
	Store *store.Store
	Up    *payx.Client
	Cfg   *config.Config
}

// List 订单分页。
func (s *PaySvc) List(d dto.PayOrderListDto) (dto.PayOrderPageView, error) {
	page, size := d.Page, d.PageSize
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}
	status := int64(-1)
	if d.Status != nil {
		status = *d.Status
	}
	kw := sql.NullString{}
	if d.Kw != nil {
		kw = sql.NullString{String: strings.TrimSpace(*d.Kw), Valid: true}
	}
	ctx := context.Background()
	arg := db.ListPayOrdersParams{
		Column1: status, Status: status,
		Column3: kw.String, Column4: kw, Column5: kw,
		Limit: size, Offset: (page - 1) * size,
	}
	rows, err := s.Store.Q().ListPayOrders(ctx, arg)
	if err != nil {
		return dto.PayOrderPageView{}, fun.Error(5000, "查询订单失败")
	}
	total, err := s.Store.Q().CountPayOrders(ctx, db.CountPayOrdersParams{
		Column1: arg.Column1, Status: arg.Status,
		Column3: arg.Column3, Column4: arg.Column4, Column5: arg.Column5,
	})
	if err != nil {
		return dto.PayOrderPageView{}, fun.Error(5000, "统计订单失败")
	}
	items := make([]dto.PayOrderView, 0, len(rows))
	for _, r := range rows {
		items = append(items, toPayOrderView(r))
	}
	return dto.PayOrderPageView{Total: total, Items: items}, nil
}

// Stats 今日与累计统计。
func (s *PaySvc) Stats() (dto.PayStatsView, error) {
	now := time.Now()
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Unix()
	ctx := context.Background()
	today, err := s.Store.Q().PayOrderStats(ctx, dayStart)
	if err != nil {
		return dto.PayStatsView{}, fun.Error(5000, "统计失败")
	}
	total, err := s.Store.Q().PayOrderStats(ctx, 0)
	if err != nil {
		return dto.PayStatsView{}, fun.Error(5000, "统计失败")
	}
	return dto.PayStatsView{
		TodayCount: today.Cnt, TodayMoney: anyI64(today.Money), TodayPaidCount: anyI64(today.Paid),
		TotalCount: total.Cnt, TotalMoney: anyI64(total.Money), PendingCount: anyI64(total.Pending),
		TotalPaidCount: anyI64(total.Paid),
	}, nil
}

// UpstreamStatus 上游账号与商品状态（管理页展示）。
func (s *PaySvc) UpstreamStatus() (dto.PayUpstreamStatusView, error) {
	v := dto.PayUpstreamStatusView{
		Username:    strSetting(s.Store, keyUpUsername),
		Nickname:    strSetting(s.Store, keyUpNickname),
		Shop:        strSetting(s.Store, keyUpShop),
		GoodsName:   strSetting(s.Store, keyUpGoodsName),
		LastError:   strSetting(s.Store, keyUpLastErr),
		TokenAge:    s.Up.TokenAge(),
		GoodsId:     intSetting(s.Store, keyUpGoodsID),
		GoodsKey:    strSetting(s.Store, keyUpGoodsKey),
		UnitPrice:   payUnitFen,
		TargetMoney: payTargetFen,
	}
	if payEnsuring.Load() {
		v.Ensuring = 1
	}
	if v.Username != "" {
		v.Configured = 1
	}
	if v.GoodsId > 0 && v.Configured == 1 {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if _, _, stock, err := s.Up.GoodsInfo(ctx, v.GoodsId); err == nil {
			v.StockCount = stock
		}
		if avail, frozen, err := s.Up.WalletInfo(ctx); err == nil {
			v.WalletOk, v.WalletAvail, v.WalletFrozen = 1, avail, frozen
		}
	}
	return v, nil
}

// SaveUpstream 保存上游账号并登录验证；成功后异步触发分类/商品自检与库存补齐。
func (s *PaySvc) SaveUpstream(d dto.PayUpstreamDto) (dto.PayUpstreamStatusView, error) {
	username := strings.TrimSpace(d.Username)
	if username == "" {
		return dto.PayUpstreamStatusView{}, fun.Error(4001, "上游账号不能为空")
	}
	password := d.Password
	if password == "" {
		u, p, err := s.decryptUpPassword()
		if err != nil || u != username || p == "" {
			return dto.PayUpstreamStatusView{}, fun.Error(4001, "请输入上游密码")
		}
		password = p
	}
	if err := s.Up.SetCredentials(username, password); err != nil {
		return dto.PayUpstreamStatusView{}, fun.Error(4002, "上游登录失败："+err.Error())
	}
	nickname, shop, err := s.Up.Userinfo(context.Background())
	if err != nil {
		return dto.PayUpstreamStatusView{}, fun.Error(5000, "获取商户信息失败："+err.Error())
	}
	enc, err := vault.Encrypt(s.Cfg.VaultKey, password)
	if err != nil {
		return dto.PayUpstreamStatusView{}, fun.Error(5000, "凭据加密失败")
	}
	old := strSetting(s.Store, keyUpUsername)
	sets := map[string]string{
		keyUpUsername: username, keyUpPassword: enc,
		keyUpNickname: nickname, keyUpShop: shop,
	}
	if old != "" && old != username {
		sets[keyUpGoodsID], sets[keyUpGoodsKey], sets[keyUpGoodsName] = "", "", ""
	}
	for k, v := range sets {
		if err := s.Store.SetSetting(k, v); err != nil {
			return dto.PayUpstreamStatusView{}, fun.Error(5000, "保存配置失败")
		}
	}
	ensureGoodsAsync(s.Store, s.Up)
	return s.UpstreamStatus()
}

// TestPay 创建一笔真实订单走完整上游流程（收银台二维码 + 轮询结算）。
func (s *PaySvc) TestPay(d dto.PayTestPayDto) (dto.PayTestPayView, error) {
	money, err := yuanToFen(d.Money)
	if err != nil || money <= 0 || money > payMaxFen {
		return dto.PayTestPayView{}, fun.Error(4001, "金额非法（0-2000 元）")
	}
	order, err := createPayOrder(s.Store, s.Up, createPayOrderIn{
		Channel: "alipay", Subject: "测试支付", Money: money,
	})
	if err != nil {
		return dto.PayTestPayView{}, fun.Error(5000, err.Error())
	}
	return dto.PayTestPayView{
		TradeNo: order.TradeNo, Payurl: order.Payurl, Qrcode: order.Qrcode,
		Money: order.Money, Cashier: "/cashier/" + order.TradeNo,
	}, nil
}

// CashierInfo 收银台公开信息（trade_no 不可枚举即凭证）。
func (s *PaySvc) CashierInfo(d dto.PayCashierDto) (dto.PayCashierView, error) {
	o, err := s.Store.Q().GetPayOrderByTradeNo(context.Background(), strings.TrimSpace(d.TradeNo))
	if err != nil {
		return dto.PayCashierView{}, fun.Error(4004, "订单不存在")
	}
	lazyCloseExpired(s.Store, &o)
	return dto.PayCashierView{
		TradeNo: o.TradeNo, OutTradeNo: o.OutTradeNo, Subject: o.Subject,
		Money: o.Money, Channel: o.Channel, Status: o.Status,
		Payurl: o.Payurl, Qrcode: o.Qrcode, Cards: o.Cards,
		CreatedAt: o.CreatedAt, ExpiredAt: o.ExpiredAt,
	}, nil
}

// Watch 收银台状态流：服务端推送订单状态变化，终态（已支付/已关闭）自动关闭。
func (s *PaySvc) Watch(d dto.PayCashierDto) (*fun.Stream[dto.PayCashierStatusView], error) {
	tradeNo := strings.TrimSpace(d.TradeNo)
	o, err := s.Store.Q().GetPayOrderByTradeNo(context.Background(), tradeNo)
	if err != nil {
		return nil, fun.Error(4004, "订单不存在")
	}
	st := &fun.Stream[dto.PayCashierStatusView]{}
	go s.watchLoop(st, tradeNo, o)
	return st, nil
}

func (s *PaySvc) watchLoop(st *fun.Stream[dto.PayCashierStatusView], tradeNo string, first db.PayOrder) {
	defer st.Close()
	last := first.Status
	push := func(status int64, o db.PayOrder) bool {
		msg := dto.PayCashierStatusView{Status: status}
		if status == payStatusPaid && o.ReturnUrl != "" {
			if pid, key, err := payMerchant(s.Store); err == nil {
				msg.ReturnUrl = buildSignedURL(o.ReturnUrl, notifyParams(o, pid), key)
			}
		}
		return st.Send(msg) == nil
	}
	if !push(last, first) || last != payStatusPending {
		return
	}
	deadline := time.Now().Add(40 * time.Minute)
	t := time.NewTicker(2 * time.Second)
	defer t.Stop()
	for time.Now().Before(deadline) {
		<-t.C
		o, err := s.Store.Q().GetPayOrderByTradeNo(context.Background(), tradeNo)
		if err != nil {
			return
		}
		lazyCloseExpired(s.Store, &o)
		if o.Status != last {
			if !push(o.Status, o) {
				return
			}
			last = o.Status
			if o.Status != payStatusPending {
				return
			}
		}
	}
}

// CashierStatus 收银台轮询：已支付且配置了 return_url 时返回签名回跳地址。
func (s *PaySvc) CashierStatus(d dto.PayCashierDto) (dto.PayCashierStatusView, error) {
	o, err := s.Store.Q().GetPayOrderByTradeNo(context.Background(), strings.TrimSpace(d.TradeNo))
	if err != nil {
		return dto.PayCashierStatusView{}, fun.Error(4004, "订单不存在")
	}
	res := dto.PayCashierStatusView{Status: o.Status}
	if o.Status == payStatusPaid && o.ReturnUrl != "" {
		pid, key, err := payMerchant(s.Store)
		if err == nil {
			res.ReturnUrl = buildSignedURL(o.ReturnUrl, notifyParams(o, pid), key)
		}
	}
	return res, nil
}

func toPayOrderView(r db.PayOrder) dto.PayOrderView {
	return dto.PayOrderView{
		Id: r.ID, TradeNo: r.TradeNo, OutTradeNo: r.OutTradeNo, Channel: r.Channel,
		Subject: r.Subject, Money: r.Money, Status: r.Status, Notified: r.Notified,
		Payurl: r.Payurl, PaidAt: r.PaidAt, ExpiredAt: r.ExpiredAt, CreatedAt: r.CreatedAt,
	}
}

func lazyCloseExpired(st *store.Store, o *db.PayOrder) {
	if o.Status == payStatusPending && o.ExpiredAt > 0 && o.ExpiredAt < time.Now().Unix() {
		_ = st.Q().SetPayOrderStatus(context.Background(), db.SetPayOrderStatusParams{
			Status: payStatusClosed, PaidAt: o.PaidAt, Cards: o.Cards, ID: o.ID,
		})
		o.Status = payStatusClosed
	}
}

func (s *PaySvc) decryptUpPassword() (string, string, error) {
	u := strSetting(s.Store, keyUpUsername)
	enc := strSetting(s.Store, keyUpPassword)
	if u == "" || enc == "" {
		return "", "", fmt.Errorf("未保存上游凭据")
	}
	p, err := vault.Decrypt(s.Cfg.VaultKey, enc)
	return u, p, err
}

// ---- 商品自检 + 库存保障 ----

var payEnsuring atomic.Bool

// ensureGoodsAsync 后台幂等确保分类「CDK」与商品「CDK0.01元」存在并补齐库存。
func ensureGoodsAsync(st *store.Store, up *payx.Client) {
	if !payEnsuring.CompareAndSwap(false, true) {
		return
	}
	go func() {
		defer payEnsuring.Store(false)
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		defer cancel()
		if err := ensureGoods(ctx, st, up); err != nil {
			_ = st.SetSetting(keyUpLastErr, err.Error())
			log.Printf("[pay] 上游商品自检失败: %v", err)
			return
		}
		_ = st.SetSetting(keyUpLastErr, "")
	}()
}

// ensureGoods 幂等自检：分类 CDK → 商品 CDK0.01元（¥0.01）→ 库存补到 1000 元（10w 张，1w/批）。
func ensureGoods(ctx context.Context, st *store.Store, up *payx.Client) error {
	goodsID := intSetting(st, keyUpGoodsID)
	goodsKey := strSetting(st, keyUpGoodsKey)
	target := payTargetFen / payUnitFen // 10w 张
	if goodsID > 0 && goodsKey != "" {
		return ensureStockTo(ctx, up, goodsID, target)
	}
	list, total, err := up.GoodsList(ctx, "", 1)
	if err != nil {
		return err
	}
	for page := int64(1); ; page++ {
		for _, it := range list {
			if it.Name == payGoodsName && it.Price == payUnitFen && it.Status == 1 {
				goodsID, goodsKey = it.ID, it.GoodsKey
			}
		}
		if goodsID != 0 || page*20 >= total {
			break
		}
		list, _, err = up.GoodsList(ctx, "", page+1)
		if err != nil {
			return err
		}
	}
	if goodsID == 0 {
		var catID int64
		cats, err := up.CategoryListAll(ctx)
		if err != nil {
			return err
		}
		for _, cat := range cats {
			if cat.Name == payCatName {
				catID = cat.ID
				break
			}
		}
		if catID == 0 {
			if err := up.CategoryAdd(ctx, payCatName); err != nil {
				return err
			}
			cats, err = up.CategoryListAll(ctx)
			if err != nil {
				return err
			}
			for _, cat := range cats {
				if cat.Name == payCatName {
					catID = cat.ID
					break
				}
			}
		}
		if catID == 0 {
			return fmt.Errorf("上游分类「%s」创建失败", payCatName)
		}
		if goodsID, goodsKey, err = up.GoodsAdd(ctx, payGoodsName, catID, 0.01); err != nil {
			return err
		}
		if goodsID <= 0 || goodsKey == "" {
			return fmt.Errorf("上游商品创建未返回有效标识")
		}
		log.Printf("[pay] 已创建上游商品「%s」id=%d", payGoodsName, goodsID)
	}
	for k, v := range map[string]string{
		keyUpGoodsID:   strconv.FormatInt(goodsID, 10),
		keyUpGoodsKey:  goodsKey,
		keyUpGoodsName: payGoodsName,
	} {
		if err := st.SetSetting(k, v); err != nil {
			return err
		}
	}
	return ensureStockTo(ctx, up, goodsID, target)
}

// ensureStockTo 全程带锁把库存补到 target 张。
func ensureStockTo(ctx context.Context, up *payx.Client, goodsID, target int64) error {
	unlock := up.StockLock()
	defer unlock()
	_, err := up.EnsureStock(ctx, goodsID, target)
	return err
}

// ---- 下单 ----

type createPayOrderIn struct {
	OutTradeNo string
	Channel    string
	Subject    string
	Money      int64 // 分
	NotifyURL  string
	ReturnURL  string
	ClientIP   string
}

// createPayOrder 本地建单 → 库存保障（带锁）→ 上游下单 → 写回 payurl/二维码。
// 幂等：同 out_trade_no 复用未支付订单。
func createPayOrder(st *store.Store, up *payx.Client, in createPayOrderIn) (db.PayOrder, error) {
	now := time.Now().Unix()
	tradeNo := genTradeNo()
	outNo := in.OutTradeNo
	if outNo == "" {
		outNo = tradeNo // 内部测试单：商户单号=平台单号
	}
	if exist, err := st.Q().GetPayOrderByOutNo(context.Background(), outNo); err == nil {
		switch {
		case exist.Money != in.Money:
			return db.PayOrder{}, fmt.Errorf("商户订单号重复且金额不一致")
		case exist.Status == payStatusPaid:
			return db.PayOrder{}, fmt.Errorf("该订单已支付")
		case exist.Status != payStatusPending || (exist.ExpiredAt > 0 && exist.ExpiredAt < now):
			return db.PayOrder{}, fmt.Errorf("订单已关闭，请更换订单号")
		default:
			return exist, nil
		}
	}
	goodsID := intSetting(st, keyUpGoodsID)
	goodsKey := strSetting(st, keyUpGoodsKey)
	if goodsID <= 0 || goodsKey == "" {
		return db.PayOrder{}, fmt.Errorf("上游商品未就绪：请先在支付配置页保存上游账号")
	}
	if in.Money%payUnitFen != 0 || in.Money/payUnitFen <= 0 {
		return db.PayOrder{}, fmt.Errorf("金额必须是 ¥0.01 的整数倍")
	}
	quantity := in.Money / payUnitFen
	id, err := st.Q().CreatePayOrder(context.Background(), db.CreatePayOrderParams{
		TradeNo: tradeNo, OutTradeNo: outNo, Channel: in.Channel,
		Subject: in.Subject, Money: in.Money, Fee: 0,
		NotifyUrl: in.NotifyURL, ReturnUrl: in.ReturnURL, ClientIp: in.ClientIP,
		ExpiredAt: now + 30*60, CreatedAt: now,
	})
	if err != nil {
		return db.PayOrder{}, fmt.Errorf("创建订单失败")
	}
	o, err := st.Q().GetPayOrder(context.Background(), id)
	if err != nil {
		return db.PayOrder{}, fmt.Errorf("创建订单失败")
	}
	if err := placeUpstream(st, up, &o, goodsID, goodsKey, quantity); err != nil {
		if o.UpstreamTradeNo == "" {
			_ = st.Q().SetPayOrderStatus(context.Background(), db.SetPayOrderStatusParams{
				Status: payStatusClosed, PaidAt: 0, Cards: "", ID: o.ID,
			})
		}
		return db.PayOrder{}, err
	}
	return o, nil
}

// placeUpstream 库存保障（全程带锁）→ 上游下单 → 回写。
func placeUpstream(st *store.Store, up *payx.Client, o *db.PayOrder, goodsID int64, goodsKey string, quantity int64) error {
	ctx := context.Background()
	shop := strSetting(st, keyUpShop)
	if shop == "" {
		return fmt.Errorf("上游店铺未配置")
	}
	channelID, err := up.AlipayChannelID(ctx, shop)
	if err != nil {
		return fmt.Errorf("上游支付通道不可用：%v", err)
	}
	contact, queryPwd := payx.RandContact(), payx.RandQueryPwd()

	unlock := up.StockLock()
	res, err := placeUnderLock(ctx, up, goodsID, goodsKey, quantity, channelID, contact, queryPwd)
	unlock()
	if err != nil {
		return err
	}
	o.Payurl = res.PayURL
	o.UpstreamTradeNo = res.TradeNo
	if res.TotalAmount != o.Money {
		log.Printf("[pay] 订单 %s 上游金额 %s 与本地 %s 不一致", o.TradeNo, fenToYuan(res.TotalAmount), fenToYuan(o.Money))
	}
	if err := st.Q().SetPayOrderUpstream(ctx, db.SetPayOrderUpstreamParams{
		UpstreamTradeNo: res.TradeNo, GoodsKey: goodsKey, Quantity: quantity,
		UnitPrice: payUnitFen, Payurl: res.PayURL, BuyerContact: contact,
		QueryPwd: queryPwd, ID: o.ID,
	}); err != nil {
		return fmt.Errorf("回写上游订单失败")
	}
	if qr, qrErr := up.FetchQRCode(ctx, res.TradeNo); qrErr == nil && qr != "" {
		_ = st.Q().SetPayOrderQrcode(ctx, db.SetPayOrderQrcodeParams{Qrcode: qr, ID: o.ID})
		o.Qrcode = qr
	} else if qrErr != nil {
		log.Printf("[pay] 订单 %s 取二维码失败: %v（payurl 仍可用）", o.TradeNo, qrErr)
	}
	return nil
}

// placeUnderLock 持锁执行：库存不足先补齐（补到 max(10w, 当前+需求)）再下单。
func placeUnderLock(ctx context.Context, up *payx.Client, goodsID int64, goodsKey string, quantity, channelID int64, contact, queryPwd string) (payx.PayOrderResult, error) {
	target := payTargetFen / payUnitFen
	_, _, stock, err := up.GoodsInfo(ctx, goodsID)
	if err != nil {
		return payx.PayOrderResult{}, fmt.Errorf("上游库存查询失败：%v", err)
	}
	if stock < quantity {
		goal := target
		if stock+quantity > goal {
			goal = stock + quantity
		}
		log.Printf("[pay] 库存 %d 不足订单需求 %d，补到 %d", stock, quantity, goal)
		if _, err := up.EnsureStock(ctx, goodsID, goal); err != nil {
			return payx.PayOrderResult{}, fmt.Errorf("上游库存补充失败：%v", err)
		}
	}
	res, err := up.CreateOrder(ctx, goodsKey, quantity, channelID, contact, queryPwd)
	if err != nil && strings.Contains(err.Error(), "库存") {
		if _, serr := up.EnsureStock(ctx, goodsID, target+quantity); serr != nil {
			return payx.PayOrderResult{}, fmt.Errorf("上游库存补充失败：%v", serr)
		}
		res, err = up.CreateOrder(ctx, goodsKey, quantity, channelID, contact, queryPwd)
	}
	if err != nil {
		return payx.PayOrderResult{}, fmt.Errorf("上游下单失败：%v", err)
	}
	return res, nil
}

// ---- 支付轮询 ----

// PayPoller 待支付订单轮询器（fun.Wired 单例，5s 一扫）。
type PayPoller struct {
	Store  *store.Store  `fun:"auto"`
	Up     *payx.Client  `fun:"auto"`
	Notify *PayNotifier  `fun:"auto"`
}

func (p *PayPoller) New() {
	go p.loop()
}

func (p *PayPoller) loop() {
	t := time.NewTicker(5 * time.Second)
	defer t.Stop()
	for range t.C {
		p.sweep()
	}
}

func (p *PayPoller) sweep() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	rows, err := p.Store.Q().ListPendingPayOrders(ctx)
	if err != nil {
		return
	}
	now := time.Now().Unix()
	for _, o := range rows {
		if o.ExpiredAt > 0 && o.ExpiredAt < now {
			_ = p.Store.Q().SetPayOrderStatus(ctx, db.SetPayOrderStatusParams{
				Status: payStatusClosed, PaidAt: 0, Cards: "", ID: o.ID,
			})
			continue
		}
		paid, err := p.Up.OrderPaid(ctx, o.UpstreamTradeNo)
		if err != nil || !paid {
			continue
		}
		cards, _ := p.Up.OrderCards(ctx, o.UpstreamTradeNo)
		_ = p.Store.Q().SetPayOrderStatus(ctx, db.SetPayOrderStatusParams{
			Status: payStatusPaid, PaidAt: now, Cards: strings.Join(cards, "\n"), ID: o.ID,
		})
		log.Printf("[pay] 订单 %s 已支付", o.TradeNo)
		p.Notify.OnPaid(o.TradeNo)
		// 库存被消耗：异步补回目标水位
		if g := intSetting(p.Store, keyUpGoodsID); g > 0 {
			ensureStockAsync(p.Store, p.Up, g, payTargetFen/payUnitFen)
		}
	}
}

var stockFilling atomic.Bool

func ensureStockAsync(st *store.Store, up *payx.Client, goodsID, target int64) {
	if !stockFilling.CompareAndSwap(false, true) {
		return
	}
	go func() {
		defer stockFilling.Store(false)
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		defer cancel()
		if err := ensureStockTo(ctx, up, goodsID, target); err != nil {
			log.Printf("[pay] 库存回充失败: %v", err)
		}
	}()
}

// ---- 商户异步通知 ----

// PayNotifier 商户异步通知器（fun.Wired 单例）。
type PayNotifier struct {
	Store *store.Store `fun:"auto"`
}

var notifyBackoff = []time.Duration{0, 15 * time.Second, time.Minute, 5 * time.Minute, 15 * time.Minute}

// notifyParams 通知/回跳参数（未签名）。
func notifyParams(o db.PayOrder, pid string) map[string]string {
	return map[string]string{
		"pid":          pid,
		"trade_no":     o.TradeNo,
		"out_trade_no": o.OutTradeNo,
		"type":         o.Channel,
		"name":         o.Subject,
		"money":        fenToYuan(o.Money),
		"trade_status": "TRADE_SUCCESS",
	}
}

// buildSignedURL 签名参数拼接到回调地址。
func buildSignedURL(base string, params map[string]string, key string) string {
	u, err := url.Parse(base)
	if err != nil {
		return base
	}
	q := u.Query()
	for k, v := range params {
		q.Set(k, v)
	}
	q.Set("sign_type", "MD5")
	q.Set("sign", epaySign(params, key))
	u.RawQuery = q.Encode()
	return u.String()
}

// OnPaid 异步投递商户 notify_url（退避：立即/15s/1m/5m/15m）。
func (n *PayNotifier) OnPaid(tradeNo string) {
	go n.deliver(tradeNo)
}

func (n *PayNotifier) deliver(tradeNo string) {
	for i, delay := range notifyBackoff {
		if delay > 0 {
			time.Sleep(delay)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		o, err := n.Store.Q().GetPayOrderByTradeNo(ctx, tradeNo)
		cancel()
		if err != nil || o.Status != payStatusPaid || o.Notified == 1 || o.NotifyUrl == "" {
			return
		}
		if n.attempt(o) {
			_ = n.Store.Q().SetPayNotified(context.Background(), o.ID)
			return
		}
		_ = n.Store.Q().IncPayNotifyAttempts(context.Background(), o.ID)
		log.Printf("[pay] %s 第 %d 次通知未确认", tradeNo, i+1)
	}
}

func (n *PayNotifier) attempt(o db.PayOrder) bool {
	pid, key, err := payMerchant(n.Store)
	if err != nil {
		return false
	}
	params := notifyParams(o, pid)
	params["sign"] = epaySign(params, key)
	params["sign_type"] = "MD5"
	form := url.Values{}
	for k, v := range params {
		form.Set(k, v)
	}
	cli := &http.Client{Timeout: 10 * time.Second}
	if resp, err := cli.Post(o.NotifyUrl, "application/x-www-form-urlencoded",
		strings.NewReader(form.Encode())); err == nil && notifyRespOK(resp) {
		return true
	}
	sep := "?"
	if strings.Contains(o.NotifyUrl, "?") {
		sep = "&"
	}
	if resp, err := cli.Get(o.NotifyUrl + sep + form.Encode()); err == nil {
		return notifyRespOK(resp)
	}
	return false
}

func notifyRespOK(resp *http.Response) bool {
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	return resp.StatusCode == 200 && strings.Contains(strings.ToLower(string(body)), "success")
}

// BootPay 启动时装配上游凭据（若有）并触发商品自检。
func BootPay(st *store.Store, up *payx.Client, cfg *config.Config) {
	username := strSetting(st, keyUpUsername)
	enc := strSetting(st, keyUpPassword)
	if username == "" || enc == "" {
		return
	}
	pass, err := vault.Decrypt(cfg.VaultKey, enc)
	if err != nil || pass == "" {
		log.Printf("[pay] 上游凭据解密失败: %v", err)
		return
	}
	up.SetCredentialsLazy(username, pass)
	ensureGoodsAsync(st, up)
}
