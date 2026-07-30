// Explore compatibility tests cover uncapped reversal and single-book pages.
package book

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/otwako/novelreader/internal/analyzer"
	"github.com/otwako/novelreader/internal/booksource"
)

func TestExplorePageUsesConfiguredSourceTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(40 * time.Millisecond)
		_, _ = fmt.Fprint(w, `<a class="book" href="/book/1">Book</a>`)
	}))
	defer server.Close()
	source := booksource.BookSource{
		BookSourceURL: server.URL, BookSourceName: "Slow Explore", EnabledExplore: true,
		ExploreURL: "Books::" + server.URL, RuleExplore: `{"bookList":".book","name":"text","bookUrl":"href"}`,
	}
	limits := DefaultSearcherLimits()
	limits.ExploreSourceTimeout = 100 * time.Millisecond
	searcher := NewSearcherWithLimits(nil, nil, nil, exploreSourceFixtureStore{source: source}, nil, limits)
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

func TestExplorePageUsesWholeSearchRulesWhenExploreBookListIsBlank(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, `<ul class="mh-list col7"><li><div class="mh-item"><a href="/book/1"><img src="/search-cover"></a><h2><a href="/book/1">Book</a></h2><p class="chapter">Chapter</p></div></li></ul>`)
	}))
	defer server.Close()
	source := booksource.BookSource{
		BookSourceURL: server.URL, BookSourceName: "Partial Explore rules", EnabledExplore: true,
		ExploreURL:  "Books::" + server.URL,
		RuleSearch:  `{"author":"","bookList":"class.mh-list col7@li","bookUrl":"tag.a@href","coverUrl":"class.mh-item@tag.a@tag.img@src","intro":"","kind":"","lastChapter":"class.chapter@text","name":"class.mh-item@a@text"}`,
		RuleExplore: `{"coverUrl":"@js:'explore-cover'"}`,
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
	if len(page.Books) != 1 || page.Books[0].Name != "Book" || page.Books[0].BookURL != server.URL+"/book/1" || page.Books[0].CoverURL != server.URL+"/search-cover" {
		t.Fatalf("page=%+v", page)
	}
}

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

func TestExplorePageExpandsEncodedCategoryPageSelector(t *testing.T) {
	var offsets []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		offsets = append(offsets, r.URL.Query().Get("offset"))
		_, _ = fmt.Fprintf(w, `{"data":[{"title":"Book %s","id":"%s"}]}`, r.URL.Query().Get("offset"), r.URL.Query().Get("offset"))
	}))
	defer server.Close()
	source := booksource.BookSource{
		BookSourceURL: server.URL, BookSourceName: "Encoded paging", EnabledExplore: true,
		ExploreURL:  "Books::" + server.URL + `/books?offset=%3C0%2C150%2C300%3E`,
		RuleExplore: `{"bookList":"$.data[*]","name":"$.title","bookUrl":"$.id"}`,
	}
	searcher := NewSearcher(nil, analyzer.NewJSVM(), nil, exploreSourceFixtureStore{source: source}, nil)
	catalog, err := searcher.OpenExplore(t.Context(), source.BookSourceURL)
	if err != nil {
		t.Fatal(err)
	}
	first, err := searcher.GetExplorePage(t.Context(), ExplorePageRequest{SessionID: catalog.SessionID, CategoryID: "entry-0", Page: 1})
	if err != nil {
		t.Fatal(err)
	}
	second, err := searcher.GetExplorePage(t.Context(), ExplorePageRequest{SessionID: catalog.SessionID, CategoryID: "entry-0", Page: 2})
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(offsets) != "[0 150]" || first.Books[0].Name != "Book 0" || second.Books[0].Name != "Book 150" {
		t.Fatalf("offsets=%v first=%+v second=%+v", offsets, first, second)
	}
}

func TestExplorePageDirectJavaScriptReceivesStructuredJSONResult(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, `{"data":{"list":[{"title":"Book","comic_id":42,"author_title":"Author"}]}}`)
	}))
	defer server.Close()
	source := booksource.BookSource{
		BookSourceURL: server.URL, BookSourceName: "Structured JSON result", EnabledExplore: true,
		ExploreURL:  "Books::" + server.URL,
		RuleExplore: `{"bookList":"$.data.list.*","name":"@js:java.put('comic_id',result.comic_id);result.title","author":"$.author_title","bookUrl":"@js:'/book/'+java.get('comic_id')"}`,
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
	if len(page.Books) != 1 || page.Books[0].Name != "Book" || page.Books[0].Author != "Author" || page.Books[0].BookURL != server.URL+"/book/42" {
		t.Fatalf("page=%+v", page)
	}
}

func TestExplorePageFieldJavaScriptCanReadEarlierBookFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, `{"data":[{"title":"Book","groupID":"resource_42"}]}`)
	}))
	defer server.Close()
	source := booksource.BookSource{
		BookSourceURL: server.URL, BookSourceName: "Mutable result", EnabledExplore: true,
		ExploreURL:  "Books::" + server.URL,
		RuleExplore: `{"bookList":"$.data[*]","name":"$.title","kind":"$.groupID##.*_##","bookUrl":"@js:'/book/'+book.kind"}`,
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
	if len(page.Books) != 1 || page.Books[0].BookURL != server.URL+"/book/42" {
		t.Fatalf("page=%+v", page)
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

func TestExplorePageEvaluatesJavaScriptSourceHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Referer") != r.Host+"/from-source" {
			http.Error(w, "missing evaluated header", http.StatusForbidden)
			return
		}
		_, _ = fmt.Fprint(w, `<a class="book" href="/book/1">Book</a>`)
	}))
	defer server.Close()
	source := booksource.BookSource{
		BookSourceURL: server.URL, BookSourceName: "JavaScript headers", EnabledExplore: true,
		Header: `@js:JSON.stringify({Referer:source.bookSourceUrl.replace(/^https?:\/\//,'')+'/from-source'})`, ExploreURL: "Books::" + server.URL,
		RuleExplore: `{"bookList":".book","name":"text","bookUrl":"href"}`,
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
	if len(page.Books) != 1 {
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
