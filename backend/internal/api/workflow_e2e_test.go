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
	"github.com/otwako/novelreader/internal/candidate"
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
			index := r.URL.Path[len("/chapter/"):]
			_, _ = fmt.Fprintf(w, `<article class="content">content %s begins here with enough meaningful narrative prose to verify this source as readable while preserving deterministic first, middle, and last chapter checks.</article>`, index)
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

	searchResponse := performAPIRequest(server, http.MethodGet, "/api/search/stream?q=Fixture&batchSize=10&concurrency=2", nil)
	events := decodeSSE(t, searchResponse)
	var results []book.SearchResult
	for _, event := range events {
		if event["type"] != "results" {
			continue
		}
		payload, _ := json.Marshal(event["data"])
		if err := json.Unmarshal(payload, &results); err != nil {
			t.Fatal(err)
		}
		break
	}
	if len(results) != 1 {
		t.Fatalf("search results=%+v events=%+v", results, events)
	}
	started := performAPIRequest(server, http.MethodPost, "/api/candidate-resolutions", candidateRequestBody(results[0].SourceURL, nil))
	var operation candidate.Snapshot
	if err := json.Unmarshal(started.Body.Bytes(), &operation); err != nil || started.Code != http.StatusAccepted {
		t.Fatalf("start status=%d operation=%+v err=%v body=%s", started.Code, operation, err, started.Body.String())
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		current := performAPIRequest(server, http.MethodGet, "/api/candidate-resolutions/"+operation.ID, nil)
		if err := json.Unmarshal(current.Body.Bytes(), &operation); err != nil {
			t.Fatal(err)
		}
		if operation.State == candidate.StateVerified {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if operation.State != candidate.StateVerified {
		t.Fatalf("operation=%+v", operation)
	}
	commitBody, _ := json.Marshal(map[string]string{"bookId": "book-1"})
	committed := performAPIRequest(server, http.MethodPost, "/api/candidate-resolutions/"+operation.ID+"/shelve", commitBody)
	if committed.Code != http.StatusCreated {
		t.Fatalf("commit status=%d body=%s", committed.Code, committed.Body.String())
	}

	response = waitForCatalogRoute(t, server, "book-1")
	var chapters []book.Chapter
	if err := json.Unmarshal(response.Body.Bytes(), &chapters); err != nil || response.Code != http.StatusOK || len(chapters) != 5 {
		t.Fatalf("toc status=%d chapters=%d err=%v body=%s", response.Code, len(chapters), err, response.Body.String())
	}
	for _, index := range []int{0, 2, 4} {
		response = performAPIRequest(server, http.MethodGet, fmt.Sprintf("/api/books/book-1/chapters/%d/content", index), nil)
		var content processor.ProcessResult
		if err := json.Unmarshal(response.Body.Bytes(), &content); err != nil || response.Code != http.StatusOK || len(content.Paragraphs) == 0 || content.Paragraphs[0] != fmt.Sprintf("content %d begins here with enough meaningful narrative prose to verify this source as readable while preserving deterministic first, middle, and last chapter checks.", index+1) {
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
	initializeBookAndSourceAPITestSchema(t, db)
	client := fetcher.NewInsecure(2 * time.Second)
	jsVM := analyzer.NewJSVM()
	jsVM.SetFetcher(client)
	searcher := book.NewSearcher(client, jsVM, nil, sourceStore, bookStore)
	server := NewServer(sourceStore, bookStore, searcher, nil, client, jsVM, nil, processor.Config{}, t.TempDir(), db)
	return server, func() { _ = server.Close(); _ = db.Close() }
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
