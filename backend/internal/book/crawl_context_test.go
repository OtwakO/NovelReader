// Integration tests for complete book/chapter JavaScript context across crawl stages.
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

func TestTOCRulesReceiveCompleteBookAndChapterContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<a class="chapter" href="/chapter/1">ignored</a>`))
	}))
	defer server.Close()

	s := NewSearcher(fetcher.NewInsecure(3*time.Second), analyzer.NewJSVM(), nil, nil, nil)
	src := booksource.BookSource{BookSourceURL: server.URL, BookSourceName: "Fixture", RuleToc: `{
		"chapterList":"@css:.chapter",
		"chapterName":"<js>book.name+'|'+book.author+'|'+chapter.index+'|'+chapter.bookUrl+'|'+chapter.baseUrl</js>",
		"chapterUrl":"@href"
	}`}
	b := &Book{SourceURL: server.URL, BookURL: server.URL + "/book", Origin: "Fixture", Name: "Context Book", Author: "Context Author"}

	chapters, err := s.GetChapterListForBook(src, b, server.URL+"/toc")
	if err != nil {
		t.Fatal(err)
	}
	if len(chapters) != 1 {
		t.Fatalf("chapters = %+v", chapters)
	}
	want := "Context Book|Context Author|0|" + server.URL + "/book|" + server.URL + "/toc"
	if chapters[0].Title != want {
		t.Fatalf("title = %q, want %q", chapters[0].Title, want)
	}
}

func TestTOCFormatJSRunsAfterFinalOrderingWithPersistentBindings(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<a class="chapter" href="/chapter/1">One</a><a class="chapter" href="/chapter/2">Two</a><a class="chapter" href="/chapter/2">Duplicate Two</a><a class="chapter" href="/chapter/3">Three</a>`))
	}))
	defer server.Close()

	s := NewSearcher(fetcher.NewInsecure(3*time.Second), analyzer.NewJSVMWithPoolSize(1), nil, nil, nil)
	src := booksource.BookSource{BookSourceURL: server.URL, BookSourceName: "Fixture", RuleToc: `{
		"chapterList":"@css:.chapter", "chapterName":"text", "chapterUrl":"@href",
		"formatJs":"gInt += index; chapter.tag = 'tag-' + index; if (index === 2) throw new Error('skip second'); title + '|' + index + '|' + chapter.index + '|' + gInt"
	}`}

	chapters, err := s.GetChapterListForBook(src, &Book{BookURL: server.URL + "/book"}, server.URL+"/toc")
	if err != nil {
		t.Fatal(err)
	}
	if len(chapters) != 3 {
		t.Fatalf("chapters=%+v, want three formatted chapters", chapters)
	}
	if chapters[0].Title != "One|1|0|1" || chapters[0].Tag != "tag-1" {
		t.Fatalf("first=%+v, want one-based format index and zero-based chapter index", chapters[0])
	}
	if chapters[1].Title != "Duplicate Two" || chapters[1].Tag != "tag-2" {
		t.Fatalf("second=%+v, want mutation retained but title unchanged after format error", chapters[1])
	}
	if chapters[2].Title != "Three|3|2|6" || chapters[2].Tag != "tag-3" {
		t.Fatalf("third=%+v, want formatting to continue with persistent gInt", chapters[2])
	}
}

func TestTOCRulesPersistBookVariablesAcrossFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<a class="chapter" href="/chapter/1">ignored</a>`))
	}))
	defer server.Close()

	s := NewSearcher(fetcher.NewInsecure(3*time.Second), analyzer.NewJSVM(), nil, nil, nil)
	src := booksource.BookSource{BookSourceURL: server.URL, BookSourceName: "Fixture", RuleToc: `{
		"chapterList":"@css:.chapter",
		"chapterName":"<js>book.putVariable('token','persisted'); 'Chapter'</js>",
		"chapterUrl":"<js>book.getVariable('token') + '.html'</js>"
	}`}
	b := &Book{BookURL: server.URL + "/book"}

	chapters, err := s.GetChapterListForBook(src, b, server.URL+"/toc")
	if err != nil || len(chapters) != 1 {
		t.Fatalf("chapters=%+v err=%v", chapters, err)
	}
	if chapters[0].URL != server.URL+"/persisted.html" {
		t.Fatalf("chapter URL=%q", chapters[0].URL)
	}
	if b.VariableMap != `{"token":"persisted"}` {
		t.Fatalf("book variableMap=%q", b.VariableMap)
	}
}

func TestTOCFormatJSSupportsActiveSuffixRemovalContract(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<a class="chapter" href="/chapter/1">第一章【123456】</a><a class="chapter" href="/chapter/2">第二章</a>`))
	}))
	defer server.Close()

	s := NewSearcher(fetcher.NewInsecure(3*time.Second), analyzer.NewJSVMWithPoolSize(1), nil, nil, nil)
	src := booksource.BookSource{BookSourceURL: server.URL, RuleToc: `{
		"chapterList":"@css:.chapter", "chapterName":"text", "chapterUrl":"@href",
		"formatJs":"title.replace(/【\\d{6}】$/,'')"
	}`}
	chapters, err := s.GetChapterListForBook(src, &Book{BookURL: server.URL + "/book"}, server.URL+"/toc")
	if err != nil {
		t.Fatal(err)
	}
	if len(chapters) != 2 || chapters[0].Title != "第一章" || chapters[1].Title != "第二章" {
		t.Fatalf("chapters=%+v, want active formatJs suffix removal", chapters)
	}
}

func TestTOCPaginationURLReceivesBookContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/toc":
			_, _ = w.Write([]byte(`<a class="chapter" href="/chapter/1">第一章</a>`))
		case "/toc-2":
			_, _ = w.Write([]byte(`<a class="chapter" href="/chapter/2">第二章</a>`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	s := NewSearcher(fetcher.NewInsecure(3*time.Second), analyzer.NewJSVM(), nil, nil, nil)
	src := booksource.BookSource{BookSourceURL: server.URL, BookSourceName: "Fixture", RuleToc: `{
		"chapterList":"@css:.chapter", "chapterName":"text", "chapterUrl":"@href",
		"nextTocUrl":"<js>book.name === 'Context Book' ? '/toc-2' : ''</js>"
	}`}
	b := &Book{SourceURL: server.URL, BookURL: server.URL + "/book", Origin: "Fixture", Name: "Context Book"}
	chapters, err := s.GetChapterListForBook(src, b, server.URL+"/toc")
	if err != nil || len(chapters) != 2 {
		t.Fatalf("chapters=%+v err=%v, want two context-selected pages", chapters, err)
	}
}

func TestTOCChapterMutationsSurviveFieldRules(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<div class="entry"><span class="name">Chapter Title</span></div>`))
	}))
	defer server.Close()

	s := NewSearcher(fetcher.NewInsecure(3*time.Second), analyzer.NewJSVM(), nil, nil, nil)
	src := booksource.BookSource{BookSourceURL: server.URL, BookSourceName: "Fixture", RuleToc: `{
		"chapterList":"@css:.entry", "chapterName":"<js>chapter.custom='custom'; 'Chapter Title'</js>",
		"chapterUrl":"<js>chapter.custom + '.html'</js>"
	}`}
	chapters, err := s.GetChapterListForBook(src, &Book{BookURL: server.URL + "/book"}, server.URL+"/toc")
	if err != nil || len(chapters) != 1 || chapters[0].URL != server.URL+"/custom.html" {
		t.Fatalf("chapters=%+v err=%v, want mutation-derived URL", chapters, err)
	}
}

func TestTOCChapterURLSeesExtractedTitle(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<div class="entry"><span class="name">Chapter Title</span></div>`))
	}))
	defer server.Close()

	s := NewSearcher(fetcher.NewInsecure(3*time.Second), analyzer.NewJSVM(), nil, nil, nil)
	src := booksource.BookSource{BookSourceURL: server.URL, BookSourceName: "Fixture", RuleToc: `{
		"chapterList":"@css:.entry", "chapterName":"@css:.name@text",
		"chapterUrl":"<js>chapter.title + '.html'</js>"
	}`}
	chapters, err := s.GetChapterListForBook(src, &Book{BookURL: server.URL + "/book"}, server.URL+"/toc")
	if err != nil || len(chapters) != 1 || chapters[0].URL != server.URL+"/Chapter%20Title.html" {
		t.Fatalf("chapters=%+v err=%v, want title-derived URL", chapters, err)
	}
}

func TestTOCChapterFieldsFollowLegadoFallbacks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<div class="entry volume"><span class="name">Volume One</span><span class="volume-flag">yes</span></div><div class="entry ordinary"><span class="name">Chapter One</span><span class="pay">paid</span></div><div class="entry null"><span class="name">Null Volume</span><span class="volume-flag">null</span><a href="/chapter/null">link</a></div>`))
	}))
	defer server.Close()

	s := NewSearcher(fetcher.NewInsecure(3*time.Second), analyzer.NewJSVM(), nil, nil, nil)
	src := booksource.BookSource{BookSourceURL: server.URL, BookSourceName: "Fixture", RuleToc: `{
		"chapterList":"@css:.entry", "chapterName":"@css:.name@text", "chapterUrl":"@css:a@href",
		"isVolume":"@css:.volume-flag@text", "isVip":"<js>chapter.isVolume === false && chapter.url.indexOf('/toc') >= 0 ? 'yes' : 'no'</js>",
		"isPay":"@css:.pay@text"
	}`}
	b := &Book{SourceURL: server.URL, BookURL: server.URL + "/book", Origin: "Fixture"}
	chapters, err := s.GetChapterListForBook(src, b, server.URL+"/toc")
	if err != nil || len(chapters) != 3 {
		t.Fatalf("chapters=%+v err=%v", chapters, err)
	}
	if !chapters[0].IsVolume || chapters[0].IsVip || chapters[0].URL != "Volume One0" {
		t.Fatalf("volume=%+v, want synthetic volume URL", chapters[0])
	}
	if chapters[1].IsVolume || chapters[1].URL != server.URL+"/toc" || !chapters[1].IsPay || !chapters[1].IsVip {
		t.Fatalf("ordinary=%+v, want page URL fallback, vip=true, and pay=true", chapters[1])
	}
	if chapters[2].IsVolume || chapters[2].URL != server.URL+"/chapter/null" {
		t.Fatalf("null=%+v, want false volume and a distinct chapter URL", chapters[2])
	}
}

func TestContentRulesReceiveCurrentAndExactNextChapterContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<div class="content">ignored</div>`))
	}))
	defer server.Close()

	s := NewSearcher(fetcher.NewInsecure(3*time.Second), analyzer.NewJSVM(), nil, nil, nil)
	src := booksource.BookSource{
		BookSourceURL:  server.URL,
		BookSourceName: "Fixture",
		RuleContent: `{
			"title":"<js>book.name+':'+chapter.title+':'+nextChapter.title</js>",
			"content":"<js>book.author+'|'+chapter.index+'|'+chapter.url+'|'+chapter.baseUrl+'|'+nextChapter.url</js>"
		}`,
	}
	b := &Book{SourceURL: server.URL, BookURL: server.URL + "/book", Origin: "Fixture", Name: "Context Book", Author: "Context Author"}
	current := &Chapter{Index: 4, Title: "Current", URL: server.URL + "/chapter/4", BaseURL: server.URL + "/toc"}
	next := &Chapter{Index: 5, Title: "Next", URL: server.URL + "/chapter/5", BaseURL: server.URL + "/toc"}

	content, title, err := s.GetChapterContentForBook(src, b, current, next)
	if err != nil {
		t.Fatal(err)
	}
	wantTitle := "Context Book:Current:Next"
	wantContent := "Context Author|4|" + server.URL + "/chapter/4" + "|" + server.URL + "/toc|" + server.URL + "/chapter/5"
	if title != wantTitle || content != wantContent {
		t.Fatalf("title=%q content=%q, want title=%q content=%q", title, content, wantTitle, wantContent)
	}
}
