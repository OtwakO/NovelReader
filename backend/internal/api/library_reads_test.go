package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/otwako/novelreader/internal/book"
	"github.com/otwako/novelreader/internal/library"
)

func TestLibraryReadsSeparateSharedStateFromBookSourceContext(t *testing.T) {
	server, closeDB := newWorkflowAPIServer(t)
	defer closeDB()
	if err := server.standalone.bookStore.AddBook(&book.Book{ID: "native", Name: "Same title", SourceID: "source", SourceURL: "https://source.test", BookURL: "https://source.test/book", Origin: "Fixture source"}); err != nil {
		t.Fatal(err)
	}
	tx, err := server.standalone.db.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if err := library.InsertTx(t.Context(), tx, library.Item{ID: "independent", Provider: "fixture", Name: "Same title", ContentRevision: 3, StateVersion: 2}); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}

	response := performAPIRequest(server, http.MethodGet, "/api/books", nil)
	var items []map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &items); err != nil || response.Code != http.StatusOK || len(items) != 2 {
		t.Fatalf("shelf status=%d body=%s err=%v", response.Code, response.Body.String(), err)
	}
	for _, item := range items {
		id := item["id"].(string)
		detail := performAPIRequest(server, http.MethodGet, "/api/books/"+id, nil)
		var shared map[string]any
		if err := json.Unmarshal(detail.Body.Bytes(), &shared); err != nil || detail.Code != http.StatusOK {
			t.Fatalf("detail status=%d body=%s err=%v", detail.Code, detail.Body.String(), err)
		}
		for _, value := range []map[string]any{item, shared} {
			if value["name"] != "Same title" || value["provider"] == nil || value["contentRevision"] == nil {
				t.Fatalf("shared fields missing: %+v", value)
			}
			for _, field := range []string{"sourceId", "sourceUrl", "bookUrl", "variableMap", "activeSource", "alternateSources"} {
				if _, exists := value[field]; exists {
					t.Fatalf("native field %q leaked into shared response", field)
				}
			}
		}
	}
	native := performAPIRequest(server, http.MethodGet, "/api/books/native/booksource", nil)
	var context book.Book
	if err := json.Unmarshal(native.Body.Bytes(), &context); err != nil || native.Code != http.StatusOK || context.SourceID != "source" || context.Name != "Same title" {
		t.Fatalf("native status=%d body=%s err=%v", native.Code, native.Body.String(), err)
	}
	for _, path := range []string{"/api/books/independent/booksource", "/api/books/missing"} {
		if response := performAPIRequest(server, http.MethodGet, path, nil); response.Code != http.StatusNotFound {
			t.Fatalf("%s status=%d body=%s", path, response.Code, response.Body.String())
		}
	}
}

func TestLibraryRemovalCascadesBookSourceAndSharedChildren(t *testing.T) {
	server, closeDB := newWorkflowAPIServer(t)
	defer closeDB()
	if err := server.standalone.bookStore.AddBook(&book.Book{ID: "native", Name: "Novel", SourceID: "source", SourceURL: "source", BookURL: "book"}); err != nil {
		t.Fatal(err)
	}
	if err := server.standalone.bookStore.SaveChapters("native", []book.Chapter{{Index: 0, Title: "One", URL: "chapter"}}); err != nil {
		t.Fatal(err)
	}
	if err := server.standalone.bookStore.SaveChapterCache(book.CachedChapter{BookID: "native", SourceID: "source", ContentRevision: 1, ChapterIndex: 0, ChapterURL: "chapter", Paragraphs: []string{"text"}}); err != nil {
		t.Fatal(err)
	}
	response := performAPIRequest(server, http.MethodPost, "/api/books/native/bookmarks", []byte(`{"id":"mark","contentRevision":1,"stateVersion":0,"chapterIndex":0,"position":0.5}`))
	if response.Code != http.StatusCreated {
		t.Fatalf("bookmark status=%d body=%s", response.Code, response.Body.String())
	}
	for _, table := range []string{"books", "chapters", "chapter_cache", "bookmarks", "library_items"} {
		var count int
		if err := server.standalone.db.QueryRow(`SELECT count(*) FROM ` + table).Scan(&count); err != nil || count != 1 {
			t.Fatalf("before deletion %s count=%d err=%v", table, count, err)
		}
	}
	response = performAPIRequest(server, http.MethodDelete, "/api/books?id=native", nil)
	if response.Code != http.StatusOK {
		t.Fatalf("delete status=%d body=%s", response.Code, response.Body.String())
	}
	for _, table := range []string{"books", "chapters", "chapter_cache", "bookmarks", "library_items"} {
		var count int
		if err := server.standalone.db.QueryRow(`SELECT count(*) FROM ` + table).Scan(&count); err != nil || count != 0 {
			t.Fatalf("after deletion %s count=%d err=%v", table, count, err)
		}
	}
}
