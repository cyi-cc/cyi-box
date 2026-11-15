// captcha.go 滑块拼图验证码：接入方密钥对管理（admin）+ 内存态题面/token 池。
// 对外三端点在 captcha_routes.go：GET /captcha/widget（iframe 整页）、
// /v1/captcha/verify（widget 内部调，出 token）、/v1/captcha/check（对方后端核销）。
package service

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
	"sync"
	"time"

	"github.com/cyi-cc/fun"

	"github.com/cyi-cc/cyi-box/backend/internal/dto"
	"github.com/cyi-cc/cyi-box/backend/internal/store"
)

const (
	capPuzzleTTL = 5 * time.Minute  // 题面有效期
	capTokenTTL  = 2 * time.Minute  // token 有效期（核销窗口）
	capMaxTry    = 5                // 单题最大试错次数
	capMaxSlots  = 10               // 单 IP 并发题位上限
	capSlotTTL   = 30 * time.Minute // 题位未释放兜底自动到期
	capMaxIPFail = 5                // 单 IP 滑窗内打错上限
	capFailTTL   = 30 * time.Minute // 打错计数的滚动窗口/封禁时长
)

type capPuzzle struct {
	answerX int
	keyID   int64
	exp     int64
	tries   int
}

type capToken struct {
	keyID int64
	exp   int64
}

var (
	capMu      sync.Mutex
	capPuzzles = map[string]*capPuzzle{}
	capTokens  = map[string]*capToken{}
	// capSlots 题位队列：ip → 各题位领取时间。纯限流语义：验证/作废均不释放，
	// 每位固定占 capSlotTTL（30 分钟）后自动回收，满 capMaxSlots 即拒。
	capSlots = map[string][]int64{}
	// capFails 打错封禁：ip → [滑窗内累计打错次数, 窗口到期时间]。
	// capFailTTL 滚动窗内打错满 capMaxIPFail 次即封禁；验证通过清零。
	capFails = map[string][2]int64{}
)

// capGCLocked 惰性清理过期项（写路径上顺带跑）
func capGCLocked(now int64) {
	if len(capPuzzles) > 2000 {
		for k, v := range capPuzzles {
			if v.exp < now {
				delete(capPuzzles, k)
			}
		}
	}
	if len(capTokens) > 2000 {
		for k, v := range capTokens {
			if v.exp < now {
				delete(capTokens, k)
			}
		}
	}
	if len(capSlots) > 5000 {
		for ip := range capSlots {
			purgeSlotsLocked(ip, now)
		}
	}
	if len(capFails) > 5000 {
		for ip, f := range capFails {
			if f[1] <= now {
				delete(capFails, ip)
			}
		}
	}
}

func capRand(prefix string, n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return prefix + hex.EncodeToString(b)
}

// purgeSlotsLocked 回收超期题位，返回剩余占用数
func purgeSlotsLocked(ip string, now int64) int {
	slots := capSlots[ip]
	live := slots[:0]
	for _, t := range slots {
		if now-t <= int64(capSlotTTL/time.Second) {
			live = append(live, t)
		}
	}
	if len(live) == 0 {
		delete(capSlots, ip)
		return 0
	}
	capSlots[ip] = live
	return len(live)
}

// capSlotFree 预检题位（便宜路径，正式占位仍在 acquirePuzzle 锁内重查）
func capSlotFree(ip string) bool {
	now := time.Now().Unix()
	capMu.Lock()
	defer capMu.Unlock()
	return purgeSlotsLocked(ip, now) < capMaxSlots
}

// acquirePuzzle 为 ip 占一个题位并登记题目：10 个位占满返回 ""（调用方回 429）。
// 题位只随 capSlotTTL 到期回收，验证通过也不提前释放。
func acquirePuzzle(ip string, p *capPuzzle) string {
	now := time.Now().Unix()
	capMu.Lock()
	defer capMu.Unlock()
	capGCLocked(now)
	if purgeSlotsLocked(ip, now) >= capMaxSlots {
		return ""
	}
	capSlots[ip] = append(capSlots[ip], now)
	id := capRand("c_", 12)
	capPuzzles[id] = p
	return id
}

// capBanned 该 IP 是否因打错超限处于封禁期（窗口到期自动解封清零）
func capBanned(ip string) bool {
	now := time.Now().Unix()
	capMu.Lock()
	defer capMu.Unlock()
	f := capFails[ip]
	if f[1] <= now {
		delete(capFails, ip)
		return false
	}
	return f[0] >= capMaxIPFail
}

// tryPuzzle 校验滑动位置：正确则销题并返回出题密钥 id；错一次记一次，
// 单题超限销题；并按 IP 滑窗累计打错，超限封禁（验证通过清零）。
func tryPuzzle(ip, id string, x int) (int64, bool) {
	now := time.Now().Unix()
	capMu.Lock()
	defer capMu.Unlock()
	p, ok := capPuzzles[id]
	if !ok || p.exp < now {
		delete(capPuzzles, id)
		return 0, false
	}
	d := p.answerX - x
	if d < 0 {
		d = -d
	}
	if d <= 4 {
		delete(capPuzzles, id)
		delete(capFails, ip)
		return p.keyID, true
	}
	p.tries++
	if p.tries >= capMaxTry {
		delete(capPuzzles, id)
	}
	f := capFails[ip]
	if f[1] <= now {
		f = [2]int64{}
	}
	f[0]++
	f[1] = now + int64(capFailTTL/time.Second)
	capFails[ip] = f
	return 0, false
}

// mintToken 签发一次性核销 token，绑定出题密钥
func mintToken(keyID int64) string {
	now := time.Now().Unix()
	capMu.Lock()
	defer capMu.Unlock()
	capGCLocked(now)
	t := capRand("ct_", 16)
	capTokens[t] = &capToken{keyID: keyID, exp: now + int64(capTokenTTL/time.Second)}
	return t
}

// consumeToken 校验+销毁：存在、未过期且 keyID 匹配才通过
func consumeToken(token string, keyID int64) bool {
	now := time.Now().Unix()
	capMu.Lock()
	defer capMu.Unlock()
	t, ok := capTokens[token]
	if !ok || t.exp < now {
		return false
	}
	if keyID != 0 && t.keyID != keyID {
		return false
	}
	delete(capTokens, token)
	return true
}

// ---- 接入方密钥管理（仅管理员） ----

type CaptchaSvc struct {
	fun.Ctx
	Store *store.Store
}

func captchaKeyView(k store.CaptchaKey) dto.CaptchaKeyView {
	return dto.CaptchaKeyView{
		Id: k.Id, Sitekey: k.Sitekey, Skey: k.Skey, Name: k.Name,
		Domains: k.Domains, Enabled: k.Enabled, UsedCount: k.UsedCount,
		CreatedAt: k.CreatedAt,
	}
}

func (s *CaptchaSvc) ListKeys() ([]dto.CaptchaKeyView, error) {
	keys, err := s.Store.ListCaptchaKeys()
	if err != nil {
		return nil, err
	}
	out := make([]dto.CaptchaKeyView, 0, len(keys))
	for _, k := range keys {
		out = append(out, captchaKeyView(k))
	}
	return out, nil
}

// SaveKey Id=0 新建（自动生成 pk_/sk_ 密钥对）；否则改名/域名白名单/开关
func (s *CaptchaSvc) SaveKey(d dto.CaptchaKeySaveDto) (dto.CaptchaKeyView, error) {
	domains := normalizeDomains(d.Domains)
	if d.Id == 0 {
		k := &store.CaptchaKey{
			Sitekey: capRand("pk_", 8), Skey: capRand("sk_", 20),
			Name: strings.TrimSpace(d.Name), Domains: domains, Enabled: true,
		}
		if err := s.Store.InsertCaptchaKey(k); err != nil {
			return dto.CaptchaKeyView{}, err
		}
		return captchaKeyView(*k), nil
	}
	enabled := true
	if d.Enabled != nil {
		enabled = *d.Enabled != 0
	}
	k := &store.CaptchaKey{
		Id: d.Id, Name: strings.TrimSpace(d.Name), Domains: domains, Enabled: enabled,
	}
	if err := s.Store.UpdateCaptchaKey(k); err != nil {
		return dto.CaptchaKeyView{}, err
	}
	keys, err := s.Store.ListCaptchaKeys()
	if err != nil {
		return dto.CaptchaKeyView{}, err
	}
	for _, kk := range keys {
		if kk.Id == d.Id {
			return captchaKeyView(kk), nil
		}
	}
	return dto.CaptchaKeyView{}, fun.Error(4001, "密钥不存在")
}

func (s *CaptchaSvc) DeleteKey(d dto.IdDto) (dto.OkView, error) {
	if err := s.Store.DeleteCaptchaKey(d.Id); err != nil {
		return dto.OkView{}, err
	}
	return dto.OkView{Message: "已删除"}, nil
}

// normalizeDomains 规整域名白名单：逗号分隔、去协议/路径/端口、小写去重
func normalizeDomains(raw string) string {
	seen := map[string]bool{}
	var out []string
	for _, p := range strings.Split(raw, ",") {
		d := normalizeDomain(p)
		if d == "" || seen[d] {
			continue
		}
		seen[d] = true
		out = append(out, d)
	}
	return strings.Join(out, ",")
}
