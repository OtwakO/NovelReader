// Explore service tests cover source eligibility, paging, and idempotent replay.
package book

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/otwako/novelreader/internal/booksource"
)

type exploreSourceFixtureStore struct {
	source booksource.BookSource
}

func (s exploreSourceFixtureStore) ListEnabled() ([]booksource.BookSource, error) { return nil, nil }
func (s exploreSourceFixtureStore) GetExploreEnabledByID(id string) (*booksource.BookSource, error) {
	sourceID := s.source.ID
	if sourceID == "" {
		sourceID = s.source.BookSourceURL
	}
	if id != sourceID {
		return nil, nil
	}
	source := s.source
	if source.ID == "" {
		source.ID = source.BookSourceURL
	}
	return &source, nil
}

func (s exploreSourceFixtureStore) ListExploreEnabled() ([]booksource.BookSource, error) {
	source := s.source
	if source.ID == "" {
		source.ID = source.BookSourceURL
	}
	return []booksource.BookSource{source}, nil
}

func TestExploreServicePagesOneSourceSequentiallyAndReplaysLastSuccess(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.URL.Query().Get("page") == "" || r.Header.Get("X-Source") != "fixture" {
			t.Errorf("request query=%q headers=%v", r.URL.RawQuery, r.Header)
		}
		_, _ = fmt.Fprint(w, `<div class="book"><a class="name" href="/book/1">第一本</a><span class="author">作者</span></div>`)
	}))
	defer server.Close()

	source := booksource.BookSource{
		ID:              server.URL,
		BookSourceURL:   server.URL,
		BookSourceName:  "Explore fixture",
		BookSourceGroup: "Fixtures",
		Enabled:         false,
		EnabledExplore:  true,
		ExploreURL:      "分类::" + server.URL + "/books?page={{page}}",
		Header:          `{"X-Source":"fixture"}`,
		RuleExplore:     `{"bookList":".book","name":".name@text","author":".author@text","bookUrl":".name@href"}`,
	}
	searcher := NewSearcher(nil, nil, nil, exploreSourceFixtureStore{source: source}, nil)

	sources, err := searcher.ExploreSources()
	if err != nil {
		t.Fatal(err)
	}
	if len(sources) != 1 || sources[0].ID != source.BookSourceURL || len(sources[0].Capabilities) != 2 || sources[0].Capabilities[0] != "explore" || sources[0].Capabilities[1] != "headers" {
		t.Fatalf("sources=%+v", sources)
	}
	catalog, err := searcher.OpenExplore(t.Context(), source.BookSourceURL)
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Entries) != 1 || catalog.Entries[0].ID != "entry-0" || !catalog.Entries[0].Selectable || catalog.Entries[0].Title != "分类" {
		t.Fatalf("catalog=%+v", catalog)
	}

	first, err := searcher.GetExplorePage(t.Context(), ExplorePageRequest{SessionID: catalog.SessionID, CategoryID: "entry-0", Page: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Books) != 1 || first.Books[0].BookURL != server.URL+"/book/1" || first.NextPage != 2 || first.Exhausted {
		t.Fatalf("first=%+v", first)
	}
	replayed, err := searcher.GetExplorePage(t.Context(), ExplorePageRequest{SessionID: catalog.SessionID, CategoryID: "entry-0", Page: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(replayed.Books) != 1 || requests.Load() != 1 {
		t.Fatalf("replayed=%+v requests=%d", replayed, requests.Load())
	}

	_, err = searcher.GetExplorePage(t.Context(), ExplorePageRequest{SessionID: catalog.SessionID, CategoryID: "entry-0", Page: 3})
	exploreErr, ok := err.(*ExploreError)
	if !ok || exploreErr.Code != "page_conflict" || exploreErr.ExpectedPage != 2 {
		t.Fatalf("skip error=%T %v", err, err)
	}
	second, err := searcher.GetExplorePage(t.Context(), ExplorePageRequest{SessionID: catalog.SessionID, CategoryID: "entry-0", Page: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Books) != 0 || !second.Exhausted || requests.Load() != 2 {
		t.Fatalf("second=%+v requests=%d", second, requests.Load())
	}
	_, err = searcher.GetExplorePage(t.Context(), ExplorePageRequest{SessionID: catalog.SessionID, CategoryID: "entry-0", Page: 3})
	if exploreErr, ok := err.(*ExploreError); !ok || exploreErr.Code != "page_exhausted" || requests.Load() != 2 {
		t.Fatalf("exhausted error=%T %v requests=%d", err, err, requests.Load())
	}
}

func TestOpenExploreRejectsDisabledOrMissingSources(t *testing.T) {
	source := booksource.BookSource{BookSourceURL: "https://disabled.test", EnabledExplore: false, ExploreURL: "分类::/books"}
	searcher := NewSearcher(nil, nil, nil, exploreSourceFixtureStore{source: source}, nil)
	for _, id := range []string{source.BookSourceURL, "https://missing.test"} {
		if _, err := searcher.OpenExplore(t.Context(), id); err == nil {
			t.Fatalf("expected %q to be rejected", id)
		}
	}
}
