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

func TestGetChapterContentStopsBeforeNextTOCChapter(t *testing.T) {
	nextChapterFetched := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/chapter/1":
			_, _ = w.Write([]byte(`<div class="content">当前章节</div><a class="next" href="/chapter/2">下一章</a>`))
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
