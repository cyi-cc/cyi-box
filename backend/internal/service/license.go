// license.go 授权系统管理：项目（appid/secret）+ 激活卡密（绑域名、按时长计时）。
// 对外校验走 /v1/license，仅供其他项目客户端调用。
package service

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/cyi-cc/fun"

	"github.com/cyi-cc/cyi-box/backend/internal/db"
	"github.com/cyi-cc/cyi-box/backend/internal/dto"
	"github.com/cyi-cc/cyi-box/backend/internal/store"
)

// LicenseSvc 授权管理（仅管理员）。
type LicenseSvc struct {
	fun.Ctx
	Store *store.Store
}

const (
	licenseUnused  = 0 // 未使用
	licenseActive  = 1 // 已激活（有效期内）
	licenseExpired = 2 // 已过期
)

// ---- 项目管理 ----

func (s *LicenseSvc) Apps() (dto.LicenseAppsView, error) {
	rows, err := s.Store.Q().ListLicenseApps(context.Background())
	if err != nil {
		return dto.LicenseAppsView{}, err
	}
	items := make([]dto.LicenseAppView, 0, len(rows))
	for _, r := range rows {
		items = append(items, dto.LicenseAppView{
			Id: r.ID, Appid: r.Appid, Name: r.Name,
			Enabled: r.Enabled, CardCount: r.CardCount, CreatedAt: r.CreatedAt,
		})
	}
	return dto.LicenseAppsView{Items: items}, nil
}

// SaveApp id=0 新建（自动生成 appid+secret）；否则改名/开关/重置 secret。
func (s *LicenseSvc) SaveApp(d dto.LicenseAppSaveDto) (dto.LicenseAppView, error) {
	ctx := context.Background()
	if d.Id == 0 {
		name := strings.TrimSpace(d.Name)
		if name == "" || len(name) > 50 {
			return dto.LicenseAppView{}, fun.Error(4001, "项目名必填且不超过 50 字")
		}
		appid := "app-" + randLowerHex(4)
		id, err := s.Store.Q().CreateLicenseApp(ctx, db.CreateLicenseAppParams{
			Appid: appid, Secret: "", Name: name, CreatedAt: time.Now().Unix(),
		})
		if err != nil {
			return dto.LicenseAppView{}, fmt.Errorf("创建项目失败: %w", err)
		}
		return dto.LicenseAppView{Id: id, Appid: appid, Name: name, Enabled: 1}, nil
	}
	name := strings.TrimSpace(d.Name)
	if name == "" || len(name) > 50 {
		return dto.LicenseAppView{}, fun.Error(4001, "项目名必填且不超过 50 字")
	}
	enabled := int64(1)
	if d.Enabled != nil {
		enabled = *d.Enabled
	}
	if err := s.Store.Q().UpdateLicenseApp(ctx, db.UpdateLicenseAppParams{
		Name: name, Enabled: enabled, ID: d.Id,
	}); err != nil {
		return dto.LicenseAppView{}, err
	}
	return dto.LicenseAppView{Id: d.Id, Name: name, Enabled: enabled}, nil
}

func (s *LicenseSvc) DeleteApp(d dto.LicenseIdDto) (dto.OkView, error) {
	if err := s.Store.Q().DeleteLicenseApp(context.Background(), d.Id); err != nil {
		return dto.OkView{}, err
	}
	return dto.OkView{Message: "已删除"}, nil
}

// ---- 卡密管理 ----

func licenseStatus(c db.LicenseCard, now int64) int64 {
	if c.ActivatedAt == 0 {
		return licenseUnused
	}
	if c.ExpiresAt > 0 && c.ExpiresAt <= now { // expires_at=0 表示永久
		return licenseExpired
	}
	return licenseActive
}

func licenseView(c db.LicenseCard, now int64) dto.LicenseCardView {
	return dto.LicenseCardView{
		Id: c.ID, Card: c.Card, Hours: c.Hours, Domain: c.Domain,
		Status:      licenseStatus(c, now),
		ActivatedAt: c.ActivatedAt, ExpiresAt: c.ExpiresAt, CreatedAt: c.CreatedAt,
	}
}

func (s *LicenseSvc) List(d dto.LicenseListDto) (dto.LicensePageView, error) {
	kw := sql.NullString{}
	if d.Kw != nil {
		kw = sql.NullString{String: strings.TrimSpace(*d.Kw), Valid: true}
	}
	page := max(d.Page, 1)
	size := min(max(d.PageSize, 1), 200)
	ctx := context.Background()
	total, err := s.Store.Q().CountLicenseCards(ctx, db.CountLicenseCardsParams{
		AppID: d.AppId, Column2: kw.String, Column3: kw, Column4: kw,
	})
	if err != nil {
		return dto.LicensePageView{}, err
	}
	rows, err := s.Store.Q().ListLicenseCards(ctx, db.ListLicenseCardsParams{
		AppID: d.AppId, Column2: kw.String, Column3: kw, Column4: kw,
		Limit: size, Offset: (page - 1) * size,
	})
	if err != nil {
		return dto.LicensePageView{}, err
	}
	now := time.Now().Unix()
	items := make([]dto.LicenseCardView, 0, len(rows))
	for _, r := range rows {
		items = append(items, licenseView(r, now))
	}
	return dto.LicensePageView{Total: total, Items: items}, nil
}

func (s *LicenseSvc) Stats(d dto.LicenseStatsDto) (dto.LicenseStatsView, error) {
	now := time.Now().Unix()
	r, err := s.Store.Q().LicenseCardStats(context.Background(), db.LicenseCardStatsParams{
		ExpiresAt: now, ExpiresAt_2: now, AppID: d.AppId,
	})
	if err != nil {
		return dto.LicenseStatsView{}, err
	}
	return dto.LicenseStatsView{Total: r.Total, Unused: anyI64(r.Unused), Active: anyI64(r.Active), Expired: anyI64(r.Expired)}, nil
}

// Generate 批量生成卡密：大写分组格式（XXXX-XXXX-XXXX-XXXX），一次最多 500。
func (s *LicenseSvc) Generate(d dto.LicenseGenDto) (dto.LicenseGenView, error) {
	if d.AppId <= 0 {
		return dto.LicenseGenView{}, fun.Error(4001, "请先选择项目")
	}
	count := min(max(d.Count, 1), 500)
	hours := min(max(d.Hours, 0), 24*3650) // 0 = 永久
	ctx := context.Background()
	now := time.Now().Unix()
	cards := make([]string, 0, count)
	for i := int64(0); i < count; i++ {
		card := genLicenseCard()
		if err := s.Store.Q().CreateLicenseCard(ctx, db.CreateLicenseCardParams{
			AppID: d.AppId, Card: card, Hours: hours, CreatedAt: now,
		}); err != nil {
			return dto.LicenseGenView{}, fmt.Errorf("生成卡密失败（%d/%d）: %w", i, count, err)
		}
		cards = append(cards, card)
	}
	return dto.LicenseGenView{Cards: cards}, nil
}

func (s *LicenseSvc) Delete(d dto.LicenseIdDto) (dto.OkView, error) {
	if err := s.Store.Q().DeleteLicenseCard(context.Background(), d.Id); err != nil {
		return dto.OkView{}, err
	}
	return dto.OkView{Message: "已删除"}, nil
}

// genLicenseCard XXXX-XXXX-XXXX-XXXX 大写卡密，排除易混淆字符（0/O、1/I/L）。
func genLicenseCard() string {
	const chars = "23456789ABCDEFGHJKMNPQRSTUVWXYZ"
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	var sb strings.Builder
	for i, v := range b {
		if i > 0 && i%4 == 0 {
			sb.WriteByte('-')
		}
		sb.WriteByte(chars[int(v)%len(chars)])
	}
	return sb.String()
}

// randLowerHex n 字节随机小写 hex。
func randLowerHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
