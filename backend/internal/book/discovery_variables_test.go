package book

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/otwako/novelreader/internal/analyzer"
	"github.com/otwako/novelreader/internal/booksource"
	"github.com/otwako/novelreader/internal/fetcher"
)

func TestDiscoveryVariablesBelongToEachResult(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<a class="book" href="/one">One</a><a class="book" href="/two">Two</a>`))
	}))
	defer server.Close()
	source := booksource.BookSource{
		ID: "source", BookSourceURL: server.URL, Enabled: true, EnabledExplore: true,
		SearchURL:    `@js:java.put('request', key); source.bookSourceUrl + '/search'`,
		ExploreURL:   `Books::@js:java.put('request', 'explore'); source.bookSourceUrl + '/search'`,
		RuleSearch:   `{"bookList":".book@put:{listValue:'.book.0@text'}","name":"@js:if (java.get('item')) throw new Error('sibling variables leaked'); java.put('item', java.getString('text'))","author":"@get:{request}","bookUrl":"href"}`,
		RuleBookInfo: `{"name":"@js:java.get('item') + ':' + java.get('request') + ':' + java.get('listValue')"}`,
	}
	s := NewSearcher(fetcher.NewInsecure(3*time.Second), analyzer.NewJSVM(), nil, exploreSourceFixtureStore{source: source}, nil)
	for _, operation := range []string{"search", "explore"} {
		t.Run(operation, func(t *testing.T) {
			var results []SearchResult
			var err error
			if operation == "search" {
				results, err = s.searchSource(t.Context(), source, operation)
			} else {
				catalog, openErr := s.OpenExplore(t.Context(), source.ID)
				if openErr != nil {
					t.Fatal(openErr)
				}
				page, pageErr := s.GetExplorePage(t.Context(), ExplorePageRequest{SessionID: catalog.SessionID, CategoryID: "entry-0", Page: 1})
				results, err = page.Books, pageErr
			}
			if err != nil || len(results) != 2 {
				t.Fatalf("results=%+v err=%v", results, err)
			}
			// A later request on the same source must not overwrite earlier result snapshots.
			if _, err := s.searchSource(t.Context(), source, "later"); err != nil {
				t.Fatal(err)
			}
			for index, name := range []string{"One", "Two"} {
				result := results[index]
				var variables map[string]string
				if err := json.Unmarshal([]byte(result.VariableMap), &variables); err != nil {
					t.Fatal(err)
				}
				if variables["item"] != name || variables["request"] != operation || variables["listValue"] != "One" || result.Author != operation {
					t.Fatalf("incorrect result context: %+v", result)
				}
				b := &Book{SourceID: source.ID, SourceURL: source.BookSourceURL, BookURL: result.BookURL, VariableMap: result.VariableMap}
				detail, err := s.GetBookInfoForBookContext(t.Context(), source, b, b.BookURL)
				if err != nil || detail.Name != name+":"+operation+":One" {
					t.Fatalf("detail=%+v err=%v", detail, err)
				}
			}
		})
	}
}
