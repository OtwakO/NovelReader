// Raw-source API workflow coverage from import through first/middle/last reading.
package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/otwako/novelreader/internal/analyzer"
	"github.com/otwako/novelreader/internal/book"
	"github.com/otwako/novelreader/internal/booksource"
	"github.com/otwako/novelreader/internal/database"
	"github.com/otwako/novelreader/internal/fetcher"
	"github.com/otwako/novelreader/internal/processor"
)

func TestRawSourceAPIWorkflowReadsFirstMiddleLastChapters(t *testing.T) {
	sourceServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/search":
			_, _ = fmt.Fprint(w, `<div class="book"><a class="name" href="/book">Fixture Novel</a><span class="author">Fixture Author</span></div>`)
		case "/book":
			_, _ = fmt.Fprint(w, `<h1 class="name">Fixture Novel</h1><span class="author">Fixture Author</span><a class="toc" href="/toc">目录</a>`)
		case "/toc":
			for i := 1; i <= 5; i++ {
				_, _ = fmt.Fprintf(w, `<a class="chapter" href="/chapter/%d">Chapter %d</a>`, i, i)
			}
		case "/chapter/1", "/chapter/2", "/chapter/3", "/chapter/4", "/chapter/5":
			_, _ = fmt.Fprintf(w, `<article class="content">content %s</article>`, r.URL.Path[len("/chapter/"):])
		default:
			http.NotFound(w, r)
		}
	}))
	defer sourceServer.Close()

	server, closeDB := newWorkflowAPIServer(t)
	defer closeDB()
	rawSource, err := json.Marshal([]map[string]interface{}{{
		"bookSourceUrl": sourceServer.URL, "bookSourceName": "raw workflow fixture", "bookSourceType": 0, "enabled": true,
		"searchUrl":    sourceServer.URL + "/search?q={{key}}",
		"ruleSearch":   map[string]string{"bookList": ".book", "name": ".name@text", "author": ".author@text", "bookUrl": ".name@href"},
		"ruleBookInfo": map[string]string{"name": ".name@text", "author": ".author@text", "tocUrl": ".toc@href"},
		"ruleToc":      map[string]string{"chapterList": ".chapter", "chapterName": "text", "chapterUrl": "href"},
		"ruleContent":  map[string]string{"content": ".content@text"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	response := performAPIRequest(server, http.MethodPost, "/api/sources", rawSource)
	if response.Code != http.StatusOK {
		t.Fatalf("import status=%d body=%s", response.Code, response.Body.String())
	}

	response = performAPIRequest(server, http.MethodGet, "/api/search?q=Fixture", nil)
	var results []book.SearchResult
	if err := json.Unmarshal(response.Body.Bytes(), &results); err != nil || response.Code != http.StatusOK || len(results) != 1 {
		t.Fatalf("search status=%d results=%+v err=%v body=%s", response.Code, results, err, response.Body.String())
	}
	enrichRequest, _ := json.Marshal(map[string]interface{}{
		"id": "book-1", "name": results[0].Name, "author": results[0].Author,
		"sourceUrl": results[0].SourceURL, "bookUrl": results[0].BookURL,
	})
	response = performAPIRequest(server, http.MethodPost, "/api/books/enrich", enrichRequest)
	if response.Code != http.StatusOK {
		t.Fatalf("enrich status=%d body=%s", response.Code, response.Body.String())
	}

	response = performAPIRequest(server, http.MethodGet, "/api/books/book-1/chapters", nil)
	var chapters []book.Chapter
	if err := json.Unmarshal(response.Body.Bytes(), &chapters); err != nil || response.Code != http.StatusOK || len(chapters) != 5 {
		t.Fatalf("toc status=%d chapters=%d err=%v body=%s", response.Code, len(chapters), err, response.Body.String())
	}
	for _, index := range []int{0, 2, 4} {
		response = performAPIRequest(server, http.MethodGet, fmt.Sprintf("/api/books/book-1/chapters/%d/content", index), nil)
		var content processor.ProcessResult
		if err := json.Unmarshal(response.Body.Bytes(), &content); err != nil || response.Code != http.StatusOK || len(content.Paragraphs) == 0 || content.Paragraphs[0] != fmt.Sprintf("content %d", index+1) {
			t.Fatalf("chapter %d status=%d content=%+v err=%v body=%s", index, response.Code, content, err, response.Body.String())
		}
	}
}

func newWorkflowAPIServer(t *testing.T) (*Server, func()) {
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
	client := fetcher.NewInsecure(2 * time.Second)
	jsVM := analyzer.NewJSVM()
	jsVM.SetFetcher(client)
	searcher := book.NewSearcher(client, jsVM, nil, sourceStore, bookStore)
	server := NewServer(sourceStore, bookStore, searcher, nil, client, jsVM, nil, processor.Config{}, t.TempDir(), db)
	return server, func() { _ = db.Close() }
}

func performAPIRequest(server *Server, method, path string, body []byte) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, bytes.NewReader(body))
	if len(body) > 0 {
		request.Header.Set("Content-Type", "application/json")
	}
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	return response
}
