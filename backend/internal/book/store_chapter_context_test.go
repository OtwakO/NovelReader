// Regression coverage for persisted chapter context used by later content rules.
package book

import (
	"path/filepath"
	"reflect"
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
	}, {Index: 4, Title: "Volume", IsVolume: true}, {Index: 9, Title: "Last", URL: "https://example.test/chapter/9"}}
	if err := store.SaveChapters("book-1", want); err != nil {
		t.Fatal(err)
	}
	got, err := store.GetChapters("book-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(want) || got[0].IsPay != want[0].IsPay || got[0].BaseURL != want[0].BaseURL || got[0].Tag != want[0].Tag || got[0].WordCount != want[0].WordCount {
		t.Fatalf("chapters = %+v, want %+v", got, want)
	}
	for i, expected := range got {
		chapter, next, err := store.GetChapterWithNext(t.Context(), "book-1", expected.Index)
		if err != nil || chapter == nil || !reflect.DeepEqual(*chapter, expected) {
			t.Fatalf("chapter=%+v err=%v", chapter, err)
		}
		if i+1 < len(got) {
			if next == nil || !reflect.DeepEqual(*next, got[i+1]) {
				t.Fatalf("successor=%+v want=%+v", next, got[i+1])
			}
		} else if next != nil {
			t.Fatalf("last chapter has successor: %+v", next)
		}
		readable, err := store.HasReadableChapter(t.Context(), "book-1", expected.Index)
		wantReadable := !expected.IsVolume
		if err != nil || readable != wantReadable {
			t.Fatalf("readability=%v err=%v", readable, err)
		}
	}
	for _, missing := range []struct {
		bookID string
		index  int
	}{{"book-1", 2}, {"other-book", 1}} {
		chapter, next, err := store.GetChapterWithNext(t.Context(), missing.bookID, missing.index)
		if err != nil || chapter != nil || next != nil {
			t.Fatalf("missing chapter resolved: %+v %+v %v", chapter, next, err)
		}
		readable, err := store.HasReadableChapter(t.Context(), missing.bookID, missing.index)
		if err != nil || readable {
			t.Fatalf("missing chapter readable=%v err=%v", readable, err)
		}
	}
}
