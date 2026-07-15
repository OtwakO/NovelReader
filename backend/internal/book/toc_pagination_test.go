// Integration tests for TOC pagination and partial-failure reporting.
package book

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
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

func TestTOCSkipsRedirectAliasesToProcessedPages(t *testing.T) {
	var tocRequests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/toc":
			if tocRequests.Add(1) == 1 {
				_, _ = w.Write([]byte(`<a class="chapter" href="/chapter/1">第一章</a><a class="next" href="/alias">别名</a>`))
				return
			}
			_, _ = w.Write([]byte(`<a class="chapter" href="/chapter/2">不应解析</a>`))
		case "/alias":
			http.Redirect(w, r, "/toc", http.StatusFound)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	s := NewSearcher(fetcher.NewInsecure(3*time.Second), analyzer.NewJSVM(), nil, nil, nil)
	src := booksource.BookSource{
		BookSourceURL: server.URL,
		RuleToc:       `{"chapterList":"@css:.chapter","chapterName":"text","chapterUrl":"@href","nextTocUrl":"@css:.next@href"}`,
	}
	chapters, err := s.GetChapterList(src, server.URL+"/book", server.URL+"/toc")
	if err != nil || len(chapters) != 1 || chapters[0].Title != "第一章" || tocRequests.Load() != 2 {
		t.Fatalf("chapters=%+v err=%v tocRequests=%d, want redirect response skipped after fetch", chapters, err, tocRequests.Load())
	}
}

func TestTOCTraversesAllNextURLsAndPreservesRequestOptions(t *testing.T) {
	var page2Attempts atomic.Int32
	var page2Header atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/toc":
			_, _ = w.Write([]byte(`<a class="chapter" href="/chapter/1">第一章</a><a class="next" href='/toc-2,{"headers":{"X-Page":"two"},"retry":1}'>第二页</a><a class="next" href="/toc-3">第三页</a>`))
		case "/toc-2":
			page2Header.Store(r.Header.Get("X-Page") == "two")
			if page2Attempts.Add(1) == 1 {
				w.WriteHeader(http.StatusBadGateway)
				return
			}
			_, _ = w.Write([]byte(`<a class="chapter" href="/chapter/2">第二章</a>`))
		case "/toc-3":
			_, _ = w.Write([]byte(`<a class="chapter" href="/chapter/3">第三章</a>`))
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
	if err != nil || len(chapters) != 3 || chapters[0].Title != "第一章" || chapters[1].Title != "第二章" || chapters[2].Title != "第三章" {
		t.Fatalf("chapters=%+v err=%v, want all next URLs in order", chapters, err)
	}
	if page2Attempts.Load() != 2 || !page2Header.Load() {
		t.Fatalf("page2 attempts=%d header=%v, want retry=1 and URL-option header", page2Attempts.Load(), page2Header.Load())
	}
}

func TestTOCAllowsDistinctOptionsForOneEndpoint(t *testing.T) {
	var validBody atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/toc" {
			http.NotFound(w, r)
			return
		}
		if r.Method == http.MethodPost {
			body, _ := io.ReadAll(r.Body)
			validBody.Store(string(body) == "page=2")
			_, _ = w.Write([]byte(`<a class="chapter" href="/chapter/2">第二章</a>`))
			return
		}
		_, _ = w.Write([]byte(`<a class="chapter" href="/chapter/1">第一章</a><a class="next" href='/toc,{"method":"POST","body":"page=2"}'>下一页</a>`))
	}))
	defer server.Close()

	s := NewSearcher(fetcher.NewInsecure(3*time.Second), analyzer.NewJSVM(), nil, nil, nil)
	src := booksource.BookSource{
		BookSourceURL: server.URL,
		RuleToc:       `{"chapterList":"@css:.chapter","chapterName":"text","chapterUrl":"@href","nextTocUrl":"@css:.next@href"}`,
	}
	chapters, err := s.GetChapterList(src, server.URL+"/book", server.URL+"/toc")
	if err != nil || len(chapters) != 2 || chapters[0].Title != "第一章" || chapters[1].Title != "第二章" || !validBody.Load() {
		t.Fatalf("chapters=%+v err=%v validBody=%v, want distinct same-endpoint request options", chapters, err, validBody.Load())
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
	var paginationErr *TOCPaginationError
	if err == nil || !errors.As(err, &paginationErr) || paginationErr.PagesFetched != 1 || paginationErr.ChaptersFetched != 1 || paginationErr.FailedURL == "" || !strings.Contains(err.Error(), "next") {
		t.Fatalf("error = %v, pagination=%+v, want typed next-page failure with partial count", err, paginationErr)
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
	var paginationErr *TOCPaginationError
	if err == nil || !errors.As(err, &paginationErr) || paginationErr.Operation != "nextTocUrl" || paginationErr.PagesFetched != 1 || paginationErr.ChaptersFetched != 1 || !strings.Contains(err.Error(), "nextTocUrl") {
		t.Fatalf("error = %v, pagination=%+v, want typed next-rule failure with partial count", err, paginationErr)
	}
}
