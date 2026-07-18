package book

import (
	"context"
	"testing"

	"github.com/otwako/novelreader/internal/analyzer"
	"github.com/otwako/novelreader/internal/booksource"
)

func TestExploreResolvesBookURLBeforePatternFilter(t *testing.T) {
	source := booksource.BookSource{
		BookSourceName: "captured relative-link source",
		BookSourceURL:  "https://m.22biqu.com/",
		BookURLPattern: `https?://m\.22biqu\.com/biqu\d+/`,
	}
	rules := `{"bookList":".hot_sale","name":".title@text","bookUrl":"a.0@href"}`
	html := `<div class="hot_sale"><a href="/biqu1234/"><span class="title">One</span></a></div>
<div class="hot_sale"><a href="/biqu5678/"><span class="title">Two</span></a></div>`
	searcher := NewSearcher(nil, analyzer.NewJSVM(), analyzer.NewCacheManager(), nil, nil)

	results, err := searcher.parseSearchResultWithRuleStateContextAtURLLimit(
		context.Background(), source, html, rules, "https://m.22biqu.com/rank/", nil, 0, false, true,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("results = %d, want 2", len(results))
	}
	if results[0].BookURL != "https://m.22biqu.com/biqu1234/" || results[1].BookURL != "https://m.22biqu.com/biqu5678/" {
		t.Fatalf("resolved URLs = %q, %q", results[0].BookURL, results[1].BookURL)
	}
}
