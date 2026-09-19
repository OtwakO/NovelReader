package epubstore

import (
	"bytes"
	"crypto/rand"
	"errors"
	"testing"

	"github.com/otwako/novelreader/internal/epub"
)

func TestPreparationGenerationOwnership(t *testing.T) {
	store, _ := receiptStore(t)
	r, err := store.Receive(t.Context(), rand.Text(), "novel.epub", bytes.NewReader(stagedFixture(t, false)), epub.OptimizedImages)
	if err != nil {
		t.Fatal(err)
	}
	first, err := store.QueuePreparation(t.Context(), r.ID)
	if err != nil || first.Generation != 1 || first.ImageMode != epub.OptimizedImages || first.State != PreparationQueued {
		t.Fatalf("queued: %+v %v", first, err)
	}
	stored, err := store.GetPreparation(t.Context(), r.ID, first.Generation)
	if err != nil || stored != first {
		t.Fatal("queue evidence not persisted", err)
	}
	// Two workers cannot both claim the same generation.
	results := make(chan error, 2)
	for range 2 {
		go func() { _, err := store.ClaimPreparation(t.Context(), r.ID, first.Generation); results <- err }()
	}
	successes := 0
	for range 2 {
		err := <-results
		if err == nil {
			successes++
		} else if !errors.Is(err, ErrStateChanged) {
			t.Fatal(err)
		}
	}
	if successes != 1 {
		t.Fatalf("claims: %d", successes)
	}
	next, err := store.QueuePreparation(t.Context(), r.ID)
	if err != nil || next.Generation != 2 || next.ImageMode != first.ImageMode {
		t.Fatalf("retry: %+v %v", next, err)
	}
	if _, err = store.ClaimPreparation(t.Context(), r.ID, first.Generation); !errors.Is(err, ErrStateChanged) {
		t.Fatal("stale claim", err)
	}
	cause := errors.New("synthetic preparation failure")
	if err = store.FailPreparation(t.Context(), r.ID, first.Generation, cause); !errors.Is(err, ErrStateChanged) {
		t.Fatal("stale failure", err)
	}
	stored, err = store.GetPreparation(t.Context(), r.ID, next.Generation)
	if err != nil || stored != next {
		t.Fatal("stale worker changed replacement", err)
	}
	previous, err := store.GetPreparation(t.Context(), r.ID, first.Generation)
	if err != nil || previous.State != PreparationFailed || previous.Error == "" {
		t.Fatal("superseded evidence lost", err)
	}
	if _, err = store.ClaimPreparation(t.Context(), r.ID, next.Generation); err != nil {
		t.Fatal(err)
	}
	if err = store.FailPreparation(t.Context(), r.ID, next.Generation, cause); err != nil {
		t.Fatal(err)
	}
	stored, err = store.GetPreparation(t.Context(), r.ID, next.Generation)
	if err != nil || stored.State != PreparationFailed || stored.Error != "epub_preparation_failed" {
		t.Fatal("failure not persisted", err)
	}
	original, err := store.Get(t.Context(), r.ID)
	if err != nil || original.State != Acquired || original.PreparationGeneration != next.Generation {
		t.Fatal("preparation changed acquisition", err)
	}
	if err = store.Discard(t.Context(), r.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = store.GetPreparation(t.Context(), r.ID, next.Generation); !errors.Is(err, ErrNotFound) {
		t.Fatal("orphan attempt", err)
	}
	if _, err = store.ClaimPreparation(t.Context(), r.ID, next.Generation); !errors.Is(err, ErrStateChanged) {
		t.Fatal("deleted receipt claim", err)
	}
}

func TestQueuePreparationIsAtomic(t *testing.T) {
	store, _ := receiptStore(t)
	r, err := store.Receive(t.Context(), rand.Text(), "novel.epub", bytes.NewReader(nil), "")
	if err != nil {
		t.Fatal(err)
	}
	first, err := store.QueuePreparation(t.Context(), r.ID)
	if err != nil {
		t.Fatal(err)
	}
	running, err := store.ClaimPreparation(t.Context(), r.ID, first.Generation)
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.db.ExecContext(t.Context(), `CREATE TRIGGER reject_attempt BEFORE INSERT ON epub_preparations BEGIN SELECT RAISE(FAIL,'synthetic queue failure'); END`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.QueuePreparation(t.Context(), r.ID); err == nil {
		t.Fatal("expected queue failure")
	}
	original, err := store.Get(t.Context(), r.ID)
	if err != nil || original.PreparationGeneration != first.Generation {
		t.Fatal("counter advanced on failure", err)
	}
	stored, err := store.GetPreparation(t.Context(), r.ID, first.Generation)
	if err != nil || stored != running {
		t.Fatal("previous attempt superseded on failure", err)
	}
	if _, err = store.db.ExecContext(t.Context(), `DROP TRIGGER reject_attempt`); err != nil {
		t.Fatal(err)
	}
	next, err := store.QueuePreparation(t.Context(), r.ID)
	if err != nil || next.Generation != first.Generation+1 {
		t.Fatal("retry after rollback", err)
	}
	// An incomplete acquisition cannot start preparation.
	if _, err = store.db.ExecContext(t.Context(), `UPDATE epub_files SET state='receiving' WHERE id=?`, r.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = store.QueuePreparation(t.Context(), r.ID); !errors.Is(err, ErrStateChanged) {
		t.Fatal("queued incomplete receipt", err)
	}
	if _, err = store.ClaimPreparation(t.Context(), r.ID, next.Generation); !errors.Is(err, ErrStateChanged) {
		t.Fatal("claimed incomplete receipt", err)
	}
}
