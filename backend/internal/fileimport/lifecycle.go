package fileimport

import (
	"context"
	"errors"
	"slices"

	"github.com/otwako/novelreader/internal/readerstore"
)

// Quiesce blocks notifications and cancels/joins this reader's current attempt.
// Stop HTTP/intake admission first. The caller serializes exclusive operations for
// a reader and must Resume after restore/abort, or Forget after deletion. A context
// timeout leaves the reader paused; it is not permission to replace a leased home.
func (p *Pool) Quiesce(ctx context.Context, id readerstore.UserID) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return ErrClosed
	}
	entry := p.entries[id]
	if entry == nil {
		entry = &readerWork{}
		p.entries[id] = entry
	}
	entry.paused = true
	p.queue = slices.DeleteFunc(p.queue, func(queued readerstore.UserID) bool { return queued == id })
	if len(p.queue) == 0 {
		p.queue = nil
	}
	done := entry.done
	if entry.cancel != nil {
		entry.cancel()
	}
	p.mu.Unlock()
	if done != nil {
		select {
		case <-done:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

// Resume lifts the barrier and queries the current database again. No old receipt
// IDs, file paths or cleanup instructions are replayed into a replaced home.
func (p *Pool) Resume(id readerstore.UserID) {
	p.mu.Lock()
	defer p.mu.Unlock()
	entry := p.entries[id]
	if p.closed || entry == nil || !entry.paused {
		return
	}
	entry.paused = false
	if entry.cancel != nil {
		entry.again = true
	} else {
		p.queue = append(p.queue, id)
		p.signalLocked()
	}
}

// Forget retires the lifecycle barrier after deletion without querying that home
// again. Only a drained, paused reader may be forgotten; repeated calls are safe.
func (p *Pool) Forget(id readerstore.UserID) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	entry := p.entries[id]
	if entry == nil {
		return nil
	}
	if !entry.paused || entry.cancel != nil {
		return errors.New("fileimport: reader must be quiescent before forgetting")
	}
	delete(p.entries, id)
	return nil
}

// Close stops admission and joins all workers before reader storage may be closed.
// Pending work stays in TXT records; no job goroutine or scheduling history survives.
func (p *Pool) Close() {
	p.mu.Lock()
	if !p.closed {
		p.closed = true
		p.queue = nil
		for id, entry := range p.entries {
			if entry.cancel == nil {
				delete(p.entries, id)
			}
		}
		p.cancel()
		p.signalLocked()
	}
	p.mu.Unlock()
	p.workers.Wait()
}
