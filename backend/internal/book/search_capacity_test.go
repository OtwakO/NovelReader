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
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		current := active.Add(1)
		for {
			old := peak.Load()
			if current <= old || peak.CompareAndSwap(old, current) {
				break
			}
		}
		defer active.Add(-1)
		time.Sleep(2 * time.Millisecond)
		_, _ = fmt.Fprint(w, `<div class="book"><a class="name" href="/book">fixture</a></div>`)
	}))
	defer server.Close()

	sources := make([]booksource.BookSource, 200)
	for i := range sources {
		sources[i] = booksource.BookSource{
			BookSourceURL: server.URL, BookSourceName: "fixture-" + strconv.Itoa(i),
			SearchURL:  server.URL + "/search?source=" + strconv.Itoa(i),
			RuleSearch: `{"bookList":".book","name":".name@text","bookUrl":".name@href"}`,
		}
	}
	searcher := NewSearcher(
		fetcher.NewInsecureStateless(3*time.Second), analyzer.NewJSVM(), nil,
		loadSourceStore{sources: sources}, nil,
	)

	var wg sync.WaitGroup
	errors := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errors <- searcher.SearchStream(context.Background(), "fixture", func(booksource.BookSource, []SearchResult, error) {})
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
	if peak.Load() > maxConcurrentGlobalSearch {
		t.Fatalf("peak source concurrency=%d, want <= %d", peak.Load(), maxConcurrentGlobalSearch)
	}
}
