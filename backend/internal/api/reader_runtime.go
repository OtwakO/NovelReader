package api

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/otwako/novelreader/internal/analyzer"
	"github.com/otwako/novelreader/internal/book"
	"github.com/otwako/novelreader/internal/booksource"
	"github.com/otwako/novelreader/internal/fontstore"
	"github.com/otwako/novelreader/internal/readerstore"
)

var ErrReaderRuntimeCapacity = errors.New("api: reader runtime capacity is exhausted")

type readerRuntime struct {
	home        *readerstore.Home
	sourceStore *booksource.Store
	bookStore   *book.Store
	searcher    *book.Searcher
	fontStore   *fontstore.Store
	lastUsed    time.Time
	references  int
}

type readerRuntimeManager struct {
	readers  *readerstore.Manager
	searcher *book.Searcher
	jsVM     *analyzer.JSVM
	limits   book.SearcherLimits
	capacity int
	idleTTL  time.Duration
	now      func() time.Time
	mu       sync.Mutex
	runtimes map[readerstore.UserID]*readerRuntime
}

func newReaderRuntimeManager(readers *readerstore.Manager, searcher *book.Searcher, jsVM *analyzer.JSVM, limits book.SearcherLimits, capacity int, idleTTL time.Duration) *readerRuntimeManager {
	if capacity < 1 {
		capacity = 32
	}
	if idleTTL <= 0 {
		idleTTL = 30 * time.Minute
	}
	limits.MaxSessions = max(1, limits.MaxSessions/capacity)
	return &readerRuntimeManager{
		readers: readers, searcher: searcher, jsVM: jsVM, limits: limits,
		capacity: capacity, idleTTL: idleTTL, now: time.Now,
		runtimes: make(map[readerstore.UserID]*readerRuntime),
	}
}

func (m *readerRuntimeManager) acquire(ctx context.Context, userID readerstore.UserID) (*readerRuntime, func(), error) {
	m.mu.Lock()
	now := m.now()
	m.evictIdleLocked(now)
	if runtime := m.runtimes[userID]; runtime != nil {
		runtime.references++
		runtime.lastUsed = now
		m.mu.Unlock()
		return runtime, func() { m.release(userID, runtime) }, nil
	}
	if len(m.runtimes) >= m.capacity && !m.evictOldestIdleLocked() {
		m.mu.Unlock()
		return nil, nil, ErrReaderRuntimeCapacity
	}
	m.mu.Unlock()

	home, err := m.readers.Open(ctx, userID)
	if err != nil {
		return nil, nil, fmt.Errorf("api: open reader home: %w", err)
	}
	sourceStore := booksource.NewStore(home.DB())
	bookStore := book.NewStore(home.DB())
	fontStore := fontstore.NewStore(home.DB(), home.Files())
	runtime := &readerRuntime{
		home: home, sourceStore: sourceStore, bookStore: bookStore, fontStore: fontStore,
		searcher: m.searcher.ForkReader(m.jsVM.ForkState(), analyzer.NewCacheManager(), sourceStore, bookStore, m.limits),
		lastUsed: now, references: 1,
	}

	m.mu.Lock()
	if existing := m.runtimes[userID]; existing != nil {
		existing.references++
		existing.lastUsed = m.now()
		m.mu.Unlock()
		_ = home.Close()
		return existing, func() { m.release(userID, existing) }, nil
	}
	if len(m.runtimes) >= m.capacity && !m.evictOldestIdleLocked() {
		m.mu.Unlock()
		_ = home.Close()
		return nil, nil, ErrReaderRuntimeCapacity
	}
	m.runtimes[userID] = runtime
	m.mu.Unlock()
	return runtime, func() { m.release(userID, runtime) }, nil
}

func (m *readerRuntimeManager) release(userID readerstore.UserID, runtime *readerRuntime) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.runtimes[userID] != runtime || runtime.references == 0 {
		return
	}
	runtime.references--
	runtime.lastUsed = m.now()
}

func (m *readerRuntimeManager) evictOldestIdleLocked() bool {
	var oldestID readerstore.UserID
	var oldest *readerRuntime
	for userID, runtime := range m.runtimes {
		if runtime.references == 0 && (oldest == nil || runtime.lastUsed.Before(oldest.lastUsed)) {
			oldestID, oldest = userID, runtime
		}
	}
	if oldest == nil {
		return false
	}
	delete(m.runtimes, oldestID)
	_ = oldest.home.Close()
	return true
}

func (m *readerRuntimeManager) evictIdleLocked(now time.Time) {
	for userID, runtime := range m.runtimes {
		if runtime.references == 0 && now.Sub(runtime.lastUsed) >= m.idleTTL {
			delete(m.runtimes, userID)
			_ = runtime.home.Close()
		}
	}
}

func (m *readerRuntimeManager) Close() error {
	m.mu.Lock()
	runtimes := m.runtimes
	m.runtimes = make(map[readerstore.UserID]*readerRuntime)
	m.mu.Unlock()
	var closeErr error
	for _, runtime := range runtimes {
		closeErr = errors.Join(closeErr, runtime.home.Close())
	}
	return closeErr
}
