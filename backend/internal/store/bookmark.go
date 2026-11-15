package store

import (
	"time"

	"github.com/cyi-cc/cyi-box/backend/internal/db"
)

type Bookmark struct {
	Id        int64
	Title     string
	Url       string
	Icon      string
	Sort      int64
	CreatedAt int64
}

func bookmarkFromRow(b db.Bookmark) Bookmark {
	return Bookmark{
		Id: b.ID, Title: b.Title, Url: b.Url,
		Icon: b.Icon, Sort: b.Sort, CreatedAt: b.CreatedAt,
	}
}

func (s *Store) ListBookmarks() ([]Bookmark, error) {
	rows, err := s.q.ListBookmarks(storeCtx)
	if err != nil {
		return nil, err
	}
	items := make([]Bookmark, 0, len(rows))
	for _, r := range rows {
		items = append(items, bookmarkFromRow(r))
	}
	return items, nil
}

func (s *Store) SaveBookmark(b *Bookmark) error {
	if b.Id > 0 {
		n, err := s.q.UpdateBookmark(storeCtx, db.UpdateBookmarkParams{
			Title: b.Title, Url: b.Url, Icon: b.Icon, Sort: b.Sort,
			ID: b.Id,
		})
		if err != nil {
			return err
		}
		if n == 0 {
			return ErrNotFound
		}
		return nil
	}
	now := time.Now().Unix()
	id, err := s.q.InsertBookmark(storeCtx, db.InsertBookmarkParams{
		Title: b.Title, Url: b.Url, Icon: b.Icon,
		Sort: b.Sort, CreatedAt: now,
	})
	if err != nil {
		return err
	}
	b.Id, b.CreatedAt = id, now
	return nil
}

func (s *Store) DeleteBookmark(id int64) error {
	n, err := s.q.DeleteBookmark(storeCtx, id)
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
