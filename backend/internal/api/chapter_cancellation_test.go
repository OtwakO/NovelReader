package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/otwako/novelreader/internal/book"
)

func TestChapterRequestCancellationStopsUpstream(t *testing.T) {
	started, stopped := make(chan struct{}), make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-r.Context().Done()
		close(stopped)
	}))
	defer upstream.Close()
	server, cleanup := newWorkflowAPIServer(t)
	defer cleanup()
	raw, _ := json.Marshal([]map[string]any{{"bookSourceUrl": upstream.URL, "bookSourceName": "cancellation fixture", "enabled": true, "ruleContent": map[string]string{"content": "article@text"}}})
	if r := performAPIRequest(server, http.MethodPost, "/api/sources", raw); r.Code != http.StatusOK {
		t.Fatal(r.Code)
	}
	sources, err := server.standalone.sourceStore.ListEnabled()
	if err != nil || len(sources) != 1 {
		t.Fatal("source setup", err)
	}
	if err := server.standalone.bookStore.AddBook(&book.Book{ID: "book", Name: "Book", SourceID: sources[0].ID, SourceURL: upstream.URL, BookURL: upstream.URL}); err != nil {
		t.Fatal(err)
	}
	if err := server.standalone.bookStore.SaveChapters("book", []book.Chapter{{Index: 0, Title: "Chapter", URL: upstream.URL + "/chapter"}}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		server.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/books/book/chapters/0/content", nil).WithContext(ctx))
	}()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("chapter fetch did not start")
	}
	cancel()
	select {
	case <-stopped:
	case <-time.After(3 * time.Second):
		t.Fatal("upstream request was not canceled")
	}
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("chapter handler did not return")
	}
}
