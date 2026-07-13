// Integration test for book-info requests using Legado URL options.
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

func TestGetBookInfoUsesDetailURLOptions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/book" {
			if r.Method != http.MethodPost {
				t.Errorf("method = %s, want POST", r.Method)
			}
			if err := r.ParseForm(); err != nil || r.Form.Get("id") != "1" {
				t.Errorf("form = %v, want id=1", r.Form)
			}
			http.Redirect(w, r, "/details/", http.StatusFound)
			return
		}
		_, _ = w.Write([]byte(`<html><h1 class="name">凡人修仙传</h1><span class="author">忘语</span><img class="cover" src="cover.jpg"></html>`))
	}))
	defer server.Close()

	s := NewSearcher(fetcher.NewInsecure(3*time.Second), analyzer.NewJSVM(), nil, nil, nil)
	src := booksource.BookSource{
		BookSourceURL:  server.URL,
		BookSourceName: "fixture",
		RuleBookInfo:   `{"name":".name@text","author":".author@text","coverUrl":".cover@src"}`,
	}

	book, err := s.GetBookInfo(src, `/book,{"method":"POST","body":"id=1"}`)
	if err != nil {
		t.Fatal(err)
	}
	if book.Name != "凡人修仙传" || book.Author != "忘语" {
		t.Fatalf("book = %+v, want parsed detail", book)
	}
	if book.CoverURL != server.URL+"/details/cover.jpg" {
		t.Fatalf("cover URL = %q, want final-page-relative URL", book.CoverURL)
	}
}
