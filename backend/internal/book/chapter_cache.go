// Chapter cache stores bounded processed snapshots for identity-qualified, fresh reuse.
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
	ContentRevision int64
	BookID          string
	SourceID        string
	ChapterIndex    int
	ChapterURL      string
	Title           string
	Paragraphs      []string
	Blocks          []processor.ProseBlock
	CachedAt        int64
	SourceIdentity  string
	BookContext     map[string]any
	ChapterContext  map[string]any
}

func (s *Store) SaveChapterCache(entry CachedChapter) error {
	paragraphs, err := json.Marshal(entry.Paragraphs)
	if err != nil {
		return err
	}
	blocks, err := json.Marshal(chapterCachePayload{Version: 1, Blocks: entry.Blocks, SourceIdentity: entry.SourceIdentity, BookContext: entry.BookContext, ChapterContext: entry.ChapterContext})
	if err != nil {
		return err
	}
	now := time.Now().UnixNano()
	if entry.CachedAt == 0 {
		entry.CachedAt = now
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	if _, err := tx.Exec(`INSERT INTO chapter_cache (book_id, source_id, chapter_index, chapter_url, title, paragraphs, blocks, cached_at, last_accessed, content_revision)
		SELECT ?, ?, ?, ?, ?, ?, ?, ?, ?, ? WHERE EXISTS (SELECT 1 FROM books b JOIN library_items l ON l.id = b.id WHERE b.id = ? AND b.source_id = ? AND l.content_revision = ?)
		ON CONFLICT(book_id, source_id, chapter_index) DO UPDATE SET content_revision=excluded.content_revision, chapter_url=excluded.chapter_url, title=excluded.title, paragraphs=excluded.paragraphs, blocks=excluded.blocks, cached_at=excluded.cached_at, last_accessed=excluded.last_accessed`,
		entry.BookID, entry.SourceID, entry.ChapterIndex, entry.ChapterURL, entry.Title, string(paragraphs), string(blocks), entry.CachedAt, now, entry.ContentRevision, entry.BookID, entry.SourceID, entry.ContentRevision); err != nil {
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

func (s *Store) GetChapterCache(bookID, sourceID string, chapterIndex int, chapterURL string, contentRevision int64) (*CachedChapter, error) {
	var entry CachedChapter
	var paragraphs, blocks string
	err := s.db.QueryRow(`SELECT book_id, source_id, chapter_index, chapter_url, title, paragraphs, blocks, cached_at, content_revision FROM chapter_cache
		WHERE book_id = ? AND source_id = ? AND chapter_index = ? AND chapter_url = ? AND content_revision = ?
		AND EXISTS (SELECT 1 FROM library_items WHERE id = chapter_cache.book_id AND content_revision = chapter_cache.content_revision)`,
		bookID, sourceID, chapterIndex, chapterURL, contentRevision).
		Scan(&entry.BookID, &entry.SourceID, &entry.ChapterIndex, &entry.ChapterURL, &entry.Title, &paragraphs, &blocks, &entry.CachedAt, &entry.ContentRevision)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(paragraphs), &entry.Paragraphs); err != nil {
		return nil, err
	}
	if len(blocks) > 0 && blocks[0] == '[' {
		if err := json.Unmarshal([]byte(blocks), &entry.Blocks); err != nil {
			return nil, err
		}
	} else {
		var payload chapterCachePayload
		if err := json.Unmarshal([]byte(blocks), &payload); err != nil {
			return nil, err
		}
		if payload.Version != 1 {
			return nil, nil
		}
		entry.Blocks, entry.SourceIdentity = payload.Blocks, payload.SourceIdentity
		entry.BookContext, entry.ChapterContext = payload.BookContext, payload.ChapterContext
	}
	if _, err := s.db.Exec(`UPDATE chapter_cache SET last_accessed = ? WHERE book_id = ? AND source_id = ? AND chapter_index = ?`,
		time.Now().UnixNano(), bookID, sourceID, chapterIndex); err != nil {
		slog.Warn("chapter cache: failed to update access time", "book_id", bookID, "chapter_index", chapterIndex, "error", err)
	}
	return &entry, nil
}

// Disposable payload versioning keeps the durable reader schema unchanged.
// Old binaries may miss these cache entries; they can still retrieve upstream.
type chapterCachePayload struct {
	Version        int                    `json:"version"`
	Blocks         []processor.ProseBlock `json:"blocks"`
	SourceIdentity string                 `json:"sourceIdentity"`
	BookContext    map[string]any         `json:"bookContext,omitempty"`
	ChapterContext map[string]any         `json:"chapterContext,omitempty"`
}
