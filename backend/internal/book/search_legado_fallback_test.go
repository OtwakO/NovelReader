package book

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"sync/atomic"
	"testing"
	"time"

	"github.com/otwako/novelreader/internal/analyzer"
	"github.com/otwako/novelreader/internal/booksource"
	"github.com/otwako/novelreader/internal/fetcher"
)

func TestParseSearchResultDefaultBookURLUsesFirstMatch(t *testing.T) {
	source := booksource.BookSource{
		BookSourceURL:  "https://example.test",
		BookSourceName: "Default URL result",
		RuleSearch:     `{"bookList":"class.result","name":"tag.h2@text","bookUrl":"a@href"}`,
	}
	html := `<div class="result"><h2>Book</h2><a href="/book/1">Detail</a><a href="/chapter/1">Chapter</a></div>`
	searcher := NewSearcher(nil, analyzer.NewJSVM(), analyzer.NewCacheManager(), nil, nil)

	results, err := searcher.ParseSearchResultWithStateAtURL(source, html, "https://example.test/search", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("results = %d, want 1", len(results))
	}
	if results[0].BookURL != "https://example.test/book/1" {
		t.Fatalf("book URL = %q, want first Default/JSoup match", results[0].BookURL)
	}
}

func TestParseSearchResultDefaultsEmptyBookURLToResponseURL(t *testing.T) {
	source := booksource.BookSource{
		BookSourceURL:  "https://example.test",
		BookSourceName: "Detail-shaped search result",
		RuleSearch:     `{"bookList":".book-info","name":"h1@text","bookUrl":".missing@href"}`,
	}
	searcher := NewSearcher(nil, analyzer.NewJSVM(), analyzer.NewCacheManager(), nil, nil)

	results, err := searcher.ParseSearchResultWithStateAtURL(source, `<div class="book-info"><h1>Detail Book</h1></div>`, "https://example.test/book/1", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Name != "Detail Book" || results[0].BookURL != "https://example.test/book/1" {
		t.Fatalf("results = %+v", results)
	}
}

func TestDiscoveryDetailRouting(t *testing.T) {
	for _, test := range []struct {
		name, pattern, list           string
		search, wantDetail, wantError bool
	}{
		{"search pattern bypasses list", `https://example\.test/(?=book/)book/1`, `@js:throw new Error('list must not run')`, true, true, false},
		{"empty search list", "", ".missing", true, true, false},
		{"empty Explore list", "", ".missing", false, true, false},
		{"Explore does not force pattern", `https://example\.test/book/1`, ".missing", false, false, false},
		{"pattern requires whole URL", `book/1`, ".missing", true, false, true},
		{"nameless items are not an empty list", "", "h1", true, false, false},
		{"broken list is not empty", "", `@js:throw new Error('broken')`, true, false, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			source := booksource.BookSource{ID: "source", BookSourceURL: "https://example.test", BookSourceName: "Fixture", BookURLPattern: test.pattern,
				RuleBookInfo: `{"name":"h1@text","author":"@get:{request}","intro":"@js:java.put('detail', 'saved')","tocUrl":".toc@href"}`}
			rules, _ := json.Marshal(map[string]string{"bookList": test.list, "name": ".name@text"})
			s := NewSearcher(nil, analyzer.NewJSVM(), nil, nil, nil)
			results, err := s.parseDiscoveryResults(t.Context(), source, `<h1>Detail</h1><a class="toc" href="toc">TOC</a>`, string(rules), "https://example.test/book/1", nil,
				discoveryParseOptions{search: test.search, allowEmpty: !test.search, variables: map[string]string{"request": "original"}})
			if (err != nil) != test.wantError {
				t.Fatalf("results=%+v err=%v", results, err)
			}
			if test.wantDetail {
				if len(results) != 1 || results[0].Name != "Detail" || results[0].Author != "original" || results[0].BookURL != "https://example.test/book/1" || results[0].SourceID != source.ID || results[0].VariableMap != `{"detail":"saved","request":"original"}` {
					t.Fatalf("lost detail identity/context: %+v", results)
				}
			} else if len(results) != 0 {
				t.Fatalf("unexpected detail result: %+v", results)
			}
		})
	}
}

func TestSearchRedirectUsesDetailRulesWithoutRefetch(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.URL.Path == "/search" {
			http.Redirect(w, r, "/book/1", http.StatusFound)
			return
		}
		_, _ = w.Write([]byte(`<h1>Detail</h1>`))
	}))
	defer server.Close()
	s := NewSearcher(fetcher.NewInsecureStateless(time.Second), analyzer.NewJSVM(), nil, nil, nil)
	for _, pattern := range []string{"", regexp.QuoteMeta(server.URL) + `/book/1`} {
		source := booksource.BookSource{ID: "source", BookSourceURL: server.URL, BookURLPattern: pattern,
			SearchURL:  `@js:java.put('request', key); source.bookSourceUrl + '/search'`,
			RuleSearch: `{"bookList":".missing"}`, RuleBookInfo: `{"name":"h1@text","author":"@get:{request}"}`}
		before := requests.Load()
		results, err := s.searchSource(t.Context(), source, "original")
		if err != nil || len(results) != 1 {
			t.Fatalf("results=%+v err=%v", results, err)
		}
		if results[0].BookURL != server.URL+"/book/1" || results[0].Author != "original" || requests.Load()-before != 2 {
			t.Fatalf("redirect response/context was not reused: results=%+v requests=%d", results, requests.Load()-before)
		}
	}
}
