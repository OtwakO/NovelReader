package txtimport

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"
)

func TestAdmissionQuiesceDrainsOneReaderAndInvalidatesOldGrants(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		admission := NewAdmission()
		defer admission.Close()
		alice := requestTicket(t, admission, "alice")
		aliceCtx, finishAlice := beginTransfer(t, admission, "alice", alice)
		defer finishAlice()
		bobCtx, finishBob := beginTransfer(t, admission, "bob", requestTicket(t, admission, "bob"))
		defer finishBob()
		queued := requestTicket(t, admission, "queued")

		ctx, cancel := context.WithCancel(t.Context())
		stopped := make(chan error, 1)
		go func() { stopped <- admission.Quiesce(ctx, "alice") }()
		synctest.Wait()
		if aliceCtx.Err() == nil || bobCtx.Err() != nil {
			t.Fatal("quiesce did not isolate the reader")
		}
		if _, err := admission.Request("alice"); !errors.Is(err, ErrPaused) {
			t.Fatalf("paused reader admitted: %v", err)
		}
		select {
		case err := <-stopped:
			t.Fatalf("quiesce skipped lease cleanup: %v", err)
		default:
		}
		cancel()
		if err := <-stopped; !errors.Is(err, context.Canceled) {
			t.Fatalf("drain timeout: %v", err)
		}
		if err := admission.Forget("alice"); err == nil {
			t.Fatal("forgot an undrained reader")
		}
		finishAlice()
		if err := admission.Quiesce(t.Context(), "alice"); err != nil {
			t.Fatal(err)
		}
		admission.Resume("alice")
		if _, _, err := admission.Begin(t.Context(), "alice", alice.ID); !errors.Is(err, ErrTicketNotFound) {
			t.Fatalf("pre-restore ticket survived: %v", err)
		}
		if fresh := requestTicket(t, admission, "alice"); fresh.ID == alice.ID || fresh.State != TicketWaiting {
			t.Fatalf("restored reader replayed old admission: %+v", fresh)
		}
		// Removing an unused grant lets the next reader advance without any home I/O.
		if err := admission.Quiesce(t.Context(), "queued"); err != nil {
			t.Fatal(err)
		}
		if err := admission.Forget("queued"); err != nil {
			t.Fatal(err)
		}
		if err := admission.Forget("queued"); err != nil {
			t.Fatal(err)
		}
		if _, err := admission.Status("queued", queued.ID); !errors.Is(err, ErrTicketNotFound) {
			t.Fatalf("deleted reader retained its ticket: %v", err)
		}
	})
}

func TestAdmissionCancelAndCloseWaitForTransferCleanup(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		admission := NewAdmission()
		alice := requestTicket(t, admission, "alice")
		aliceCtx, finishAlice := beginTransfer(t, admission, "alice", alice)
		defer finishAlice()
		bobCtx, finishBob := beginTransfer(t, admission, "bob", requestTicket(t, admission, "bob"))
		defer finishBob()
		requestTicket(t, admission, "waiting")
		if err := admission.Cancel("alice", alice.ID); err != nil || aliceCtx.Err() == nil {
			t.Fatalf("active cancellation: %v", err)
		}
		closed := make(chan struct{})
		go func() { admission.Close(); close(closed) }()
		synctest.Wait()
		if bobCtx.Err() == nil {
			t.Fatal("shutdown did not cancel transfers")
		}
		if _, err := admission.Request("new"); !errors.Is(err, ErrClosed) {
			t.Fatalf("shutdown admitted work: %v", err)
		}
		select {
		case <-closed:
			t.Fatal("shutdown skipped transfer cleanup")
		default:
		}
		finishAlice()
		finishBob()
		<-closed
		if len(admission.entries) != 0 || len(admission.paused) != 0 || admission.queue != nil {
			t.Fatal("shutdown retained admission state")
		}
		admission.Close()
	})
}
