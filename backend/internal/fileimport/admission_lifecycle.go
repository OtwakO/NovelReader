package fileimport

import (
	"context"
	"errors"
	"time"

	"github.com/otwako/novelreader/internal/readerstore"
)

// Quiesce invalidates queued/granted admission and cancels/joins an active transfer
// for one reader. A timeout leaves the barrier in place. The lifecycle owner must
// Resume after restore/abort, or Forget only after successful home removal.
func (a *Admission) Quiesce(ctx context.Context, id readerstore.UserID) error {
	a.mu.Lock()
	if a.closed {
		a.mu.Unlock()
		return ErrClosed
	}
	a.paused[id] = true
	var done chan struct{}
	if entry := a.entries[id]; entry != nil {
		if entry.cancel != nil {
			entry.cancel()
			done = entry.done
		} else {
			delete(a.entries, id)
		}
	}
	a.advanceLocked(time.Now())
	a.mu.Unlock()
	return waitTransfer(ctx, done)
}

func waitTransfer(ctx context.Context, done <-chan struct{}) error {
	if done != nil {
		select {
		case <-done:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

// Resume permits fresh admission without replaying pre-restore tickets.
func (a *Admission) Resume(id readerstore.UserID) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.paused, id)
}

// Forget retires a drained deletion barrier. It never opens or recreates a home.
func (a *Admission) Forget(id readerstore.UserID) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.paused[id] && a.entries[id] == nil {
		return nil
	}
	if !a.paused[id] || a.entries[id] != nil {
		return errors.New("fileimport: intake must be quiescent before forgetting")
	}
	delete(a.paused, id)
	return nil
}

// Close stops admission and joins every started transfer before storage closes.
// The Begin caller remains responsible for ending I/O and releasing its lease.
func (a *Admission) Close() {
	a.mu.Lock()
	a.closed = true
	for id, entry := range a.entries {
		if entry.cancel != nil {
			entry.cancel()
		} else {
			delete(a.entries, id)
		}
	}
	a.queue = nil
	clear(a.paused)
	a.mu.Unlock()
	a.active.Wait()
}
