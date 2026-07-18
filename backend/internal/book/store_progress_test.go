// Reading-progress tests keep persisted resume state valid and explicit.
package book

import (
	"errors"
	"math"
	"path/filepath"
	"testing"

	"github.com/otwako/novelreader/internal/database"
)

func TestStoreUpdateProgressValidatesAndReportsMissingBooks(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "progress.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := NewStore(db)
	if err := store.Init(); err != nil {
		t.Fatal(err)
	}
	if err := store.AddBook(&Book{ID: "book-1", Name: "Book", SourceURL: "source", BookURL: "book"}); err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateProgress("book-1", 3, 0.75); err != nil {
		t.Fatal(err)
	}
	book, err := store.GetBook("book-1")
	if err != nil || book.DurChapterIndex != 3 || book.DurChapterPos != 0.75 {
		t.Fatalf("book=%+v err=%v", book, err)
	}
	if err := store.UpdateProgress("missing", 0, 0); !errors.Is(err, ErrBookNotFound) {
		t.Fatalf("missing error=%v", err)
	}
	for _, invalid := range []struct {
		chapter int
		pos     float64
	}{{-1, 0}, {0, -0.1}, {0, 1.1}, {0, math.NaN()}, {0, math.Inf(1)}} {
		if err := store.UpdateProgress("book-1", invalid.chapter, invalid.pos); !errors.Is(err, ErrInvalidProgress) {
			t.Fatalf("chapter=%d pos=%v error=%v", invalid.chapter, invalid.pos, err)
		}
	}
}

func TestStoreInitAddsAlternateSourcesToOlderBookSchema(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "old.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, err = db.Exec(`CREATE TABLE books (
		id TEXT PRIMARY KEY, name TEXT NOT NULL, author TEXT DEFAULT '', cover_url TEXT DEFAULT '', intro TEXT DEFAULT '', kind TEXT DEFAULT '',
		source_url TEXT NOT NULL, book_url TEXT NOT NULL, toc_url TEXT DEFAULT '', origin TEXT DEFAULT '', variable_map TEXT DEFAULT '',
		last_chapter TEXT DEFAULT '', update_time TEXT DEFAULT '', word_count TEXT DEFAULT '', dur_chapter_index INTEGER DEFAULT 0,
		dur_chapter_pos REAL DEFAULT 0, total_chapter_num INTEGER DEFAULT 0, created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL
	)`)
	if err != nil {
		t.Fatal(err)
	}
	store := NewStore(db)
	if err := store.Init(); err != nil {
		t.Fatal(err)
	}
	if err := store.AddBook(&Book{ID: "book-1", Name: "Book", SourceURL: "source", BookURL: "book"}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ListBooks(); err != nil {
		t.Fatal(err)
	}
}
