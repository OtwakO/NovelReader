// Bookmarks persist annotated reader locations independently of chapter content.
package book

import (
	"errors"
	"math"
	"time"
)

var (
	ErrBookmarkNotFound = errors.New("book: bookmark not found")
	ErrBookmarkConflict = errors.New("book: bookmark ID conflict")
	ErrInvalidBookmark  = errors.New("book: invalid bookmark")
)

type Bookmark struct {
	ID           string  `json:"id"`
	BookID       string  `json:"bookId"`
	ChapterIndex int     `json:"chapterIndex"`
	ChapterTitle string  `json:"chapterTitle"`
	Position     float64 `json:"position"`
	Note         string  `json:"note"`
	Orphaned     bool    `json:"orphaned"`
	CreatedAt    int64   `json:"createdAt"`
}

func (s *Store) AddBookmark(mark Bookmark, sourceURL string, stateVersion int64) error {
	if mark.ID == "" || mark.BookID == "" || mark.ChapterIndex < 0 || math.IsNaN(mark.Position) || math.IsInf(mark.Position, 0) || mark.Position < 0 || mark.Position > 1 {
		return ErrInvalidBookmark
	}
	if mark.CreatedAt == 0 {
		mark.CreatedAt = time.Now().UnixMilli()
	}
	result, err := s.db.Exec(`INSERT OR IGNORE INTO bookmarks (id, book_id, chapter_index, chapter_title, position, note, orphaned, created_at)
		SELECT ?, ?, ?, ?, ?, ?, 0, ? WHERE EXISTS (SELECT 1 FROM books WHERE id = ? AND source_url = ? AND state_version = ?)`,
		mark.ID, mark.BookID, mark.ChapterIndex, mark.ChapterTitle, mark.Position, mark.Note, mark.CreatedAt,
		mark.BookID, sourceURL, stateVersion)
	if err != nil {
		return err
	}
	inserted, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if inserted > 0 {
		return nil
	}
	var existing Bookmark
	if err := s.db.QueryRow(`SELECT book_id, chapter_index, chapter_title, position, note FROM bookmarks WHERE id = ?`, mark.ID).
		Scan(&existing.BookID, &existing.ChapterIndex, &existing.ChapterTitle, &existing.Position, &existing.Note); err == nil {
		if existing.BookID == mark.BookID && existing.ChapterIndex == mark.ChapterIndex && existing.ChapterTitle == mark.ChapterTitle && existing.Position == mark.Position && existing.Note == mark.Note {
			return nil
		}
		return ErrBookmarkConflict
	}
	book, err := s.GetBook(mark.BookID)
	if err != nil {
		return err
	}
	if book == nil {
		return ErrBookNotFound
	}
	return ErrBookStateChanged
}

func (s *Store) ListBookmarks(bookID string) ([]Bookmark, error) {
	rows, err := s.db.Query(`SELECT id, book_id, chapter_index, chapter_title, position, note, orphaned, created_at FROM bookmarks WHERE book_id = ? ORDER BY created_at DESC, id`, bookID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	marks := make([]Bookmark, 0)
	for rows.Next() {
		var mark Bookmark
		if err := rows.Scan(&mark.ID, &mark.BookID, &mark.ChapterIndex, &mark.ChapterTitle, &mark.Position, &mark.Note, &mark.Orphaned, &mark.CreatedAt); err != nil {
			return nil, err
		}
		marks = append(marks, mark)
	}
	return marks, rows.Err()
}

func (s *Store) DeleteBookmark(bookID, bookmarkID string) error {
	result, err := s.db.Exec(`DELETE FROM bookmarks WHERE id = ? AND book_id = ?`, bookmarkID, bookID)
	if err != nil {
		return err
	}
	deleted, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if deleted == 0 {
		return ErrBookmarkNotFound
	}
	return nil
}
