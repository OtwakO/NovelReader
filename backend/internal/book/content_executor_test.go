// Integration test for chapter content requests using Legado URL options.
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

func TestGetChapterContentUsesChapterURLOptions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if err := r.ParseForm(); err != nil || r.Form.Get("id") != "1" {
			t.Errorf("form = %v, want id=1", r.Form)
		}
		_, _ = w.Write([]byte(`<h1 class="title">第一章</h1><div class="content">正文内容在这里。</div>`))
	}))
	defer server.Close()

	s := NewSearcher(fetcher.NewInsecure(3*time.Second), analyzer.NewJSVM(), nil, nil, nil)
	src := booksource.BookSource{
		BookSourceURL:  server.URL,
		BookSourceName: "fixture",
		RuleContent:    `{"title":"@css:.title@text","content":"@css:.content@text"}`,
	}

	content, title, err := s.GetChapterContent(src, `/chapter,{"method":"POST","body":"id=1"}`)
	if err != nil {
		t.Fatal(err)
	}
	if title != "第一章" || content != "正文内容在这里。" {
		t.Fatalf("content=%q title=%q", content, title)
	}
}
