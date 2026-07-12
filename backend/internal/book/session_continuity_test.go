// Integration test for detail-to-TOC-to-content session continuity.
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

func TestBookWorkflowCarriesCookiesFromDetailToTOCAndContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/book" {
			http.SetCookie(w, &http.Cookie{Name: "auth", Value: "fixture", Path: "/"})
			_, _ = w.Write([]byte(`<h1 class="name">凡人修仙传</h1><a class="toc" href="/toc">目录</a>`))
			return
		}
		if r.Header.Get("Cookie") != "auth=fixture" {
			http.Error(w, "missing auth cookie", http.StatusForbidden)
			return
		}
		switch r.URL.Path {
		case "/toc":
			_, _ = w.Write([]byte(`<a class="chapter" href="/chapter/1">第一章</a>`))
		case "/chapter/1":
			_, _ = w.Write([]byte(`<div class="content">正文</div>`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	s := NewSearcher(fetcher.NewInsecure(3*time.Second), analyzer.NewJSVM(), nil, nil, nil)
	src := booksource.BookSource{
		BookSourceURL:  server.URL,
		BookSourceName: "fixture",
		RuleBookInfo:   `{"name":"@css:.name@text","tocUrl":"@css:.toc@href"}`,
		RuleToc:        `{"chapterList":"@css:.chapter","chapterName":"text","chapterUrl":"@href"}`,
		RuleContent:    `{"content":"@css:.content@text"}`,
	}

	book, err := s.GetBookInfo(src, server.URL+"/book")
	if err != nil {
		t.Fatal(err)
	}
	chapters, err := s.GetChapterList(src, book.BookURL, book.TocURL)
	if err != nil {
		t.Fatal(err)
	}
	if len(chapters) != 1 {
		t.Fatalf("chapters = %+v", chapters)
	}
	content, _, err := s.GetChapterContent(src, chapters[0].URL)
	if err != nil {
		t.Fatal(err)
	}
	if content != "正文" {
		t.Fatalf("content = %q", content)
	}
}
