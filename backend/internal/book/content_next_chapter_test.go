// Regression test for stopping content pagination at the next TOC chapter.
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

func TestGetChapterContentDrainsQueuedPagesBeforeNextTOCChapter(t *testing.T) {
	nextChapterFetched := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/chapter/1":
			_, _ = w.Write([]byte(`<div class="content">当前章节</div><a class="next" href="/page-2">下一页</a><a class="next" href="/chapter/2">下一章</a>`))
		case "/page-2":
			_, _ = w.Write([]byte(`<div class="content">当前章节第二页</div>`))
		case "/chapter/2":
			nextChapterFetched = true
			http.Error(w, "next chapter must not be fetched", http.StatusInternalServerError)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	s := NewSearcher(fetcher.NewInsecure(3*time.Second), analyzer.NewJSVM(), nil, nil, nil)
	src := booksource.BookSource{
		BookSourceURL: server.URL,
		RuleContent:   `{"content":"@css:.content@text","nextContentUrl":"@css:.next@href"}`,
	}
	current := &Chapter{Index: 1, URL: server.URL + "/chapter/1"}
	next := &Chapter{Index: 2, URL: server.URL + "/chapter/2"}
	content, _, err := s.GetChapterContentForBook(src, &Book{BookURL: server.URL + "/book"}, current, next)
	if err != nil || content != "当前章节\n当前章节第二页" || nextChapterFetched {
		t.Fatalf("content=%q err=%v nextChapterFetched=%v, want queued page before TOC boundary", content, err, nextChapterFetched)
	}
}

func TestGetChapterContentDrainsAfterQueuedRedirectToNextTOCChapter(t *testing.T) {
	var chapterTwoRequests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/chapter/1":
			_, _ = w.Write([]byte(`<div class="content">当前章节</div><a class="next" href="/redirect-page">跳转页</a><a class="next" href="/page-2">有效页</a><a class="next" href="/chapter/2">下一章</a>`))
		case "/redirect-page":
			http.Redirect(w, r, "/chapter/2", http.StatusFound)
		case "/page-2":
			_, _ = w.Write([]byte(`<div class="content">当前章节有效第二页</div>`))
		case "/chapter/2":
			chapterTwoRequests++
			_, _ = w.Write([]byte(`<div class="content">下一章不应合并</div>`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	s := NewSearcher(fetcher.NewInsecure(3*time.Second), analyzer.NewJSVM(), nil, nil, nil)
	src := booksource.BookSource{
		BookSourceURL: server.URL,
		RuleContent:   `{"content":"@css:.content@text","nextContentUrl":"@css:.next@href"}`,
	}
	current := &Chapter{Index: 1, URL: server.URL + "/chapter/1"}
	next := &Chapter{Index: 2, URL: server.URL + "/chapter/2"}
	content, _, err := s.GetChapterContentForBook(src, &Book{BookURL: server.URL + "/book"}, current, next)
	if err != nil || content != "当前章节\n当前章节有效第二页" || chapterTwoRequests != 1 {
		t.Fatalf("content=%q err=%v chapterTwoRequests=%d, want queued pages drained after redirect boundary", content, err, chapterTwoRequests)
	}
}

func TestGetChapterContentStopsAfterRedirectToNextTOCChapter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/chapter/1":
			_, _ = w.Write([]byte(`<div class="content">当前章节</div><a class="next" href="/page-next">下一页</a>`))
		case "/page-next":
			http.Redirect(w, r, "/chapter/2", http.StatusFound)
		case "/chapter/2":
			_, _ = w.Write([]byte(`<div class="content">下一章正文不应合并</div>`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	s := NewSearcher(fetcher.NewInsecure(3*time.Second), analyzer.NewJSVM(), nil, nil, nil)
	src := booksource.BookSource{
		ID:             server.URL,
		BookSourceURL:  server.URL,
		BookSourceName: "fixture",
		RuleContent:    `{"content":"@css:.content@text","nextContentUrl":"@css:.next@href"}`,
	}
	current := &Chapter{Index: 1, URL: server.URL + "/chapter/1"}
	next := &Chapter{Index: 2, URL: server.URL + "/chapter/2"}
	content, _, err := s.GetChapterContentForBook(src, &Book{BookURL: server.URL + "/book"}, current, next)
	if err != nil {
		t.Fatal(err)
	}
	if content != "当前章节" {
		t.Fatalf("content=%q, want current chapter only", content)
	}
}

func TestGetChapterContentStopsBeforeNextTOCChapter(t *testing.T) {
	nextChapterFetched := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/chapter/1":
			_, _ = w.Write([]byte(`<div class="content">当前章节</div><a class="next" href='/chapter/2,{"headers":{"X-Page":"next"}}'>下一章</a>`))
		case "/chapter/2":
			nextChapterFetched = true
			http.Error(w, "next chapter must not be fetched as a page", http.StatusInternalServerError)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	s := NewSearcher(fetcher.NewInsecure(3*time.Second), analyzer.NewJSVM(), nil, nil, nil)
	src := booksource.BookSource{
		ID:             server.URL,
		BookSourceURL:  server.URL,
		BookSourceName: "fixture",
		RuleContent:    `{"content":"@css:.content@text","nextContentUrl":"@css:.next@href"}`,
	}
	s.sessions.GetOrCreateBook(src.BookSourceURL, "book")
	s.sessions.AssociateChapter(src.BookSourceURL, "book", server.URL+"/chapter/2")

	content, _, err := s.GetChapterContent(src, server.URL+"/chapter/1")
	if err != nil {
		t.Fatal(err)
	}
	if content != "当前章节" || nextChapterFetched {
		t.Fatalf("content=%q nextChapterFetched=%v", content, nextChapterFetched)
	}
}
