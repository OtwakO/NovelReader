package book

import (
	"path/filepath"
	"testing"

	"github.com/otwako/novelreader/internal/database"
)

func TestStoreInitAddsBlocksToExistingChapterCache(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "migration.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE chapter_cache (
		book_id TEXT NOT NULL,
		source_url TEXT NOT NULL,
		chapter_index INTEGER NOT NULL,
		chapter_url TEXT NOT NULL,
		title TEXT NOT NULL,
		paragraphs TEXT NOT NULL,
		cached_at INTEGER NOT NULL,
		last_accessed INTEGER NOT NULL,
		PRIMARY KEY (book_id, source_url, chapter_index)
	)`); err != nil {
		t.Fatal(err)
	}
	if err := NewStore(db).Init(); err != nil {
		t.Fatal(err)
	}
	var defaultValue string
	if err := db.QueryRow(`SELECT dflt_value FROM pragma_table_info('chapter_cache') WHERE name = 'blocks'`).Scan(&defaultValue); err != nil {
		t.Fatal(err)
	}
	if defaultValue != "'[]'" {
		t.Fatalf("blocks default=%q", defaultValue)
	}
}
