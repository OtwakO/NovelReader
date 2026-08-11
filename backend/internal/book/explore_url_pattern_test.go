package book

import (
	"context"
	"testing"

	"github.com/otwako/novelreader/internal/analyzer"
	"github.com/otwako/novelreader/internal/booksource"
)

func TestParseSearchResultDoesNotFilterByBookURLPattern(t *testing.T) {
	source := booksource.BookSource{
		BookSourceName: "stale search pattern",
		BookSourceURL:  "https://example.test/",
		BookURLPattern: `https://old\.example\.test/book/\d+`,
		RuleSearch:     `{"bookList":".book","name":".title@text","bookUrl":"a@href"}`,
	}
	html := `<div class="book"><a href="/current/1"><span class="title">One</span></a></div>`
	searcher := NewSearcher(nil, analyzer.NewJSVM(), analyzer.NewCacheManager(), nil, nil)

	results, err := searcher.ParseSearchResultWithStateAtURL(source, html, "https://example.test/search", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("results = %d, want 1 despite stale bookUrlPattern", len(results))
	}
	if results[0].BookURL != "https://example.test/current/1" {
		t.Fatalf("book URL = %q, want resolved current URL", results[0].BookURL)
	}
}

func TestParseExploreResultDoesNotFilterByBookURLPattern(t *testing.T) {
	source := booksource.BookSource{
		BookSourceName: "stale Explore pattern",
		BookSourceURL:  "https://m.example.test/",
		BookURLPattern: `https?://m\.example\.test/legacy\d+/`,
	}
	rules := `{"bookList":".hot_sale","name":".title@text","bookUrl":"a@href"}`
	html := `<div class="hot_sale"><a href="/current/1234/"><span class="title">One</span></a></div>
<div class="hot_sale"><a href="/current/5678/"><span class="title">Two</span></a></div>`
	searcher := NewSearcher(nil, analyzer.NewJSVM(), analyzer.NewCacheManager(), nil, nil)

	results, err := searcher.parseSearchResultWithRuleStateContextAtURLLimit(
		context.Background(), source, html, rules, "https://m.example.test/rank/", nil, 0, false, true,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("results = %d, want 2 despite stale bookUrlPattern", len(results))
	}
	if results[0].BookURL != "https://m.example.test/current/1234/" || results[1].BookURL != "https://m.example.test/current/5678/" {
		t.Fatalf("resolved URLs = %q, %q", results[0].BookURL, results[1].BookURL)
	}
}
