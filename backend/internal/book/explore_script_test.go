// Explore script tests execute generated categories inside source-scoped state.
package book

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/otwako/novelreader/internal/analyzer"
	"github.com/otwako/novelreader/internal/booksource"
	"github.com/otwako/novelreader/internal/fetcher"
)

func TestOpenExploreExecutesPinnedJavaScriptControlCatalog(t *testing.T) {
	source := pinnedExploreSource(t, 916)
	searcher := NewSearcher(nil, analyzer.NewJSVM(), nil, exploreSourceFixtureStore{source: source}, nil)

	catalog, err := searcher.OpenExplore(t.Context(), source.BookSourceURL)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range catalog.Entries {
		if entry.Type == "select" && entry.Title == "选择榜单" && len(entry.Options) == 4 && entry.Value == "完结榜" {
			return
		}
	}
	t.Fatalf("generated catalog omitted pinned select control: %+v", catalog.Entries)
}

func TestOpenExploreJavaScriptUsesSourceHeadersAndSessionCache(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Source") != "fixture" {
			http.Error(w, "missing source header", http.StatusForbidden)
			return
		}
		_, _ = fmt.Fprint(w, `分类::/books`)
	}))
	defer server.Close()
	vm := analyzer.NewJSVM()
	vm.SetFetcher(fetcher.NewWithTimeout(2 * time.Second))
	source := booksource.BookSource{
		BookSourceURL: server.URL, BookSourceName: "Script", EnabledExplore: true,
		Header:     `{"X-Source":"fixture"}`,
		ExploreURL: `@js:cache.put('visited','yes'); if (cache.get('visited') !== 'yes') throw new Error('cache'); java.ajax('` + server.URL + `')`,
	}
	searcher := NewSearcher(nil, vm, nil, exploreSourceFixtureStore{source: source}, nil)

	catalog, err := searcher.OpenExplore(t.Context(), source.BookSourceURL)
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Entries) != 1 || catalog.Entries[0].Title != "分类" || !catalog.Entries[0].Selectable {
		t.Fatalf("catalog=%+v", catalog)
	}
}

func TestOpenExploreJavaScriptConvertsTraditionalChinese(t *testing.T) {
	source := booksource.BookSource{
		BookSourceURL: "https://t2s.test", EnabledExplore: true,
		ExploreURL: `@js:JSON.stringify([{title:java.t2s('熱門小說'),url:'/books'}])`,
	}
	searcher := NewSearcher(nil, analyzer.NewJSVM(), nil, exploreSourceFixtureStore{source: source}, nil)

	catalog, err := searcher.OpenExplore(t.Context(), source.BookSourceURL)
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Entries) != 1 || catalog.Entries[0].Title != "热门小说" {
		t.Fatalf("catalog=%+v", catalog)
	}
}

func TestOpenExploreJavaScriptLeavesSimplifiedChineseUnchanged(t *testing.T) {
	source := booksource.BookSource{
		BookSourceURL: "https://t2s-identity.test", EnabledExplore: true,
		ExploreURL: `@js:JSON.stringify([{title:java.t2s('热门小说'),url:'/books'}])`,
	}
	searcher := NewSearcher(nil, analyzer.NewJSVM(), nil, exploreSourceFixtureStore{source: source}, nil)

	catalog, err := searcher.OpenExplore(t.Context(), source.BookSourceURL)
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Entries) != 1 || catalog.Entries[0].Title != "热门小说" {
		t.Fatalf("catalog=%+v", catalog)
	}
}

func TestOpenExploreExecutesTaggedJavaScript(t *testing.T) {
	source := booksource.BookSource{
		BookSourceURL: "https://tagged.test", EnabledExplore: true,
		ExploreURL: `<js>JSON.stringify([{title:'Tagged',url:'/books'}])</js>`,
	}
	searcher := NewSearcher(nil, analyzer.NewJSVM(), nil, exploreSourceFixtureStore{source: source}, nil)
	catalog, err := searcher.OpenExplore(t.Context(), source.BookSourceURL)
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Entries) != 1 || catalog.Entries[0].Title != "Tagged" {
		t.Fatalf("catalog=%+v", catalog)
	}
}

func TestOpenExploreJavaScriptFailureIsTyped(t *testing.T) {
	source := booksource.BookSource{
		BookSourceURL: "https://script.test", EnabledExplore: true,
		ExploreURL: `@js:throw new Error('private rule detail')`,
	}
	searcher := NewSearcher(nil, analyzer.NewJSVM(), nil, exploreSourceFixtureStore{source: source}, nil)
	_, err := searcher.OpenExplore(t.Context(), source.BookSourceURL)
	exploreErr, ok := err.(*ExploreError)
	if !ok || exploreErr.Code != "category_script_failed" || exploreErr.Message == "" {
		t.Fatalf("error=%T %v", err, err)
	}
}

func TestOpenExploreGeneratedMalformedCategoriesAreParseFailure(t *testing.T) {
	source := booksource.BookSource{
		BookSourceURL: "https://malformed.test", EnabledExplore: true,
		ExploreURL: `@js:'['`,
	}
	searcher := NewSearcher(nil, analyzer.NewJSVM(), nil, exploreSourceFixtureStore{source: source}, nil)
	_, err := searcher.OpenExplore(t.Context(), source.BookSourceURL)
	if exploreErr, ok := err.(*ExploreError); !ok || exploreErr.Code != "category_parse_failed" {
		t.Fatalf("error=%T %v", err, err)
	}
}
