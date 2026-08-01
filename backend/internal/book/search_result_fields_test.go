// Search result field tests cover list metadata shared by Search and Explore.
package book

import (
	"testing"

	"github.com/otwako/novelreader/internal/analyzer"
	"github.com/otwako/novelreader/internal/booksource"
)

func TestParseSearchResultPreservesListWordCountAndUpdateTime(t *testing.T) {
	searcher := NewSearcher(nil, analyzer.NewJSVM(), nil, nil, nil)
	source := booksource.BookSource{
		BookSourceURL: "https://example.test",
		RuleSearch: `{
			"bookList":"@css:.book",
			"name":"@css:.name@text",
			"bookUrl":"@css:.name@href",
			"wordCount":"@css:.words@text",
			"updateTime":"@css:.updated@text"
		}`,
	}
	html := `<article class="book"><a class="name" href="/book/1">Book</a><span class="words">123万字</span><time class="updated">2026-08-01</time></article>`

	results, err := searcher.ParseSearchResult(source, html)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].WordCount != "123万字" || results[0].UpdateTime != "2026-08-01" {
		t.Fatalf("results=%+v, want list word count and update time", results)
	}
}
