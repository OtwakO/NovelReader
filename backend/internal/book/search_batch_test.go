// Deterministic coverage for stateless search batches and cursors.
package book

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/otwako/novelreader/internal/analyzer"
	"github.com/otwako/novelreader/internal/booksource"
	"github.com/otwako/novelreader/internal/fetcher"
)

func TestPrepareSearchBatchPartitionsDeterministically(t *testing.T) {
	store := &mutableSourceStore{sources: []booksource.BookSource{
		batchSource("https://c.test", "Same", 2),
		batchSource("https://b.test", "Same", 1),
		batchSource("https://a.test", "Same", 1),
		{BookSourceURL: "https://ignored.test", BookSourceName: "Ignored", CustomOrder: 0},
	}}
	searcher := NewSearcher(nil, nil, nil, store, nil)

	first, err := searcher.PrepareSearchBatch(SearchBatchOptions{Limit: 2, Concurrency: 8})
	if err != nil {
		t.Fatal(err)
	}
	assertBatchSources(t, first, "https://a.test", "https://b.test")
	if first.Offset != 0 || first.Eligible != 3 || first.SourcesInBatch != 2 || first.NextCursor == "" {
		t.Fatalf("unexpected first batch metadata: %+v", first)
	}
	if first.RequestedConcurrency != 8 || first.EffectiveConcurrency != 8 {
		t.Fatalf("unexpected concurrency metadata: %+v", first)
	}

	second, err := searcher.PrepareSearchBatch(SearchBatchOptions{Limit: 2, Concurrency: 999, Cursor: first.NextCursor})
	if err != nil {
		t.Fatal(err)
	}
	assertBatchSources(t, second, "https://c.test")
	if second.Offset != 2 || second.SourcesInBatch != 1 || second.NextCursor != "" || second.HasMore {
		t.Fatalf("unexpected final batch metadata: %+v", second)
	}
	if second.EffectiveConcurrency != searcher.concurrentPerSearch {
		t.Fatalf("effective concurrency=%d, want deployment cap %d", second.EffectiveConcurrency, searcher.concurrentPerSearch)
	}
}

func TestPrepareSearchBatchRejectsStaleCursorBeforeExecution(t *testing.T) {
	store := &mutableSourceStore{sources: []booksource.BookSource{
		batchSource("https://a.test", "A", 1),
		batchSource("https://b.test", "B", 2),
	}}
	searcher := NewSearcher(nil, nil, nil, store, nil)
	first, err := searcher.PrepareSearchBatch(SearchBatchOptions{Limit: 1})
	if err != nil {
		t.Fatal(err)
	}

	store.sources = append(store.sources, batchSource("https://c.test", "C", 3))
	_, err = searcher.PrepareSearchBatch(SearchBatchOptions{Limit: 1, Cursor: first.NextCursor})
	if !errors.Is(err, ErrStaleSearchCursor) {
		t.Fatalf("error=%v, want ErrStaleSearchCursor", err)
	}
}

func TestPrepareSearchBatchValidatesInput(t *testing.T) {
	searcher := NewSearcher(nil, nil, nil, &mutableSourceStore{}, nil)
	for _, options := range []SearchBatchOptions{
		{Limit: 0},
		{Limit: MaxSearchBatchSize + 1},
		{Limit: 1, Concurrency: -1},
		{Limit: 1, Cursor: "not-a-cursor"},
	} {
		if _, err := searcher.PrepareSearchBatch(options); err == nil {
			t.Fatalf("PrepareSearchBatch(%+v) succeeded", options)
		}
	}
}

func TestSearchBatchHonorsRequestedConcurrency(t *testing.T) {
	var active atomic.Int64
	var peak atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		current := active.Add(1)
		updatePeak(&peak, current)
		defer active.Add(-1)
		time.Sleep(5 * time.Millisecond)
		_, _ = fmt.Fprint(w, `<div class="book"><a class="name" href="/book">fixture</a></div>`)
	}))
	defer server.Close()

	sources := make([]booksource.BookSource, 6)
	for i := range sources {
		sources[i] = batchSource(server.URL+"/"+strconv.Itoa(i), strconv.Itoa(i), i)
		sources[i].SearchURL = server.URL + "/search?q={{key}}&source=" + strconv.Itoa(i)
		sources[i].RuleSearch = `{"bookList":".book","name":".name@text","bookUrl":".name@href"}`
	}
	limits := DefaultSearcherLimits()
	limits.ConcurrentPerSearch = 5
	searcher := NewSearcherWithLimits(fetcher.NewInsecureStateless(time.Second), analyzer.NewJSVM(), nil, &mutableSourceStore{sources: sources}, nil, limits)
	plan, err := searcher.PrepareSearchBatch(SearchBatchOptions{Limit: 6, Concurrency: 2})
	if err != nil {
		t.Fatal(err)
	}
	completed := 0
	if err := searcher.SearchBatch(context.Background(), "fixture", plan, func(booksource.BookSource, []SearchResult, error) { completed++ }); err != nil {
		t.Fatal(err)
	}
	if completed != 6 {
		t.Fatalf("completed=%d, want 6", completed)
	}
	if peak.Load() > 2 {
		t.Fatalf("peak concurrency=%d, want <= 2", peak.Load())
	}
}

type mutableSourceStore struct{ sources []booksource.BookSource }

func (s *mutableSourceStore) ListEnabled() ([]booksource.BookSource, error) {
	return append([]booksource.BookSource(nil), s.sources...), nil
}

func batchSource(url, name string, order int) booksource.BookSource {
	return booksource.BookSource{
		BookSourceURL:  url,
		BookSourceName: name,
		CustomOrder:    order,
		SearchURL:      url + "/search?q={{key}}",
		RuleSearch:     `{"bookList":".book"}`,
	}
}

func assertBatchSources(t *testing.T, plan SearchBatchPlan, want ...string) {
	t.Helper()
	if len(plan.sources) != len(want) {
		t.Fatalf("batch sources=%d, want %d", len(plan.sources), len(want))
	}
	for i, source := range plan.sources {
		if source.BookSourceURL != want[i] {
			t.Fatalf("source[%d]=%q, want %q", i, source.BookSourceURL, want[i])
		}
	}
}
