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

func TestBookWorkflowEvaluatesSourceHeadersForEveryRequest(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Source") != "fixture" {
			http.Error(w, "missing source header", http.StatusForbidden)
			return
		}
		paths = append(paths, r.URL.Path)
		switch r.URL.Path {
		case "/book":
			_, _ = w.Write([]byte(`<h1 class="name">Book</h1><a class="toc" href="/toc">TOC</a>`))
		case "/toc":
			_, _ = w.Write([]byte(`<a class="chapter" href="/chapter/1">One</a>`))
		case "/chapter/1":
			_, _ = w.Write([]byte(`<div class="content">Page one</div><a class="next" href="/chapter/2">Next</a>`))
		case "/chapter/2":
			_, _ = w.Write([]byte(`<div class="content">Page two</div>`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	s := NewSearcher(fetcher.NewInsecure(3*time.Second), analyzer.NewJSVM(), nil, nil, nil)
	src := booksource.BookSource{
		BookSourceURL: server.URL, BookSourceName: "fixture",
		Header:       `@js:JSON.stringify({'X-Source':'fixture'})`,
		RuleBookInfo: `{"name":"@css:.name@text","tocUrl":"@css:.toc@href"}`,
		RuleToc:      `{"chapterList":"@css:.chapter","chapterName":"text","chapterUrl":"@href"}`,
		RuleContent:  `{"content":"@css:.content@text","nextContentUrl":"@css:.next@href"}`,
	}
	book, err := s.GetBookInfo(src, server.URL+"/book")
	if err != nil {
		t.Fatal(err)
	}
	chapters, err := s.GetChapterListForBook(src, book, book.TocURL)
	if err != nil || len(chapters) != 1 {
		t.Fatalf("chapters=%+v err=%v", chapters, err)
	}
	content, _, err := s.GetChapterContentForBook(src, book, &chapters[0], nil)
	if err != nil || content != "Page one\nPage two" {
		t.Fatalf("content=%q err=%v", content, err)
	}
	if len(paths) != 4 {
		t.Fatalf("paths=%v", paths)
	}
}

func TestBookWorkflowRespectsEnabledCookieJar(t *testing.T) {
	for _, test := range []struct {
		name             string
		enabledCookieJar *bool
		wantCookie       bool
	}{
		{name: "omitted defaults enabled", wantCookie: true},
		{name: "explicit false disables automatic cookies", enabledCookieJar: boolPointer(false), wantCookie: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/book":
					http.SetCookie(w, &http.Cookie{Name: "auth", Value: "fixture", Path: "/"})
					_, _ = w.Write([]byte(`<h1 class="name">Book</h1><a class="toc" href="/toc">TOC</a>`))
				case "/toc":
					hasCookie := r.Header.Get("Cookie") == "auth=fixture"
					if hasCookie != test.wantCookie {
						http.Error(w, "unexpected automatic cookie state", http.StatusForbidden)
						return
					}
					_, _ = w.Write([]byte(`<a class="chapter" href="/chapter/1">One</a>`))
				default:
					http.NotFound(w, r)
				}
			}))
			defer server.Close()

			s := NewSearcher(fetcher.NewInsecure(3*time.Second), analyzer.NewJSVM(), nil, nil, nil)
			src := booksource.BookSource{
				BookSourceURL: server.URL, BookSourceName: "fixture", EnabledCookieJar: test.enabledCookieJar,
				RuleBookInfo: `{"name":"@css:.name@text","tocUrl":"@css:.toc@href"}`,
				RuleToc:      `{"chapterList":"@css:.chapter","chapterName":"text","chapterUrl":"@href"}`,
			}
			book, err := s.GetBookInfo(src, server.URL+"/book")
			if err != nil {
				t.Fatal(err)
			}
			chapters, err := s.GetChapterListForBook(src, book, book.TocURL)
			if err != nil || len(chapters) != 1 {
				t.Fatalf("chapters=%+v err=%v", chapters, err)
			}
		})
	}
}

func boolPointer(value bool) *bool { return &value }

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
