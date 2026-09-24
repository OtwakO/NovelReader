package fileimport

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"

	"github.com/otwako/novelreader/internal/readerstore"
)

type blockedAttempt struct {
	id     readerstore.UserID
	ctx    context.Context
	finish chan bool
}

func controlledPool(workers int) (*Pool, <-chan blockedAttempt) {
	started := make(chan blockedAttempt, workers)
	pool := newPool(workers, func(ctx context.Context, id readerstore.UserID) (bool, error) {
		attempt := blockedAttempt{id: id, ctx: ctx, finish: make(chan bool, 1)}
		started <- attempt
		worked := <-attempt.finish // Includes delayed cancellation/lease cleanup.
		return worked, ctx.Err()
	})
	return pool, started
}

func expectAttempt(t *testing.T, started <-chan blockedAttempt, id readerstore.UserID) blockedAttempt {
	t.Helper()
	synctest.Wait()
	select {
	case attempt := <-started:
		if attempt.id != id {
			t.Fatalf("reader=%s, want %s", attempt.id, id)
		}
		return attempt
	default:
		t.Fatalf("reader %s did not get a turn", id)
		return blockedAttempt{}
	}
}

func assertRetired(t *testing.T, pool *Pool) {
	t.Helper()
	pool.mu.Lock()
	defer pool.mu.Unlock()
	if len(pool.entries) != 0 || pool.queue != nil {
		t.Fatalf("idle scheduling state retained: entries=%d, queue=%v", len(pool.entries), pool.queue)
	}
}

func TestPoolBoundsFairTurnsAndCoalescesWakeups(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		pool, started := controlledPool(2)
		pool.Notify("alice")
		alice := expectAttempt(t, started, "alice")
		for range 1000 {
			pool.Notify("alice")
		}
		pool.Notify("bob")
		bob := expectAttempt(t, started, "bob")
		pool.Notify("carol")
		synctest.Wait()
		select {
		case attempt := <-started:
			t.Fatalf("exceeded active bound: %s", attempt.id)
		default:
		}
		pool.mu.Lock()
		if len(pool.entries) != 3 || len(pool.queue) != 1 {
			t.Errorf("duplicate wake-ups grew scheduling state")
		}
		pool.mu.Unlock()

		alice.finish <- true
		carol := expectAttempt(t, started, "carol") // Waiting readers precede Alice's next file.
		bob.finish <- false
		alice = expectAttempt(t, started, "alice")
		// The worker has observed no work, but a new receipt arrives before it retires.
		pool.Notify("alice")
		alice.finish <- false
		alice = expectAttempt(t, started, "alice")
		carol.finish <- false
		alice.finish <- false
		synctest.Wait()
		assertRetired(t, pool)
		pool.Close()
	})
}

func TestPoolQuiesceDrainsOneReaderAndCloseJoinsWorkers(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		pool, started := controlledPool(2)
		pool.Notify("alice")
		alice := expectAttempt(t, started, "alice")
		pool.Notify("bob")
		bob := expectAttempt(t, started, "bob")
		ctx, cancel := context.WithCancel(context.Background())
		stopped := make(chan error, 1)
		go func() { stopped <- pool.Quiesce(ctx, "alice") }()
		synctest.Wait()
		if alice.ctx.Err() == nil || bob.ctx.Err() != nil {
			t.Fatal("quiesce did not isolate the reader")
		}
		if err := pool.Notify("alice"); !errors.Is(err, ErrPaused) {
			t.Fatalf("paused notification=%v", err)
		}
		select {
		case err := <-stopped:
			t.Fatalf("quiesce skipped cleanup: %v", err)
		default:
		}
		cancel()
		if err := <-stopped; !errors.Is(err, context.Canceled) {
			t.Fatalf("quiesce cancellation=%v", err)
		}
		if err := pool.Forget("alice"); err == nil {
			t.Fatal("forgot an undrained reader")
		}
		alice.finish <- true
		synctest.Wait()
		if err := pool.Quiesce(context.Background(), "alice"); err != nil {
			t.Fatal(err)
		}
		pool.Resume("alice")
		alice = expectAttempt(t, started, "alice")
		if alice.ctx.Err() != nil {
			t.Fatal("resumed work inherited cancellation")
		}

		closed := make(chan struct{})
		go func() { pool.Close(); close(closed) }()
		synctest.Wait()
		if alice.ctx.Err() == nil || bob.ctx.Err() == nil {
			t.Fatal("shutdown did not cancel active workers")
		}
		select {
		case <-closed:
			t.Fatal("shutdown skipped cleanup")
		default:
		}
		alice.finish <- false
		bob.finish <- false
		<-closed
		assertRetired(t, pool)
		if err := pool.Notify("alice"); !errors.Is(err, ErrClosed) {
			t.Fatalf("closed notification=%v", err)
		}
		pool.Close()
	})
}

func TestPoolForgetDropsQueuedReaderWithoutStartingIt(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		pool, started := controlledPool(1)
		pool.Notify("alice")
		alice := expectAttempt(t, started, "alice")
		pool.Notify("removed")
		if err := pool.Quiesce(context.Background(), "removed"); err != nil {
			t.Fatal(err)
		}
		if err := pool.Forget("removed"); err != nil {
			t.Fatal(err)
		}
		if err := pool.Forget("removed"); err != nil {
			t.Fatal(err)
		}
		alice.finish <- false
		synctest.Wait()
		select {
		case attempt := <-started:
			t.Fatalf("deleted reader started: %s", attempt.id)
		default:
		}
		assertRetired(t, pool)
		pool.Close()
	})
}
