// Regression coverage for persisted chapter context used by later content rules.
package book

import (
	"path/filepath"
	"testing"

	"github.com/otwako/novelreader/internal/database"
)

func TestStorePersistsChapterContext(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "books.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	store := NewStore(db)
	initializeBookTestSchema(t, db)
	if err := store.AddBook(&Book{ID: "book-1", Name: "Book", SourceURL: "source", BookURL: "book"}); err != nil {
		t.Fatal(err)
	}
	want := []Chapter{{
		Index: 1, Title: "Chapter", URL: "https://example.test/chapter/1", IsVip: true,
		IsVolume: false, IsPay: true, BaseURL: "https://example.test/toc", Tag: "updated", WordCount: "1234",
	}}
	if err := store.SaveChapters("book-1", want); err != nil {
		t.Fatal(err)
	}
	got, err := store.GetChapters("book-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].IsPay != want[0].IsPay || got[0].BaseURL != want[0].BaseURL || got[0].Tag != want[0].Tag || got[0].WordCount != want[0].WordCount {
		t.Fatalf("chapters = %+v, want %+v", got, want)
	}
}
