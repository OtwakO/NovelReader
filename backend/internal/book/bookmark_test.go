// Bookmark tests cover idempotent capture, stale-state rejection, and deletion.
package book

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/otwako/novelreader/internal/database"
)

func TestStoreBookmarksAreIdempotentAndVersionGuarded(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "bookmarks.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := NewStore(db)
	if err := store.Init(); err != nil {
		t.Fatal(err)
	}
	if err := store.AddBook(&Book{ID: "book", Name: "Book", SourceURL: "source", BookURL: "url"}); err != nil {
		t.Fatal(err)
	}
	bookmark := Bookmark{ID: "mark-1", BookID: "book", ChapterIndex: 2, ChapterTitle: "Chapter", Position: 0.4, Note: "note"}
	if err := store.AddBookmark(bookmark, "source", 0); err != nil {
		t.Fatal(err)
	}
	if err := store.AddBookmark(bookmark, "source", 0); err != nil {
		t.Fatal(err)
	}
	if err := store.AddBookmark(Bookmark{ID: "mark-2", BookID: "book"}, "old", 0); !errors.Is(err, ErrBookStateChanged) {
		t.Fatalf("stale error=%v", err)
	}
	marks, err := store.ListBookmarks("book")
	if err != nil || len(marks) != 1 || marks[0].Note != "note" || marks[0].Position != 0.4 {
		t.Fatalf("bookmarks=%+v err=%v", marks, err)
	}
	if err := store.DeleteBookmark("book", "mark-1"); err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteBookmark("book", "mark-1"); !errors.Is(err, ErrBookmarkNotFound) {
		t.Fatalf("missing delete error=%v", err)
	}
}
