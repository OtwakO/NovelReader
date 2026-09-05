// API boundary tests for typed crawl failures and resource classifications.
package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/otwako/novelreader/internal/analyzer"
	"github.com/otwako/novelreader/internal/book"
	"github.com/otwako/novelreader/internal/booksource"
	"github.com/otwako/novelreader/internal/database"
	"github.com/otwako/novelreader/internal/fetcher"
)

func TestHandleGetChaptersExposesTypedPaginationFailure(t *testing.T) {
	sourceServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/toc" {
			_, _ = w.Write([]byte(`<a class="chapter" href="/chapter/1">第一章</a><a class="next" href="/toc-2">下一页</a>`))
			return
		}
		http.Error(w, "upstream unavailable", http.StatusBadGateway)
	}))
	defer sourceServer.Close()

	server, store, closeDB := newCrawlAPIServer(t)
	defer closeDB()
	source := &booksource.BookSource{
		BookSourceURL: sourceServer.URL, BookSourceName: "fixture", RuleToc: `{"chapterList":"@css:.chapter","chapterName":"@text","chapterUrl":"@href","nextTocUrl":"@css:.next@href"}`,
	}
	if err := store.sourceStore.Upsert(source); err != nil {
		t.Fatal(err)
	}
	if err := store.bookStore.AddBook(&book.Book{ID: "book-1", Name: "Fixture", SourceID: source.ID, SourceURL: sourceServer.URL, BookURL: sourceServer.URL + "/book", TocURL: sourceServer.URL + "/toc", Origin: "fixture"}); err != nil {
		t.Fatal(err)
	}

	started := invokeBookRoute(server.standalone.handleGetChapters, "book-1", "")
	if started.Code != http.StatusAccepted {
		t.Fatalf("start status=%d body=%s", started.Code, started.Body.String())
	}
	response := waitForCatalogResponse(t, server, "book-1", http.StatusBadGateway)
	var payload crawlErrorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusBadGateway || payload.Code != "toc_pagination_failed" || payload.Workflow != "toc" || payload.PagesFetched != 1 || payload.FailedURL == "" {
		t.Fatalf("status=%d payload=%+v, want typed TOC failure", response.Code, payload)
	}
	chapters, err := store.bookStore.GetChapters("book-1")
	if err != nil || len(chapters) != 0 {
		t.Fatalf("chapters=%+v err=%v, want fail-closed cache", chapters, err)
	}
}

func TestHandleGetChaptersStartsAndReturnsSynchronizedCatalog(t *testing.T) {
	sourceServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<a class="chapter" href="/chapter/1">第一章</a>`))
	}))
	defer sourceServer.Close()
	server, store, closeDB := newCrawlAPIServer(t)
	defer closeDB()
	source := &booksource.BookSource{BookSourceURL: sourceServer.URL, BookSourceName: "fixture", RuleToc: `{"chapterList":".chapter","chapterName":"text","chapterUrl":"href"}`}
	if err := store.sourceStore.Upsert(source); err != nil {
		t.Fatal(err)
	}
	if err := store.bookStore.AddBook(&book.Book{ID: "book-1", Name: "Fixture", SourceID: source.ID, SourceURL: sourceServer.URL, BookURL: sourceServer.URL + "/book", TocURL: sourceServer.URL}); err != nil {
		t.Fatal(err)
	}
	started := invokeBookRoute(server.standalone.handleGetChapters, "book-1", "")
	if started.Code != http.StatusAccepted || started.Header().Get("Retry-After") != "1" {
		t.Fatalf("start status=%d headers=%v body=%s", started.Code, started.Header(), started.Body.String())
	}
	ready := waitForCatalogResponse(t, server, "book-1", http.StatusOK)
	var chapters []book.Chapter
	if err := json.Unmarshal(ready.Body.Bytes(), &chapters); err != nil || len(chapters) != 1 || chapters[0].Title != "第一章" {
		t.Fatalf("chapters=%+v error=%v", chapters, err)
	}
}

func TestHandleRetryChaptersRestartsRetainedFailure(t *testing.T) {
	var fail atomic.Bool
	fail.Store(true)
	sourceServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if fail.Load() {
			http.Error(w, "unavailable", http.StatusBadGateway)
			return
		}
		_, _ = w.Write([]byte(`<a class="chapter" href="/chapter/1">第一章</a>`))
	}))
	defer sourceServer.Close()
	server, store, closeDB := newCrawlAPIServer(t)
	defer closeDB()
	source := &booksource.BookSource{BookSourceURL: sourceServer.URL, BookSourceName: "fixture", RuleToc: `{"chapterList":".chapter","chapterName":"text","chapterUrl":"href"}`}
	if err := store.sourceStore.Upsert(source); err != nil {
		t.Fatal(err)
	}
	if err := store.bookStore.AddBook(&book.Book{ID: "book-1", Name: "Fixture", SourceID: source.ID, SourceURL: sourceServer.URL, BookURL: sourceServer.URL + "/book", TocURL: sourceServer.URL}); err != nil {
		t.Fatal(err)
	}
	invokeBookRoute(server.standalone.handleGetChapters, "book-1", "")
	waitForCatalogResponse(t, server, "book-1", http.StatusBadGateway)
	fail.Store(false)
	retry := invokeBookRoute(server.standalone.handleRetryChapters, "book-1", "")
	if retry.Code != http.StatusAccepted {
		t.Fatalf("retry status=%d body=%s", retry.Code, retry.Body.String())
	}
	waitForCatalogResponse(t, server, "book-1", http.StatusOK)
}

func TestHandleGetChapterContentExposesTypedPaginationFailure(t *testing.T) {
	sourceServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/chapter/1" {
			_, _ = w.Write([]byte(`<div class="content">正文</div><a class="next" href="/missing">下一页</a>`))
			return
		}
		http.Error(w, "upstream unavailable", http.StatusBadGateway)
	}))
	defer sourceServer.Close()

	server, store, closeDB := newCrawlAPIServer(t)
	defer closeDB()
	source := &booksource.BookSource{
		BookSourceURL: sourceServer.URL, BookSourceName: "fixture", RuleContent: `{"content":"@css:.content@text","nextContentUrl":"@css:.next@href"}`,
	}
	if err := store.sourceStore.Upsert(source); err != nil {
		t.Fatal(err)
	}
	if err := store.bookStore.AddBook(&book.Book{ID: "book-1", Name: "Fixture", SourceID: source.ID, SourceURL: sourceServer.URL, BookURL: sourceServer.URL + "/book", Origin: "fixture"}); err != nil {
		t.Fatal(err)
	}
	if err := store.bookStore.SaveChapters("book-1", []book.Chapter{{ID: "book-1_0", BookID: "book-1", Index: 0, Title: "第一章", URL: sourceServer.URL + "/chapter/1"}}); err != nil {
		t.Fatal(err)
	}

	response := invokeBookRoute(server.standalone.handleGetChapterContent, "book-1", "0")
	var payload crawlErrorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusBadGateway || payload.Code != "content_pagination_failed" || payload.Workflow != "content" || payload.PagesFetched != 1 || payload.FailedURL == "" {
		t.Fatalf("status=%d payload=%+v, want typed content failure", response.Code, payload)
	}
}

func TestHandlersDistinguishNotFoundFromStorageFailure(t *testing.T) {
	db, err := database.Open(t.TempDir() + "/books.db")
	if err != nil {
		t.Fatal(err)
	}
	bookStore := book.NewStore(db)
	initializeBookAPITestSchema(t, db)
	sourceStore := booksource.NewStore(db)
	searcher := book.NewSearcher(fetcher.NewInsecure(time.Second), analyzer.NewJSVM(), nil, sourceStore, bookStore)
	server := newReaderTestServer(&readerRuntime{bookStore: bookStore, sourceStore: sourceStore, searcher: searcher})
	server.standalone.catalogs = book.NewCatalogs(bookStore, sourceStore, searcher)
	defer server.standalone.catalogs.Close()

	missing := invokeBookRoute(server.standalone.handleGetChapters, "missing", "")
	if missing.Code != http.StatusAccepted {
		t.Fatalf("missing book start status=%d, want 202", missing.Code)
	}
	missing = waitForCatalogResponse(t, server, "missing", http.StatusNotFound)
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	storageFailure := invokeBookRoute(server.standalone.handleGetChapters, "other", "")
	if storageFailure.Code != http.StatusInternalServerError {
		t.Fatalf("storage failure status=%d, want 500; body=%s", storageFailure.Code, storageFailure.Body.String())
	}
}

type crawlStores struct {
	sourceStore *booksource.Store
	bookStore   *book.Store
}

func newCrawlAPIServer(t *testing.T) (*Server, crawlStores, func()) {
	t.Helper()
	db, err := database.Open(t.TempDir() + "/books.db")
	if err != nil {
		t.Fatal(err)
	}
	sourceStore := booksource.NewStore(db)
	bookStore := book.NewStore(db)
	initializeBookAndSourceAPITestSchema(t, db)
	searcher := book.NewSearcher(fetcher.NewInsecure(2*time.Second), analyzer.NewJSVM(), nil, sourceStore, bookStore)
	server := newReaderTestServer(&readerRuntime{sourceStore: sourceStore, bookStore: bookStore, searcher: searcher})
	server.standalone.catalogs = book.NewCatalogs(bookStore, sourceStore, searcher)
	return server, crawlStores{sourceStore, bookStore}, func() { server.standalone.catalogs.Close(); _ = db.Close() }
}

func waitForCatalogRoute(t *testing.T, server *Server, bookID string) *httptest.ResponseRecorder {
	t.Helper()
	return waitForCatalogResponse(t, server, bookID, http.StatusOK)
}

func waitForCatalogResponse(t *testing.T, server *Server, bookID string, status int) *httptest.ResponseRecorder {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	var response *httptest.ResponseRecorder
	for time.Now().Before(deadline) {
		response = invokeBookRoute(server.standalone.handleGetChapters, bookID, "")
		if response.Code == status {
			return response
		}
		if response.Code != http.StatusAccepted {
			t.Fatalf("catalog status=%d body=%s", response.Code, response.Body.String())
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("catalog did not return status %d; last=%d body=%s", status, response.Code, response.Body.String())
	return response
}

func invokeBookRoute(handler func(http.ResponseWriter, *http.Request), bookID, index string) *httptest.ResponseRecorder {
	path := "/api/books/" + bookID + "/chapters"
	if index != "" {
		path += "/" + index + "/content"
	}
	request := httptest.NewRequest(http.MethodGet, path, nil)
	request.SetPathValue("id", bookID)
	if index != "" {
		request.SetPathValue("idx", index)
	}
	response := httptest.NewRecorder()
	handler(response, request)
	return response
}
