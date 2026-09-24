package library

import (
	"errors"
	"testing"

	"github.com/otwako/novelreader/internal/readerstore"
)

func TestReadingRevisionsAndCallerOwnedPublication(t *testing.T) {
	home, store := newLibraryFixture(t)
	ctx := t.Context()
	version, err := store.UpdateProgress(ctx, "first", Revision{}, Location{ChapterIndex: 1, Position: 0.5, ChapterTitle: "Two"})
	if err != nil || version != 1 {
		t.Fatalf("progress: version=%d error=%v", version, err)
	}
	tx, err := home.DB().BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	// The catalog captured content revision 0 before that progress write.
	if err := PublishCatalogTx(ctx, tx, "first", 0, 3, "Two"); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	item, err := store.Get(ctx, "first")
	if err != nil {
		t.Fatal(err)
	}
	if item.Revision() != (Revision{Content: 1, State: 1}) || item.DurChapterPos != 0.5 {
		t.Fatalf("catalog changed reading state: %+v", item)
	}
	if _, err := store.UpdateProgress(ctx, "first", Revision{State: 1}, Location{}); !errors.Is(err, ErrStateChanged) {
		t.Fatalf("stale interpretation accepted: %v", err)
	}

	for _, commit := range []bool{false, true} {
		tx, err := home.DB().BeginTx(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		if err := ReplaceInterpretationTx(ctx, tx, "first", item.Revision(), 4, Location{ChapterIndex: 2, ChapterTitle: "Mapped"}); err != nil {
			t.Fatal(err)
		}
		updated := *item
		updated.Name = "Changed alongside interpretation"
		if err := UpdateMetadataTx(ctx, tx, updated); err != nil {
			t.Fatal(err)
		}
		if commit {
			err = tx.Commit()
		} else {
			err = tx.Rollback()
		}
		if err != nil {
			t.Fatal(err)
		}
		got, err := store.Get(ctx, "first")
		if err != nil {
			t.Fatal(err)
		}
		if commit {
			if got.Revision() != (Revision{Content: 2, State: 2}) || got.Name != updated.Name || got.CurrentChapterTitle != "Mapped" {
				t.Fatalf("incomplete commit: %+v", got)
			}
		} else if *got != *item {
			t.Fatalf("rollback changed item: %+v", got)
		}
	}
}

func TestBookmarksShareStateVersionAndRetryIdentity(t *testing.T) {
	_, store := newLibraryFixture(t)
	ctx := t.Context()
	mark := Bookmark{ID: "mark", BookID: "first", ChapterIndex: 1, ChapterTitle: "Two", Position: 0.4, Note: "note"}
	version, err := store.AddBookmark(ctx, &mark, Revision{})
	if err != nil || version != 1 {
		t.Fatalf("add: version=%d error=%v", version, err)
	}
	if retryVersion, err := store.AddBookmark(ctx, &mark, Revision{}); err != nil || retryVersion != version {
		t.Fatalf("retry: version=%d error=%v", retryVersion, err)
	}
	conflicting := mark
	conflicting.BookID = "second"
	if _, err := store.AddBookmark(ctx, &conflicting, Revision{}); !errors.Is(err, ErrBookmarkConflict) {
		t.Fatalf("ID reused across items: %v", err)
	}
	if _, err := store.UpdateProgress(ctx, "first", Revision{}, Location{}); !errors.Is(err, ErrStateChanged) {
		t.Fatalf("bookmark did not invalidate stale state: %v", err)
	}
	if _, err := store.DeleteBookmark(ctx, "first", mark.ID, Revision{}); !errors.Is(err, ErrStateChanged) {
		t.Fatalf("stale deletion accepted: %v", err)
	}
	if version, err := store.DeleteBookmark(ctx, "first", mark.ID, Revision{State: 1}); err != nil || version != 2 {
		t.Fatalf("delete: version=%d error=%v", version, err)
	}
	marks, err := store.GetBookmarks(ctx, "first")
	if err != nil || len(marks) != 0 {
		t.Fatalf("bookmarks=%+v error=%v", marks, err)
	}
}

func newLibraryFixture(t *testing.T) (*readerstore.Home, *Store) {
	t.Helper()
	manager, err := readerstore.NewManager(t.TempDir(), 1, ReaderSchema())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := manager.Close(); err != nil {
			t.Error(err)
		}
	})
	id := readerstore.UserID("11111111-1111-4111-8111-111111111111")
	if err := manager.Create(t.Context(), id); err != nil {
		t.Fatal(err)
	}
	home, err := manager.Open(t.Context(), id)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := home.Close(); err != nil {
			t.Error(err)
		}
	})
	tx, err := home.DB().BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	// Identical display identity does not merge separate publications.
	for _, id := range []string{"first", "second"} {
		if err := InsertTx(t.Context(), tx, Item{ID: id, Provider: "txt", Name: "Same title", Author: "Same author"}); err != nil {
			t.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	store := NewStore(home.DB())
	items, err := store.List(t.Context())
	if err != nil || len(items) != 2 {
		t.Fatalf("library admission merged titles: %+v, %v", items, err)
	}
	return home, store
}
