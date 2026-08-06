// Integration test for content next-page aggregation.
package book

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/otwako/novelreader/internal/analyzer"
	"github.com/otwako/novelreader/internal/booksource"
	"github.com/otwako/novelreader/internal/fetcher"
	"github.com/otwako/novelreader/internal/sourceexec"
)

type recordingContentWebViewTransport struct {
	mu        sync.Mutex
	requests  []sourceexec.RequestSpec
	responses map[string]sourceexec.Response
}

func (t *recordingContentWebViewTransport) Do(_ context.Context, spec sourceexec.RequestSpec) (sourceexec.Response, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.requests = append(t.requests, spec)
	return t.responses[spec.URL], nil
}

func TestGetChapterContentUsesRuleWebJSForWebViewPagesWithOptionPrecedence(t *testing.T) {
	ruleWebJS := `<js>
var content = result;
content = content.replace(/<script[^>]*>[\s\S]*?<\/script>/g, '');
content = content.replace(/<img src="\/asset\/fonts\/\d+\.png"[^>]*>/g, '');
result = content;
</js>`
	browser := &recordingContentWebViewTransport{responses: map[string]sourceexec.Response{
		"https://fixture.test/chapter/1": {
			StatusCode: http.StatusOK, FinalURL: "https://fixture.test/chapter/1", Transport: "webview",
			Body: `<div class="content">第一页正文</div><a class="next" href='https://fixture.test/page-2,{"webView":true,"webJs":"optionScript()"}'>下一页</a>`,
		},
		"https://fixture.test/page-2": {
			StatusCode: http.StatusOK, FinalURL: "https://fixture.test/page-2", Transport: "webview",
			Body: `<div class="content">第二页正文</div>`,
		},
	}}
	s := NewSearcher(nil, analyzer.NewJSVM(), nil, nil, nil)
	s.SetWebViewTransportFactory(func(*sourceexec.SourceSession) sourceexec.Transport { return browser })
	ruleContent, err := json.Marshal(map[string]string{
		"content": "@css:.content@text", "nextContentUrl": "@css:.next@href",
		"webJs": ruleWebJS, "sourceRegex": `.*\.(mp3|m4a).*`,
	})
	if err != nil {
		t.Fatal(err)
	}
	src := booksource.BookSource{BookSourceURL: "https://fixture.test", RuleContent: string(ruleContent)}
	content, _, err := s.GetChapterContent(src, `https://fixture.test/chapter/1,{"webView":true}`)
	if err != nil {
		t.Fatal(err)
	}
	if content != "第一页正文\n第二页正文" {
		t.Fatalf("content=%q, want both browser-rendered pages", content)
	}
	browser.mu.Lock()
	defer browser.mu.Unlock()
	if len(browser.requests) != 2 || browser.requests[0].WebJS != ruleWebJS || browser.requests[1].WebJS != "optionScript()" ||
		browser.requests[0].SourceRegex != `.*\.(mp3|m4a).*` || browser.requests[1].SourceRegex != `.*\.(mp3|m4a).*` {
		t.Fatalf("requests=%+v, want rule fallback, URL-option precedence, and sourceRegex pagination", browser.requests)
	}
}

func TestGetChapterContentRoutesSourceRegexOnlyToWebViewPages(t *testing.T) {
	browser := &recordingContentWebViewTransport{responses: map[string]sourceexec.Response{
		"https://fixture.test/audio/1": {
			StatusCode: http.StatusOK, FinalURL: "https://fixture.test/audio/1", Transport: "webview",
			Body: "https://cdn.fixture.test/media/chapter-1.m4a?token=one",
		},
	}}
	s := NewSearcher(nil, analyzer.NewJSVM(), nil, nil, nil)
	s.SetWebViewTransportFactory(func(*sourceexec.SourceSession) sourceexec.Transport { return browser })
	src := booksource.BookSource{
		BookSourceURL:  "https://fixture.test",
		BookSourceType: 1,
		RuleContent:    `{"content":"<js>result</js>","sourceRegex":".*\\.(mp3|m4a).*"}`,
	}
	content, _, err := s.GetChapterContent(src, `https://fixture.test/audio/1,{"webView":true}`)
	if err != nil || content != "https://cdn.fixture.test/media/chapter-1.m4a?token=one" {
		t.Fatalf("content=%q err=%v", content, err)
	}
	browser.mu.Lock()
	defer browser.mu.Unlock()
	if len(browser.requests) != 1 || browser.requests[0].SourceRegex != `.*\.(mp3|m4a).*` {
		t.Fatalf("requests=%+v, want source regex on browser request", browser.requests)
	}
}

func TestGetChapterContentRuleWebJSDoesNotForceWebView(t *testing.T) {
	var browserRequests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<div class="content">普通正文</div>`))
	}))
	defer server.Close()

	s := NewSearcher(fetcher.NewInsecure(3*time.Second), analyzer.NewJSVM(), nil, nil, nil)
	s.SetWebViewTransportFactory(func(*sourceexec.SourceSession) sourceexec.Transport {
		return transportFunc(func(context.Context, sourceexec.RequestSpec) (sourceexec.Response, error) {
			browserRequests.Add(1)
			return sourceexec.Response{}, nil
		})
	})
	src := booksource.BookSource{BookSourceURL: server.URL, RuleContent: `{
		"content":"@css:.content@text",
		"webJs":"<js>var content = result; content = content.replace(/广告/g, ''); result = content;</js>",
		"sourceRegex":".*\\.(mp3|m4a).*"
	}`}
	content, _, err := s.GetChapterContent(src, server.URL+"/chapter/1")
	if err != nil || content != "普通正文" || browserRequests.Load() != 0 {
		t.Fatalf("content=%q err=%v browserRequests=%d, want ordinary HTTP without WebView escalation", content, err, browserRequests.Load())
	}
}

type transportFunc func(context.Context, sourceexec.RequestSpec) (sourceexec.Response, error)

func (f transportFunc) Do(ctx context.Context, spec sourceexec.RequestSpec) (sourceexec.Response, error) {
	return f(ctx, spec)
}

func TestGetChapterContentFollowsAllNextContentURLs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/chapter/1":
			_, _ = w.Write([]byte(`<div class="content">第一页正文</div><a class="next" href="/page-2">第二页</a><a class="next" href="/page-3">第三页</a>`))
		case "/page-2":
			_, _ = w.Write([]byte(`<div class="content">第二页正文</div><a class="next" href="/page-4">递归页</a>`))
		case "/page-3":
			_, _ = w.Write([]byte(`<div class="content">第三页正文</div>`))
		case "/page-4":
			_, _ = w.Write([]byte(`<div class="content">不应递归抓取</div>`))
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
	content, _, err := s.GetChapterContent(src, server.URL+"/chapter/1")
	if err != nil || !strings.Contains(content, "第一页正文") || !strings.Contains(content, "第二页正文") || !strings.Contains(content, "第三页正文") || strings.Contains(content, "不应递归抓取") {
		t.Fatalf("content=%q err=%v, want fixed multi-page set without recursive expansion", content, err)
	}
}

func TestGetChapterContentAppliesReplaceRegexAfterJoiningPages(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/chapter/1":
			_, _ = w.Write([]byte(`<div class="content">第一页正文</div><a class="next" href="/page-2">下一页</a>`))
		case "/page-2":
			_, _ = w.Write([]byte(`<div class="content">第二页正文</div>`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	s := NewSearcher(fetcher.NewInsecure(3*time.Second), analyzer.NewJSVM(), nil, nil, nil)
	src := booksource.BookSource{
		BookSourceURL: server.URL,
		RuleContent:   `{"content":"@css:.content@text","nextContentUrl":"@css:.next@href","replaceRegex":"##第一页正文\\n第二页正文##合并正文"}`,
	}
	content, _, err := s.GetChapterContent(src, server.URL+"/chapter/1")
	if err != nil || content != "合并正文" {
		t.Fatalf("content=%q err=%v, want content-level replacement after page aggregation", content, err)
	}
}

func TestGetChapterContentStopsAtRepeatedPage(t *testing.T) {
	var page1Requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/chapter/1":
			page1Requests.Add(1)
			_, _ = w.Write([]byte(`<div class="content">第一页正文</div><a class="next" href="/page-2">下一页</a>`))
		case "/page-2":
			_, _ = w.Write([]byte(`<div class="content">第二页正文</div><a class="next" href="/chapter/1">回到第一页</a>`))
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
	content, _, err := s.GetChapterContent(src, server.URL+"/chapter/1")
	if err != nil || !strings.Contains(content, "第一页正文") || !strings.Contains(content, "第二页正文") || page1Requests.Load() != 1 {
		t.Fatalf("content=%q err=%v page1Requests=%d, want cycle-safe two pages", content, err, page1Requests.Load())
	}
}

func TestGetChapterContentPaginationPreservesOptionsRetryAndBodyJS(t *testing.T) {
	var attempts atomic.Int32
	var headerOK atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/chapter/1":
			if r.Method == http.MethodPost {
				headerOK.Store(r.Header.Get("X-Page") == "two")
				if attempts.Add(1) == 1 {
					w.WriteHeader(http.StatusBadGateway)
					return
				}
				_, _ = w.Write([]byte(`<div class="content">raw page</div>`))
				return
			}
			_, _ = w.Write([]byte(`<div class="content">第一页正文</div><a class="next" href='/chapter/1,{"method":"POST","body":"page=2","headers":{"X-Page":"two"},"retry":1,"bodyJs":"result.replace(\"raw\",\"transformed\")"}'>第二页</a>`))
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
	content, _, err := s.GetChapterContent(src, server.URL+"/chapter/1")
	if err != nil || !strings.Contains(content, "transformed page") || attempts.Load() != 2 || !headerOK.Load() {
		t.Fatalf("content=%q err=%v attempts=%d header=%v, want option/retry/bodyJs page", content, err, attempts.Load(), headerOK.Load())
	}
}

func TestGetChapterContentReportsPaginationFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/chapter/1" {
			_, _ = w.Write([]byte(`<div class="content">第一页正文</div><a class="next" href="/missing">下一页</a>`))
			return
		}
		http.Error(w, "missing", http.StatusNotFound)
	}))
	defer server.Close()

	s := NewSearcher(fetcher.NewInsecure(3*time.Second), analyzer.NewJSVM(), nil, nil, nil)
	src := booksource.BookSource{
		BookSourceURL: server.URL,
		RuleContent:   `{"content":"@css:.content@text","nextContentUrl":"@css:.next@href"}`,
	}
	_, _, err := s.GetChapterContent(src, server.URL+"/chapter/1")
	var paginationErr *ContentPaginationError
	if err == nil || !errors.As(err, &paginationErr) || paginationErr.PagesFetched != 1 || paginationErr.FailedURL == "" {
		t.Fatalf("err=%v pagination=%+v, want typed next-page failure", err, paginationErr)
	}
}

func TestDeclaredContentRuleWinsOverScriptFallback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<div class="content">declared content</div><script>window.data={"content":"script fallback"}</script>`))
	}))
	defer server.Close()

	s := NewSearcher(fetcher.NewInsecure(3*time.Second), analyzer.NewJSVM(), nil, nil, nil)
	src := booksource.BookSource{
		BookSourceURL: server.URL,
		RuleContent:   `{"content":"@css:.content@text"}`,
	}
	content, _, err := s.GetChapterContent(src, server.URL+"/chapter/1")
	if err != nil || content != "declared content" {
		t.Fatalf("content=%q err=%v, want declared rule result", content, err)
	}
}

func TestDeclaredContentRuleDoesNotUseScriptFallback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<script>{"content":"script content that is intentionally long enough to satisfy the old heuristic fallback and must not replace a broken declared selector"}</script>`))
	}))
	defer server.Close()

	s := NewSearcher(fetcher.NewInsecure(3*time.Second), analyzer.NewJSVM(), nil, nil, nil)
	src := booksource.BookSource{
		BookSourceURL: server.URL,
		RuleContent:   `{"content":"@css:.missing@text"}`,
	}
	content, _, err := s.GetChapterContent(src, server.URL+"/chapter/1")
	if err != nil || content != "" {
		t.Fatalf("content=%q err=%v, want empty declared-rule result without heuristic replacement", content, err)
	}
}

func TestContentWithoutDeclaredRuleUsesScriptDiagnosticFallback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<script>{"content":"script content that is intentionally long enough to remain available only when no content selector is declared by the source"}</script>`))
	}))
	defer server.Close()

	s := NewSearcher(fetcher.NewInsecure(3*time.Second), analyzer.NewJSVM(), nil, nil, nil)
	src := booksource.BookSource{
		BookSourceURL: server.URL,
		RuleContent:   `{"title":"@css:.title@text"}`,
	}
	content, _, err := s.GetChapterContent(src, server.URL+"/chapter/1")
	if err != nil || !strings.Contains(content, "script content") {
		t.Fatalf("content=%q err=%v, want script diagnostic when no content selector is declared", content, err)
	}
}

func TestGetChapterContentFollowsNextContentURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/chapter/1":
			_, _ = w.Write([]byte(`<h1 class="title">第一章</h1><div class="content">第一页正文</div><a class="next" href="/chapter/2">下一页</a>`))
		case "/chapter/2":
			_, _ = w.Write([]byte(`<div class="content">第二页正文</div>`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	s := NewSearcher(fetcher.NewInsecure(3*time.Second), analyzer.NewJSVM(), nil, nil, nil)
	src := booksource.BookSource{
		BookSourceURL:  server.URL,
		BookSourceName: "fixture",
		RuleContent:    `{"title":"@css:.title@text","content":"@css:.content@text","nextContentUrl":"@css:.next@href"}`,
	}

	content, title, err := s.GetChapterContent(src, server.URL+"/chapter/1")
	if err != nil {
		t.Fatal(err)
	}
	if title != "第一章" || !strings.Contains(content, "第一页正文") || !strings.Contains(content, "第二页正文") {
		t.Fatalf("content=%q title=%q, want both pages", content, title)
	}
}
