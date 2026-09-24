package library

import (
	"errors"
	"testing"
	"time"
)

func TestLastReadTracksOnlyAcceptedProgress(t *testing.T) {
	home, store := newLibraryFixture(t)
	ctx := t.Context()
	get := func() *Item {
		t.Helper()
		item, err := store.Get(ctx, "first")
		if err != nil {
			t.Fatal(err)
		}
		return item
	}
	if get().LastReadAt != 0 {
		t.Fatal("admission counted as reading")
	}
	// Reading at the initial location must count; position is not evidence of unread state.
	before := time.Now().UnixMilli()
	if _, err := store.UpdateProgress(ctx, "first", Revision{}, Location{}); err != nil {
		t.Fatal(err)
	}
	read := get()
	if read.LastReadAt < before || read.LastReadAt > time.Now().UnixMilli() {
		t.Fatalf("invalid read timestamp: %+v", read)
	}
	// A historical fixture time makes accidental touches detectable without sleeps.
	if _, err := home.DB().ExecContext(ctx, `UPDATE library_items SET last_read_at=? WHERE id=?`, int64(1234567890000), "first"); err != nil {
		t.Fatal(err)
	}
	read = get()
	if _, err := store.UpdateProgress(ctx, "first", Revision{}, Location{ChapterIndex: 2}); !errors.Is(err, ErrStateChanged) {
		t.Fatalf("stale progress: %v", err)
	}
	if _, err := store.UpdateProgress(ctx, "first", read.Revision(), Location{Position: -1}); !errors.Is(err, ErrInvalidProgress) {
		t.Fatalf("invalid progress: %v", err)
	}
	tx, err := home.DB().BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	updated := *read
	updated.Name = "Updated metadata"
	updated.LastReadAt = 999
	if err := UpdateMetadataTx(ctx, tx, updated); err != nil {
		t.Fatal(err)
	}
	if err := TouchTx(ctx, tx, "first"); err != nil {
		t.Fatal(err)
	}
	if err := UpdateTotalTx(ctx, tx, "first", 3); err != nil {
		t.Fatal(err)
	}
	if err := PublishCatalogTx(ctx, tx, "first", 0, 3, "One"); err != nil {
		t.Fatal(err)
	}
	if err := ReplaceInterpretationTx(ctx, tx, "first", Revision{Content: 1, State: 1}, 4, Location{ChapterIndex: 2}); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	after := get()
	if after.LastReadAt != read.LastReadAt {
		t.Fatal("non-reading mutation changed last-read timestamp")
	}
	if _, err := store.AddBookmark(ctx, &Bookmark{ID: "mark", BookID: "first", ChapterIndex: 2}, after.Revision()); err != nil {
		t.Fatal(err)
	}
	if get().LastReadAt != read.LastReadAt {
		t.Fatal("bookmark changed last-read timestamp")
	}
	items, err := store.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range items {
		if item.ID == "first" && item.LastReadAt != read.LastReadAt {
			t.Fatal("list lost last-read timestamp")
		}
	}
}
