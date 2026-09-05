package api

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/otwako/novelreader/internal/analyzer"
	"github.com/otwako/novelreader/internal/book"
	"github.com/otwako/novelreader/internal/readerstore"
	"github.com/otwako/novelreader/internal/sourceinteraction"
)

var (
	ErrReaderRuntimeCapacity = errors.New("api: reader runtime capacity is exhausted")
	ErrReaderRuntimeDeleting = errors.New("api: reader runtime is being deleted")
	ErrReaderRuntimeClosed   = errors.New("api: reader runtime manager is closed")
)

type readerRuntimeManager struct {
	services *readerServices
	readers  *readerstore.Manager
	searcher *book.Searcher
	jsVM     *analyzer.JSVM
	browser  sourceinteraction.Browser
	limits   book.SearcherLimits
	capacity int
	idleTTL  time.Duration
	now      func() time.Time
	mu       sync.Mutex
	changed  chan struct{}
	runtimes map[readerstore.UserID]*readerRuntime
	deleting map[readerstore.UserID]bool
	closed   bool
}

func newReaderRuntimeManager(readers *readerstore.Manager, searcher *book.Searcher, jsVM *analyzer.JSVM, browser sourceinteraction.Browser, limits book.SearcherLimits, capacity int, idleTTL time.Duration, services *readerServices) *readerRuntimeManager {
	if capacity < 1 {
		capacity = 32
	}
	if idleTTL <= 0 {
		idleTTL = 30 * time.Minute
	}
	limits.MaxSessions = max(1, limits.MaxSessions/capacity)
	return &readerRuntimeManager{services: services, readers: readers, searcher: searcher, jsVM: jsVM, browser: browser, limits: limits,
		capacity: capacity, idleTTL: idleTTL, now: time.Now, changed: make(chan struct{}),
		runtimes: make(map[readerstore.UserID]*readerRuntime), deleting: make(map[readerstore.UserID]bool)}
}

func (m *readerRuntimeManager) acquire(ctx context.Context, userID readerstore.UserID) (*readerRuntime, func(), error) {
	for {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		m.mu.Lock()
		if m.closed || m.deleting[userID] {
			err := ErrReaderRuntimeDeleting
			if m.closed {
				err = ErrReaderRuntimeClosed
			}
			m.mu.Unlock()
			return nil, nil, err
		}
		now := m.now()
		for id, runtime := range m.runtimes {
			if runtime.closing == nil && runtime.references == 0 && now.Sub(runtime.lastUsed) >= m.idleTTL {
				m.beginCloseLocked(id, runtime)
			}
		}
		if runtime := m.runtimes[userID]; runtime != nil && runtime.closing == nil {
			runtime.references++
			runtime.lastUsed = now
			m.mu.Unlock()
			return runtime, func() { m.release(userID, runtime) }, nil
		}
		if m.runtimes[userID] != nil || len(m.runtimes) >= m.capacity {
			waiting := false
			var oldestID readerstore.UserID
			var oldest *readerRuntime
			for id, runtime := range m.runtimes {
				if runtime.closing != nil {
					waiting = true
					continue
				}
				if runtime.references == 0 && (oldest == nil || runtime.lastUsed.Before(oldest.lastUsed)) {
					oldestID, oldest = id, runtime
				}
			}
			if !waiting && oldest != nil {
				m.beginCloseLocked(oldestID, oldest)
				waiting = true
			}
			changed := m.changed
			m.mu.Unlock()
			if !waiting {
				return nil, nil, ErrReaderRuntimeCapacity
			}
			select {
			case <-changed:
				continue
			case <-ctx.Done():
				return nil, nil, ctx.Err()
			}
		}
		m.mu.Unlock()

		runtime, err := m.openRuntime(ctx, userID)
		if err != nil {
			return nil, nil, err
		}
		m.mu.Lock()
		// Another acquisition or quiesce may have won while storage was opening.
		if m.closed || m.deleting[userID] || m.runtimes[userID] != nil || len(m.runtimes) >= m.capacity || ctx.Err() != nil {
			m.mu.Unlock()
			if err := runtime.close(); err != nil {
				return nil, nil, err
			}
			continue
		}
		runtime.references = 1
		runtime.lastUsed = m.now()
		m.runtimes[userID] = runtime
		m.mu.Unlock()
		return runtime, func() { m.release(userID, runtime) }, nil
	}
}

func (m *readerRuntimeManager) release(userID readerstore.UserID, runtime *readerRuntime) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.runtimes[userID] != runtime || runtime.references == 0 {
		return
	}
	runtime.references--
	runtime.lastUsed = m.now()
	if runtime.references == 0 {
		m.signalLocked()
	}
}

// Keep the entry until cleanup finishes: it owns capacity and prevents reopening
// the same reader while browser/catalog work still holds the old home.
func (m *readerRuntimeManager) beginCloseLocked(userID readerstore.UserID, runtime *readerRuntime) {
	if runtime.closing != nil {
		return
	}
	runtime.closing = make(chan struct{})
	go func() {
		err := runtime.close()
		if err != nil {
			slog.Error("reader runtime cleanup failed", "error", err)
		}
		m.mu.Lock()
		defer m.mu.Unlock()
		runtime.closeErr = err
		delete(m.runtimes, userID)
		close(runtime.closing)
		m.signalLocked()
	}()
}

func (m *readerRuntimeManager) quiesce(ctx context.Context, userID readerstore.UserID) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		m.mu.Lock()
		m.deleting[userID] = true
		runtime := m.runtimes[userID]
		if runtime == nil {
			m.mu.Unlock()
			return nil
		}
		if runtime.references > 0 && runtime.closing == nil {
			changed := m.changed
			m.mu.Unlock()
			select {
			case <-changed:
				continue
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		m.beginCloseLocked(userID, runtime)
		done := runtime.closing
		m.mu.Unlock()
		select {
		case <-done:
			return runtime.closeErr
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (m *readerRuntimeManager) resume(userID readerstore.UserID) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.deleting, userID)
	m.signalLocked()
}

func (m *readerRuntimeManager) signalLocked() {
	close(m.changed)
	m.changed = make(chan struct{})
}

func (m *readerRuntimeManager) Close() error {
	m.mu.Lock()
	m.closed = true
	runtimes := make([]*readerRuntime, 0, len(m.runtimes))
	for userID, runtime := range m.runtimes {
		m.beginCloseLocked(userID, runtime)
		runtimes = append(runtimes, runtime)
	}
	m.mu.Unlock()
	var closeErr error
	for _, runtime := range runtimes {
		<-runtime.closing
		closeErr = errors.Join(closeErr, runtime.closeErr)
	}
	return closeErr
}
