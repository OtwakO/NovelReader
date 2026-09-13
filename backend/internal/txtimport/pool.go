// Package txtimport schedules durable TXT analysis independently of API runtimes.
package txtimport

import (
	"context"
	"errors"
	"log/slog"
	"sync"

	"github.com/otwako/novelreader/internal/readerstore"
)

// Workers is the initial process-wide analysis bound. Production storage must
// budget these active home leases in addition to foreground runtime capacity.
const Workers = 2

var (
	ErrClosed = errors.New("txtimport: worker pool closed")
	ErrPaused = errors.New("txtimport: reader imports paused")
)

type readerWork struct {
	cancel context.CancelFunc
	done   chan struct{}
	again  bool
	paused bool
}

// Pool owns fixed workers and deduplicated reader wake-ups, not per-file jobs or
// reader runtimes. Create one pool per application. Persist work before Notify;
// received records already represent pending automatic analysis. Caller-owned
// startup/intake recovery must finish before admitting transfers or notifications.
type Pool struct {
	mu      sync.Mutex
	entries map[readerstore.UserID]*readerWork
	queue   []readerstore.UserID
	changed chan struct{}
	closed  bool
	workers sync.WaitGroup
	ctx     context.Context
	cancel  context.CancelFunc
	process func(context.Context, readerstore.UserID) (bool, error)
}

func NewPool(readers *readerstore.Manager) *Pool {
	return newPool(Workers, func(ctx context.Context, id readerstore.UserID) (bool, error) {
		return analyzeNext(ctx, readers, id)
	})
}

// The private seam covers blocking storage/analysis in deterministic lifecycle
// tests; it is not a public callback-based job framework.
func newPool(workers int, process func(context.Context, readerstore.UserID) (bool, error)) *Pool {
	ctx, cancel := context.WithCancel(context.Background())
	pool := &Pool{entries: make(map[readerstore.UserID]*readerWork), changed: make(chan struct{}), ctx: ctx, cancel: cancel, process: process}
	pool.workers.Add(workers)
	for range workers {
		go pool.run()
	}
	return pool
}

// Notify is a wake-up hint for an authenticated reader's durable pending work.
// Repeated hints occupy one entry, including when a file is already running.
func (p *Pool) Notify(id readerstore.UserID) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return ErrClosed
	}
	if entry := p.entries[id]; entry != nil {
		if entry.paused {
			return ErrPaused
		}
		if entry.cancel != nil {
			entry.again = true
		}
		return nil
	}
	p.entries[id] = &readerWork{}
	p.queue = append(p.queue, id)
	p.signalLocked()
	return nil
}

func (p *Pool) run() {
	defer p.workers.Done()
	for {
		p.mu.Lock()
		for len(p.queue) == 0 && !p.closed {
			changed := p.changed
			p.mu.Unlock()
			<-changed
			p.mu.Lock()
		}
		if p.closed {
			p.mu.Unlock()
			return
		}
		id := p.queue[0]
		p.queue[0] = "" // Do not retain retired reader IDs in the backing array.
		p.queue = p.queue[1:]
		if len(p.queue) == 0 {
			p.queue = nil
		}
		entry := p.entries[id]
		ctx, cancel := context.WithCancel(p.ctx)
		entry.cancel, entry.done, entry.again = cancel, make(chan struct{}), false
		p.mu.Unlock()

		// Exactly one file (including its home lease and index) leaves scope before
		// this reader rejoins the tail, allowing other readers to take their turn.
		worked, err := p.process(ctx, id)
		if err != nil {
			level := slog.LevelWarn
			if ctx.Err() != nil {
				level = slog.LevelInfo
			}
			// Keep joined cleanup errors visible even when cancellation is expected.
			slog.Log(context.Background(), level, "TXT analysis attempt did not complete", "reader_id", id, "error", err)
		}
		cancel()
		p.mu.Lock()
		entry.cancel = nil
		close(entry.done)
		entry.done = nil
		switch {
		case p.closed:
			delete(p.entries, id)
		case entry.paused:
			// Retain only the lifecycle barrier until the owner calls Resume/Forget.
		case worked || entry.again:
			p.queue = append(p.queue, id)
		default:
			delete(p.entries, id)
		}
		p.signalLocked()
		p.mu.Unlock()
	}
}

func (p *Pool) signalLocked() {
	close(p.changed)
	p.changed = make(chan struct{})
}
