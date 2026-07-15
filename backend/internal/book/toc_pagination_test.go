// Integration tests for TOC pagination and partial-failure reporting.
package book

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/otwako/novelreader/internal/analyzer"
	"github.com/otwako/novelreader/internal/booksource"
	"github.com/otwako/novelreader/internal/fetcher"
)

func TestGetChapterListFollowsRelativeNextTOCPage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/toc":
			_, _ = w.Write([]byte(`<a class="chapter" href="/chapter/1">第一章</a><a class="next" href="/toc-2">下一页</a>`))
		case "/toc-2":
			_, _ = w.Write([]byte(`<a class="chapter" href="/chapter/2">第二章</a>`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	s := NewSearcher(fetcher.NewInsecure(3*time.Second), analyzer.NewJSVM(), nil, nil, nil)
	src := booksource.BookSource{
		BookSourceURL:  server.URL,
		BookSourceName: "fixture",
		RuleToc:        `{"chapterList":"@css:.chapter","chapterName":"text","chapterUrl":"@href","nextTocUrl":"@css:.next@href"}`,
	}

	chapters, err := s.GetChapterList(src, server.URL+"/book", server.URL+"/toc")
	if err != nil {
		t.Fatal(err)
	}
	if len(chapters) != 2 || chapters[0].Title != "第一章" || chapters[1].Title != "第二章" {
		t.Fatalf("chapters = %+v, want both pages in order", chapters)
	}
}

func TestTOCDeduplicatesByURLAndHonorsReverseMarker(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/toc":
			_, _ = w.Write([]byte(`<a class="chapter" href="/chapter/shared">旧标题</a><a class="next" href="/toc-2">下一页</a>`))
		case "/toc-2":
			_, _ = w.Write([]byte(`<a class="chapter" href="/chapter/shared">新标题</a><a class="chapter" href="/chapter/3">第三章</a>`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	newSearcher := func(rule string) []Chapter {
		s := NewSearcher(fetcher.NewInsecure(3*time.Second), analyzer.NewJSVM(), nil, nil, nil)
		src := booksource.BookSource{
			BookSourceURL:  server.URL,
			BookSourceName: "fixture",
			RuleToc:        `{"chapterList":"` + rule + `","chapterName":"text","chapterUrl":"@href","nextTocUrl":"@css:.next@href"}`,
		}
		chapters, err := s.GetChapterList(src, server.URL+"/book", server.URL+"/toc")
		if err != nil {
			t.Fatal(err)
		}
		return chapters
	}

	chapters := newSearcher(`+@css:.chapter`)
	if len(chapters) != 2 || chapters[0].Title != "新标题" || chapters[1].Title != "第三章" {
		t.Fatalf("normal chapters=%+v, want Legado reversal/dedup order", chapters)
	}

	chapters = newSearcher(`-@css:.chapter`)
	if len(chapters) != 2 || chapters[0].Title != "第三章" || chapters[1].Title != "旧标题" {
		t.Fatalf("reversed chapters=%+v, want reverse marker to reverse final order", chapters)
	}
}

func TestTOCPreservesUpdateTimeAsChapterTag(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<div class="chapter"><span class="name">Chapter</span><time>2024-01-02</time></div>`))
	}))
	defer server.Close()

	s := NewSearcher(fetcher.NewInsecure(3*time.Second), analyzer.NewJSVM(), nil, nil, nil)
	src := booksource.BookSource{
		BookSourceURL: server.URL,
		RuleToc:       `{"chapterList":"@css:.chapter","chapterName":"@css:.name@text","chapterUrl":"@href","updateTime":"@css:time@text"}`,
	}
	chapters, err := s.GetChapterList(src, server.URL+"/book", server.URL+"/toc")
	if err != nil || len(chapters) != 1 || chapters[0].Tag != "2024-01-02" {
		t.Fatalf("chapters=%+v err=%v, want updateTime preserved in tag", chapters, err)
	}
}

func TestChapterListReportsNextPageFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/toc" {
			_, _ = w.Write([]byte(`<a class="chapter" href="/chapter/1">第一章</a><a class="next" href="/missing">下一页</a>`))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	s := NewSearcher(fetcher.NewInsecure(3*time.Second), analyzer.NewJSVM(), nil, nil, nil)
	src := booksource.BookSource{
		BookSourceURL:  server.URL,
		BookSourceName: "fixture",
		RuleToc:        `{"chapterList":"@css:.chapter","chapterName":"text","chapterUrl":"@href","nextTocUrl":"@css:.next@href"}`,
	}

	_, err := s.GetChapterList(src, server.URL+"/book", server.URL+"/toc")
	if err == nil || !strings.Contains(err.Error(), "next") || !strings.Contains(err.Error(), "1 chapters") {
		t.Fatalf("error = %v, want next-page failure with partial count", err)
	}
}

func TestChapterListReportsNextRuleFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<a class="chapter" href="/chapter/1">第一章</a>`))
	}))
	defer server.Close()

	s := NewSearcher(fetcher.NewInsecure(3*time.Second), analyzer.NewJSVM(), nil, nil, nil)
	src := booksource.BookSource{
		BookSourceURL:  server.URL,
		BookSourceName: "fixture",
		RuleToc:        `{"chapterList":"@css:.chapter","chapterName":"text","chapterUrl":"@href","nextTocUrl":"<js>throw new Error('broken next rule')</js>"}`,
	}
	_, err := s.GetChapterList(src, server.URL+"/book", server.URL+"/toc")
	if err == nil || !strings.Contains(err.Error(), "nextTocUrl") || !strings.Contains(err.Error(), "1 chapters") {
		t.Fatalf("error = %v, want next-rule failure with partial count", err)
	}
}
