// Integration tests for TOC request options and documented ordering.
package book

import (
	"net/http"
	"net/http/httptest"
	"slices"
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

func TestGetChapterListRefreshTocURLReloadsDetailBeforeTOC(t *testing.T) {
	var requestedPaths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPaths = append(requestedPaths, r.URL.Path)
		switch r.URL.Path {
		case "/book":
			_, _ = w.Write([]byte(`<a class="toc" href="/fresh-toc">目录</a><span class="latest">详情已刷新</span>`))
		case "/fresh-toc":
			_, _ = w.Write([]byte(`<a class="chapter" href="/chapter/1">第一章</a>`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	s := NewSearcher(fetcher.NewInsecure(3*time.Second), analyzer.NewJSVMWithPoolSize(1), nil, nil, nil)
	src := booksource.BookSource{
		BookSourceURL: server.URL,
		RuleBookInfo:  `{"tocUrl":"@css:.toc@href","lastChapter":"@css:.latest@text"}`,
		RuleToc: `{
			"preUpdateJs":"java.refreshTocUrl()",
			"chapterList":"@css:.chapter", "chapterName":"text", "chapterUrl":"@href"
		}`,
	}
	book := &Book{SourceURL: server.URL, BookURL: server.URL + "/book", TocURL: server.URL + "/stale-toc"}

	chapters, err := s.GetChapterListForBook(src, book, book.TocURL)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(requestedPaths, []string{"/book", "/fresh-toc"}) {
		t.Fatalf("requested paths=%v, want detail refresh before refreshed TOC", requestedPaths)
	}
	if book.TocURL != server.URL+"/fresh-toc" || book.LastChapter != "详情已刷新" {
		t.Fatalf("book=%+v, want refreshed detail state", book)
	}
	if len(chapters) != 1 || chapters[0].Title != "第一章" {
		t.Fatalf("chapters=%+v, want refreshed TOC result", chapters)
	}
}

func TestGetChapterListReGetBookSearchesThenRefreshesDetailWithoutRuntimeReentry(t *testing.T) {
	var requestedPaths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPaths = append(requestedPaths, r.URL.Path)
		switch r.URL.Path {
		case "/search":
			if r.URL.Query().Get("q") != "ExistingName" {
				t.Errorf("search query=%q, want existing book name", r.URL.Query().Get("q"))
			}
			http.SetCookie(w, &http.Cookie{Name: "precise", Value: "matched", Path: "/"})
			nearMatches := strings.Repeat(`<div class="book"><span class="name">ExistingName</span><span class="author">OtherAuthor</span><a href="/wrong-book">书籍</a></div>`, 20)
			_, _ = w.Write([]byte(nearMatches + `<div class="book"><span class="name">ExistingName</span><span class="author">ExistingAuthor</span><span class="kind">SearchKind</span><a href="/new-book">书籍</a></div>`))
		case "/new-book":
			if r.Header.Get("Cookie") != "precise=matched" {
				t.Errorf("detail cookie=%q, want cookie established by precise search", r.Header.Get("Cookie"))
			}
			_, _ = w.Write([]byte(`<a class="toc" href="/fresh-toc">目录</a><span class="latest">重新搜索并刷新</span><span class="kind">SearchKind</span>`))
		case "/fresh-toc":
			if r.Header.Get("Cookie") != "precise=matched" {
				t.Errorf("TOC cookie=%q, want precise-search cookie retained", r.Header.Get("Cookie"))
			}
			_, _ = w.Write([]byte(`<a class="chapter" href="/chapter/1">第一章</a>`))
		case "/chapter/1":
			if r.Header.Get("Cookie") != "precise=matched" {
				t.Errorf("content cookie=%q, want re-searched book session retained", r.Header.Get("Cookie"))
			}
			_, _ = w.Write([]byte(`<article>正文</article>`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	s := NewSearcher(fetcher.NewInsecure(3*time.Second), analyzer.NewJSVMWithPoolSize(1), nil, nil, nil)
	src := booksource.BookSource{
		BookSourceURL:  server.URL,
		BookSourceName: "fixture",
		SearchURL:      server.URL + "/search?q={{key}}",
		RuleSearch:     `{"bookList":"@css:.book","name":"@css:.name@text","author":"@css:.author@text","kind":"@css:.kind@text","bookUrl":"@css:a@href"}`,
		RuleBookInfo:   `{"tocUrl":"@css:.toc@href","lastChapter":"@css:.latest@text","kind":"@css:.kind@text"}`,
		RuleToc: `{
			"preUpdateJs":"java.reGetBook()",
			"chapterList":"@css:.chapter", "chapterName":"text", "chapterUrl":"@href"
		}`,
		RuleContent: `{"content":"@css:article@text"}`,
	}
	book := &Book{Name: "ExistingName", Author: "ExistingAuthor", SourceURL: server.URL, BookURL: server.URL + "/old-book", TocURL: server.URL + "/stale-toc"}

	chapters, err := s.GetChapterListForBook(src, book, book.TocURL)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(requestedPaths, []string{"/search", "/new-book", "/fresh-toc"}) {
		t.Fatalf("requested paths=%v, want precise search then detail then TOC", requestedPaths)
	}
	if book.BookURL != server.URL+"/new-book" || book.TocURL != server.URL+"/fresh-toc" || book.LastChapter != "重新搜索并刷新" || book.Kind != "SearchKind" {
		t.Fatalf("book=%+v, want searched identity and refreshed detail", book)
	}
	if len(chapters) != 1 || chapters[0].Title != "第一章" {
		t.Fatalf("chapters=%+v, want refreshed TOC result", chapters)
	}
	content, _, err := s.GetChapterContentForBook(src, book, &chapters[0], nil)
	if err != nil {
		t.Fatal(err)
	}
	if content != "正文" {
		t.Fatalf("content=%q, want first chapter through retained session", content)
	}
}

func TestGetChapterListReGetBookClearsStaleTOCWhenDetailHasNoTOCURL(t *testing.T) {
	var requestedPaths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPaths = append(requestedPaths, r.URL.Path)
		switch r.URL.Path {
		case "/search":
			_, _ = w.Write([]byte(`<div class="book"><span class="name">ExistingName</span><span class="author">ExistingAuthor</span><a href="/new-book">书籍</a></div>`))
		case "/new-book":
			_, _ = w.Write([]byte(`<a class="chapter" href="/chapter/1">第一章</a>`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	s := NewSearcher(fetcher.NewInsecure(3*time.Second), analyzer.NewJSVMWithPoolSize(1), nil, nil, nil)
	src := booksource.BookSource{
		BookSourceURL: server.URL,
		SearchURL:     server.URL + "/search?q={{key}}",
		RuleSearch:    `{"bookList":"@css:.book","name":"@css:.name@text","author":"@css:.author@text","bookUrl":"@css:a@href"}`,
		RuleBookInfo:  `{}`,
		RuleToc: `{
			"preUpdateJs":"java.reGetBook()",
			"chapterList":"@css:.chapter", "chapterName":"text", "chapterUrl":"@href"
		}`,
	}
	book := &Book{Name: "ExistingName", Author: "ExistingAuthor", SourceURL: server.URL, BookURL: server.URL + "/old-book", TocURL: server.URL + "/stale-toc"}

	chapters, err := s.GetChapterListForBook(src, book, book.TocURL)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(requestedPaths, []string{"/search", "/new-book", "/new-book"}) {
		t.Fatalf("requested paths=%v, want searched detail then new book URL fallback", requestedPaths)
	}
	if book.TocURL != "" || book.BookURL != server.URL+"/new-book" {
		t.Fatalf("book=%+v, want stale TOC cleared after replacement", book)
	}
	if len(chapters) != 1 || chapters[0].Title != "第一章" {
		t.Fatalf("chapters=%+v, want TOC parsed from replacement book URL", chapters)
	}
}

func TestGetChapterListReGetBookStopsWhenExactAuthorIsMissing(t *testing.T) {
	var requestedPaths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPaths = append(requestedPaths, r.URL.Path)
		_, _ = w.Write([]byte(`<div class="book"><span class="name">ExistingName</span><span class="author">OtherAuthor</span><a href="/wrong-book">书籍</a></div>`))
	}))
	defer server.Close()

	s := NewSearcher(fetcher.NewInsecure(3*time.Second), analyzer.NewJSVMWithPoolSize(1), nil, nil, nil)
	src := booksource.BookSource{
		BookSourceURL: server.URL,
		SearchURL:     server.URL + "/search?q={{key}}",
		RuleSearch:    `{"bookList":"@css:.book","name":"@css:.name@text","author":"@css:.author@text","bookUrl":"@css:a@href"}`,
		RuleBookInfo:  `{"tocUrl":"@css:.toc@href"}`,
		RuleToc: `{
			"preUpdateJs":"java.reGetBook()",
			"chapterList":"@css:.chapter", "chapterName":"text", "chapterUrl":"@href"
		}`,
	}
	book := &Book{Name: "ExistingName", Author: "ExistingAuthor", SourceURL: server.URL, BookURL: server.URL + "/old-book", TocURL: server.URL + "/stale-toc"}

	_, err := s.GetChapterListForBook(src, book, book.TocURL)
	if err == nil || !strings.Contains(err.Error(), "preUpdateJs reGetBook") || !strings.Contains(err.Error(), "no exact match") {
		t.Fatalf("err=%v, want contextual exact-search failure", err)
	}
	if !slices.Equal(requestedPaths, []string{"/search"}) {
		t.Fatalf("requested paths=%v, want search only and no detail/TOC fetch", requestedPaths)
	}
	if book.BookURL != server.URL+"/old-book" || book.TocURL != server.URL+"/stale-toc" {
		t.Fatalf("book=%+v, want identity unchanged after failed exact search", book)
	}
}

func TestGetChapterListRefreshTocURLStopsWhenDetailRefreshFails(t *testing.T) {
	var requestedPaths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPaths = append(requestedPaths, r.URL.Path)
		http.Error(w, "detail unavailable", http.StatusBadGateway)
	}))
	defer server.Close()

	s := NewSearcher(fetcher.NewInsecure(3*time.Second), analyzer.NewJSVM(), nil, nil, nil)
	src := booksource.BookSource{
		BookSourceURL: server.URL,
		RuleBookInfo:  `{"tocUrl":"@css:.toc@href"}`,
		RuleToc: `{
			"preUpdateJs":"java.refreshTocUrl()",
			"chapterList":"@css:.chapter", "chapterName":"text", "chapterUrl":"@href"
		}`,
	}
	book := &Book{SourceURL: server.URL, BookURL: server.URL + "/book", TocURL: server.URL + "/stale-toc"}

	_, err := s.GetChapterListForBook(src, book, book.TocURL)
	if err == nil || !strings.Contains(err.Error(), "preUpdateJs") || !strings.Contains(err.Error(), "book info: status 502") {
		t.Fatalf("err=%v, want contextual detail-refresh failure", err)
	}
	if !slices.Equal(requestedPaths, []string{"/book"}) {
		t.Fatalf("requested paths=%v, want detail only and no stale TOC fetch", requestedPaths)
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
