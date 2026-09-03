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
	"github.com/otwako/novelreader/internal/sourceinteraction"
	"github.com/otwako/novelreader/internal/sourceprofile"
)

var (
	ErrReaderRuntimeCapacity = errors.New("api: reader runtime capacity is exhausted")
	ErrReaderRuntimeDeleting = errors.New("api: reader runtime is being deleted")
)

type readerRuntime struct {
	home               *readerstore.Home
	sourceStore        *booksource.Store
	bookStore          *book.Store
	searcher           *book.Searcher
	fontStore          *fontstore.Store
	sourceProfiles     *sourceprofile.Store
	sourceInteractions *sourceinteraction.Describer
	browserSessions    *sourceinteraction.BrowserSessions
	catalogs           *book.Catalogs
	lastUsed           time.Time
	references         int
}

type readerRuntimeManager struct {
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
}

func newReaderRuntimeManager(readers *readerstore.Manager, searcher *book.Searcher, jsVM *analyzer.JSVM, browser sourceinteraction.Browser, limits book.SearcherLimits, capacity int, idleTTL time.Duration) *readerRuntimeManager {
	if capacity < 1 {
		capacity = 32
	}
	if idleTTL <= 0 {
		idleTTL = 30 * time.Minute
	}
	limits.MaxSessions = max(1, limits.MaxSessions/capacity)
	return &readerRuntimeManager{
		readers: readers, searcher: searcher, jsVM: jsVM, browser: browser, limits: limits,
		capacity: capacity, idleTTL: idleTTL, now: time.Now, changed: make(chan struct{}),
		runtimes: make(map[readerstore.UserID]*readerRuntime), deleting: make(map[readerstore.UserID]bool),
	}
}

func (m *readerRuntimeManager) acquire(ctx context.Context, userID readerstore.UserID) (*readerRuntime, func(), error) {
	m.mu.Lock()
	if m.deleting[userID] {
		m.mu.Unlock()
		return nil, nil, ErrReaderRuntimeDeleting
	}
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
	sourceProfiles := sourceprofile.NewStore(home.DB(), home.CredentialsDB())
	if err := sourceProfiles.Reconcile(ctx); err != nil {
		_ = home.Close()
		return nil, nil, fmt.Errorf("api: reconcile source profiles: %w", err)
	}
	readerJS := m.jsVM.ForkStateWithDeviceID(readerstore.DeviceID(userID))
	readerSearcher := m.searcher.ForkReader(readerJS, analyzer.NewCacheManager(), sourceStore, bookStore, m.limits)
	readerSearcher.SetSourceSessionHydrator(sourceSessionHydrator(sourceProfiles))
	runtime := &readerRuntime{
		home: home, sourceStore: sourceStore, bookStore: bookStore, fontStore: fontStore, sourceProfiles: sourceProfiles,
		sourceInteractions: sourceinteraction.NewDescriber(sourceStore, sourceProfiles, readerJS.ForkState()),
		browserSessions:    sourceinteraction.NewBrowserSessions(m.browser),
		catalogs:           book.NewCatalogs(bookStore, sourceStore, readerSearcher),
		searcher:           readerSearcher,
		lastUsed:           now, references: 1,
	}

	m.mu.Lock()
	if m.deleting[userID] {
		m.mu.Unlock()
		_ = home.Close()
		return nil, nil, ErrReaderRuntimeDeleting
	}
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
	if runtime.references == 0 {
		m.signalLocked()
	}
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
	oldest.close()
	return true
}

func (m *readerRuntimeManager) evictIdleLocked(now time.Time) {
	for userID, runtime := range m.runtimes {
		if runtime.references == 0 && now.Sub(runtime.lastUsed) >= m.idleTTL {
			delete(m.runtimes, userID)
			runtime.close()
		}
	}
}

// quiesce rejects new work, drains in-flight requests, and drops all per-reader runtime state.
func (m *readerRuntimeManager) quiesce(ctx context.Context, userID readerstore.UserID) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		m.mu.Lock()
		m.deleting[userID] = true
		runtime := m.runtimes[userID]
		if runtime != nil && runtime.references > 0 {
			changed := m.changed
			m.mu.Unlock()
			select {
			case <-changed:
				continue
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		if runtime != nil {
			delete(m.runtimes, userID)
		}
		m.mu.Unlock()
		if runtime != nil {
			return runtime.close()
		}
		return nil
	}
}

// resume permits new runtime acquisitions after a temporary Reader Data replacement.
func (m *readerRuntimeManager) resume(userID readerstore.UserID) {
	m.mu.Lock()
	delete(m.deleting, userID)
	m.signalLocked()
	m.mu.Unlock()
}

func (m *readerRuntimeManager) signalLocked() {
	close(m.changed)
	m.changed = make(chan struct{})
}

func (m *readerRuntimeManager) Close() error {
	m.mu.Lock()
	runtimes := m.runtimes
	m.runtimes = make(map[readerstore.UserID]*readerRuntime)
	m.mu.Unlock()
	var closeErr error
	for _, runtime := range runtimes {
		closeErr = errors.Join(closeErr, runtime.close())
	}
	return closeErr
}

func (r *readerRuntime) close() error {
	if r == nil {
		return nil
	}
	if r.browserSessions != nil {
		r.browserSessions.CloseSource(context.Background(), "")
	}
	if r.catalogs != nil {
		r.catalogs.Close()
	}
	return r.home.Close()
}
