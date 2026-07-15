// Integration coverage for book/chapter bindings in URL and bodyJs execution.
package sourceexec

import (
	"context"
	"testing"

	"github.com/otwako/novelreader/internal/analyzer"
)

func TestExecutorCarriesCrawlContextIntoURLAndBodyJS(t *testing.T) {
	transport := &captureTransport{}
	executor := NewExecutor(analyzer.NewJSVM(), transport)
	executor.SetURLContext(&analyzer.URLContext{
		Book:        map[string]interface{}{"name": "Book", "bookUrl": "https://example.test/book"},
		Chapter:     map[string]interface{}{"index": 4, "url": "https://example.test/read/4"},
		NextChapter: map[string]interface{}{"url": "https://example.test/read/5"},
		JSLib:       "function marker(value) { return value + '-lib'; }",
	})

	response, err := executor.Execute(context.Background(), `https://example.test/{{book.name}}/{{chapter.index}},{"bodyJs":"marker(result)+'|'+book.bookUrl+'|'+chapter.url+'|'+nextChapter.url"}`, "", 1, "https://example.test/")
	if err != nil {
		t.Fatal(err)
	}
	wantURL := "https://example.test/Book/4"
	if transport.spec.URL != wantURL {
		t.Fatalf("URL = %q, want %q", transport.spec.URL, wantURL)
	}
	wantBody := "fixture-lib|https://example.test/book|https://example.test/read/4|https://example.test/read/5"
	if response.Body != wantBody {
		t.Fatalf("body = %q, want %q", response.Body, wantBody)
	}
}
