package backup

import (
	"bytes"
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/otwako/novelreader/internal/readerstore"
)

func TestRestoreOutcomeRetainsLiveOwnershipAndTerminalFailure(t *testing.T) {
	root := t.TempDir()
	readers, err := readerstore.NewManager(root, 2)
	if err != nil {
		t.Fatal(err)
	}
	defer readers.Close()
	if err := readers.Create(t.Context(), backupAlice); err != nil {
		t.Fatal(err)
	}
	entered, release := make(chan struct{}), make(chan struct{})
	service, err := NewService(readers, root, func(context.Context, readerstore.UserID) error {
		close(entered)
		<-release
		return errors.New("synthetic quiesce failure")
	}, func(readerstore.UserID) {}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	var archive bytes.Buffer
	if _, err := service.Export(t.Context(), backupAlice, "alice", time.Now(), &archive); err != nil {
		t.Fatal(err)
	}
	prepared, err := service.PrepareRestore(t.Context(), backupAlice, bytes.NewReader(archive.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { _, err := service.CommitRestore(t.Context(), backupAlice, prepared.ID); done <- err }()
	<-entered
	// Always release even when an assertion fails, so shutdown can join the owner.
	defer func() {
		select {
		case <-release:
		default:
			close(release)
		}
	}()
	status, err := service.GetRestore(backupAlice, prepared.ID)
	if err != nil || status.State != "committing" {
		t.Fatalf("running status=%+v err=%v", status, err)
	}
	if _, err := service.GetRestore(backupBob, prepared.ID); !errors.Is(err, ErrRestoreNotFound) {
		t.Fatalf("owner check=%v", err)
	}
	if err := service.CancelRestore(backupAlice, prepared.ID); !errors.Is(err, ErrRestoreConflict) {
		t.Fatalf("cancel live commit=%v", err)
	}
	if _, err := service.CommitRestore(t.Context(), backupAlice, prepared.ID); !errors.Is(err, ErrRestoreConflict) {
		t.Fatalf("repeat live commit=%v", err)
	}
	if _, err := service.PrepareRestore(t.Context(), backupAlice, bytes.NewReader(archive.Bytes())); !errors.Is(err, ErrRestoreConflict) {
		t.Fatalf("superseded live commit=%v", err)
	}
	if expired := service.takeExpiredOperations(time.Now().Add(2 * restoreLifetime)); len(expired) != 0 {
		t.Fatal("expired live commit")
	}
	close(release)
	if err := <-done; err == nil {
		t.Fatal("expected quiesce failure")
	}
	status, err = service.GetRestore(backupAlice, prepared.ID)
	if err != nil || status.State != "failed" || status.Result != nil {
		t.Fatalf("failure status=%+v err=%v", status, err)
	}
	if _, err := service.CommitRestore(t.Context(), backupAlice, prepared.ID); !errors.Is(err, ErrRestoreConflict) {
		t.Fatalf("replayed failed commit=%v", err)
	}
	// One retained result per reader; a fresh preparation retires the old result.
	next, err := service.PrepareRestore(t.Context(), backupAlice, bytes.NewReader(archive.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.GetRestore(backupAlice, prepared.ID); !errors.Is(err, ErrRestoreNotFound) {
		t.Fatalf("retained superseded result=%v", err)
	}
	if next.State != "prepared" || len(service.byID) != 1 {
		t.Fatalf("unbounded or invalid preparation: %+v", next)
	}
}

func TestRestoreCloseWaitsForDisconnectedCommittedRecovery(t *testing.T) {
	root := t.TempDir()
	readers, err := readerstore.NewManager(root, 2)
	if err != nil {
		t.Fatal(err)
	}
	defer readers.Close()
	if err := readers.Create(t.Context(), backupAlice); err != nil {
		t.Fatal(err)
	}
	entered, release := make(chan struct{}), make(chan struct{})
	service, err := NewService(readers, root, func(context.Context, readerstore.UserID) error { return nil }, func(readerstore.UserID) {}, func(ctx context.Context, _ readerstore.UserID) []string {
		close(entered)
		<-release
		if ctx.Err() != nil {
			t.Error("client disconnect canceled committed recovery")
		}
		return []string{"txt_recovery_incomplete"}
	})
	if err != nil {
		t.Fatal(err)
	}
	var archive bytes.Buffer
	if _, err := service.Export(t.Context(), backupAlice, "alice", time.Now(), &archive); err != nil {
		t.Fatal(err)
	}
	prepared, err := service.PrepareRestore(t.Context(), backupAlice, bytes.NewReader(archive.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		result, err := service.CommitRestore(ctx, backupAlice, prepared.ID)
		if err == nil && !result.Restored {
			err = errors.New("not restored")
		}
		done <- err
	}()
	<-entered
	cancel()
	closing := make(chan error, 1)
	go func() { closing <- service.Close() }()
	<-service.janitorDone
	select {
	case err := <-closing:
		close(release)
		<-done
		t.Fatalf("close retired live commit: %v", err)
	default:
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if err := <-closing; err != nil {
		t.Fatal(err)
	}
	if len(service.byID) != 0 {
		t.Fatal("close retained operation records")
	}
	if _, err := os.Stat(root); err != nil {
		t.Fatal(err)
	}
}
