package service

import (
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cyi-cc/fun"

	"github.com/cyi-cc/cyi-box/backend/internal/config"
	"github.com/cyi-cc/cyi-box/backend/internal/dto"
	"github.com/cyi-cc/cyi-box/backend/internal/store"
)

const shareCodeLen = 10
const shareAlphabet = "abcdefghijkmnpqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789"

// DiskSvc 网盘：文件列表/删除/分享链接管理（按用户隔离，属主校验）
type DiskSvc struct {
	fun.Ctx
	Store *store.Store
	Cfg   *config.Config
}

func shareCode() (string, error) {
	buf := make([]byte, shareCodeLen)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	for i := range buf {
		buf[i] = shareAlphabet[int(buf[i])%len(shareAlphabet)]
	}
	return string(buf), nil
}

func fileViews(files []store.File) []dto.FileView {
	out := make([]dto.FileView, 0, len(files))
	for _, f := range files {
		out = append(out, dto.FileView{
			Id: f.Id, Name: f.Name, Size: f.Size, Mime: f.Mime, CreatedAt: f.CreatedAt,
		})
	}
	return out
}

func shareViews(shares []store.Share) []dto.ShareView {
	now := time.Now().Unix()
	out := make([]dto.ShareView, 0, len(shares))
	for _, s := range shares {
		out = append(out, dto.ShareView{
			Id: s.Id, Code: s.Code, FileName: s.FileName,
			ExpiresAt: s.ExpiresAt, Expired: s.ExpiresAt > 0 && now > s.ExpiresAt,
			Downloads: s.Downloads, CreatedAt: s.CreatedAt,
		})
	}
	return out
}

func (s *DiskSvc) List() ([]dto.FileView, error) {
	su, err := sessionFrom(s.RequestCtx)
	if err != nil {
		return nil, err
	}
	files, err := s.Store.ListFiles(su.User.Id)
	if err != nil {
		return nil, fmt.Errorf("disk: list failed: %w", err)
	}
	return fileViews(files), nil
}

// DeleteFile 删文件：库记录 + 磁盘文件 + 关联分享一起清
func (s *DiskSvc) DeleteFile(d dto.IdDto) (dto.OkView, error) {
	su, err := sessionFrom(s.RequestCtx)
	if err != nil {
		return dto.OkView{}, err
	}
	f, err := s.Store.GetFile(d.Id)
	if errors.Is(err, store.ErrNotFound) || f.UserID != su.User.Id {
		return dto.OkView{}, fun.Error(4004, "文件不存在")
	}
	if err != nil {
		return dto.OkView{}, err
	}
	if _, err := s.Store.DeleteFile(d.Id, su.User.Id); err != nil {
		return dto.OkView{}, fmt.Errorf("disk: delete failed: %w", err)
	}
	_ = s.Store.DeleteSharesByFile(d.Id)
	_ = os.Remove(filepath.Join(s.Cfg.DiskDir, f.Stored))
	return dto.OkView{Message: "已删除"}, nil
}

// CreateShare ExpireMinutes=0 永久；>0 限时分享
func (s *DiskSvc) CreateShare(d dto.CreateShareDto) (dto.ShareView, error) {
	su, err := sessionFrom(s.RequestCtx)
	if err != nil {
		return dto.ShareView{}, err
	}
	f, err := s.Store.GetFile(d.FileId)
	if errors.Is(err, store.ErrNotFound) || f.UserID != su.User.Id {
		return dto.ShareView{}, fun.Error(4004, "文件不存在")
	}
	if err != nil {
		return dto.ShareView{}, err
	}
	if d.ExpireMinutes < 0 || d.ExpireMinutes > 60*24*365 {
		return dto.ShareView{}, fun.Error(4001, "有效期不合法")
	}
	code, err := shareCode()
	if err != nil {
		return dto.ShareView{}, err
	}
	var expires int64
	if d.ExpireMinutes > 0 {
		expires = time.Now().Add(time.Duration(d.ExpireMinutes) * time.Minute).Unix()
	}
	sh := store.Share{
		Code: code, FileID: f.Id, UserID: su.User.Id,
		FileName: f.Name, ExpiresAt: expires,
	}
	if err := s.Store.InsertShare(&sh); err != nil {
		return dto.ShareView{}, fmt.Errorf("disk: create share failed: %w", err)
	}
	return shareViews([]store.Share{sh})[0], nil
}

func (s *DiskSvc) ListShares() ([]dto.ShareView, error) {
	su, err := sessionFrom(s.RequestCtx)
	if err != nil {
		return nil, err
	}
	shares, err := s.Store.ListShares(su.User.Id)
	if err != nil {
		return nil, fmt.Errorf("disk: list shares failed: %w", err)
	}
	return shareViews(shares), nil
}

func (s *DiskSvc) DeleteShare(d dto.IdDto) (dto.OkView, error) {
	su, err := sessionFrom(s.RequestCtx)
	if err != nil {
		return dto.OkView{}, err
	}
	n, err := s.Store.DeleteShare(d.Id, su.User.Id)
	if err != nil {
		return dto.OkView{}, fmt.Errorf("disk: delete share failed: %w", err)
	}
	if n == 0 {
		return dto.OkView{}, fun.Error(4004, "分享不存在")
	}
	return dto.OkView{Message: "已删除"}, nil
}

// safeStored 落盘文件名清洗：只留安全字符，防路径穿越
func safeStored(name string) string {
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, "\\", "_")
	name = strings.ReplaceAll(name, "..", "_")
	return name
}
