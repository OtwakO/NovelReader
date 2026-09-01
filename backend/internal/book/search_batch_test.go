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
	"github.com/otwako/novelreader/internal/sourceexec"
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

func TestSearchBatchStreamsBeforeWholeBatchFinishes(t *testing.T) {
	var started atomic.Int64
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started.Add(1)
		if r.URL.Query().Get("source") != "0" {
			<-release
		}
		_, _ = fmt.Fprint(w, `<div class="book"><a class="name" href="/book">fixture</a></div>`)
	}))
	defer server.Close()

	sources := make([]booksource.BookSource, 4)
	for i := range sources {
		sources[i] = batchSource(server.URL+"/"+strconv.Itoa(i), strconv.Itoa(i), i)
		sources[i].SearchURL = server.URL + "/search?q={{key}}&source=" + strconv.Itoa(i)
		sources[i].RuleSearch = `{"bookList":".book","name":".name@text","bookUrl":".name@href"}`
	}
	searcher := NewSearcher(fetcher.NewInsecureStateless(time.Second), analyzer.NewJSVM(), nil, &mutableSourceStore{sources: sources}, nil)
	plan, err := searcher.PrepareSearchBatch(SearchBatchOptions{Limit: 4, Concurrency: 2})
	if err != nil {
		t.Fatal(err)
	}
	callbacks := make(chan struct{}, 4)
	done := make(chan error, 1)
	go func() {
		done <- searcher.SearchBatch(context.Background(), "fixture", plan, func(booksource.BookSource, []SearchResult, error) {
			callbacks <- struct{}{}
		})
	}()

	select {
	case <-callbacks:
		if started.Load() >= 4 {
			t.Fatalf("first callback arrived only after all %d sources started", started.Load())
		}
	case <-time.After(time.Second):
		t.Fatal("first result was not streamed while later sources were blocked")
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestSearchBatchExpandsCurrentSourceWithItsDefaultState(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer fixture" {
			http.Error(w, "missing authentication", http.StatusUnauthorized)
			return
		}
		provider := r.URL.Query().Get("provider")
		if provider == "configured" {
			_, _ = fmt.Fprint(w, `<a class="book" href="/configured">Fixture</a>`)
			return
		}
		_, _ = fmt.Fprint(w, `<a class="book" href="/configured">Fixture</a><a class="book" href="/alternate">Fixture</a>`)
	}))
	defer server.Close()

	source := batchSource(server.URL, "Aggregate", 0)
	source.ID = "aggregate"
	source.SearchURL = `@js:baseUrl + '/search?provider=' + (source.getVariable() || 'all')`
	source.RuleSearch = `{"bookList":".book","name":"text","bookUrl":"href"}`
	searcher := NewSearcher(fetcher.NewInsecureStateless(time.Second), analyzer.NewJSVM(), nil, &mutableSourceStore{sources: []booksource.BookSource{source}}, nil)
	searcher.SetSourceSessionHydrator(func(_ context.Context, source booksource.BookSource, session *sourceexec.SourceSession) error {
		session.PutVariable(source.BookSourceURL, "configured")
		session.SetLoginHeader(`{"Authorization":"Bearer fixture"}`)
		return nil
	})
	searcher.SetSourceAuthenticationHydrator(func(_ context.Context, _ booksource.BookSource, session *sourceexec.SourceSession) error {
		session.SetLoginHeader(`{"Authorization":"Bearer fixture"}`)
		return nil
	})
	plan, err := searcher.PrepareSearchBatch(SearchBatchOptions{Limit: 1, ExpandSourceID: source.ID})
	if err != nil {
		t.Fatal(err)
	}
	var results []SearchResult
	if err := searcher.SearchBatch(t.Context(), "Fixture", plan, func(_ booksource.BookSource, found []SearchResult, searchErr error) {
		if searchErr != nil {
			t.Fatal(searchErr)
		}
		results = found
	}); err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 || results[0].BookURL == results[1].BookURL {
		t.Fatalf("results=%+v, want configured and default-state bindings", results)
	}
}

func TestSearchBatchUsesDefaultStateWhenConfiguredCurrentSourceFails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("provider") == "configured" {
			http.Error(w, "configured provider unavailable", http.StatusBadGateway)
			return
		}
		_, _ = fmt.Fprint(w, `<a class="book" href="/alternate">Fixture</a>`)
	}))
	defer server.Close()
	source := batchSource(server.URL, "Aggregate", 0)
	source.ID = "aggregate"
	source.SearchURL = `@js:baseUrl + '/search?provider=' + (source.getVariable() || 'all')`
	source.RuleSearch = `{"bookList":".book","name":"text","bookUrl":"href"}`
	searcher := NewSearcher(fetcher.NewInsecureStateless(time.Second), analyzer.NewJSVM(), nil, &mutableSourceStore{sources: []booksource.BookSource{source}}, nil)
	searcher.SetSourceSessionHydrator(func(_ context.Context, source booksource.BookSource, session *sourceexec.SourceSession) error {
		session.PutVariable(source.BookSourceURL, "configured")
		return nil
	})
	plan, err := searcher.PrepareSearchBatch(SearchBatchOptions{Limit: 1, ExpandSourceID: source.ID})
	if err != nil {
		t.Fatal(err)
	}
	if err := searcher.SearchBatch(t.Context(), "Fixture", plan, func(_ booksource.BookSource, results []SearchResult, searchErr error) {
		if searchErr != nil || len(results) != 1 || results[0].BookURL != server.URL+"/alternate" {
			t.Fatalf("results=%+v error=%v", results, searchErr)
		}
	}); err != nil {
		t.Fatal(err)
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
