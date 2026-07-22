// Explore compatibility tests cover uncapped reversal and single-book pages.
package book

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/otwako/novelreader/internal/analyzer"
	"github.com/otwako/novelreader/internal/booksource"
)

func TestExplorePageReversesTheCompleteUncappedResultList(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		for index := range 25 {
			_, _ = fmt.Fprintf(w, `<a class="book" href="/book/%d">Book %02d</a>`, index, index)
		}
	}))
	defer server.Close()
	source := booksource.BookSource{
		BookSourceURL: server.URL, BookSourceName: "Reverse", EnabledExplore: true,
		ExploreURL: "Books::" + server.URL, BookURLPattern: `/book/`,
		RuleExplore: `{"bookList":"-.book","name":"text","bookUrl":"href"}`,
	}
	searcher := NewSearcher(nil, nil, nil, exploreSourceFixtureStore{source: source}, nil)
	catalog, err := searcher.OpenExplore(t.Context(), source.BookSourceURL)
	if err != nil {
		t.Fatal(err)
	}
	page, err := searcher.GetExplorePage(t.Context(), ExplorePageRequest{SessionID: catalog.SessionID, CategoryID: "entry-0", Page: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Books) != 25 || page.Books[0].Name != "Book 24" || page.Books[24].Name != "Book 00" {
		t.Fatalf("books=%d first=%q last=%q", len(page.Books), page.Books[0].Name, page.Books[len(page.Books)-1].Name)
	}
}

func TestExplorePageJavaScriptCanReadSourceMetadata(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, `<a class="book" href="/book/1">Book</a>`)
	}))
	defer server.Close()
	source := booksource.BookSource{
		BookSourceURL: server.URL, BookSourceName: "Metadata", BookSourceComment: ".book", EnabledExplore: true,
		ExploreURL:  "Books::" + server.URL,
		RuleExplore: `{"bookList":"@js:java.getElements(source.bookSourceComment)","name":"text","bookUrl":"href"}`,
	}
	searcher := NewSearcher(nil, analyzer.NewJSVM(), nil, exploreSourceFixtureStore{source: source}, nil)
	catalog, err := searcher.OpenExplore(t.Context(), source.BookSourceURL)
	if err != nil {
		t.Fatal(err)
	}
	page, err := searcher.GetExplorePage(t.Context(), ExplorePageRequest{SessionID: catalog.SessionID, CategoryID: "entry-0", Page: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Books) != 1 || page.Books[0].Name != "Book" {
		t.Fatalf("page=%+v", page)
	}
}

func TestExplorePageUsesLenientSourceHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Source") != "explore" {
			http.Error(w, "missing source header", http.StatusForbidden)
			return
		}
		_, _ = fmt.Fprint(w, `<a class="book" href="/book/1">Book</a>`)
	}))
	defer server.Close()
	source := booksource.BookSource{
		BookSourceURL: server.URL, BookSourceName: "Lenient headers", EnabledExplore: true,
		Header: `{'X-Source':'explore'}`, ExploreURL: "Books::" + server.URL,
		RuleExplore: `{"bookList":".book","name":"text","bookUrl":"href"}`,
	}
	searcher := NewSearcher(nil, nil, nil, exploreSourceFixtureStore{source: source}, nil)
	catalog, err := searcher.OpenExplore(t.Context(), source.BookSourceURL)
	if err != nil {
		t.Fatal(err)
	}
	page, err := searcher.GetExplorePage(t.Context(), ExplorePageRequest{SessionID: catalog.SessionID, CategoryID: "entry-0", Page: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Books) != 1 || page.Books[0].Name != "Book" {
		t.Fatalf("page=%+v", page)
	}
}

func TestExplorePageAllowsOptionalFieldRuleFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, `{"data":[{"title":"Book","id":1,"optional":null}]}`)
	}))
	defer server.Close()
	source := booksource.BookSource{
		BookSourceURL: server.URL, BookSourceName: "Optional field", EnabledExplore: true,
		ExploreURL:  "Books::" + server.URL,
		RuleExplore: `{"bookList":"$.data","name":"$.title","bookUrl":"<js>'/book/'+java.getString('$.id')</js>","lastChapter":"$.optional.value"}`,
	}
	searcher := NewSearcher(nil, analyzer.NewJSVM(), nil, exploreSourceFixtureStore{source: source}, nil)
	catalog, err := searcher.OpenExplore(t.Context(), source.BookSourceURL)
	if err != nil {
		t.Fatal(err)
	}
	page, err := searcher.GetExplorePage(t.Context(), ExplorePageRequest{SessionID: catalog.SessionID, CategoryID: "entry-0", Page: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Books) != 1 || page.Books[0].Name != "Book" || page.Books[0].LastChapter != "" {
		t.Fatalf("page=%+v", page)
	}
}

func TestExplorePageSurfacesOptionalFieldScriptFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, `<a class="book" href="/book/1">Book</a>`)
	}))
	defer server.Close()
	source := booksource.BookSource{
		BookSourceURL: server.URL, BookSourceName: "Broken optional field", EnabledExplore: true,
		ExploreURL:  "Books::" + server.URL,
		RuleExplore: `{"bookList":".book","name":"text","bookUrl":"href","lastChapter":"@js:throw new Error('broken optional')"}`,
	}
	searcher := NewSearcher(nil, analyzer.NewJSVM(), nil, exploreSourceFixtureStore{source: source}, nil)
	catalog, err := searcher.OpenExplore(t.Context(), source.BookSourceURL)
	if err != nil {
		t.Fatal(err)
	}
	_, err = searcher.GetExplorePage(t.Context(), ExplorePageRequest{SessionID: catalog.SessionID, CategoryID: "entry-0", Page: 1})
	if exploreErr, ok := err.(*ExploreError); !ok || exploreErr.Code != "result_rule_failed" {
		t.Fatalf("error=%T %v", err, err)
	}
}

func TestExplorePageSurfacesRequiredFieldRuleFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, `<a class="book" href="/book/1">Book</a>`)
	}))
	defer server.Close()
	source := booksource.BookSource{
		BookSourceURL: server.URL, BookSourceName: "Broken field", EnabledExplore: true,
		ExploreURL: "Books::" + server.URL, RuleExplore: `{"bookList":".book","name":"@js:throw new Error('broken')","bookUrl":"href"}`,
	}
	searcher := NewSearcher(nil, analyzer.NewJSVM(), nil, exploreSourceFixtureStore{source: source}, nil)
	catalog, err := searcher.OpenExplore(t.Context(), source.BookSourceURL)
	if err != nil {
		t.Fatal(err)
	}
	_, err = searcher.GetExplorePage(t.Context(), ExplorePageRequest{SessionID: catalog.SessionID, CategoryID: "entry-0", Page: 1})
	if exploreErr, ok := err.(*ExploreError); !ok || exploreErr.Code != "result_rule_failed" {
		t.Fatalf("error=%T %v", err, err)
	}
}

func TestExplorePageFallsBackToBookDetailWhenListIsEmpty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, `<h1 class="name">Detail Book</h1><span class="author">Detail Author</span>`)
	}))
	defer server.Close()
	source := booksource.BookSource{
		BookSourceURL: server.URL, BookSourceName: "Detail", EnabledExplore: true,
		ExploreURL:   "Detail::" + server.URL + "/detail",
		RuleExplore:  `{"bookList":".missing","name":".name@text","bookUrl":"@href"}`,
		RuleBookInfo: `{"name":".name@text","author":".author@text"}`,
	}
	searcher := NewSearcher(nil, nil, nil, exploreSourceFixtureStore{source: source}, nil)
	catalog, err := searcher.OpenExplore(t.Context(), source.BookSourceURL)
	if err != nil {
		t.Fatal(err)
	}
	page, err := searcher.GetExplorePage(t.Context(), ExplorePageRequest{SessionID: catalog.SessionID, CategoryID: "entry-0", Page: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Books) != 1 || page.Books[0].Name != "Detail Book" || page.Books[0].Author != "Detail Author" || !strings.HasSuffix(page.Books[0].BookURL, "/detail") {
		t.Fatalf("page=%+v", page)
	}
}
