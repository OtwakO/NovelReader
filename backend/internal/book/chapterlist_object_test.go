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
			RuleToc:        `{"chapterList":"@js:list=[{text:'第一卷',href:'/volume/1',isVolume:true},{text:'第一章',href:'/chapter/1',isVolume:false}];list","chapterName":"text","chapterUrl":"href","isVolume":"isVolume"}`,
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
	if chapters[0].Title != "第一卷" || chapters[0].URL != "https://example.test/volume/1" || !chapters[0].IsVolume {
		t.Fatalf("first chapter=%+v", chapters[0])
	}
	if chapters[1].Title != "第一章" || chapters[1].URL != "https://example.test/chapter/1" || chapters[1].IsVolume {
		t.Fatalf("second chapter=%+v", chapters[1])
	}
}
