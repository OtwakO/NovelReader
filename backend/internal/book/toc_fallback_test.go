// Declared TOC rules must not be replaced by heuristic catalog discovery.
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

func TestChapterListKeepsDeclaredBookPageTOC(t *testing.T) {
	catalogRequests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/book":
			_, _ = w.Write([]byte(`<a class="chapter" href="/book/1">Declared One</a><a class="chapter" href="/book/2">Declared Two</a><a href="/catalog">Catalog</a>`))
		case "/catalog":
			catalogRequests++
			_, _ = w.Write([]byte(`<a class="chapter" href="/chapter/wrong">Heuristic Result</a>`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	searcher := NewSearcher(fetcher.NewInsecure(2*time.Second), analyzer.NewJSVM(), nil, nil, nil)
	source := booksource.BookSource{
		BookSourceURL: server.URL,
		RuleToc:       `{"chapterList":".chapter","chapterName":"text","chapterUrl":"href"}`,
	}
	chapters, err := searcher.GetChapterListForBook(source, &Book{BookURL: server.URL + "/book"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(chapters) != 2 || chapters[0].Title != "Declared One" || chapters[1].Title != "Declared Two" || catalogRequests != 0 {
		t.Fatalf("chapters=%+v catalogRequests=%d, want declared book-page TOC only", chapters, catalogRequests)
	}
}
