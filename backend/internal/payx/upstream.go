// Package payx catfk.com 上游发卡平台对接：商户接口（merchantApi）走 Merchant-Token 鉴权，
// 买家接口（shopApi）无鉴权。提供登录、分类/商品自检、卡密库存维护、下单与支付状态查询。
package payx

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
)

// apiError 上游业务错误。
type apiError struct{ msg string }

func (e *apiError) Error() string { return e.msg }

// envelope 上游统一响应信封 {code,msg,data}；code==1 为成功。
type envelope struct {
	Code int64           `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

// Client 上游平台客户端（fun.Wired 单例）。
type Client struct {
	base    string
	httpCli *http.Client

	mu       sync.Mutex
	token    string
	tokenAt  int64
	username string
	password string
	shop     string
	nickname string
	channelID   int64
	channelAt   int64

	// 库存维护锁：确保同一时刻只有一个补库存流程（全程带锁）。
	stockMu  sync.Mutex
	ensuring chan struct{}
}

func (c *Client) New() error {
	c.base = "https://catfk.com"
	c.httpCli = &http.Client{Timeout: 30 * time.Second}
	return nil
}

// Username 已配置的上游账号。
func (c *Client) Username() string { c.mu.Lock(); defer c.mu.Unlock(); return c.username }

// Shop 当前店铺买家链接 token。
func (c *Client) Shop() string { c.mu.Lock(); defer c.mu.Unlock(); return c.shop }

// Nickname 商户昵称。
func (c *Client) Nickname() string { c.mu.Lock(); defer c.mu.Unlock(); return c.nickname }

// TokenAge 当前令牌已存活秒数（0=未登录）。
func (c *Client) TokenAge() int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.token == "" {
		return 0
	}
	return time.Now().Unix() - c.tokenAt
}

// SetCredentials 设定/更新上游账号凭据并立即登录验证。
func (c *Client) SetCredentials(username, password string) error {
	c.mu.Lock()
	c.username, c.password = username, password
	c.token, c.tokenAt = "", 0
	c.mu.Unlock()
	_, err := c.login()
	return err
}

// SetCredentialsLazy 仅落凭据不登录（启动时装配，首个请求时惰性登录）。
func (c *Client) SetCredentialsLazy(username, password string) {
	c.mu.Lock()
	c.username, c.password = username, password
	c.mu.Unlock()
}

// ClearCredentials 清除凭据（账号删除时）。
func (c *Client) ClearCredentials() {
	c.mu.Lock()
	c.username, c.password, c.token = "", "", ""
	c.mu.Unlock()
}

func (c *Client) login() (string, error) {
	c.mu.Lock()
	user, pass := c.username, c.password
	c.mu.Unlock()
	if user == "" || pass == "" {
		return "", &apiError{"上游账号未配置"}
	}
	var out struct {
		MerchantToken string `json:"merchant_token"`
	}
	if err := c.post(context.Background(), "/merchantApi/user/login",
		map[string]any{"username": user, "password": pass}, "", &out); err != nil {
		return "", err
	}
	if out.MerchantToken == "" {
		return "", &apiError{"上游登录未返回令牌"}
	}
	c.mu.Lock()
	c.token, c.tokenAt = out.MerchantToken, time.Now().Unix()
	c.mu.Unlock()
	return out.MerchantToken, nil
}

// ensureSession 返回有效令牌：无令牌或调用方要求重登时重新登录。
func (c *Client) ensureSession() (string, error) {
	c.mu.Lock()
	tok := c.token
	c.mu.Unlock()
	if tok != "" {
		return tok, nil
	}
	return c.login()
}

// merchantPost 商户接口调用：带 Merchant-Token；失败原样透出，不自动重登。
func (c *Client) merchantPost(ctx context.Context, path string, body, out any) error {
	tok, err := c.ensureSession()
	if err != nil {
		return err
	}
	return c.postOnce(ctx, path, body, tok, out)
}

// post 通用调用：merchantToken 为空=买家接口；data 解码到 out。
func (c *Client) post(ctx context.Context, path string, body any, merchantToken string, out any) error {
	return c.postOnce(ctx, path, body, merchantToken, out)
}

func (c *Client) postOnce(ctx context.Context, path string, body any, merchantToken string, out any) error {
	raw, err := c.postRawOnce(ctx, path, body, merchantToken)
	if err != nil {
		return err
	}
	trimmed := bytes.TrimSpace(raw)
	// 令牌失效/被风控时上游会回 HTML 登录页，统一归为鉴权类错误触发重登
	if len(trimmed) > 0 && trimmed[0] == '<' {
		return &apiError{"上游返回登录页（token 已失效）"}
	}
	var env envelope
	if err := json.Unmarshal(trimmed, &env); err != nil {
		return fmt.Errorf("上游响应解析失败: %w", err)
	}
	if env.Code != 1 {
		return &apiError{env.Msg}
	}
	if out != nil && len(env.Data) > 0 {
		return json.Unmarshal(env.Data, out)
	}
	return nil
}

func (c *Client) postRawOnce(ctx context.Context, path string, body any, merchantToken string) ([]byte, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+path, bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if merchantToken != "" {
		req.Header.Set("merchant-token", merchantToken)
	}
	resp, err := c.httpCli.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	// 5xx/HTML 网关故障单独报错；200 + HTML = 令牌失效被踢回登录页
	if resp.StatusCode >= 500 {
		return nil, &apiError{fmt.Sprintf("上游网关故障（HTTP %d）", resp.StatusCode)}
	}
	return data, nil
}

// postRaw 返回原始响应体（code!=1 也是合法应答的接口用）。
func (c *Client) postRaw(ctx context.Context, path string, body any) ([]byte, error) {
	return c.postRawOnce(ctx, path, body, "")
}

// ---------- 商户接口 ----------

// Userinfo 拉取商户资料，顺带刷新昵称与店铺公开 token。
func (c *Client) Userinfo(ctx context.Context) (nickname, shop string, err error) {
	var out struct {
		Nickname    string `json:"nickname"`
		LinkWebsite string `json:"link_website"`
	}
	if err := c.merchantPost(ctx, "/merchantApi/user/userinfo", map[string]any{}, &out); err != nil {
		return "", "", err
	}
	// link_website 可能是旧值，以 shop/getLink 返回的 /shop/<token> 为准
	shop = out.LinkWebsite
	var link struct {
		Link string `json:"link"`
	}
	if err := c.merchantPost(ctx, "/merchantApi/shop/getLink", map[string]any{}, &link); err == nil && link.Link != "" {
		if u, perr := url.Parse(link.Link); perr == nil && strings.HasPrefix(u.Path, "/shop/") {
			if seg := strings.Trim(u.Path[len("/shop/"):], "/"); seg != "" && !strings.Contains(seg, "/") {
				shop = seg
			}
		}
	}
	c.mu.Lock()
	if c.shop != shop {
		c.channelID, c.channelAt = 0, 0
	}
	c.nickname, c.shop = out.Nickname, shop
	c.mu.Unlock()
	return out.Nickname, shop, nil
}

// MerchantGoods 商户商品列表项。
type MerchantGoods struct {
	ID        int64   `json:"id"`
	GoodsKey  string  `json:"goods_key"`
	Name      string  `json:"name"`
	Price     int64   `json:"-"` // 分
	PriceYuan float64 `json:"price"`
	Status    int64   `json:"status"`
	Stock     int64   `json:"-"`
	Extend    struct {
		StockCount int64 `json:"stock_count"`
	} `json:"extend"`
}

// GoodsList 商户商品列表（卡密类）。
func (c *Client) GoodsList(ctx context.Context, keywords string, current int64) ([]MerchantGoods, int64, error) {
	var out struct {
		Total int64           `json:"total"`
		List  []MerchantGoods `json:"list"`
	}
	body := map[string]any{
		"current": current, "pageSize": 20, "goods_type": "card",
		"status": 999, "name": keywords, "is_proxy": "0",
	}
	if err := c.merchantPost(ctx, "/merchantApi/Goods/list", body, &out); err != nil {
		return nil, 0, err
	}
	for i := range out.List {
		out.List[i].Price = yuanFen(out.List[i].PriceYuan)
		out.List[i].Stock = out.List[i].Extend.StockCount
	}
	return out.List, out.Total, nil
}

// GoodsInfo 商品详情：售价（分）、名称、库存张数。
func (c *Client) GoodsInfo(ctx context.Context, goodsID int64) (priceFen int64, name string, stock int64, err error) {
	var out struct {
		Name   string  `json:"name"`
		Price  float64 `json:"price"`
		Extend struct {
			StockCount int64 `json:"stock_count"`
		} `json:"extend"`
	}
	if err := c.merchantPost(ctx, "/merchantApi/Goods/info", map[string]any{"id": fmt.Sprint(goodsID)}, &out); err != nil {
		return 0, "", 0, err
	}
	return yuanFen(out.Price), out.Name, out.Extend.StockCount, nil
}

// CategoryListAll 全部卡密分类（value=分类 ID）。
func (c *Client) CategoryListAll(ctx context.Context) ([]Category, error) {
	var out []struct {
		Value int64  `json:"value"`
		Label string `json:"label"`
	}
	if err := c.merchantPost(ctx, "/merchantApi/GoodsCategory/listAll", map[string]any{"goods_type": "card"}, &out); err != nil {
		return nil, err
	}
	cats := make([]Category, 0, len(out))
	for _, v := range out {
		cats = append(cats, Category{ID: v.Value, Name: v.Label})
	}
	return cats, nil
}

// Category 卡密分类。
type Category struct {
	ID   int64
	Name string
}

// CategoryAdd 新建卡密分类（响应 data 为 null，需再查 listAll 拿 ID）。
func (c *Client) CategoryAdd(ctx context.Context, name string) error {
	return c.merchantPost(ctx, "/merchantApi/GoodsCategory/update", map[string]any{
		"id": 0, "name": name, "image": "", "sort": 0, "goods_type": "card",
	}, nil)
}

// GoodsAdd 新建卡密商品，返回商品 ID 与 goods_key。
func (c *Client) GoodsAdd(ctx context.Context, name string, categoryID int64, priceYuan float64) (int64, string, error) {
	var out struct {
		ID       int64  `json:"id"`
		GoodsKey string `json:"goods_key"`
	}
	body := map[string]any{
		"goods_type": "card", "id": 0, "name": name, "image": "", "category_id": categoryID,
		"price": priceYuan, "market_price": 0, "description": "", "sort": 0, "coupon_status": 1,
		"status": 1, "fee_payer": -1, "show": 1, "contact_format": "any", "agent_status": 0,
		"agent_price1": 0, "agent_price2": 0, "agent_price3": 0, "agent_price_limit": 0,
		"description_sync": 0, "name_sync": 0, "parent_id": 0, "cost_price": 0,
		"add_type": 1, "add_rate": 0, "add_price": 0,
		"extend": map[string]any{
			"instructions": "<p><br></p>", "stock_notice": 0, "lock_card": 0,
			"limit_count": 1, "limit_count_max": 0, "show_stock_type": 0,
			"send_order": 0, "query_password_status": 0,
		},
	}
	if err := c.merchantPost(ctx, "/merchantApi/Goods/update", body, &out); err != nil {
		return 0, "", err
	}
	return out.ID, out.GoodsKey, nil
}

// CardAdd 导入卡密（一行一张）。
func (c *Client) CardAdd(ctx context.Context, goodsID int64, content string) error {
	return c.merchantPost(ctx, "/merchantApi/GoodsCardStorage/add", map[string]any{
		"goods_id": goodsID, "content": content, "first": 0, "remove_repeat": 0,
	}, nil)
}

// cardAddBatch 上游单次导入上限（1w 张/批）。
const cardAddBatch = int64(10000)

// CardAddN 批量导入 n 张随机卡密，每次 1 万张分批。
func (c *Client) CardAddN(ctx context.Context, goodsID, n int64) error {
	if n <= 0 || n > 1_000_000 {
		return fmt.Errorf("补库存数量超出限制: %d", n)
	}
	for left := n; left > 0; {
		b := left
		if b > cardAddBatch {
			b = cardAddBatch
		}
		if err := c.CardAdd(ctx, goodsID, RandCards(b)); err != nil {
			return err
		}
		left -= b
	}
	return nil
}

// StockLock 获取库存全局锁（调用方负责释放），确保补库存/下单串行。
func (c *Client) StockLock() func() {
	c.stockMu.Lock()
	return c.stockMu.Unlock
}

// EnsureStock 补库存到 target 张（调用前需持有 StockLock）。
func (c *Client) EnsureStock(ctx context.Context, goodsID, target int64) (int64, error) {
	_, _, stock, err := c.GoodsInfo(ctx, goodsID)
	if err != nil {
		return 0, err
	}
	if stock < target {
		if err := c.CardAddN(ctx, goodsID, target-stock); err != nil {
			return stock, err
		}
		if _, _, stock, err = c.GoodsInfo(ctx, goodsID); err != nil {
			return target, nil
		}
	}
	return stock, nil
}

// ---------- 买家接口（无鉴权） ----------

// Channel 支付通道。
type Channel struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	ShowName     string `json:"show_name"`
	Status       int64  `json:"status"`
	CustomStatus int64  `json:"custom_status"`
}

// AlipayChannelID 店铺可用的支付宝支付通道 id，缓存 10 分钟。
func (c *Client) AlipayChannelID(ctx context.Context, shop string) (int64, error) {
	c.mu.Lock()
	if c.channelID > 0 && time.Now().Unix()-c.channelAt < 600 {
		id := c.channelID
		c.mu.Unlock()
		return id, nil
	}
	c.mu.Unlock()
	var list []Channel
	if err := c.post(ctx, "/shopApi/Shop/getUserChannel", map[string]any{"token": shop}, "", &list); err != nil {
		if strings.Contains(err.Error(), "Attempt to read property") {
			return 0, &apiError{"上游店铺支付通道未开通"}
		}
		return 0, err
	}
	var pick int64
	for _, ch := range list {
		if ch.Status != 1 || ch.CustomStatus != 1 {
			continue
		}
		if pick == 0 {
			pick = ch.ID
		}
		if strings.Contains(ch.Name, "支付宝") || strings.Contains(ch.ShowName, "支付宝") {
			pick = ch.ID
			break
		}
	}
	if pick == 0 {
		return 0, &apiError{"上游没有可用的支付宝支付通道"}
	}
	c.mu.Lock()
	c.channelID, c.channelAt = pick, time.Now().Unix()
	c.mu.Unlock()
	return pick, nil
}

// PayOrderResult 上游下单结果。
type PayOrderResult struct {
	TradeNo     string
	TotalAmount int64 // 分
	PayURL      string
}

// CreateOrder 上游买家下单（微信支付通道），返回平台订单号与收银台地址。
func (c *Client) CreateOrder(ctx context.Context, goodsKey string, quantity, channelID int64, contact, queryPwd string) (PayOrderResult, error) {
	var raw struct {
		TradeNo     string  `json:"trade_no"`
		TotalAmount float64 `json:"total_amount"`
		PayURL      string  `json:"payurl"`
	}
	err := c.post(ctx, "/shopApi/Pay/order", map[string]any{
		"goods_key": goodsKey, "quantity": quantity, "coupon_code": "",
		"channel_id": channelID, "contact": contact, "query_password": queryPwd,
		"select_cards_ids": []any{},
		"extend":           map[string]any{"juuid": randHex(12)},
	}, "", &raw)
	if err != nil {
		return PayOrderResult{}, err
	}
	return PayOrderResult{TradeNo: raw.TradeNo, TotalAmount: yuanFen(raw.TotalAmount), PayURL: raw.PayURL}, nil
}

// FetchQRCode 提取支付宝扫码付二维码内容。
// 流程：GET payurl 收银台（自动提交表单）→ POST openapi.alipay.com 网关 →
// 302 到 excashier 收银页 → 页面内嵌 <input name="qrCode" value="https://qr.alipay.com/...">。
func (c *Client) FetchQRCode(ctx context.Context, tradeNo string) (string, error) {
	jar, _ := cookiejar.New(nil)
	cli := &http.Client{Timeout: 30 * time.Second, Jar: jar}
	ua := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0 Safari/537.36"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		c.base+"/shopApi/Pay/payment?trade_no="+tradeNo, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", ua)
	resp, err := cli.Do(req)
	if err != nil {
		return "", fmt.Errorf("获取收银台页面失败: %w", err)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	resp.Body.Close()
	if err != nil {
		return "", err
	}
	action := formActionRe.FindSubmatch(body)
	if len(action) < 2 {
		return "", fmt.Errorf("收银台页面未找到支付表单")
	}
	form := url.Values{}
	for _, m := range formFieldRe.FindAllSubmatch(body, -1) {
		form.Set(string(m[1]), string(m[2]))
	}
	req2, err := http.NewRequestWithContext(ctx, http.MethodPost, string(action[1]),
		strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req2.Header.Set("User-Agent", ua)
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp2, err := cli.Do(req2)
	if err != nil {
		return "", fmt.Errorf("收银台跳转失败: %w", err)
	}
	body2, err := io.ReadAll(io.LimitReader(resp2.Body, 2<<20))
	resp2.Body.Close()
	if err != nil {
		return "", err
	}
	m := qrInputRe.FindSubmatch(body2)
	if len(m) < 2 {
		return "", fmt.Errorf("收银台页面未找到二维码")
	}
	return string(m[1]), nil
}

var (
	formActionRe = regexp.MustCompile(`action='([^']+)'`)
	formFieldRe  = regexp.MustCompile(`name='([^']+)' value='([^']*)'`)
	qrInputRe    = regexp.MustCompile(`name="qrCode"\s+[^>]*value="([^"]+)"`)
)

// OrderPaid 上游订单是否已支付（Pay/query：未支付 code=0，已支付 code=1）。
func (c *Client) OrderPaid(ctx context.Context, tradeNo string) (bool, error) {
	raw, err := c.postRaw(ctx, "/shopApi/Pay/query", map[string]any{"trade_no": tradeNo})
	if err != nil {
		return false, err
	}
	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return false, err
	}
	return env.Code == 1, nil
}

// OrderCards 已支付订单交付的卡密列表。
func (c *Client) OrderCards(ctx context.Context, tradeNo string) ([]string, error) {
	var out struct {
		Status   int64 `json:"status"`
		Response struct {
			Cards []string `json:"cards"`
		} `json:"response"`
	}
	if err := c.post(ctx, "/shopApi/Order/info", map[string]any{"trade_no": tradeNo, "dump": 1}, "", &out); err != nil {
		return nil, err
	}
	return out.Response.Cards, nil
}

// WalletInfo 商户钱包（可提现/冻结，分）。
func (c *Client) WalletInfo(ctx context.Context) (availableFen, frozenFen int64, err error) {
	var out struct {
		Platform struct {
			AvailableMoney float64 `json:"available_money"`
			FreezeMoney    float64 `json:"freeze_money"`
		} `json:"platform"`
	}
	if err := c.merchantPost(ctx, "/merchantApi/wallet/info", map[string]any{}, &out); err != nil {
		return 0, 0, err
	}
	return yuanFen(out.Platform.AvailableMoney), yuanFen(out.Platform.FreezeMoney), nil
}

// ---------- 工具 ----------

func yuanFen(y float64) int64 { return int64(math.Round(y * 100)) }

// RandContact 随机买家联系邮箱。
func RandContact() string { return "p" + randHex(10) + "@outlook.com" }

// RandQueryPwd 随机订单查询密码。
func RandQueryPwd() string { return randHex(8) }

// RandCards 生成 n 张随机唯一卡密（一行一张）。
func RandCards(n int64) string {
	var b strings.Builder
	seen := make(map[string]struct{}, n)
	for i := int64(0); i < n; {
		card := randHex(16)
		if _, ok := seen[card]; ok {
			continue
		}
		seen[card] = struct{}{}
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(card)
		i++
	}
	return b.String()
}

func randHex(n int) string {
	raw := make([]byte, n)
	if _, err := rand.Read(raw); err != nil {
		for i := range raw {
			raw[i] = byte(time.Now().UnixNano() >> (i % 8))
		}
	}
	const digits = "0123456789abcdef"
	b := make([]byte, n)
	for i := range b {
		b[i] = digits[raw[i]%16]
	}
	return string(b)
}
