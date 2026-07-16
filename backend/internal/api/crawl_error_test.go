// API boundary tests for typed crawl failures and resource classifications.
package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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
	if err := store.sourceStore.Upsert(&booksource.BookSource{
		BookSourceURL: sourceServer.URL, BookSourceName: "fixture", RuleToc: `{"chapterList":"@css:.chapter","chapterName":"@text","chapterUrl":"@href","nextTocUrl":"@css:.next@href"}`,
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.bookStore.AddBook(&book.Book{ID: "book-1", Name: "Fixture", SourceURL: sourceServer.URL, BookURL: sourceServer.URL + "/book", TocURL: sourceServer.URL + "/toc", Origin: "fixture"}); err != nil {
		t.Fatal(err)
	}

	response := invokeBookRoute(server.handleGetChapters, "book-1", "")
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
	if err := store.sourceStore.Upsert(&booksource.BookSource{
		BookSourceURL: sourceServer.URL, BookSourceName: "fixture", RuleContent: `{"content":"@css:.content@text","nextContentUrl":"@css:.next@href"}`,
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.bookStore.AddBook(&book.Book{ID: "book-1", Name: "Fixture", SourceURL: sourceServer.URL, BookURL: sourceServer.URL + "/book", Origin: "fixture"}); err != nil {
		t.Fatal(err)
	}
	if err := store.bookStore.SaveChapters("book-1", []book.Chapter{{ID: "book-1_0", BookID: "book-1", Index: 0, Title: "第一章", URL: sourceServer.URL + "/chapter/1"}}); err != nil {
		t.Fatal(err)
	}

	response := invokeBookRoute(server.handleGetChapterContent, "book-1", "0")
	var payload crawlErrorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusBadGateway || payload.Code != "content_pagination_failed" || payload.Workflow != "content" || payload.PagesFetched != 1 || payload.FailedURL == "" {
		t.Fatalf("status=%d payload=%+v, want typed content failure", response.Code, payload)
	}
}

func TestHandleEnrichBookExposesUpstreamFailure(t *testing.T) {
	sourceServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "upstream unavailable", http.StatusBadGateway)
	}))
	defer sourceServer.Close()

	server, store, closeDB := newCrawlAPIServer(t)
	defer closeDB()
	if err := store.sourceStore.Upsert(&booksource.BookSource{
		BookSourceURL: sourceServer.URL, BookSourceName: "fixture", RuleBookInfo: `{"name":"@css:.name@text"}`,
	}); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/books/enrich", strings.NewReader(`{"id":"book-1","name":"Fixture","sourceUrl":"`+sourceServer.URL+`","bookUrl":"/book"}`))
	response := httptest.NewRecorder()
	server.handleEnrichBook(response, request)
	var payload crawlErrorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusBadGateway || payload.Code != "book_info_failed" || payload.Workflow != "book_info" {
		t.Fatalf("status=%d payload=%+v, want upstream enrichment failure", response.Code, payload)
	}
	stored, err := store.bookStore.GetBook("book-1")
	if err != nil {
		t.Fatal(err)
	}
	if stored != nil {
		t.Fatalf("failed enrichment saved book: %+v", stored)
	}
}

func TestHandlersDistinguishNotFoundFromStorageFailure(t *testing.T) {
	db, err := database.Open(t.TempDir() + "/books.db")
	if err != nil {
		t.Fatal(err)
	}
	bookStore := book.NewStore(db)
	if err := bookStore.Init(); err != nil {
		t.Fatal(err)
	}
	server := &Server{bookStore: bookStore}

	missing := invokeBookRoute(server.handleGetChapters, "missing", "")
	if missing.Code != http.StatusNotFound {
		t.Fatalf("missing book status=%d, want 404", missing.Code)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	storageFailure := invokeBookRoute(server.handleGetChapters, "missing", "")
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
	if err := sourceStore.Init(); err != nil {
		t.Fatal(err)
	}
	if err := bookStore.Init(); err != nil {
		t.Fatal(err)
	}
	searcher := book.NewSearcher(fetcher.NewInsecure(2*time.Second), analyzer.NewJSVM(), nil, nil, bookStore)
	return &Server{sourceStore: sourceStore, bookStore: bookStore, searcher: searcher}, crawlStores{sourceStore, bookStore}, func() { _ = db.Close() }
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
