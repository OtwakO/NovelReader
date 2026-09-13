package txtimport

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/otwako/novelreader/internal/library"
	"github.com/otwako/novelreader/internal/readerstore"
	"github.com/otwako/novelreader/internal/txtstore"
)

const recoveryAlice readerstore.UserID = "11111111-1111-4111-8111-111111111111"
const recoveryBob readerstore.UserID = "22222222-2222-4222-8222-222222222222"

func recoveryManager(t *testing.T, capacity int) *readerstore.Manager {
	t.Helper()
	readers, err := readerstore.NewManager(t.TempDir(), capacity, library.ReaderSchema(), txtstore.ReaderSchema())
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []readerstore.UserID{recoveryAlice, recoveryBob} {
		if err := readers.Create(t.Context(), id); err != nil {
			t.Fatal(err)
		}
	}
	return readers
}

func pendingReceipt(t *testing.T, home *readerstore.Home) txtstore.Receipt {
	t.Helper()
	value, err := txtstore.NewStore(home.DB(), home.Files()).Receive(t.Context(), "Example.txt", strings.NewReader("Chapter 1 Beginning\nSome synthetic prose.\n"))
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func TestStartRecoversBeforeDiscoveringPendingWork(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		readers := recoveryManager(t, 2)
		defer readers.Close()
		home, err := readers.Open(t.Context(), recoveryAlice)
		if err != nil {
			t.Fatal(err)
		}
		complete := pendingReceipt(t, home)
		incomplete := pendingReceipt(t, home)
		stuckRemoval := pendingReceipt(t, home)
		if _, err := home.DB().Exec(`UPDATE txt_files SET state = ?`, txtstore.Receiving); err != nil {
			t.Fatal(err)
		}
		root, err := home.Files().OpenRoot()
		if err != nil {
			t.Fatal(err)
		}
		if err := root.Remove(incomplete.Path); err != nil {
			t.Fatal(err)
		}
		if err := root.WriteFile(filepath.Join(filepath.Dir(stuckRemoval.Path), "keep.txt"), []byte("unowned"), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := home.DB().Exec(`UPDATE txt_files SET state = ? WHERE id = ?`, txtstore.Removing, stuckRemoval.ID); err != nil {
			t.Fatal(err)
		}
		root.Close()
		home.Close()
		pool, err := Start(t.Context(), readers, []readerstore.UserID{recoveryAlice})
		if err != nil {
			t.Fatal(err)
		}
		defer pool.Close()
		synctest.Wait()
		assertRetired(t, pool)
		home, err = readers.Open(t.Context(), recoveryAlice)
		if err != nil {
			t.Fatal(err)
		}
		defer home.Close()
		store := txtstore.NewStore(home.DB(), home.Files())
		ready, err := store.Get(t.Context(), complete.ID)
		if err != nil || (ready.State != txtstore.Ready && ready.State != txtstore.NeedsReview) {
			t.Fatalf("recovered original was not analyzed: %+v, %v", ready, err)
		}
		failed, err := store.Get(t.Context(), incomplete.ID)
		if err != nil || failed.State != txtstore.Failed {
			t.Fatalf("incomplete transfer: %+v, %v", failed, err)
		}
	})
}

func TestCapacityWaitDoesNotDropAcceptedWork(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		readers := recoveryManager(t, 1)
		defer readers.Close()
		home, err := readers.Open(t.Context(), recoveryBob)
		if err != nil {
			t.Fatal(err)
		}
		receipt := pendingReceipt(t, home)
		home.Close()
		blocker, err := readers.Open(t.Context(), recoveryAlice)
		if err != nil {
			t.Fatal(err)
		}
		pool := NewPool(readers)
		defer pool.Close()
		if err := pool.Notify(recoveryBob); err != nil {
			t.Fatal(err)
		}
		synctest.Wait()
		time.Sleep(11 * time.Second) // Fake time: exceeds the removed capacity timeout.
		synctest.Wait()
		pool.mu.Lock()
		waiting := pool.entries[recoveryBob] != nil
		pool.mu.Unlock()
		if !waiting {
			t.Error("capacity pressure discarded the only wake-up")
		}
		blocker.Close()
		synctest.Wait()
		assertRetired(t, pool)
		home, err = readers.Open(context.Background(), recoveryBob)
		if err != nil {
			t.Fatal(err)
		}
		defer home.Close()
		value, err := txtstore.NewStore(home.DB(), home.Files()).Get(t.Context(), receipt.ID)
		if err != nil || (value.State != txtstore.Ready && value.State != txtstore.NeedsReview) {
			t.Fatalf("waiting work did not finish: %+v, %v", value, err)
		}
	})
}
