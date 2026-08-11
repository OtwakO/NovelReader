// Integration test for search using the unified source executor and session state.
package book

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/otwako/novelreader/internal/analyzer"
	"github.com/otwako/novelreader/internal/booksource"
	"github.com/otwako/novelreader/internal/fetcher"
)

func TestSearchSourceEvaluatesJavaScriptHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Source") != "search" {
			http.Error(w, "missing source header", http.StatusForbidden)
			return
		}
		_, _ = w.Write([]byte(`<a class="book" href="/book/1">Book</a>`))
	}))
	defer server.Close()
	s := NewSearcher(fetcher.NewInsecure(3*time.Second), analyzer.NewJSVM(), nil, nil, nil)
	src := booksource.BookSource{
		BookSourceURL: server.URL, SearchURL: server.URL + "/search", Enabled: true,
		Header:     `@js:JSON.stringify({'X-Source':'search'})`,
		RuleSearch: `{"bookList":".book","name":"text","bookUrl":"href"}`,
	}
	results, err := s.searchSource(t.Context(), src, "fixture")
	if err != nil || len(results) != 1 {
		t.Fatalf("results=%+v err=%v", results, err)
	}
}

func TestParseSearchResultPreservesLenientBookURLOptions(t *testing.T) {
	searcher := NewSearcher(nil, analyzer.NewJSVM(), nil, nil, nil)
	source := booksource.BookSource{
		BookSourceURL: "https://example.test",
		RuleSearch:    `{"bookList":".book","name":".name@text","bookUrl":".name@href##$##,{Cookie:\"xmanhua_lang=2\"}"}`,
	}
	results, err := searcher.ParseSearchResultWithStateAtURL(source,
		`<div class="book"><a class="name" href="/40192xm/">凡人雷神</a></div>`,
		"https://example.test/search", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].BookURL != `https://example.test/40192xm/,{Cookie:"xmanhua_lang=2"}` {
		t.Fatalf("results=%+v", results)
	}
}

func TestSearchSourceUsesSessionBackedURLTemplate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/search" {
			http.Redirect(w, r, "/search/?"+r.URL.RawQuery, http.StatusFound)
			return
		}
		if r.URL.Query().Get("token") != "fixture" {
			_, _ = w.Write([]byte("<html><body>missing token</body></html>"))
			return
		}
		_, _ = w.Write([]byte(`<div class="book"><a class="name" href="book/1">凡人修仙传</a><span class="author">忘语</span></div>`))
	}))
	defer server.Close()

	s := NewSearcher(nil, analyzer.NewJSVM(), nil, nil, nil)
	src := booksource.BookSource{
		BookSourceURL:  server.URL,
		BookSourceName: "fixture",
		SearchURL:      "{{cookie.setCookie(baseUrl, 'token=fixture')}}/search?token={{cookie.getKey(baseUrl, 'token')}}",
		RuleSearch:     `{"bookList":".book","name":"@js:cookie.getKey(baseUrl, 'token')","author":".author@text","bookUrl":".name@href"}`,
	}

	results, err := s.searchSource(t.Context(), src, "凡人修仙传")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Name != "fixture" {
		t.Fatalf("results = %+v, want session-backed field rule", results)
	}
	if results[0].BookURL != server.URL+"/search/book/1" {
		t.Fatalf("book URL = %q, want final-page-relative URL", results[0].BookURL)
	}
}
