package readerstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"testing/synctest"
)

func TestSnapshotCoordinatesFileMutationsAcrossHomeLeases(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		manager := newBackupTestManager(t)
		for _, id := range []UserID{testUserAlice, testUserBob} {
			if err := manager.Create(t.Context(), id); err != nil {
				t.Fatal(err)
			}
		}
		home, err := manager.Open(t.Context(), testUserAlice)
		if err != nil {
			t.Fatal(err)
		}
		defer home.Close()
		bob, err := manager.Open(t.Context(), testUserBob)
		if err != nil {
			t.Fatal(err)
		}
		defer bob.Close()
		unlock, err := home.Files().LockMutation(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		// Snapshot opens another lease. It must not observe this intermediate
		// state where committed metadata and its file disagree.
		if _, err := home.DB().Exec(`INSERT INTO portable_value VALUES ('new')`); err != nil {
			t.Fatal(err)
		}
		if err := home.Files().WriteFile([]byte("old"), 0600, FontsDirectory, "font"); err != nil {
			t.Fatal(err)
		}
		destination := filepath.Join(t.TempDir(), "snapshot")
		done := make(chan error, 1)
		go func() { done <- manager.SnapshotHome(t.Context(), testUserAlice, destination) }()
		synctest.Wait()
		select {
		case err := <-done:
			t.Fatalf("snapshot bypassed unfinished mutation: %v", err)
		default:
		}
		// Neither ordinary reads/progress-like writes nor another reader's
		// file operations should wait for Alice's snapshot boundary.
		if _, err := home.Files().ReadFile(FontsDirectory, "font"); err != nil {
			t.Fatal(err)
		}
		if _, err := home.DB().Exec(`INSERT INTO retained_value VALUES ('progress')`); err != nil {
			t.Fatal(err)
		}
		bobUnlock, err := bob.Files().LockMutation(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		bobUnlock()
		if err := home.Files().WriteFile([]byte("new"), 0600, FontsDirectory, "font"); err != nil {
			t.Fatal(err)
		}
		unlock()
		if err := <-done; err != nil {
			t.Fatal(err)
		}
		assertDatabaseValue(t, filepath.Join(destination, ReaderDatabaseName), `SELECT value FROM portable_value`, "new")
		data, err := os.ReadFile(filepath.Join(destination, FilesDirectory, FontsDirectory, "font"))
		if err != nil || string(data) != "new" {
			t.Fatalf("snapshot file=%q error=%v", data, err)
		}

		unlock, err = home.Files().LockMutation(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithCancel(t.Context())
		cancelledDestination := filepath.Join(t.TempDir(), "cancelled-snapshot")
		go func() { done <- manager.SnapshotHome(ctx, testUserAlice, cancelledDestination) }()
		synctest.Wait()
		cancel()
		if err := <-done; !errors.Is(err, context.Canceled) {
			t.Fatalf("waiting snapshot cancellation: %v", err)
		}
		unlock()
		if _, err := os.Stat(cancelledDestination); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("cancelled staging remains: %v", err)
		}
	})
}

func TestSnapshotCopyReaderStopsBetweenReads(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	reads := 0
	input := copyTestReader(func(p []byte) (int, error) {
		reads++
		cancel()
		return copy(p, "first chunk"), nil
	})
	n, err := io.Copy(io.Discard, contextReader{ctx: ctx, reader: input})
	if !errors.Is(err, context.Canceled) || reads != 1 || n != int64(len("first chunk")) {
		t.Fatalf("copy cancellation: bytes=%d reads=%d error=%v", n, reads, err)
	}
}

type copyTestReader func([]byte) (int, error)

func (r copyTestReader) Read(p []byte) (int, error) { return r(p) }

func TestSnapshotValidationDoesNotBlockLiveMutation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		manager := newBackupTestManager(t)
		entered, resume := make(chan struct{}), make(chan struct{})
		manager.schemas[0].ValidatePortableFiles = func(ctx context.Context, tx *sql.Tx, root *os.Root) error {
			close(entered)
			<-resume
			var value string
			if err := tx.QueryRowContext(ctx, `SELECT value FROM portable_value`).Scan(&value); err != nil {
				return err
			}
			data, err := root.ReadFile(filepath.Join(FontsDirectory, "font"))
			if err != nil {
				return err
			}
			if value != "copied" || string(data) != "copied" {
				return fmt.Errorf("snapshot changed during validation: database=%q file=%q", value, data)
			}
			return nil
		}
		if err := manager.Create(t.Context(), testUserAlice); err != nil {
			t.Fatal(err)
		}
		home, err := manager.Open(t.Context(), testUserAlice)
		if err != nil {
			t.Fatal(err)
		}
		defer home.Close()
		if _, err = home.DB().Exec(`INSERT INTO portable_value VALUES ('copied')`); err != nil {
			t.Fatal(err)
		}
		if err = home.Files().WriteFile([]byte("copied"), 0600, FontsDirectory, "font"); err != nil {
			t.Fatal(err)
		}
		snapshotDone := make(chan error, 1)
		destination := filepath.Join(t.TempDir(), "snapshot")
		go func() { snapshotDone <- manager.SnapshotHome(t.Context(), testUserAlice, destination) }()
		<-entered
		mutationDone := make(chan error, 1)
		go func() {
			mutationDone <- func() error {
				unlock, err := home.Files().LockMutation(t.Context())
				if err != nil {
					return err
				}
				defer unlock()
				if _, err = home.DB().Exec(`UPDATE portable_value SET value='live'`); err != nil {
					return err
				}
				return home.Files().WriteFile([]byte("live"), 0600, FontsDirectory, "font")
			}()
		}()
		synctest.Wait()
		select {
		case err := <-mutationDone:
			close(resume)
			snapshotErr := <-snapshotDone
			if err != nil || snapshotErr != nil {
				t.Fatalf("mutation=%v snapshot=%v", err, snapshotErr)
			}
		default:
			// Release the validator before failing so all leases/goroutines can drain.
			close(resume)
			<-snapshotDone
			<-mutationDone
			t.Fatal("snapshot validation still holds the live mutation gate")
		}
	})
}
