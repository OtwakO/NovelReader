package book

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/otwako/novelreader/internal/analyzer"
	"github.com/otwako/novelreader/internal/booksource"
	"github.com/otwako/novelreader/internal/database"
	"github.com/otwako/novelreader/internal/fetcher"
)

type catalogSourceMap map[string]booksource.BookSource

func (s catalogSourceMap) GetByID(id string) (*booksource.BookSource, error) {
	value, ok := s[id]
	if !ok {
		return nil, nil
	}
	return &value, nil
}

func (s catalogSourceMap) ListEnabled() ([]booksource.BookSource, error) {
	values := make([]booksource.BookSource, 0, len(s))
	for _, source := range s {
		values = append(values, source)
	}
	return values, nil
}

func TestCatalogsReturnsCachedChaptersWithoutCrawl(t *testing.T) {
	store := newCatalogStore(t)
	addCatalogBook(t, store, "source", "https://unused.test/toc")
	want := []Chapter{{Index: 0, Title: "Cached", URL: "https://unused.test/chapter"}}
	if err := store.SaveCatalog("book-1", "source", 0, want); err != nil {
		t.Fatal(err)
	}
	catalogs := NewCatalogs(store, catalogSourceMap{}, nil)
	defer catalogs.Close()
	result := catalogs.Get("book-1")
	if result.State != CatalogReady || len(result.Chapters) != 1 || result.Chapters[0].Title != "Cached" {
		t.Fatalf("result=%+v", result)
	}
}

func TestCatalogsSharesOneCrawlAndAtomicallyPublishesCount(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		close(started)
		<-release
		_, _ = fmt.Fprint(w, `<a class="chapter" href="/1">One</a><a class="chapter" href="/2">Two</a>`)
	}))
	defer server.Close()
	store, sources, searcher := newCatalogFixture(t, server.URL)
	catalogs := newCatalogs(store, sources, searcher, 1, time.Second)
	defer catalogs.Close()
	if result := catalogs.Get("book-1"); result.State != CatalogSyncing {
		t.Fatalf("first=%+v", result)
	}
	<-started
	if result := catalogs.Get("book-1"); result.State != CatalogSyncing {
		t.Fatalf("joined=%+v", result)
	}
	close(release)
	result := waitCatalog(t, catalogs, "book-1", CatalogReady)
	stored, err := store.GetBook("book-1")
	if err != nil || stored == nil || stored.TotalChapterNum != 2 || len(result.Chapters) != 2 || requests.Load() != 1 {
		t.Fatalf("stored=%+v result=%+v requests=%d err=%v", stored, result, requests.Load(), err)
	}
}

func TestCatalogsRetainsFailureUntilRetry(t *testing.T) {
	var fail atomic.Bool
	fail.Store(true)
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if fail.Load() {
			http.Error(w, "unavailable", http.StatusBadGateway)
			return
		}
		_, _ = fmt.Fprint(w, `<a class="chapter" href="/1">One</a>`)
	}))
	defer server.Close()
	store, sources, searcher := newCatalogFixture(t, server.URL)
	catalogs := newCatalogs(store, sources, searcher, 1, time.Second)
	defer catalogs.Close()
	catalogs.Get("book-1")
	failed := waitCatalog(t, catalogs, "book-1", CatalogFailed)
	if failed.Err == nil || requests.Load() != 1 {
		t.Fatalf("failed=%+v requests=%d", failed, requests.Load())
	}
	if result := catalogs.Get("book-1"); result.State != CatalogFailed || requests.Load() != 1 {
		t.Fatalf("retained=%+v requests=%d", result, requests.Load())
	}
	fail.Store(false)
	if result := catalogs.Retry("book-1"); result.State != CatalogSyncing {
		t.Fatalf("retry=%+v", result)
	}
	waitCatalog(t, catalogs, "book-1", CatalogReady)
	if requests.Load() != 2 {
		t.Fatalf("requests=%d", requests.Load())
	}
}

func TestCatalogsInvalidateCancelsAndForgetsBookCrawl(t *testing.T) {
	started := make(chan struct{})
	cancelled := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-r.Context().Done()
		close(cancelled)
	}))
	defer server.Close()
	store, sources, searcher := newCatalogFixture(t, server.URL)
	catalogs := newCatalogs(store, sources, searcher, 1, time.Minute)
	defer catalogs.Close()
	catalogs.Get("book-1")
	<-started
	catalogs.Invalidate("book-1")
	select {
	case <-cancelled:
	case <-time.After(time.Second):
		t.Fatal("catalog crawl was not cancelled")
	}
	catalogs.mu.Lock()
	entry := catalogs.entries["book-1"]
	catalogs.mu.Unlock()
	if entry != nil {
		t.Fatal("invalidated catalog state remains")
	}
}

func TestCatalogsCloseCancelsAndDrainsCrawl(t *testing.T) {
	started := make(chan struct{})
	cancelled := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-r.Context().Done()
		close(cancelled)
	}))
	defer server.Close()
	store, sources, searcher := newCatalogFixture(t, server.URL)
	catalogs := newCatalogs(store, sources, searcher, 1, time.Minute)
	catalogs.Get("book-1")
	<-started
	catalogs.Close()
	select {
	case <-cancelled:
	case <-time.After(time.Second):
		t.Fatal("catalog crawl was not cancelled")
	}
	if result := catalogs.Get("book-1"); result.State != CatalogFailed || result.Err == nil {
		t.Fatalf("closed result=%+v", result)
	}
}

func TestSaveCatalogRejectsChangedSourceWithoutReplacingRows(t *testing.T) {
	store := newCatalogStore(t)
	addCatalogBook(t, store, "source-1", "http://example.test/toc")
	if err := store.SaveChapters("book-1", []Chapter{{Index: 0, Title: "Existing", URL: "/existing"}}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveCatalog("book-1", "source-2", 0, []Chapter{{Index: 0, Title: "Stale", URL: "/stale"}}); !errors.Is(err, ErrCatalogSourceChanged) {
		t.Fatalf("SaveCatalog error = %v, want ErrCatalogSourceChanged", err)
	}
	chapters, err := store.GetChapters("book-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(chapters) != 1 || chapters[0].Title != "Existing" {
		t.Fatalf("chapters = %+v, want existing catalog", chapters)
	}
}

func TestSaveCatalogDoesNotPublishCountWithoutBook(t *testing.T) {
	store := newCatalogStore(t)
	if err := store.SaveCatalog("missing", "source-1", 0, []Chapter{{Index: 0, Title: "One", URL: "/1"}}); err == nil {
		t.Fatal("missing book catalog was published")
	}
	chapters, err := store.GetChapters("missing")
	if err != nil || len(chapters) != 0 {
		t.Fatalf("chapters=%+v err=%v", chapters, err)
	}
}

func newCatalogFixture(t *testing.T, upstream string) (*Store, catalogSourceMap, *Searcher) {
	t.Helper()
	store := newCatalogStore(t)
	source := booksource.BookSource{ID: "source", BookSourceURL: upstream, BookSourceName: "Catalog", RuleToc: `{"chapterList":".chapter","chapterName":"text","chapterUrl":"href"}`}
	addCatalogBook(t, store, source.ID, upstream)
	sources := catalogSourceMap{source.ID: source}
	searcher := NewSearcher(fetcher.NewInsecure(time.Second), analyzer.NewJSVM(), analyzer.NewCacheManager(), sources, store)
	return store, sources, searcher
}

func newCatalogStore(t *testing.T) *Store {
	t.Helper()
	db, err := database.Open(filepath.Join(t.TempDir(), "catalog.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	initializeBookTestSchema(t, db)
	return NewStore(db)
}

func addCatalogBook(t *testing.T, store *Store, sourceID, tocURL string) {
	t.Helper()
	if err := store.AddBook(&Book{ID: "book-1", Name: "Book", SourceID: sourceID, SourceURL: tocURL, BookURL: tocURL, TocURL: tocURL}); err != nil {
		t.Fatal(err)
	}
}

func waitCatalog(t *testing.T, catalogs *Catalogs, bookID string, want CatalogState) CatalogResult {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	var result CatalogResult
	for time.Now().Before(deadline) {
		result = catalogs.Get(bookID)
		if result.State == want {
			return result
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("catalog did not reach %s: %+v", want, result)
	return CatalogResult{}
}
