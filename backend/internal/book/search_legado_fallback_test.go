package book

import (
	"testing"

	"github.com/otwako/novelreader/internal/analyzer"
	"github.com/otwako/novelreader/internal/booksource"
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
