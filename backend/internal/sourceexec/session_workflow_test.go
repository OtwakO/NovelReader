package sourceexec

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"
	"time"
)

func TestWorkflowPinsIdentityAndCancellationDoesNotReleaseAnotherOwner(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		registry := NewSessionRegistryWithLimits(1, time.Minute)
		first, release, err := registry.AcquireWorkflow(t.Context(), "source", "book", "")
		if err != nil {
			t.Fatal(err)
		}
		registry.AssociateChapter("source", "book", "chapter")
		waiting, cancel := context.WithCancel(t.Context())
		canceled := make(chan error, 1)
		go func() {
			_, unlock, err := registry.AcquireWorkflow(waiting, "source", "", "chapter")
			if err == nil {
				unlock()
			}
			canceled <- err
		}()
		synctest.Wait()
		// Unrelated work can execute despite the active session and an idle-cache
		// capacity of one. Expiry must not create a second identity for active work.
		other, otherRelease, err := registry.AcquireWorkflow(t.Context(), "source", "other", "")
		if err != nil || other == first {
			t.Fatalf("unrelated workflow: %v", err)
		}
		otherRelease()
		time.Sleep(2 * time.Minute)
		if registry.GetBook("source", "book") != first {
			t.Fatal("active identity expired")
		}
		cancel()
		if err := <-canceled; !errors.Is(err, context.Canceled) {
			t.Fatalf("wait cancellation: %v", err)
		}
		acquired := make(chan *SourceSession, 1)
		done := make(chan struct{})
		go func() {
			next, unlock, err := registry.AcquireWorkflow(t.Context(), "source", "book", "")
			if err != nil {
				t.Error(err)
				close(done)
				return
			}
			acquired <- next
			unlock()
			close(done)
		}()
		synctest.Wait()
		select {
		case <-acquired:
			t.Fatal("canceled waiter released active owner")
		default:
		}
		release()
		if next := <-acquired; next != first {
			t.Fatal("waiting workflow lost session continuity")
		}
		<-done
		registry.mu.Lock()
		defer registry.mu.Unlock()
		if len(registry.workflows) != 0 || len(registry.lastUsed) > 1 {
			t.Fatal("workflow retention did not settle to idle bound")
		}
	})
}

func TestSourceRetirementRejectsQueuedWorkAndAliasResurrection(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		registry := NewSessionRegistry()
		old, release, err := registry.AcquireWorkflow(t.Context(), "source", "book", "")
		if err != nil {
			t.Fatal(err)
		}
		result := make(chan error, 1)
		go func() {
			_, unlock, err := registry.AcquireWorkflow(t.Context(), "source", "book", "")
			if err == nil {
				unlock()
			}
			result <- err
		}()
		synctest.Wait()
		registry.DeleteSource("source")
		if err := registry.AssociateBook("source", "replacement", old); !errors.Is(err, ErrSessionRetired) {
			t.Fatalf("retired association: %v", err)
		}
		if registry.GetBook("source", "replacement") != nil {
			t.Fatal("retired work resurrected an alias")
		}
		release()
		if err := <-result; !errors.Is(err, ErrSessionRetired) {
			t.Fatalf("queued result: %v", err)
		}
		fresh, unlock, err := registry.AcquireWorkflow(t.Context(), "source", "book", "")
		if err != nil {
			t.Fatal(err)
		}
		defer unlock()
		if fresh == old {
			t.Fatal("new workflow reused retired state")
		}
	})
}

func TestWorkflowAliasCannotReplaceAnotherOwner(t *testing.T) {
	registry := NewSessionRegistry()
	incoming, releaseIncoming, err := registry.AcquireWorkflow(t.Context(), "source", "old-book", "")
	if err != nil {
		t.Fatal(err)
	}
	defer releaseIncoming()
	existing, releaseExisting, err := registry.AcquireWorkflow(t.Context(), "source", "new-book", "")
	if err != nil {
		t.Fatal(err)
	}
	defer releaseExisting()
	if err := registry.AssociateBook("source", "new-book", incoming); !errors.Is(err, ErrSessionAliasConflict) {
		t.Fatalf("association conflict: %v", err)
	}
	if err := registry.AssociateBook("source", "old-book", incoming); err != nil {
		t.Fatalf("same owner association: %v", err)
	}
	if registry.GetBook("source", "new-book") != existing {
		t.Fatal("alias replacement bypassed the destination's active workflow owner")
	}
}
