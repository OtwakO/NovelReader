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
	initializeBookTestSchema(t, db)
	if err := store.AddBook(&Book{ID: "book-1", Name: "Book", SourceID: "source", SourceURL: "source", BookURL: "book"}); err != nil {
		t.Fatal(err)
	}
	if version, err := store.UpdateProgress("book-1", "source", 0, 3, 0.75); err != nil || version != 1 {
		t.Fatalf("version=%d err=%v", version, err)
	}
	book, err := store.GetBook("book-1")
	if err != nil || book.DurChapterIndex != 3 || book.DurChapterPos != 0.75 {
		t.Fatalf("book=%+v err=%v", book, err)
	}
	if _, err := store.UpdateProgress("missing", "source", 0, 0, 0); !errors.Is(err, ErrBookNotFound) {
		t.Fatalf("missing error=%v", err)
	}
	if _, err := store.UpdateProgress("book-1", "old-source", 1, 0, 0); !errors.Is(err, ErrBookStateChanged) {
		t.Fatalf("stale source error=%v", err)
	}
	for _, invalid := range []struct {
		chapter int
		pos     float64
	}{{-1, 0}, {0, -0.1}, {0, 1.1}, {0, math.NaN()}, {0, math.Inf(1)}} {
		if _, err := store.UpdateProgress("book-1", "source", 1, invalid.chapter, invalid.pos); !errors.Is(err, ErrInvalidProgress) {
			t.Fatalf("chapter=%d pos=%v error=%v", invalid.chapter, invalid.pos, err)
		}
	}
}
