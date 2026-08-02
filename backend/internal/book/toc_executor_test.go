// Integration tests for TOC request options and documented ordering.
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

func TestGetChapterListRunsPreUpdateJSBeforeFetchingMutatedTOCURL(t *testing.T) {
	var requestedPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		_, _ = w.Write([]byte(`<a class="chapter" href="/chapter/1">第一章</a>`))
	}))
	defer server.Close()

	s := NewSearcher(fetcher.NewInsecure(3*time.Second), analyzer.NewJSVM(), nil, nil, nil)
	src := booksource.BookSource{
		BookSourceURL:  server.URL,
		BookSourceName: "fixture",
		RuleToc: `{
			"preUpdateJs":"book.tocUrl = String(book.tocUrl).replace('/stale-toc', '/fresh-toc'); book.lastChapter = 'script ran'",
			"chapterList":"@css:.chapter", "chapterName":"text", "chapterUrl":"@href"
		}`,
	}
	book := &Book{
		SourceURL: server.URL,
		BookURL:   server.URL + "/book",
		TocURL:    server.URL + "/stale-toc",
	}

	chapters, err := s.GetChapterListForBook(src, book, book.TocURL)
	if err != nil {
		t.Fatal(err)
	}
	if requestedPath != "/fresh-toc" {
		t.Fatalf("requested path=%q, want preUpdateJs-mutated TOC path", requestedPath)
	}
	if book.TocURL != server.URL+"/fresh-toc" || book.LastChapter != "script ran" {
		t.Fatalf("book=%+v, want script mutations synchronized before fetch", book)
	}
	if len(chapters) != 1 || chapters[0].Title != "第一章" {
		t.Fatalf("chapters=%+v, want parsed chapter from mutated URL", chapters)
	}
}

func TestGetChapterListStopsWhenPreUpdateJSFails(t *testing.T) {
	var requested bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requested = true
		_, _ = w.Write([]byte(`<a class="chapter" href="/chapter/1">第一章</a>`))
	}))
	defer server.Close()

	s := NewSearcher(fetcher.NewInsecure(3*time.Second), analyzer.NewJSVM(), nil, nil, nil)
	src := booksource.BookSource{
		BookSourceURL: server.URL,
		RuleToc: `{
			"preUpdateJs":"throw new Error('refresh failed')",
			"chapterList":"@css:.chapter", "chapterName":"text", "chapterUrl":"@href"
		}`,
	}
	book := &Book{SourceURL: server.URL, BookURL: server.URL + "/book", TocURL: server.URL + "/toc"}

	_, err := s.GetChapterListForBook(src, book, book.TocURL)
	if err == nil || !strings.Contains(err.Error(), "preUpdateJs") || !strings.Contains(err.Error(), "refresh failed") {
		t.Fatalf("err=%v, want contextual preUpdateJs failure", err)
	}
	if requested {
		t.Fatal("TOC was fetched after preUpdateJs failed")
	}
}

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
