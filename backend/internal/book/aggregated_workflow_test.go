package book

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/otwako/novelreader/internal/analyzer"
	"github.com/otwako/novelreader/internal/booksource"
	"github.com/otwako/novelreader/internal/fetcher"
)

func TestAggregateStyleTypedDataWorkflowReadsContent(t *testing.T) {
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/search":
			if r.URL.Query().Get("q") != "Fixture" {
				http.Error(w, "wrong search query", http.StatusBadRequest)
				return
			}
			_, _ = fmt.Fprint(w, `{"data":[{"id":"book-7","name":"Fixture Novel","author":"Fixture Author"}]}`)
		case "/detail":
			if r.URL.Query().Get("bookId") != "book-7" {
				http.Error(w, "wrong detail id", http.StatusBadRequest)
				return
			}
			_, _ = fmt.Fprint(w, `{"data":{"id":"book-7","name":"Fixture Novel","author":"Fixture Author"}}`)
		case "/catalog":
			if r.URL.Query().Get("bookId") != "book-7" {
				http.Error(w, "wrong catalog id", http.StatusBadRequest)
				return
			}
			_, _ = fmt.Fprint(w, `{"data":[{"id":"chapter-1","title":"Chapter One"}]}`)
		case "/content":
			if r.URL.Query().Get("bookId") != "book-7" || r.URL.Query().Get("chapterId") != "Chapter One" {
				http.Error(w, "wrong content identity", http.StatusBadRequest)
				return
			}
			_, _ = fmt.Fprint(w, `{"content":"Aggregate content is readable."}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer gateway.Close()

	jsLib := fmt.Sprintf(`
var gateway = %q;
function typed(payload, type) {
  return 'data:;base64,' + java.base64Encode(JSON.stringify(payload)) + ',' + JSON.stringify({type:type});
}
`, gateway.URL)
	src := booksource.BookSource{
		ID:             "aggregate-fixture",
		BookSourceURL:  "aggregate-fixture",
		BookSourceName: "Aggregate fixture",
		BookSourceType: 0,
		Enabled:        true,
		JSLib:          jsLib,
		SearchURL:      `@js:typed({query:key,page:page},'fixture-search')`,
		RuleSearch:     `{"bookList":"<js>var request=JSON.parse(java.hexDecodeToString(result)); JSON.parse(java.ajax(gateway+'/search?q='+request.query)).data</js>","name":"$.name","author":"$.author","bookUrl":"<js>typed({bookId:result.id},'fixture-detail')</js>"}`,
		RuleBookInfo:   `{"init":"<js>var request=JSON.parse(java.hexDecodeToString(result)); JSON.parse(java.ajax(gateway+'/detail?bookId='+request.bookId)).data</js>","name":"$.name","author":"$.author","tocUrl":"<js>typed({bookId:result.id},'fixture-catalog')</js>"}`,
		RuleToc:        `{"chapterList":"<js>var request=JSON.parse(java.hexDecodeToString(result)); java.put('book_id',request.bookId); JSON.parse(java.ajax(gateway+'/catalog?bookId='+request.bookId)).data</js>","chapterName":"$.title","chapterUrl":"<js>typed({bookId:java.get('book_id'),chapterId:chapter.title},'fixture-content')</js>"}`,
		RuleContent:    `{"content":"<js>var request=JSON.parse(java.hexDecodeToString(result)); if (request.bookId !== book.getVariable('book_id')) throw new Error('book variable lost'); JSON.parse(java.ajax(gateway+'/content?bookId='+request.bookId+'&chapterId='+java.encodeURI(request.chapterId))).content</js>"}`,
	}

	client := fetcher.NewInsecure(3 * time.Second)
	jsVM := analyzer.NewJSVM()
	jsVM.SetFetcher(client)
	searcher := NewSearcher(client, jsVM, nil, nil, nil)
	results, err := searcher.searchSource(t.Context(), src, "Fixture")
	if err != nil || len(results) != 1 {
		t.Fatalf("search results=%+v err=%v", results, err)
	}

	book := &Book{}
	applySearchResultToBook(book, results[0])
	if _, err := searcher.GetBookInfoForBook(src, book, book.BookURL); err != nil {
		t.Fatal(err)
	}
	chapters, err := searcher.GetChapterListForBook(src, book, book.TocURL)
	if err != nil || len(chapters) != 1 {
		t.Fatalf("chapters=%+v err=%v", chapters, err)
	}
	if book.VariableMap != `{"book_id":"book-7"}` {
		t.Fatalf("book variableMap=%q before content", book.VariableMap)
	}
	if chapters[0].URL == "" {
		t.Fatal("aggregate chapter URL is empty")
	}
	content, title, err := searcher.GetChapterContentForBook(src, book, &chapters[0], nil)
	if err != nil {
		t.Fatal(err)
	}
	if content != "Aggregate content is readable." || title != "" {
		t.Fatalf("content=%q title=%q chapter=%+v book=%+v", content, title, chapters[0], book)
	}
	if book.VariableMap != `{"book_id":"book-7"}` {
		t.Fatalf("book variableMap=%q", book.VariableMap)
	}
}
