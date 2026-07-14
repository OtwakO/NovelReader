// API contract coverage for legacy and batched SSE search.
package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/otwako/novelreader/internal/analyzer"
	"github.com/otwako/novelreader/internal/book"
	"github.com/otwako/novelreader/internal/booksource"
	"github.com/otwako/novelreader/internal/fetcher"
)

func TestHandleSearchStreamEmitsBatchProgressAndContinuation(t *testing.T) {
	apiServer, store, calls, closeSource := newSearchAPIFixture(t, 2)
	defer closeSource()

	response := performSearch(t, apiServer, "/api/search/stream?q=fixture&batchSize=1&concurrency=2")
	events := decodeSSE(t, response)
	assertEventTypes(t, events, "start", "results", "done")
	if events[0]["eligible"] != float64(2) || events[0]["sourcesInBatch"] != float64(1) {
		t.Fatalf("unexpected start event: %#v", events[0])
	}
	if events[1]["checked"] != float64(1) || events[1]["eligible"] != float64(2) {
		t.Fatalf("unexpected progress event: %#v", events[1])
	}
	nextCursor, _ := events[2]["nextCursor"].(string)
	if nextCursor == "" || events[2]["hasMore"] != true {
		t.Fatalf("unexpected done event: %#v", events[2])
	}

	store.sources = append(store.sources, apiSource("https://added.test", "Added", 3))
	before := calls.Load()
	response = performSearch(t, apiServer, "/api/search/stream?q=fixture&batchSize=1&concurrency=2&cursor="+url.QueryEscape(nextCursor))
	events = decodeSSE(t, response)
	assertEventTypes(t, events, "stale")
	if calls.Load() != before {
		t.Fatalf("stale continuation executed source work: calls %d -> %d", before, calls.Load())
	}
}

func TestHandleSearchStreamValidatesBatchParameters(t *testing.T) {
	apiServer, _, _, closeSource := newSearchAPIFixture(t, 1)
	defer closeSource()
	for _, path := range []string{
		"/api/search/stream?q=fixture&batchSize=0",
		"/api/search/stream?q=fixture&batchSize=501",
		"/api/search/stream?q=fixture&batchSize=1&concurrency=-1",
		"/api/search/stream?q=fixture&batchSize=1&cursor=bad",
	} {
		response := performSearch(t, apiServer, path)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("%s status=%d, want 400; body=%s", path, response.Code, response.Body.String())
		}
	}
}

func TestHandleSearchStreamKeepsLegacyEventContract(t *testing.T) {
	apiServer, _, _, closeSource := newSearchAPIFixture(t, 1)
	defer closeSource()
	events := decodeSSE(t, performSearch(t, apiServer, "/api/search/stream?q=fixture"))
	assertEventTypes(t, events, "results", "done")
	if _, ok := events[len(events)-1]["merged"]; !ok {
		t.Fatalf("legacy done event lost merged results: %#v", events[len(events)-1])
	}
}

type apiSourceStore struct{ sources []booksource.BookSource }

func (s *apiSourceStore) ListEnabled() ([]booksource.BookSource, error) {
	return append([]booksource.BookSource(nil), s.sources...), nil
}

func newSearchAPIFixture(t *testing.T, count int) (*Server, *apiSourceStore, *atomic.Int64, func()) {
	t.Helper()
	calls := &atomic.Int64{}
	sourceServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		_, _ = fmt.Fprint(w, `<div class="book"><a class="name" href="/book">fixture</a></div>`)
	}))
	store := &apiSourceStore{}
	for i := 0; i < count; i++ {
		source := apiSource(sourceServer.URL+"/"+fmt.Sprint(i), fmt.Sprintf("Source %d", i), i)
		source.SearchURL = sourceServer.URL + "/search?q={{key}}&source=" + fmt.Sprint(i)
		store.sources = append(store.sources, source)
	}
	searcher := book.NewSearcher(fetcher.NewInsecureStateless(time.Second), analyzer.NewJSVM(), nil, store, nil)
	return &Server{searcher: searcher}, store, calls, sourceServer.Close
}

func apiSource(sourceURL, name string, order int) booksource.BookSource {
	return booksource.BookSource{
		BookSourceURL:  sourceURL,
		BookSourceName: name,
		CustomOrder:    order,
		SearchURL:      sourceURL + "/search?q={{key}}",
		RuleSearch:     `{"bookList":".book","name":".name@text","bookUrl":".name@href"}`,
	}
}

func performSearch(t *testing.T, server *Server, path string) *httptest.ResponseRecorder {
	t.Helper()
	response := httptest.NewRecorder()
	server.handleSearchStream(response, httptest.NewRequest(http.MethodGet, path, nil))
	return response
}

func decodeSSE(t *testing.T, response *httptest.ResponseRecorder) []map[string]any {
	t.Helper()
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200; body=%s", response.Code, response.Body.String())
	}
	var events []map[string]any
	for _, block := range strings.Split(strings.TrimSpace(response.Body.String()), "\n\n") {
		payload := strings.TrimPrefix(block, "data: ")
		var event map[string]any
		if err := json.Unmarshal([]byte(payload), &event); err != nil {
			t.Fatalf("decode event %q: %v", block, err)
		}
		events = append(events, event)
	}
	return events
}

func assertEventTypes(t *testing.T, events []map[string]any, want ...string) {
	t.Helper()
	if len(events) != len(want) {
		t.Fatalf("event count=%d, want %d: %#v", len(events), len(want), events)
	}
	for i, eventType := range want {
		if events[i]["type"] != eventType {
			t.Fatalf("event[%d] type=%v, want %q", i, events[i]["type"], eventType)
		}
	}
}
