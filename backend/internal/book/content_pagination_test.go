// Integration test for content next-page aggregation.
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
