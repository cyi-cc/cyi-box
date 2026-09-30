package store

import (
	"database/sql"
	"errors"
	"time"

	"github.com/cyi-cc/cyi-box/backend/internal/db"
)

// ---- disk 网盘 ----

type File struct {
	Id        int64
	UserID    int64
	Name      string // 原始文件名
	Stored    string // 落盘文件名（随机）
	Size      int64
	Mime      string
	CreatedAt int64
}

type Share struct {
	Id        int64
	Code      string
	FileID    int64
	UserID    int64
	FileName  string
	ExpiresAt int64 // 0 = 永久
	Downloads int64
	CreatedAt int64
}

func fileFromRow(f db.File) File {
	return File{
		Id: f.ID, UserID: f.UserID, Name: f.Name, Stored: f.Stored,
		Size: f.Size, Mime: f.Mime, CreatedAt: f.CreatedAt,
	}
}

func shareFromRow(s db.Share) Share {
	return Share{
		Id: s.ID, Code: s.Code, FileID: s.FileID, UserID: s.UserID,
		FileName: s.FileName, ExpiresAt: s.ExpiresAt,
		Downloads: s.Downloads, CreatedAt: s.CreatedAt,
	}
}

func (s *Store) InsertFile(f *File) error {
	id, err := s.q.InsertFile(storeCtx, db.InsertFileParams{
		UserID: f.UserID, Name: f.Name, Stored: f.Stored,
		Size: f.Size, Mime: f.Mime, CreatedAt: time.Now().Unix(),
	})
	if err != nil {
		return err
	}
	f.Id = id
	return nil
}

func (s *Store) ListFiles(userID int64) ([]File, error) {
	rows, err := s.q.ListFiles(storeCtx, userID)
	if err != nil {
		return nil, err
	}
	files := make([]File, 0, len(rows))
	for _, r := range rows {
		files = append(files, fileFromRow(r))
	}
	return files, nil
}

func (s *Store) GetFile(id int64) (File, error) {
	f, err := s.q.GetFile(storeCtx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return File{}, ErrNotFound
	}
	return fileFromRow(f), err
}

// DeleteFile 仅限属主；返回被删行数（0 = 不存在或非属主）
func (s *Store) DeleteFile(id, userID int64) (int64, error) {
	return s.q.DeleteFile(storeCtx, db.DeleteFileParams{ID: id, UserID: userID})
}

func (s *Store) InsertShare(sh *Share) error {
	sh.CreatedAt = time.Now().Unix()
	id, err := s.q.InsertShare(storeCtx, db.InsertShareParams{
		Code: sh.Code, FileID: sh.FileID, UserID: sh.UserID,
		FileName: sh.FileName, ExpiresAt: sh.ExpiresAt,
		CreatedAt: sh.CreatedAt,
	})
	if err != nil {
		return err
	}
	sh.Id = id
	return nil
}

func (s *Store) ListShares(userID int64) ([]Share, error) {
	rows, err := s.q.ListShares(storeCtx, userID)
	if err != nil {
		return nil, err
	}
	shares := make([]Share, 0, len(rows))
	for _, r := range rows {
		shares = append(shares, shareFromRow(r))
	}
	return shares, nil
}

// ShareFileView 公开下载需要的联表视图
type ShareFileView struct {
	Share
	Stored string
	Size   int64
	Mime   string
}

func (s *Store) GetShareFile(code string) (ShareFileView, error) {
	r, err := s.q.GetShareByCode(storeCtx, code)
	if errors.Is(err, sql.ErrNoRows) {
		return ShareFileView{}, ErrNotFound
	}
	if err != nil {
		return ShareFileView{}, err
	}
	return ShareFileView{
		Share: Share{
			Id: r.ID, Code: r.Code, FileID: r.FileID, UserID: r.UserID,
			FileName: r.FileName, ExpiresAt: r.ExpiresAt,
			Downloads: r.Downloads, CreatedAt: r.CreatedAt,
		},
		Stored: r.Stored, Size: r.Size, Mime: r.Mime,
	}, nil
}

func (s *Store) DeleteShare(id, userID int64) (int64, error) {
	return s.q.DeleteShare(storeCtx, db.DeleteShareParams{ID: id, UserID: userID})
}

func (s *Store) DeleteSharesByFile(fileID int64) error {
	return s.q.DeleteSharesByFile(storeCtx, fileID)
}

func (s *Store) IncShareDownloads(code string) error {
	return s.q.IncShareDownloads(storeCtx, code)
}
