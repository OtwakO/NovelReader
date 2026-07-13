// Integration test for search using the unified source executor and session state.
package book

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/otwako/novelreader/internal/analyzer"
	"github.com/otwako/novelreader/internal/booksource"
)

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
		RuleSearch:     `{"bookList":".book","name":".name@text","author":".author@text","bookUrl":".name@href"}`,
	}

	results, err := s.searchSource(t.Context(), src, "凡人修仙传")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Name != "凡人修仙传" {
		t.Fatalf("results = %+v, want one fixture result", results)
	}
	if results[0].BookURL != server.URL+"/search/book/1" {
		t.Fatalf("book URL = %q, want final-page-relative URL", results[0].BookURL)
	}
}
