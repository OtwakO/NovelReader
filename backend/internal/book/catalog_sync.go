package book

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/otwako/novelreader/internal/booksource"
)

var (
	ErrCatalogBookNotFound   = errors.New("catalog: book not found")
	ErrCatalogSourceNotFound = errors.New("catalog: book source not found")
	ErrCatalogSourceChanged  = errors.New("catalog: book source changed")
)

type CatalogState string

type CatalogFailure string

const (
	CatalogSyncing CatalogState = "syncing"
	CatalogReady   CatalogState = "ready"
	CatalogFailed  CatalogState = "failed"

	CatalogFailureStorage        CatalogFailure = "storage"
	CatalogFailureUpstream       CatalogFailure = "upstream"
	CatalogFailureBookNotFound   CatalogFailure = "book_not_found"
	CatalogFailureSourceNotFound CatalogFailure = "source_not_found"
)

type CatalogResult struct {
	State    CatalogState
	Failure  CatalogFailure
	Chapters []Chapter
	Err      error
}

type catalogSync struct {
	result CatalogResult
	cancel context.CancelFunc
	done   chan struct{}
}

// Catalogs owns the process-local synchronization state for one reader's books.
type Catalogs struct {
	store   *Store
	sources interface {
		GetByID(string) (*booksource.BookSource, error)
	}
	searcher *Searcher
	timeout  time.Duration
	slots    chan struct{}
	ctx      context.Context
	cancel   context.CancelFunc

	mu      sync.Mutex
	closed  bool
	entries map[string]*catalogSync
	wait    sync.WaitGroup
}

func NewCatalogs(store *Store, sources interface {
	GetByID(string) (*booksource.BookSource, error)
}, searcher *Searcher) *Catalogs {
	return newCatalogs(store, sources, searcher, 2, 2*time.Minute)
}

func newCatalogs(store *Store, sources interface {
	GetByID(string) (*booksource.BookSource, error)
}, searcher *Searcher, concurrency int, timeout time.Duration) *Catalogs {
	if concurrency < 1 {
		concurrency = 1
	}
	if timeout <= 0 {
		timeout = 2 * time.Minute
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Catalogs{store: store, sources: sources, searcher: searcher, timeout: timeout, slots: make(chan struct{}, concurrency), ctx: ctx, cancel: cancel, entries: make(map[string]*catalogSync)}
}

func (c *Catalogs) Get(bookID string) CatalogResult {
	c.mu.Lock()
	if current := c.entries[bookID]; current != nil {
		result := cloneCatalogResult(current.result)
		c.mu.Unlock()
		return result
	}
	closed := c.closed
	c.mu.Unlock()
	if closed {
		return CatalogResult{State: CatalogFailed, Failure: CatalogFailureUpstream, Err: context.Canceled}
	}
	chapters, err := c.store.GetChapters(bookID)
	if err != nil {
		return CatalogResult{State: CatalogFailed, Failure: CatalogFailureStorage, Err: err}
	}
	if len(chapters) > 0 {
		return CatalogResult{State: CatalogReady, Chapters: chapters}
	}
	return c.start(bookID)
}

func (c *Catalogs) Retry(bookID string) CatalogResult {
	c.mu.Lock()
	if current := c.entries[bookID]; current != nil {
		if current.result.State != CatalogFailed {
			result := cloneCatalogResult(current.result)
			c.mu.Unlock()
			return result
		}
		delete(c.entries, bookID)
		result := c.startLocked(bookID)
		c.mu.Unlock()
		return result
	}
	c.mu.Unlock()
	return c.Get(bookID)
}

func (c *Catalogs) start(bookID string) CatalogResult {
	c.mu.Lock()
	defer c.mu.Unlock()
	if current := c.entries[bookID]; current != nil {
		return cloneCatalogResult(current.result)
	}
	return c.startLocked(bookID)
}

func (c *Catalogs) startLocked(bookID string) CatalogResult {
	if c.closed {
		return CatalogResult{State: CatalogFailed, Failure: CatalogFailureUpstream, Err: context.Canceled}
	}
	entry := &catalogSync{result: CatalogResult{State: CatalogSyncing}, done: make(chan struct{})}
	ctx, cancel := context.WithCancel(c.ctx)
	entry.cancel = cancel
	c.entries[bookID] = entry
	c.wait.Add(1)
	go func() {
		defer cancel()
		c.run(ctx, bookID, entry)
	}()
	return entry.result
}

func (c *Catalogs) run(parent context.Context, bookID string, entry *catalogSync) {
	defer c.wait.Done()
	defer close(entry.done)
	result := CatalogResult{State: CatalogFailed, Failure: CatalogFailureUpstream, Err: context.Canceled}
	defer func() { c.finish(bookID, entry, result) }()
	ctx, cancel := context.WithTimeout(parent, c.timeout)
	defer cancel()
	select {
	case c.slots <- struct{}{}:
		defer func() { <-c.slots }()
	case <-ctx.Done():
		result.Err = ctx.Err()
		return
	}

	value, err := c.store.GetBook(bookID)
	if err != nil {
		result.Failure, result.Err = CatalogFailureStorage, err
		return
	}
	if value == nil {
		result.Failure, result.Err = CatalogFailureBookNotFound, ErrCatalogBookNotFound
		return
	}
	source, err := c.sources.GetByID(value.SourceID)
	if err != nil {
		result.Failure, result.Err = CatalogFailureStorage, err
		return
	}
	if source == nil {
		result.Failure, result.Err = CatalogFailureSourceNotFound, ErrCatalogSourceNotFound
		return
	}
	chapters, err := c.searcher.GetChapterListForBookContext(ctx, *source, value, value.TocURL)
	if err != nil {
		result.Failure, result.Err = CatalogFailureUpstream, err
		return
	}
	if len(chapters) == 0 {
		result.Failure, result.Err = CatalogFailureUpstream, errors.New("chapter catalog is empty")
		return
	}
	if err := c.store.SaveCatalog(bookID, value.SourceID, value.StateVersion, chapters); err != nil {
		if errors.Is(err, ErrCatalogBookNotFound) {
			result.Failure, result.Err = CatalogFailureBookNotFound, err
		} else if errors.Is(err, ErrCatalogSourceChanged) {
			result.Failure, result.Err = CatalogFailureUpstream, err
		} else {
			result.Failure, result.Err = CatalogFailureStorage, err
		}
		return
	}
	result = CatalogResult{State: CatalogReady, Chapters: chapters}
}

func (c *Catalogs) finish(bookID string, entry *catalogSync, result CatalogResult) {
	c.mu.Lock()
	if c.entries[bookID] == entry {
		if result.State == CatalogReady {
			delete(c.entries, bookID)
		} else {
			entry.result = cloneCatalogResult(result)
		}
	}
	c.mu.Unlock()
}

func (c *Catalogs) Invalidate(bookID string) {
	if c == nil {
		return
	}
	c.mu.Lock()
	entry := c.entries[bookID]
	if entry != nil {
		delete(c.entries, bookID)
		entry.cancel()
	}
	c.mu.Unlock()
	if entry != nil {
		<-entry.done
	}
}

func (c *Catalogs) Close() {
	if c == nil {
		return
	}
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
	c.closed = true
	c.cancel()
	c.mu.Unlock()
	c.wait.Wait()
}

func cloneCatalogResult(value CatalogResult) CatalogResult {
	value.Chapters = append([]Chapter(nil), value.Chapters...)
	return value
}
