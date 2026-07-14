// Deterministic capacity coverage for concurrent source fan-out.
package book

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/otwako/novelreader/internal/analyzer"
	"github.com/otwako/novelreader/internal/booksource"
	"github.com/otwako/novelreader/internal/fetcher"
)

type loadSourceStore struct{ sources []booksource.BookSource }

func (s loadSourceStore) ListEnabled() ([]booksource.BookSource, error) { return s.sources, nil }

func TestSearchStreamBoundsConcurrentHundredsSourceRequests(t *testing.T) {
	var active atomic.Int64
	var peak atomic.Int64
	var searchActive [2]atomic.Int64
	var searchPeak [2]atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		index := 0
		if r.URL.Query().Get("query") == "fixture-b" {
			index = 1
		}
		current := active.Add(1)
		currentSearch := searchActive[index].Add(1)
		updatePeak(&peak, current)
		updatePeak(&searchPeak[index], currentSearch)
		defer active.Add(-1)
		defer searchActive[index].Add(-1)
		time.Sleep(2 * time.Millisecond)
		_, _ = fmt.Fprint(w, `<div class="book"><a class="name" href="/book">fixture</a></div>`)
	}))
	defer server.Close()

	sources := make([]booksource.BookSource, 200)
	for i := range sources {
		sources[i] = booksource.BookSource{
			BookSourceURL: server.URL, BookSourceName: "fixture-" + strconv.Itoa(i),
			SearchURL:  server.URL + "/search?query={{key}}&source=" + strconv.Itoa(i),
			RuleSearch: `{"bookList":".book","name":".name@text","bookUrl":".name@href"}`,
		}
	}
	limits := DefaultSearcherLimits()
	limits.ConcurrentPerSearch = 5
	limits.ConcurrentGlobal = 7
	searcher := NewSearcherWithLimits(
		fetcher.NewInsecureStateless(3*time.Second), analyzer.NewJSVM(), nil,
		loadSourceStore{sources: sources}, nil, limits,
	)

	var wg sync.WaitGroup
	errors := make(chan error, 2)
	for _, query := range []string{"fixture-a", "fixture-b"} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errors <- searcher.SearchStream(context.Background(), query, func(booksource.BookSource, []SearchResult, error) {})
		}()
	}
	wg.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}

	stats := searcher.CapacityStats()
	if stats.ActiveSearches != 0 || stats.ActiveSourceFetches != 0 {
		t.Fatalf("active work leaked: %+v", stats)
	}
	if stats.TotalSourceFetches != 400 || stats.CompletedSources != 400 || stats.FailedSources != 0 {
		t.Fatalf("unexpected capacity stats: %+v", stats)
	}
	if peak.Load() > int64(limits.ConcurrentGlobal) {
		t.Fatalf("peak source concurrency=%d, want <= %d", peak.Load(), limits.ConcurrentGlobal)
	}
	for index := range searchPeak {
		if searchPeak[index].Load() > int64(limits.ConcurrentPerSearch) {
			t.Fatalf("search %d peak=%d, want <= %d", index, searchPeak[index].Load(), limits.ConcurrentPerSearch)
		}
	}
}

func updatePeak(peak *atomic.Int64, current int64) {
	for {
		old := peak.Load()
		if current <= old || peak.CompareAndSwap(old, current) {
			return
		}
	}
}
