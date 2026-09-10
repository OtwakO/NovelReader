package library

import (
	"database/sql"
	"fmt"

	"github.com/otwako/novelreader/internal/readerstore"
)

func ReaderSchema() readerstore.ReaderSchema {
	return readerstore.ReaderSchema{Initialize: func(tx *sql.Tx) error {
		for _, statement := range []string{
			`CREATE TABLE IF NOT EXISTS library_items (
				id TEXT PRIMARY KEY,
				provider TEXT NOT NULL,
				name TEXT NOT NULL,
				author TEXT NOT NULL DEFAULT '',
				cover_url TEXT NOT NULL DEFAULT '',
				intro TEXT NOT NULL DEFAULT '',
				kind TEXT NOT NULL DEFAULT '',
				last_chapter TEXT NOT NULL DEFAULT '',
				update_time TEXT NOT NULL DEFAULT '',
				word_count TEXT NOT NULL DEFAULT '',
				dur_chapter_index INTEGER NOT NULL DEFAULT 0,
				dur_chapter_pos REAL NOT NULL DEFAULT 0,
				total_chapter_num INTEGER NOT NULL DEFAULT 0,
				current_chapter_title TEXT NOT NULL DEFAULT '',
				content_revision INTEGER NOT NULL DEFAULT 0,
				state_version INTEGER NOT NULL DEFAULT 0,
				created_at INTEGER NOT NULL,
				updated_at INTEGER NOT NULL
			)`,
			`CREATE INDEX IF NOT EXISTS idx_library_updated ON library_items(updated_at DESC, id)`,
			`CREATE TABLE IF NOT EXISTS bookmarks (
				id TEXT PRIMARY KEY,
				book_id TEXT NOT NULL REFERENCES library_items(id) ON DELETE CASCADE,
				content_revision INTEGER NOT NULL,
				chapter_index INTEGER NOT NULL,
				chapter_title TEXT NOT NULL,
				position REAL NOT NULL,
				note TEXT NOT NULL DEFAULT '',
				orphaned INTEGER NOT NULL DEFAULT 0,
				created_at INTEGER NOT NULL
			)`,
			`CREATE INDEX IF NOT EXISTS idx_bookmarks_book_id ON bookmarks(book_id, created_at DESC)`,
		} {
			if _, err := tx.Exec(statement); err != nil {
				return fmt.Errorf("library: initialize schema: %w", err)
			}
		}
		return nil
	}}
}
