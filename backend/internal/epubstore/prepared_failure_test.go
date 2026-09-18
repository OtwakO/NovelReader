package epubstore

import (
	"errors"
	"os"
	"path"
	"testing"

	"github.com/otwako/novelreader/internal/epub"
)

func TestPreparationIndexFailureRollsBackAllOutput(t *testing.T) {
	store, root := receiptStore(t)
	r := acquiredFixture(t, store, epub.OriginalImages)
	a, err := store.QueuePreparation(t.Context(), r.ID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.db.Exec(`CREATE TRIGGER reject_resource BEFORE INSERT ON epub_resources BEGIN SELECT RAISE(FAIL,'synthetic resource index failure'); END`)
	if err != nil {
		t.Fatal(err)
	}
	if err = store.Prepare(t.Context(), r.ID, a.Generation); err == nil {
		t.Fatal("expected index failure")
	}
	a, err = store.GetPreparation(t.Context(), r.ID, a.Generation)
	if err != nil || a.State != PreparationFailed {
		t.Fatal(a, err)
	}
	for _, table := range []string{"epub_sections", "epub_resources"} {
		var count int
		if err = store.db.QueryRow(`SELECT count(*) FROM ` + table).Scan(&count); err != nil || count != 0 {
			t.Fatal("partial index committed", err)
		}
	}
	if _, err = root.Stat(preparationPath(r.ID, a.Generation)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("partial output installed", err)
	}
	work, err := root.Open(receiptPreparationWorkPath(r.ID))
	if err != nil {
		t.Fatal(err)
	}
	entries, err := work.ReadDir(-1)
	closeErr := work.Close()
	if err != nil || closeErr != nil || len(entries) != 0 {
		t.Fatal("staging retained", err, closeErr)
	}
}

func TestPreparationRecoveryFailsInterruptedWorker(t *testing.T) {
	store, root := receiptStore(t)
	r := acquiredFixture(t, store, epub.OriginalImages)
	a, err := store.QueuePreparation(t.Context(), r.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.ClaimPreparation(t.Context(), r.ID, a.Generation); err != nil {
		t.Fatal(err)
	}
	staged := stageReceipt(t, root, r)
	if err = staged.work.Close(); err != nil {
		t.Fatal(err)
	}
	if err = store.RecoverPreparations(t.Context()); err != nil {
		t.Fatal(err)
	}
	a, err = store.GetPreparation(t.Context(), r.ID, a.Generation)
	if err != nil || a.State != PreparationFailed {
		t.Fatal(a, err)
	}
	if _, err = root.Stat(preparationWorkPath()); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("abandoned work retained", err)
	}
	next, err := store.QueuePreparation(t.Context(), r.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err = store.Prepare(t.Context(), r.ID, next.Generation); err != nil {
		t.Fatal("retry failed", err)
	}
}

func TestDiscardOwnsOnlyItsPreparationWork(t *testing.T) {
	store, root := receiptStore(t)
	removed := acquiredFixture(t, store, epub.OriginalImages)
	retained := acquiredFixture(t, store, epub.OriginalImages)
	for _, r := range []Receipt{removed, retained} {
		directory := receiptPreparationWorkPath(r.ID)
		if err := root.MkdirAll(directory, 0700); err != nil {
			t.Fatal(err)
		}
		if err := root.WriteFile(path.Join(directory, "unfinished"), []byte("synthetic work"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.Discard(t.Context(), removed.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := root.Stat(receiptPreparationWorkPath(removed.ID)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("owned work retained", err)
	}
	if data, err := root.ReadFile(path.Join(receiptPreparationWorkPath(retained.ID), "unfinished")); err != nil || string(data) != "synthetic work" {
		t.Fatal("other receipt work changed", err)
	}
}
