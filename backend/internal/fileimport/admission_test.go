package fileimport

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"testing/synctest"
	"time"

	"github.com/otwako/novelreader/internal/readerstore"
)

func requestTicket(t *testing.T, admission *Admission, id readerstore.UserID) AdmissionTicket {
	t.Helper()
	ticket, err := admission.Request(id)
	if err != nil {
		t.Fatal(err)
	}
	return ticket
}

func beginTransfer(t *testing.T, admission *Admission, id readerstore.UserID, ticket AdmissionTicket) (context.Context, func()) {
	t.Helper()
	ctx, release, err := admission.Begin(t.Context(), id, ticket.ID)
	if err != nil {
		t.Fatal(err)
	}
	return ctx, release
}

func TestAdmissionFairTurnsAndReaderBoundGrants(t *testing.T) {
	admission := NewAdmission()
	defer admission.Close()
	alice := requestTicket(t, admission, "alice")
	bob := requestTicket(t, admission, "bob")
	carol := requestTicket(t, admission, "carol")
	dora := requestTicket(t, admission, "dora")
	if alice.State != TicketGranted || bob.State != TicketGranted || carol.State != TicketWaiting {
		t.Fatalf("unexpected admission: %+v %+v %+v", alice, bob, carol)
	}
	if _, err := admission.Status("bob", alice.ID); !errors.Is(err, ErrTicketNotFound) {
		t.Fatalf("cross-reader ticket lookup: %v", err)
	}
	if _, _, err := admission.Begin(t.Context(), "bob", alice.ID); !errors.Is(err, ErrTicketNotFound) {
		t.Fatalf("cross-reader grant use: %v", err)
	}
	if _, _, err := admission.Begin(t.Context(), "carol", carol.ID); !errors.Is(err, ErrTicketNotReady) {
		t.Fatalf("waiting reader started: %v", err)
	}
	_, finishAlice := beginTransfer(t, admission, "alice", alice)
	defer finishAlice()
	_, finishBob := beginTransfer(t, admission, "bob", bob)
	defer finishBob()
	if _, _, err := admission.Begin(t.Context(), "alice", alice.ID); !errors.Is(err, ErrTicketNotReady) {
		t.Fatalf("grant used twice: %v", err)
	}
	finishAlice()
	finishAlice() // Releasing an attempt twice must not release someone else's slot.
	alice = requestTicket(t, admission, "alice")
	if alice.State != TicketWaiting {
		t.Fatal("returning reader overtook waiting readers")
	}
	carol, err := admission.Status("carol", carol.ID)
	if err != nil || carol.State != TicketGranted {
		t.Fatalf("next reader not granted: %+v %v", carol, err)
	}
	finishBob()
	dora, err = admission.Status("dora", dora.ID)
	if err != nil || dora.State != TicketGranted {
		t.Fatalf("FIFO order lost: %+v %v", dora, err)
	}
	if err := admission.Cancel(t.Context(), "carol", carol.ID); err != nil {
		t.Fatal(err)
	}
	alice, err = admission.Status("alice", alice.ID)
	if err != nil || alice.State != TicketGranted {
		t.Fatalf("cancelled grant retained capacity: %+v %v", alice, err)
	}
}

func TestAdmissionBoundsAndDeduplicatesWaitingMetadata(t *testing.T) {
	admission := NewAdmission()
	defer admission.Close()
	first := requestTicket(t, admission, "first")
	for i := 1; i < maxAdmissionTickets; i++ {
		requestTicket(t, admission, readerstore.UserID(fmt.Sprintf("reader-%d", i)))
	}
	for range 10 {
		if repeated := requestTicket(t, admission, "first"); repeated.ID != first.ID {
			t.Fatal("duplicate request allocated another ticket")
		}
	}
	if _, err := admission.Request("overflow"); !errors.Is(err, ErrAdmissionFull) {
		t.Fatalf("unbounded waiting queue: %v", err)
	}
	if err := admission.Cancel(t.Context(), "first", first.ID); err != nil {
		t.Fatal(err)
	}
	if next := requestTicket(t, admission, "overflow"); next.State != TicketWaiting {
		t.Fatal("new arrival skipped existing waiters")
	}
}

func TestAdmissionExpiresAbandonedTicketsWithoutLeakingActiveSlots(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		admission := NewAdmission()
		defer admission.Close()
		_, finishAlice := beginTransfer(t, admission, "alice", requestTicket(t, admission, "alice"))
		defer finishAlice()
		bobCtx, finishBob := beginTransfer(t, admission, "bob", requestTicket(t, admission, "bob"))
		defer finishBob()
		stale := requestTicket(t, admission, "stale")
		live := requestTicket(t, admission, "live")
		time.Sleep(waitingTicketTTL / 2)
		if _, err := admission.Status("live", live.ID); err != nil {
			t.Fatal(err)
		}
		time.Sleep(waitingTicketTTL/2 + time.Second)
		if _, err := admission.Status("stale", stale.ID); !errors.Is(err, ErrTicketNotFound) {
			t.Fatalf("abandoned waiter retained: %v", err)
		}
		finishAlice()
		live, err := admission.Status("live", live.ID)
		if err != nil || live.State != TicketGranted {
			t.Fatalf("live waiter lost: %+v %v", live, err)
		}
		time.Sleep(grantedTicketTTL + time.Second)
		if _, err := admission.Status("live", live.ID); !errors.Is(err, ErrTicketNotFound) {
			t.Fatalf("unused grant retained: %v", err)
		}
		ready := requestTicket(t, admission, "ready")
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		if _, _, err := admission.Begin(ctx, "ready", ready.ID); !errors.Is(err, context.Canceled) {
			t.Fatalf("cancelled request started: %v", err)
		}
		_, finishReady := beginTransfer(t, admission, "ready", ready)
		defer finishReady()
		time.Sleep(transferTimeout)
		if !errors.Is(bobCtx.Err(), context.DeadlineExceeded) {
			t.Fatalf("transfer deadline missing: %v", bobCtx.Err())
		}
		// Cancellation is not cleanup: slots stay occupied until callers release.
		if queued := requestTicket(t, admission, "queued"); queued.State != TicketWaiting {
			t.Fatal("cancelled but undrained transfer released capacity")
		}
	})
}
