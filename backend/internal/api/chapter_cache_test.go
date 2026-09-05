// Chapter-cache API tests verify network-first writes and explicit outage fallback.
package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/otwako/novelreader/internal/book"
	"github.com/otwako/novelreader/internal/processor"
)

func TestChapterContentFallsBackToExactCachedCopy(t *testing.T) {
	var mode atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch mode.Load() {
		case 1:
			http.Error(w, "offline", http.StatusServiceUnavailable)
		case 2:
			_, _ = fmt.Fprint(w, `<article class="content"></article>`)
		default:
			_, _ = fmt.Fprint(w, `<article class="content"><p>cached paragraph</p><img src="/chapter.png"></article>`)
		}
	}))
	defer upstream.Close()
	server, closeDB := newWorkflowAPIServer(t)
	defer closeDB()
	raw, _ := json.Marshal([]map[string]interface{}{{
		"bookSourceUrl": upstream.URL, "bookSourceName": "cache fixture", "bookSourceType": 0, "enabled": true,
		"ruleContent": map[string]string{"content": ".content@html"},
	}})
	if response := performAPIRequest(server, http.MethodPost, "/api/sources", raw); response.Code != http.StatusOK {
		t.Fatalf("import status=%d body=%s", response.Code, response.Body.String())
	}
	sources, err := server.standalone.sourceStore.ListEnabled()
	if err != nil || len(sources) != 1 {
		t.Fatalf("sources=%+v err=%v", sources, err)
	}
	if err := server.standalone.bookStore.AddBook(&book.Book{ID: "book", Name: "Book", SourceID: sources[0].ID, SourceURL: upstream.URL, BookURL: upstream.URL}); err != nil {
		t.Fatal(err)
	}
	chapter := book.Chapter{Index: 0, Title: "Chapter", URL: upstream.URL + "/chapter"}
	if err := server.standalone.bookStore.SaveChapters("book", []book.Chapter{chapter}); err != nil {
		t.Fatal(err)
	}

	response := performAPIRequest(server, http.MethodGet, "/api/books/book/chapters/0/content", nil)
	var fresh chapterContentResponse
	if err := json.Unmarshal(response.Body.Bytes(), &fresh); err != nil || response.Code != http.StatusOK || fresh.OfflineCopy || fresh.Version != proseDocumentVersion || fresh.Document.Kind != "prose" || len(fresh.Document.Blocks) != 2 || fresh.Document.Blocks[1].Resource == nil {
		t.Fatalf("fresh status=%d result=%+v err=%v body=%s", response.Code, fresh, err, response.Body.String())
	}
	for _, index := range []string{"00", "1"} {
		response := performAPIRequest(server, http.MethodGet, "/api/books/book/chapters/"+index+"/content", nil)
		if response.Code != http.StatusNotFound {
			t.Fatalf("non-exact chapter %q: status=%d", index, response.Code)
		}
	}
	var cached chapterContentResponse
	for _, upstreamMode := range []int32{2, 1} {
		mode.Store(upstreamMode)
		response = performAPIRequest(server, http.MethodGet, "/api/books/book/chapters/0/content", nil)
		if err := json.Unmarshal(response.Body.Bytes(), &cached); err != nil || response.Code != http.StatusOK || !cached.OfflineCopy || cached.Version != fresh.Version || cached.Document.Title != fresh.Document.Title || len(cached.Document.Blocks) != len(fresh.Document.Blocks) || cached.Document.Blocks[1].Resource == nil || cached.Document.Blocks[1].Resource.Href != fresh.Document.Blocks[1].Resource.Href {
			t.Fatalf("mode=%d cached status=%d result=%+v err=%v body=%s", upstreamMode, response.Code, cached, err, response.Body.String())
		}
	}
	chapter.URL = upstream.URL + "/changed"
	if err := server.standalone.bookStore.SaveChapters("book", []book.Chapter{chapter}); err != nil {
		t.Fatal(err)
	}
	if response := performAPIRequest(server, http.MethodGet, "/api/books/book/chapters/0/content", nil); response.Code == http.StatusOK {
		t.Fatalf("changed URL unexpectedly used cache: %s", response.Body.String())
	}
}

func TestChapterContentTranslatesLegacyTextBlocksAtResponseSeam(t *testing.T) {
	blocks := []processor.ProseBlock{
		{Kind: "text", Text: "before"},
		{Kind: processor.ProseBlockImage, Src: "https://source.test/image"},
		{Kind: "text", Text: "after"},
	}

	response := newChapterContentResponse("book", 2, "Chapter", nil, blocks, true)
	if len(response.Document.Blocks) != 3 || response.Document.Blocks[0].Kind != processor.ProseBlockParagraph || response.Document.Blocks[1].Kind != processor.ProseBlockImage || response.Document.Blocks[2].Kind != processor.ProseBlockParagraph {
		t.Fatalf("blocks=%+v", response.Document.Blocks)
	}
	if response.Document.Blocks[1].Resource == nil || response.Document.Blocks[1].Resource.Href != "/api/books/book/chapters/2/images/0" {
		t.Fatalf("image block=%+v", response.Document.Blocks[1])
	}
}
