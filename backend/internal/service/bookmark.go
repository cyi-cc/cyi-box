package service

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/cyi-cc/fun"

	"github.com/cyi-cc/cyi-box/backend/internal/dto"
	"github.com/cyi-cc/cyi-box/backend/internal/store"
)

// BookmarkSvc 个人书签：列表驱动侧边栏快捷入口与书签页
type BookmarkSvc struct {
	fun.Ctx
	Store *store.Store
}

func (s *BookmarkSvc) List() ([]dto.BookmarkView, error) {
	su, err := sessionFrom(s.RequestCtx)
	if err != nil {
		return nil, err
	}
	items, err := s.Store.ListBookmarks(su.User.Id)
	if err != nil {
		return nil, fmt.Errorf("bookmark: list failed: %w", err)
	}
	views := make([]dto.BookmarkView, 0, len(items))
	for _, b := range items {
		views = append(views, bookmarkView(b))
	}
	return views, nil
}

func (s *BookmarkSvc) Save(d dto.SaveBookmarkDto) (dto.BookmarkView, error) {
	su, err := sessionFrom(s.RequestCtx)
	if err != nil {
		return dto.BookmarkView{}, err
	}
	title := strings.TrimSpace(d.Title)
	raw := strings.TrimSpace(d.Url)
	if title == "" || len(title) > 64 {
		return dto.BookmarkView{}, fun.Error(4001, "名称必填且不超过 64 字符")
	}
	if raw != "" && !strings.Contains(raw, "://") {
		raw = "https://" + raw // 裸域名自动补协议
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" ||
		(u.Scheme != "http" && u.Scheme != "https") {
		return dto.BookmarkView{}, fun.Error(4001, "网址无效，需 http(s) 地址")
	}

	b := store.Bookmark{UserID: su.User.Id, Title: title, Url: raw}
	if d.Id != nil {
		b.Id = *d.Id
	}
	if d.Icon != nil {
		b.Icon = strings.TrimSpace(*d.Icon)
	}
	if d.Sort != nil {
		b.Sort = *d.Sort
	}
	if err := s.Store.SaveBookmark(&b); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return dto.BookmarkView{}, fun.Error(4001, "书签不存在")
		}
		return dto.BookmarkView{}, fmt.Errorf("bookmark: save failed: %w", err)
	}
	return bookmarkView(b), nil
}

func (s *BookmarkSvc) Delete(d dto.BookmarkIdDto) (dto.OkView, error) {
	su, err := sessionFrom(s.RequestCtx)
	if err != nil {
		return dto.OkView{}, err
	}
	if err := s.Store.DeleteBookmark(su.User.Id, d.Id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return dto.OkView{}, fun.Error(4001, "书签不存在")
		}
		return dto.OkView{}, fmt.Errorf("bookmark: delete failed: %w", err)
	}
	return dto.OkView{Message: "已删除"}, nil
}

func bookmarkView(b store.Bookmark) dto.BookmarkView {
	return dto.BookmarkView{Id: b.Id, Title: b.Title, Url: b.Url, Icon: b.Icon, Sort: b.Sort}
}
