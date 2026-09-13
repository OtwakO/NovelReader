package txtimport

import (
	"context"
	"sync"
	"time"

	"github.com/otwako/novelreader/internal/readerstore"
)

// Begin consumes one reader-bound grant. Use the returned context for acquisition
// and release only after I/O and the reader-home lease have ended. Cancellation
// requests an end; it does not prove cleanup. Release is safe to call twice.
// HTTP callers must also interrupt blocked body reads on context cancellation.
func (a *Admission) Begin(ctx context.Context, id readerstore.UserID, ticketID string) (context.Context, func(), error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := a.availableLocked(id); err != nil {
		return nil, nil, err
	}
	a.advanceLocked(time.Now())
	entry := a.entries[id]
	if entry == nil || entry.ticket.ID != ticketID {
		return nil, nil, ErrTicketNotFound
	}
	if entry.ticket.State != TicketGranted {
		return nil, nil, ErrTicketNotReady
	}
	ctx, cancel := context.WithTimeout(ctx, transferTimeout)
	entry.ticket.State = TicketTransferring
	entry.ticket.ExpiresAt, _ = ctx.Deadline()
	entry.cancel, entry.done = cancel, make(chan struct{})
	a.active.Add(1)
	var once sync.Once
	release := func() {
		once.Do(func() {
			cancel()
			a.mu.Lock()
			delete(a.entries, id)
			close(entry.done)
			a.advanceLocked(time.Now())
			a.mu.Unlock()
			a.active.Done()
		})
	}
	return ctx, release, nil
}

// Cancel abandons a waiting/granted ticket or requests active cancellation. An
// active ticket remains visible and occupies its slot until its caller releases.
// Missing tickets are already retired; a stale cancellation cannot cancel a new one.
func (a *Admission) Cancel(id readerstore.UserID, ticketID string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		return ErrClosed
	}
	entry := a.entries[id]
	if entry == nil || entry.ticket.ID != ticketID {
		return nil
	}
	if entry.cancel != nil {
		entry.cancel()
	} else {
		delete(a.entries, id)
	}
	a.advanceLocked(time.Now())
	return nil
}
