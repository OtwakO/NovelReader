// Explore capacity tests keep page work bounded and active sessions leased.
package book

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"sync/atomic"
	"testing"
	"time"

	"github.com/otwako/novelreader/internal/booksource"
	"github.com/otwako/novelreader/internal/fetcher"
	"github.com/otwako/novelreader/internal/sourceexec"
)

type blockingExploreTransport struct {
	started chan<- struct{}
	release <-chan struct{}
}

func (t blockingExploreTransport) Do(ctx context.Context, spec sourceexec.RequestSpec) (sourceexec.Response, error) {
	select {
	case t.started <- struct{}{}:
	case <-ctx.Done():
		return sourceexec.Response{}, ctx.Err()
	}
	select {
	case <-t.release:
		return sourceexec.Response{
			Body: `<a class="book" href="/book/1">Book</a>`, StatusCode: 200, FinalURL: spec.URL,
		}, nil
	case <-ctx.Done():
		return sourceexec.Response{}, ctx.Err()
	}
}

func (blockingExploreTransport) CloseIdleConnections() {}

func TestExplorePagesShareGlobalSourceCapacity(t *testing.T) {
	started := make(chan struct{}, 2)
	release := make(chan struct{})
	searcher, source := blockingExploreSearcher(2, 1, started, release)
	firstCatalog, err := searcher.OpenExplore(t.Context(), source.BookSourceURL)
	if err != nil {
		t.Fatal(err)
	}
	secondCatalog, err := searcher.OpenExplore(t.Context(), source.BookSourceURL)
	if err != nil {
		t.Fatal(err)
	}

	done := make(chan error, 2)
	go func() {
		_, err := searcher.GetExplorePage(t.Context(), ExplorePageRequest{SessionID: firstCatalog.SessionID, CategoryID: "entry-0", Page: 1})
		done <- err
	}()
	<-started
	go func() {
		_, err := searcher.GetExplorePage(t.Context(), ExplorePageRequest{SessionID: secondCatalog.SessionID, CategoryID: "entry-0", Page: 1})
		done <- err
	}()
	select {
	case <-started:
		t.Fatal("second Explore request bypassed global capacity")
	case <-time.After(50 * time.Millisecond):
	}
	close(release)
	<-started
	for range 2 {
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	}
}

func TestExploreActiveSessionCannotBeEvicted(t *testing.T) {
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	searcher, source := blockingExploreSearcher(1, 1, started, release)
	catalog, err := searcher.OpenExplore(t.Context(), source.BookSourceURL)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := searcher.GetExplorePage(t.Context(), ExplorePageRequest{SessionID: catalog.SessionID, CategoryID: "entry-0", Page: 1})
		done <- err
	}()
	<-started
	if _, err := searcher.OpenExplore(t.Context(), source.BookSourceURL); err == nil {
		t.Fatal("expected session capacity while the only session is active")
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if _, err := searcher.GetExplorePage(t.Context(), ExplorePageRequest{SessionID: catalog.SessionID, CategoryID: "entry-0", Page: 1}); err != nil {
		t.Fatalf("active session was evicted: %v", err)
	}
}

func TestExploreRetainedBookCapacityReturnsBoundedPageAndWarning(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.URL.Query().Get("page") == "1" {
			if _, err := fmt.Fprint(w, `<a class="book" href="/book/prior">Prior</a>`); err != nil {
				t.Error(err)
			}
			return
		}
		for index := range maxExploreRetainedBooks + 1 {
			if _, err := fmt.Fprintf(w, `<a class="book" href="/book/%d">Book %d</a>`, index, index); err != nil {
				t.Error(err)
				return
			}
		}
	}))
	defer server.Close()

	source := booksource.BookSource{
		BookSourceURL: server.URL, BookSourceName: "Capacity", EnabledExplore: true,
		ExploreURL: "Books::" + server.URL + "/books?page={{page}}", BookURLPattern: `/book/`,
		RuleExplore: `{"bookList":".book","name":"text","bookUrl":"href"}`,
	}
	searcher := NewSearcher(nil, nil, nil, exploreSourceFixtureStore{source: source}, nil)
	catalog, err := searcher.OpenExplore(t.Context(), source.BookSourceURL)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = searcher.GetExplorePage(t.Context(), ExplorePageRequest{SessionID: catalog.SessionID, CategoryID: "entry-0", Page: 1}); err != nil {
		t.Fatal(err)
	}
	page, err := searcher.GetExplorePage(t.Context(), ExplorePageRequest{SessionID: catalog.SessionID, CategoryID: "entry-0", Page: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Books) != maxExploreRetainedBooks-1 || page.Books[0].BookURL != server.URL+"/book/0" || page.Books[len(page.Books)-1].BookURL != server.URL+"/book/1998" {
		t.Fatalf("books=%d first=%q last=%q", len(page.Books), page.Books[0].BookURL, page.Books[len(page.Books)-1].BookURL)
	}
	if !page.Exhausted || len(page.Diagnostics) != 1 {
		t.Fatalf("page exhausted=%v diagnostics=%+v", page.Exhausted, page.Diagnostics)
	}
	diagnostic := page.Diagnostics[0]
	if diagnostic.Code != "result_truncated" || diagnostic.Stage != "capacity" || diagnostic.Severity != "warning" || diagnostic.Retryable {
		t.Fatalf("diagnostic=%+v", diagnostic)
	}
	replayed, err := searcher.GetExplorePage(t.Context(), ExplorePageRequest{SessionID: catalog.SessionID, CategoryID: "entry-0", Page: 2})
	if err != nil || len(replayed.Books) != len(page.Books) || requests.Load() != 2 {
		t.Fatalf("replayed books=%d err=%v requests=%d", len(replayed.Books), err, requests.Load())
	}
}

func TestConcurrentSourceRateLimitsReserveSpacedRequests(t *testing.T) {
	searcher := NewSearcher(nil, nil, nil, nil, nil)
	source := booksource.BookSource{BookSourceURL: "https://spaced.test", ConcurrentRate: "30"}
	start := make(chan struct{})
	completed := make(chan time.Time, 3)
	for range 3 {
		go func() {
			<-start
			if err := searcher.rateLimitWait(t.Context(), source); err != nil {
				t.Error(err)
			}
			completed <- time.Now()
		}()
	}
	close(start)
	times := []time.Time{<-completed, <-completed, <-completed}
	sort.Slice(times, func(i, j int) bool { return times[i].Before(times[j]) })
	for index := 1; index < len(times); index++ {
		if spacing := times[index].Sub(times[index-1]); spacing < 20*time.Millisecond {
			t.Fatalf("requests %d and %d spaced by %v", index, index+1, spacing)
		}
	}
}

func TestSourceRateLimitWaitHonorsCancellation(t *testing.T) {
	searcher := NewSearcher(nil, nil, nil, nil, nil)
	source := booksource.BookSource{BookSourceURL: "https://rate.test", ConcurrentRate: "10000"}
	if err := searcher.rateLimitWait(t.Context(), source); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := searcher.rateLimitWait(ctx, source); !errors.Is(err, context.Canceled) {
		t.Fatalf("error=%v, want context cancellation", err)
	}
}

func blockingExploreSearcher(maxSessions, global int, started chan<- struct{}, release <-chan struct{}) (*Searcher, booksource.BookSource) {
	source := booksource.BookSource{
		BookSourceURL: "https://fixture.test", BookSourceName: "Capacity", EnabledExplore: true,
		ExploreURL: "Books::https://fixture.test/books?page={{page}}", BookURLPattern: `/book/`,
		RuleExplore: `{"bookList":".book","name":"text","bookUrl":"href"}`,
	}
	searcher := NewSearcherWithLimits(nil, nil, nil, exploreSourceFixtureStore{source: source}, nil, SearcherLimits{
		ConcurrentPerSearch: 1, ConcurrentGlobal: global, MaxSessions: maxSessions,
		SessionTTL: time.Minute, WorkflowTimeout: time.Second,
	})
	searcher.SetTransportFactory(func(_ *fetcher.Client, _ *sourceexec.SourceSession) sourceexec.Transport {
		return blockingExploreTransport{started: started, release: release}
	})
	return searcher, source
}
