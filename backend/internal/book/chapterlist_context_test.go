package book

import (
	"context"
	"errors"
	"testing"

	"github.com/otwako/novelreader/internal/analyzer"
	"github.com/otwako/novelreader/internal/booksource"
)

func TestChapterListStopsWhenContextEndsAfterFetch(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	parser := &ChapterListParser{
		src: booksource.BookSource{
			BookSourceName: "Cancelled TOC",
			RuleToc:        `{"chapterList":".chapter","chapterName":"text","chapterUrl":"href"}`,
		},
		jsVM: analyzer.NewJSVM(),
		ctx:  ctx,
		fetch: func(string) (string, string, error) {
			cancel()
			return `<a class="chapter" href="/1">Chapter 1</a>`, "https://source.test/toc", nil
		},
	}

	_, err := parser.ParseChapterList("https://source.test/toc", "https://source.test")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error=%v, want context cancellation", err)
	}
}
