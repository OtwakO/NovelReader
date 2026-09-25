// Chapter-cache API tests cover freshness, explicit Refresh and late-result rejection.
package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/otwako/novelreader/internal/book"
	"github.com/otwako/novelreader/internal/reading"
)

func TestChapterContentCacheFirstRefreshAndExpiry(t *testing.T) {
	var mode, calls atomic.Int32
	started, release := make(chan struct{}), make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		switch mode.Load() {
		case 1:
			http.Error(w, "offline", http.StatusServiceUnavailable)
		case 3:
			close(started)
			select {
			case <-release:
			case <-r.Context().Done():
				return
			}
			_, _ = fmt.Fprint(w, `<article class="content">late content</article>`)
		case 2:
			_, _ = fmt.Fprint(w, `<article class="content"></article>`)
		default:
			_, _ = fmt.Fprint(w, `<article class="content"><p>cached paragraph</p><img src="/chapter.png"></article>`)
		}
	}))
	defer upstream.Close()
	server, closeDB := newWorkflowAPIServer(t)
	defer closeDB()
	raw, _ := json.Marshal([]map[string]interface{}{{
		"bookSourceUrl": upstream.URL, "bookSourceName": "cache fixture", "bookSourceType": 0, "enabled": true,
		"ruleContent": map[string]string{"content": ".content@html"},
	}})
	if response := performAPIRequest(server, http.MethodPost, "/api/sources", raw); response.Code != http.StatusOK {
		t.Fatalf("import status=%d body=%s", response.Code, response.Body.String())
	}
	sources, err := server.standalone.sourceStore.ListEnabled()
	if err != nil || len(sources) != 1 {
		t.Fatalf("sources=%+v err=%v", sources, err)
	}
	if err := server.standalone.bookStore.AddBook(&book.Book{ID: "book", Name: "Book", SourceID: sources[0].ID, SourceURL: upstream.URL, BookURL: upstream.URL}); err != nil {
		t.Fatal(err)
	}
	chapter := book.Chapter{Index: 0, Title: "Chapter", URL: upstream.URL + "/chapter"}
	if err := server.standalone.bookStore.SaveChapters("book", []book.Chapter{chapter}); err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		query  string
		status int
	}{{"", 400}, {"?contentRevision=-1", 400}, {"?contentRevision=0", 409}, {"?contentRevision=1&refresh=garbage", 400}} {
		response := performAPIRequest(server, http.MethodGet, "/api/books/book/chapters/0/content"+test.query, nil)
		if response.Code != test.status {
			t.Fatalf("query=%q status=%d body=%s", test.query, response.Code, response.Body.String())
		}
	}
	if calls.Load() != 0 {
		t.Fatal("invalid or stale revision reached upstream")
	}
	response := performAPIRequest(server, http.MethodGet, "/api/books/book/chapters/0/content?contentRevision=1", nil)
	var fresh reading.Content
	if err := json.Unmarshal(response.Body.Bytes(), &fresh); err != nil || response.Code != http.StatusOK || fresh.OfflineCopy || fresh.ContentRevision != 1 || fresh.Version != reading.DocumentVersion || fresh.Document.Kind != "prose" || len(fresh.Document.Blocks) != 2 || fresh.Document.Blocks[1].Resource == nil {
		t.Fatalf("fresh status=%d result=%+v err=%v body=%s", response.Code, fresh, err, response.Body.String())
	}
	for _, index := range []string{"00", "1"} {
		response := performAPIRequest(server, http.MethodGet, "/api/books/book/chapters/"+index+"/content?contentRevision=1", nil)
		if response.Code != http.StatusNotFound {
			t.Fatalf("non-exact chapter %q: status=%d", index, response.Code)
		}
	}
	original, err := server.standalone.bookStore.GetChapterCache("book", sources[0].ID, 0, chapter.URL, 1)
	if err != nil || original == nil {
		t.Fatalf("original=%v err=%v", original, err)
	}
	var cached reading.Content
	for _, upstreamMode := range []int32{2, 1} {
		mode.Store(upstreamMode)
		response = performAPIRequest(server, http.MethodGet, "/api/books/book/chapters/0/content?contentRevision=1", nil)
		if err := json.Unmarshal(response.Body.Bytes(), &cached); err != nil || response.Code != http.StatusOK || cached.OfflineCopy || cached.Version != fresh.Version || cached.Document.Title != fresh.Document.Title || len(cached.Document.Blocks) != len(fresh.Document.Blocks) || cached.Document.Blocks[1].Resource == nil || cached.Document.Blocks[1].Resource.Href != fresh.Document.Blocks[1].Resource.Href {
			t.Fatalf("mode=%d cached status=%d result=%+v err=%v body=%s", upstreamMode, response.Code, cached, err, response.Body.String())
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("fresh cache reached upstream: %d calls", calls.Load())
	}
	saved, err := server.standalone.bookStore.GetChapterCache("book", sources[0].ID, 0, chapter.URL, 1)
	if err != nil || saved == nil || saved.CachedAt != original.CachedAt {
		t.Fatalf("saved=%v err=%v", saved, err)
	}
	// Refresh must report upstream failure rather than return even a fresh copy.
	if response := performAPIRequest(server, http.MethodGet, "/api/books/book/chapters/0/content?contentRevision=1&refresh=true", nil); response.Code != http.StatusBadGateway {
		t.Fatalf("refresh: %s", response.Body.String())
	}
	saved.CachedAt = time.Now().Add(-25 * time.Hour).UnixNano()
	if err := server.standalone.bookStore.SaveChapterCache(*saved); err != nil {
		t.Fatal(err)
	}
	if response := performAPIRequest(server, http.MethodGet, "/api/books/book/chapters/0/content?contentRevision=1", nil); response.Code != http.StatusBadGateway {
		t.Fatalf("expired fallback: %s", response.Body.String())
	}
	// Renew successfully, without advancing the interpretation revision.
	mode.Store(0)
	response = performAPIRequest(server, http.MethodGet, "/api/books/book/chapters/0/content?contentRevision=1&refresh=true", nil)
	if response.Code != http.StatusOK {
		t.Fatal(response.Body.String())
	}
	renewed, err := server.standalone.bookStore.GetChapterCache("book", sources[0].ID, 0, chapter.URL, 1)
	if err != nil || renewed == nil || renewed.CachedAt <= saved.CachedAt {
		t.Fatalf("renewal=%v err=%v", renewed, err)
	}
	// A changed source definition must miss even a fresh copy.
	sources[0].RuleContent = `{"content":".content@html","title":"title@text"}`
	if err := server.standalone.sourceStore.Upsert(&sources[0]); err != nil {
		t.Fatal(err)
	}
	mode.Store(3)
	completed := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		completed <- performAPIRequest(server, http.MethodGet, "/api/books/book/chapters/0/content?contentRevision=1", nil)
	}()
	<-started
	err = server.standalone.bookStore.SaveChapters("book", []book.Chapter{chapter})
	close(release)
	if err != nil {
		t.Fatal(err)
	}
	if late := <-completed; late.Code != http.StatusConflict {
		t.Fatalf("late content status=%d body=%s", late.Code, late.Body.String())
	}
	mode.Store(1)
	chapter.URL = upstream.URL + "/changed"
	if err := server.standalone.bookStore.SaveChapters("book", []book.Chapter{chapter}); err != nil {
		t.Fatal(err)
	}
	if response := performAPIRequest(server, http.MethodGet, "/api/books/book/chapters/0/content?contentRevision=3", nil); response.Code != http.StatusBadGateway {
		t.Fatalf("changed URL unexpectedly used cache: %s", response.Body.String())
	}
}
