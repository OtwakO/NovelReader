// Chapter cache stores bounded processed content for upstream-outage fallback.
package book

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/otwako/novelreader/internal/processor"
)

const (
	chapterCachePerBook = 100
	chapterCacheGlobal  = 500
)

type CachedChapter struct {
	BookID       string
	SourceID     string
	ChapterIndex int
	ChapterURL   string
	Title        string
	Paragraphs   []string
	Blocks       []processor.ContentBlock
	CachedAt     int64
}

func (s *Store) SaveChapterCache(entry CachedChapter) error {
	paragraphs, err := json.Marshal(entry.Paragraphs)
	if err != nil {
		return err
	}
	blocks, err := json.Marshal(entry.Blocks)
	if err != nil {
		return err
	}
	now := time.Now().UnixNano()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	if _, err := tx.Exec(`INSERT INTO chapter_cache (book_id, source_id, chapter_index, chapter_url, title, paragraphs, blocks, cached_at, last_accessed)
		SELECT ?, ?, ?, ?, ?, ?, ?, ?, ? WHERE EXISTS (SELECT 1 FROM books WHERE id = ? AND source_id = ?)
		ON CONFLICT(book_id, source_id, chapter_index) DO UPDATE SET chapter_url=excluded.chapter_url, title=excluded.title, paragraphs=excluded.paragraphs, blocks=excluded.blocks, cached_at=excluded.cached_at, last_accessed=excluded.last_accessed`,
		entry.BookID, entry.SourceID, entry.ChapterIndex, entry.ChapterURL, entry.Title, string(paragraphs), string(blocks), now, now, entry.BookID, entry.SourceID); err != nil {
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

func (s *Store) GetChapterCache(bookID, sourceID string, chapterIndex int, chapterURL string) (*CachedChapter, error) {
	var entry CachedChapter
	var paragraphs, blocks string
	err := s.db.QueryRow(`SELECT book_id, source_id, chapter_index, chapter_url, title, paragraphs, blocks, cached_at FROM chapter_cache
		WHERE book_id = ? AND source_id = ? AND chapter_index = ? AND chapter_url = ?`,
		bookID, sourceID, chapterIndex, chapterURL).
		Scan(&entry.BookID, &entry.SourceID, &entry.ChapterIndex, &entry.ChapterURL, &entry.Title, &paragraphs, &blocks, &entry.CachedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(paragraphs), &entry.Paragraphs); err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(blocks), &entry.Blocks); err != nil {
		return nil, err
	}
	if _, err := s.db.Exec(`UPDATE chapter_cache SET last_accessed = ? WHERE book_id = ? AND source_id = ? AND chapter_index = ?`,
		time.Now().UnixNano(), bookID, sourceID, chapterIndex); err != nil {
		slog.Warn("chapter cache: failed to update access time", "book_id", bookID, "chapter_index", chapterIndex, "error", err)
	}
	return &entry, nil
}
