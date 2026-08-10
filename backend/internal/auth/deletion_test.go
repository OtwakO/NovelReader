package auth

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/otwako/novelreader/internal/readerstore"
)

func TestReaderDeletionRequiresExactUsernameAndProtectsAdministrators(t *testing.T) {
	store := openTestStore(t)
	defer store.Close()
	readers := deletionTestManager(t, store)
	defer readers.Close()
	insertTestUserWithID(t, store, testUserID, "Administrator", RoleAdmin, StatusActive)
	insertTestUserWithID(t, store, secondTestUserID, "Bob", RoleReader, StatusActive)
	if err := readers.Create(context.Background(), testUserID); err != nil {
		t.Fatal(err)
	}
	if err := readers.Create(context.Background(), secondTestUserID); err != nil {
		t.Fatal(err)
	}
	admin, _ := accountByID(context.Background(), store.db, testUserID)
	service := NewDeletionService(store, readers, func(context.Context, readerstore.UserID) error { return nil })
	if _, err := service.Delete(context.Background(), secondTestUserID, "bob", admin, 200); !errors.Is(err, ErrUsernameConfirmation) {
		t.Fatalf("confirmation error=%v", err)
	}
	account, _ := accountByID(context.Background(), store.db, secondTestUserID)
	if account.Status != StatusActive {
		t.Fatalf("status changed after bad confirmation: %s", account.Status)
	}
	if _, err := service.Delete(context.Background(), testUserID, "Administrator", admin, 200); !errors.Is(err, ErrProtectedAccount) {
		t.Fatalf("administrator deletion error=%v", err)
	}
}

func TestReaderDeletionRollsBackStatusWhenDurableJobCreationFails(t *testing.T) {
	store := openTestStore(t)
	defer store.Close()
	readers := deletionTestManager(t, store)
	defer readers.Close()
	insertTestUserWithID(t, store, testUserID, "Administrator", RoleAdmin, StatusActive)
	insertTestUserWithID(t, store, secondTestUserID, "Bob", RoleReader, StatusActive)
	admin, _ := accountByID(context.Background(), store.db, testUserID)
	if _, err := store.db.Exec(`
		CREATE TRIGGER reject_deletion_job
		BEFORE INSERT ON account_deletions
		BEGIN SELECT RAISE(ABORT, 'reject job'); END
	`); err != nil {
		t.Fatal(err)
	}
	service := NewDeletionService(store, readers, func(context.Context, readerstore.UserID) error { return nil })
	if _, err := service.Delete(context.Background(), secondTestUserID, "Bob", admin, 200); err == nil {
		t.Fatal("deletion started without durable job")
	}
	account, err := accountByID(context.Background(), store.db, secondTestUserID)
	if err != nil || account.Status != StatusActive || account.AuthVersion != 1 {
		t.Fatalf("account=%#v error=%v", account, err)
	}
	var jobs int
	if err := store.db.QueryRow(`SELECT count(*) FROM account_deletions`).Scan(&jobs); err != nil || jobs != 0 {
		t.Fatalf("jobs=%d error=%v", jobs, err)
	}
}

func TestReaderDeletionCompletesDurablyAndRetriesIdempotently(t *testing.T) {
	store := openTestStore(t)
	defer store.Close()
	readers := deletionTestManager(t, store)
	defer readers.Close()
	insertTestUserWithID(t, store, testUserID, "Administrator", RoleAdmin, StatusActive)
	insertTestUserWithID(t, store, secondTestUserID, "Bob", RoleReader, StatusActive)
	if err := readers.Create(context.Background(), secondTestUserID); err != nil {
		t.Fatal(err)
	}
	admin, _ := accountByID(context.Background(), store.db, testUserID)
	quiesced := 0
	service := NewDeletionService(store, readers, func(_ context.Context, userID readerstore.UserID) error {
		if userID != secondTestUserID {
			t.Fatalf("quiesced user=%s", userID)
		}
		quiesced++
		return nil
	})
	job, err := service.Delete(context.Background(), secondTestUserID, "Bob", admin, 200)
	if err != nil || job.Status != "complete" || job.CompletedAt == nil {
		t.Fatalf("job=%#v error=%v", job, err)
	}
	if quiesced != 1 {
		t.Fatalf("quiesce calls=%d", quiesced)
	}
	if _, err := accountByID(context.Background(), store.db, secondTestUserID); !errors.Is(err, ErrAccountNotFound) {
		t.Fatalf("account lookup error=%v", err)
	}
	homePath := filepath.Join(filepath.Dir(store.Path()), readerstore.UsersDirectory, string(secondTestUserID))
	if _, err := os.Lstat(homePath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("home remains: %v", err)
	}
	retried, err := service.Delete(context.Background(), secondTestUserID, "ignored after durable job", admin, 201)
	if err != nil || retried.ID != job.ID || retried.Status != "complete" || quiesced != 1 {
		t.Fatalf("retried=%#v error=%v quiesced=%d", retried, err, quiesced)
	}
}

func TestReaderDeletionConcurrentRetriesConvergeOnCompletedJob(t *testing.T) {
	store := openTestStore(t)
	defer store.Close()
	readers := deletionTestManager(t, store)
	defer readers.Close()
	insertTestUserWithID(t, store, testUserID, "Administrator", RoleAdmin, StatusActive)
	insertTestUserWithID(t, store, secondTestUserID, "Bob", RoleReader, StatusActive)
	if err := readers.Create(context.Background(), secondTestUserID); err != nil {
		t.Fatal(err)
	}
	admin, _ := accountByID(context.Background(), store.db, testUserID)
	service := NewDeletionService(store, readers, func(context.Context, readerstore.UserID) error { return nil })
	start := make(chan struct{})
	results := make(chan struct {
		job DeletionJob
		err error
	}, 2)
	var group sync.WaitGroup
	for range 2 {
		group.Add(1)
		go func() {
			defer group.Done()
			<-start
			job, err := service.Delete(context.Background(), secondTestUserID, "Bob", admin, 200)
			results <- struct {
				job DeletionJob
				err error
			}{job, err}
		}()
	}
	close(start)
	group.Wait()
	close(results)
	var jobID string
	for result := range results {
		if result.err != nil || result.job.Status != "complete" {
			t.Fatalf("job=%#v error=%v", result.job, result.err)
		}
		if jobID == "" {
			jobID = result.job.ID
		} else if result.job.ID != jobID {
			t.Fatalf("job IDs differ: %s vs %s", jobID, result.job.ID)
		}
	}
}

func TestReaderDeletionRemovingAccountRestartRechecksAndRemovesPresentHome(t *testing.T) {
	store := openTestStore(t)
	defer store.Close()
	readers := deletionTestManager(t, store)
	defer readers.Close()
	insertTestUserWithID(t, store, testUserID, "Administrator", RoleAdmin, StatusActive)
	insertTestUserWithID(t, store, secondTestUserID, "Bob", RoleReader, StatusDeleting)
	if err := readers.Create(context.Background(), secondTestUserID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec(`
		INSERT INTO account_deletions (id, user_id, status, created_at, updated_at)
		VALUES ('restart-job', ?, 'removing_account', 100, 150)
	`, string(secondTestUserID)); err != nil {
		t.Fatal(err)
	}
	admin, _ := accountByID(context.Background(), store.db, testUserID)
	quiesced := 0
	service := NewDeletionService(store, readers, func(context.Context, readerstore.UserID) error {
		quiesced++
		return nil
	})
	completed, err := service.Delete(context.Background(), secondTestUserID, "ignored", admin, 200)
	if err != nil || completed.Status != "complete" || quiesced != 1 {
		t.Fatalf("completed=%#v error=%v quiesced=%d", completed, err, quiesced)
	}
	homePath := filepath.Join(filepath.Dir(store.Path()), readerstore.UsersDirectory, string(secondTestUserID))
	if _, err := os.Lstat(homePath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("restart home remains: %v", err)
	}
}

func TestReaderDeletionFailureKeepsDeletingAccountAndRetryRollsForward(t *testing.T) {
	store := openTestStore(t)
	defer store.Close()
	readers := deletionTestManager(t, store)
	defer readers.Close()
	insertTestUserWithID(t, store, testUserID, "Administrator", RoleAdmin, StatusActive)
	insertTestUserWithID(t, store, secondTestUserID, "Bob", RoleReader, StatusActive)
	if err := readers.Create(context.Background(), secondTestUserID); err != nil {
		t.Fatal(err)
	}
	admin, _ := accountByID(context.Background(), store.db, testUserID)
	fail := true
	service := NewDeletionService(store, readers, func(context.Context, readerstore.UserID) error {
		if fail {
			return errors.New("in-flight request did not drain")
		}
		return nil
	})
	if _, err := service.Delete(context.Background(), secondTestUserID, "Bob", admin, 200); err == nil {
		t.Fatal("deletion unexpectedly succeeded")
	}
	account, err := accountByID(context.Background(), store.db, secondTestUserID)
	if err != nil || account.Status != StatusDeleting {
		t.Fatalf("account=%#v error=%v", account, err)
	}
	job, err := service.jobByUserID(context.Background(), secondTestUserID)
	if err != nil || job.Status != "failed" || job.LastError == "" {
		t.Fatalf("job=%#v error=%v", job, err)
	}
	fail = false
	completed, err := service.Delete(context.Background(), secondTestUserID, "Bob", admin, 201)
	if err != nil || completed.Status != "complete" {
		t.Fatalf("completed=%#v error=%v", completed, err)
	}
}

func deletionTestManager(t *testing.T, store *Store) *readerstore.Manager {
	t.Helper()
	manager, err := readerstore.NewManager(filepath.Dir(store.Path()), 4)
	if err != nil {
		t.Fatal(err)
	}
	return manager
}
