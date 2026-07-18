// Chapter cache stores bounded processed content for upstream-outage fallback.
package book

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"time"
)

const (
	chapterCachePerBook = 100
	chapterCacheGlobal  = 500
)

type CachedChapter struct {
	BookID       string
	SourceURL    string
	ChapterIndex int
	ChapterURL   string
	Title        string
	Paragraphs   []string
	CachedAt     int64
}

func (s *Store) SaveChapterCache(entry CachedChapter) error {
	paragraphs, err := json.Marshal(entry.Paragraphs)
	if err != nil {
		return err
	}
	now := time.Now().UnixNano()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	if _, err := tx.Exec(`INSERT INTO chapter_cache (book_id, source_url, chapter_index, chapter_url, title, paragraphs, cached_at, last_accessed)
		SELECT ?, ?, ?, ?, ?, ?, ?, ? WHERE EXISTS (SELECT 1 FROM books WHERE id = ? AND source_url = ?)
		ON CONFLICT(book_id, source_url, chapter_index) DO UPDATE SET chapter_url=excluded.chapter_url, title=excluded.title, paragraphs=excluded.paragraphs, cached_at=excluded.cached_at, last_accessed=excluded.last_accessed`,
		entry.BookID, entry.SourceURL, entry.ChapterIndex, entry.ChapterURL, entry.Title, string(paragraphs), now, now, entry.BookID, entry.SourceURL); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM chapter_cache WHERE rowid IN (
		SELECT rowid FROM chapter_cache WHERE book_id = ? ORDER BY last_accessed DESC, rowid DESC LIMIT -1 OFFSET ?
	)`, entry.BookID, chapterCachePerBook); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM chapter_cache WHERE rowid IN (
		SELECT rowid FROM chapter_cache ORDER BY last_accessed DESC, rowid DESC LIMIT -1 OFFSET ?
	)`, chapterCacheGlobal); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) GetChapterCache(bookID, sourceURL string, chapterIndex int, chapterURL string) (*CachedChapter, error) {
	var entry CachedChapter
	var paragraphs string
	err := s.db.QueryRow(`SELECT book_id, source_url, chapter_index, chapter_url, title, paragraphs, cached_at FROM chapter_cache
		WHERE book_id = ? AND source_url = ? AND chapter_index = ? AND chapter_url = ?`,
		bookID, sourceURL, chapterIndex, chapterURL).
		Scan(&entry.BookID, &entry.SourceURL, &entry.ChapterIndex, &entry.ChapterURL, &entry.Title, &paragraphs, &entry.CachedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(paragraphs), &entry.Paragraphs); err != nil {
		return nil, err
	}
	if _, err := s.db.Exec(`UPDATE chapter_cache SET last_accessed = ? WHERE book_id = ? AND source_url = ? AND chapter_index = ?`,
		time.Now().UnixNano(), bookID, sourceURL, chapterIndex); err != nil {
		slog.Warn("chapter cache: failed to update access time", "book_id", bookID, "chapter_index", chapterIndex, "error", err)
	}
	return &entry, nil
}
