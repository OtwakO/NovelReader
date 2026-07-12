// Integration tests for TOC request options and documented ordering.
package book

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/otwako/novelreader/internal/analyzer"
	"github.com/otwako/novelreader/internal/booksource"
	"github.com/otwako/novelreader/internal/fetcher"
)

func TestGetChapterListUsesTOCURLAndPreservesRuleOrder(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if err := r.ParseForm(); err != nil || r.Form.Get("page") != "1" {
			t.Errorf("form = %v, want page=1", r.Form)
		}
		_, _ = w.Write([]byte(`<a class="chapter" href="/chapter/1">第一章</a><a class="chapter" href="/chapter/2">第二章</a>`))
	}))
	defer server.Close()

	s := NewSearcher(fetcher.NewInsecure(3*time.Second), analyzer.NewJSVM(), nil, nil, nil)
	src := booksource.BookSource{
		BookSourceURL:  server.URL,
		BookSourceName: "fixture",
		RuleToc:        `{"chapterList":"@css:.chapter","chapterName":"text","chapterUrl":"@href"}`,
	}

	chapters, err := s.GetChapterList(src, server.URL+"/book", `/toc,{"method":"POST","body":"page=1"}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(chapters) != 2 || chapters[0].Title != "第一章" || chapters[1].Title != "第二章" {
		t.Fatalf("chapters = %+v, want source order", chapters)
	}
	if chapters[0].URL != server.URL+"/chapter/1" {
		t.Fatalf("chapter URL = %q", chapters[0].URL)
	}
}
