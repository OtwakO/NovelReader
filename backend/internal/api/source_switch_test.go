// Source-switch API tests prove target TOCs before atomically replacing active state.
package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/otwako/novelreader/internal/book"
)

func TestSourceSwitchValidatesTargetAndMigratesCanonicalProgress(t *testing.T) {
	primary := newSwitchFixture(t, []string{"第1章 开始", "第2章 风雨", "第3章 结束"})
	target := newSwitchFixture(t, []string{"序", "第 2 章：风雨！", "终"})
	bad := newSwitchFixture(t, nil)
	server, closeDB := newWorkflowAPIServer(t)
	defer closeDB()

	for name, sourceURL := range map[string]string{"Primary": primary.URL, "Target": target.URL, "Bad": bad.URL} {
		raw, _ := json.Marshal([]map[string]interface{}{{
			"bookSourceUrl": sourceURL, "bookSourceName": name, "bookSourceType": 0, "enabled": true,
			"ruleBookInfo": map[string]string{"name": ".name@text", "tocUrl": ".toc@href"},
			"ruleToc":      map[string]string{"chapterList": ".chapter", "chapterName": "text", "chapterUrl": "href"},
			"ruleContent":  map[string]string{"content": ".content@text"},
		}})
		response := performAPIRequest(server, http.MethodPost, "/api/sources", raw)
		if response.Code != http.StatusOK {
			t.Fatalf("import %s status=%d body=%s", name, response.Code, response.Body.String())
		}
	}

	sources, err := server.sourceStore.ListEnabled()
	if err != nil || len(sources) != 3 {
		t.Fatalf("sources=%+v err=%v", sources, err)
	}
	sourceID := func(sourceURL string) string {
		for index := range sources {
			if sources[index].BookSourceURL == sourceURL {
				return sources[index].ID
			}
		}
		return ""
	}

	stored := &book.Book{
		ID: "book-1", Name: "Fixture", SourceID: sourceID(primary.URL), SourceURL: primary.URL, BookURL: primary.URL + "/book", TocURL: primary.URL + "/toc", Origin: "Primary",
		ActiveSource: &book.AltSource{SourceID: sourceID(primary.URL), SourceURL: primary.URL, BookURL: primary.URL + "/book", SourceName: "Primary", DiscoveryQuery: "Fixture@primary", SourceGroup: "Primary type", Capabilities: []string{"search"}},
		AlternateSources: []book.AltSource{
			{SourceID: sourceID(target.URL), SourceURL: target.URL, BookURL: target.URL + "/book", SourceName: "Target", DiscoveryQuery: "Fixture@target", SourceGroup: "Target type", Capabilities: []string{"explore"}},
			{SourceID: sourceID(bad.URL), SourceURL: bad.URL, BookURL: bad.URL + "/book", SourceName: "Bad"},
		},
	}
	if err := server.bookStore.AddBook(stored); err != nil {
		t.Fatal(err)
	}
	if response := waitForCatalogRoute(t, server, "book-1"); response.Code != http.StatusOK {
		t.Fatalf("primary toc status=%d body=%s", response.Code, response.Body.String())
	}
	progressBody, _ := json.Marshal(map[string]interface{}{"sourceId": sourceID(primary.URL), "stateVersion": 0, "chapterIndex": 1, "position": 0.65})
	if response := performAPIRequest(server, http.MethodPut, "/api/books/book-1/progress", progressBody); response.Code != http.StatusOK {
		t.Fatalf("progress status=%d body=%s", response.Code, response.Body.String())
	}

	for _, mark := range []map[string]interface{}{
		{"id": "matching", "sourceId": sourceID(primary.URL), "stateVersion": 1, "chapterIndex": 1, "position": 0.3, "note": "matched note"},
		{"id": "orphan", "sourceId": sourceID(primary.URL), "stateVersion": 1, "chapterIndex": 2, "position": 0.8, "note": "orphan note"},
	} {
		body, _ := json.Marshal(mark)
		if response := performAPIRequest(server, http.MethodPost, "/api/books/book-1/bookmarks", body); response.Code != http.StatusCreated {
			t.Fatalf("bookmark status=%d body=%s", response.Code, response.Body.String())
		}
	}

	badRequest, _ := json.Marshal(map[string]string{"sourceId": sourceID(bad.URL), "sourceUrl": bad.URL, "bookUrl": bad.URL + "/book"})
	if response := performAPIRequest(server, http.MethodPut, "/api/books/book-1/source", badRequest); response.Code != http.StatusBadGateway {
		t.Fatalf("bad target status=%d body=%s", response.Code, response.Body.String())
	}
	assertStoredSource(t, server, primary.URL, 1, 0.65, 1, 3)

	targetRequest, _ := json.Marshal(map[string]string{"sourceId": sourceID(target.URL), "sourceUrl": target.URL, "bookUrl": target.URL + "/book"})
	response := performAPIRequest(server, http.MethodPut, "/api/books/book-1/source", targetRequest)
	if response.Code != http.StatusOK {
		t.Fatalf("switch status=%d body=%s", response.Code, response.Body.String())
	}
	var result struct {
		Book    book.Book `json:"book"`
		Mapping string    `json:"mapping"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil || result.Mapping != "title" {
		t.Fatalf("result=%+v err=%v body=%s", result, err, response.Body.String())
	}
	assertStoredSource(t, server, target.URL, 1, 0.65, 2, 3)
	bookmarkResponse := performAPIRequest(server, http.MethodGet, "/api/books/book-1/bookmarks", nil)
	var bookmarks []book.Bookmark
	if err := json.Unmarshal(bookmarkResponse.Body.Bytes(), &bookmarks); err != nil || len(bookmarks) != 2 {
		t.Fatalf("bookmarks=%+v err=%v body=%s", bookmarks, err, bookmarkResponse.Body.String())
	}
	byID := map[string]book.Bookmark{bookmarks[0].ID: bookmarks[0], bookmarks[1].ID: bookmarks[1]}
	if byID["matching"].Orphaned || byID["matching"].ChapterIndex != 1 || !byID["orphan"].Orphaned {
		t.Fatalf("migrated bookmarks=%+v", byID)
	}
	staleProgress, _ := json.Marshal(map[string]interface{}{"sourceId": sourceID(primary.URL), "stateVersion": 1, "chapterIndex": 0, "position": 0.1})
	if response := performAPIRequest(server, http.MethodPut, "/api/books/book-1/progress", staleProgress); response.Code != http.StatusConflict {
		t.Fatalf("stale progress status=%d body=%s", response.Code, response.Body.String())
	}
	assertStoredSource(t, server, target.URL, 1, 0.65, 2, 3)
	if result.Book.ActiveSource == nil || result.Book.ActiveSource.DiscoveryQuery != "Fixture@target" || result.Book.ActiveSource.SourceGroup != "Target type" || len(result.Book.ActiveSource.Capabilities) != 1 {
		t.Fatalf("active target metadata was not preserved: %+v", result.Book.ActiveSource)
	}
	if len(result.Book.AlternateSources) != 2 || result.Book.AlternateSources[0].SourceURL != primary.URL || result.Book.AlternateSources[0].DiscoveryQuery != "Fixture@primary" || result.Book.AlternateSources[0].SourceGroup != "Primary type" || len(result.Book.AlternateSources[0].Capabilities) != 1 {
		t.Fatalf("switch was not reversible: %+v", result.Book.AlternateSources)
	}
	primaryRequest, _ := json.Marshal(map[string]string{"sourceId": sourceID(primary.URL), "sourceUrl": primary.URL, "bookUrl": primary.URL + "/book"})
	if response := performAPIRequest(server, http.MethodPut, "/api/books/book-1/source", primaryRequest); response.Code != http.StatusOK {
		t.Fatalf("switch back status=%d body=%s", response.Code, response.Body.String())
	}
	assertStoredSource(t, server, primary.URL, 1, 0.65, 3, 3)
	restored, err := server.bookStore.GetBook("book-1")
	if err != nil || restored.ActiveSource == nil || restored.ActiveSource.DiscoveryQuery != "Fixture@primary" || len(restored.AlternateSources) != 2 || restored.AlternateSources[0].DiscoveryQuery != "Fixture@target" {
		t.Fatalf("A-B-A binding metadata was not preserved: book=%+v err=%v", restored, err)
	}
	bookmarkResponse = performAPIRequest(server, http.MethodGet, "/api/books/book-1/bookmarks", nil)
	if err := json.Unmarshal(bookmarkResponse.Body.Bytes(), &bookmarks); err != nil {
		t.Fatal(err)
	}
	byID = map[string]book.Bookmark{bookmarks[0].ID: bookmarks[0], bookmarks[1].ID: bookmarks[1]}
	if byID["orphan"].Orphaned || byID["orphan"].ChapterIndex != 2 {
		t.Fatalf("restored bookmark=%+v", byID["orphan"])
	}
	if response := performAPIRequest(server, http.MethodPut, "/api/books/book-1/progress", staleProgress); response.Code != http.StatusConflict {
		t.Fatalf("A-B-A stale progress status=%d body=%s", response.Code, response.Body.String())
	}
}

func newSwitchFixture(t *testing.T, titles []string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/book":
			_, _ = fmt.Fprint(w, `<h1 class="name">Fixture</h1><a class="toc" href="/toc">TOC</a>`)
		case "/toc":
			for index, title := range titles {
				_, _ = fmt.Fprintf(w, `<a class="chapter" href="/chapter/%d">%s</a>`, index, title)
			}
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func assertStoredSource(t *testing.T, server *Server, sourceURL string, index int, position float64, version int64, chapterCount int) {
	t.Helper()
	stored, err := server.bookStore.GetBook("book-1")
	if err != nil || stored.SourceURL != sourceURL || stored.DurChapterIndex != index || stored.DurChapterPos != position || stored.StateVersion != version {
		t.Fatalf("stored=%+v err=%v", stored, err)
	}
	chapters, err := server.bookStore.GetChapters("book-1")
	if err != nil || len(chapters) != chapterCount {
		t.Fatalf("chapters=%d err=%v", len(chapters), err)
	}
}
