// Regression test for Legado JavaScript TOC rules returning typed chapter objects.
package book

import (
	"testing"

	"github.com/otwako/novelreader/internal/analyzer"
	"github.com/otwako/novelreader/internal/booksource"
)

func TestChapterListPreservesJavaScriptChapterObjects(t *testing.T) {
	parser := &ChapterListParser{
		src: booksource.BookSource{
			BookSourceName: "js toc fixture",
			RuleToc:        `{"chapterList":"@js:list=[{text:'第一章',href:'/chapter/1'},{text:'第二章',href:'/chapter/2'}];list","chapterName":"text","chapterUrl":"href"}`,
		},
		jsVM:  analyzer.NewJSVM(),
		fetch: func(string) (string, string, error) { return "<html></html>", "https://example.test/book", nil },
	}

	chapters, err := parser.ParseChapterList("https://example.test/book", "https://example.test")
	if err != nil {
		t.Fatal(err)
	}
	if len(chapters) != 2 {
		t.Fatalf("chapters=%+v", chapters)
	}
	if chapters[0].Title != "第一章" || chapters[0].URL != "https://example.test/chapter/1" {
		t.Fatalf("first chapter=%+v", chapters[0])
	}
}
