package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/otwako/novelreader/internal/book"
	"github.com/otwako/novelreader/internal/booksource"
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
	// Until that provider's lifecycle is routed here, shared deletion must not
	// bypass it and orphan native records or files.
	if got := performAPIRequest(server, http.MethodDelete, "/api/books?id=independent", nil); got.Code != http.StatusNotImplemented {
		t.Fatalf("unrouted provider deletion: status=%d body=%s", got.Code, got.Body.String())
	}
	if got := performAPIRequest(server, http.MethodGet, "/api/books/independent", nil); got.Code != http.StatusOK {
		t.Fatalf("unrouted publication was lost: status=%d", got.Code)
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

func TestReaderEntryQualification(t *testing.T) {
	server, closeDB := newWorkflowAPIServer(t)
	defer closeDB()
	source := &booksource.BookSource{ID: "entry-source", BookSourceURL: "https://source.test", BookSourceName: "Synthetic source"}
	if err := server.standalone.sourceStore.Upsert(source); err != nil {
		t.Fatal(err)
	}
	if err := server.standalone.bookStore.AddBook(&book.Book{ID: "entry-book", Name: "Novel", SourceID: source.ID, BookURL: "https://source.test/book"}); err != nil {
		t.Fatal(err)
	}
	read := func() libraryBookResponse {
		t.Helper()
		response := performAPIRequest(server, http.MethodGet, "/api/books/entry-book", nil)
		var value libraryBookResponse
		if err := json.Unmarshal(response.Body.Bytes(), &value); err != nil || response.Code != http.StatusOK {
			t.Fatalf("entry: status=%d err=%v", response.Code, err)
		}
		return value
	}
	first := read()
	identity, err := source.DefinitionIdentity()
	if err != nil {
		t.Fatal(err)
	}
	if first.ReadingContext == nil || first.ReadingContext.SourceIdentity != identity {
		t.Fatalf("missing definition qualification: %+v", first.ReadingContext)
	}
	// Entry metadata needs neither a prepared catalog nor an upstream crawl.
	if first.TotalChapterNum != 0 {
		t.Fatal("unexpected catalog")
	}
	source.Header = `{"X-Fixture":"changed"}`
	if err := server.standalone.sourceStore.Upsert(source); err != nil {
		t.Fatal(err)
	}
	changed := read()
	if changed.ContentRevision != first.ContentRevision || changed.StateVersion != first.StateVersion || changed.ReadingContext.SourceIdentity == identity {
		t.Fatal("definition edit must change qualification, not reading revisions")
	}
	response := performAPIRequest(server, http.MethodGet, "/api/books", nil)
	var shelf []libraryBookResponse
	if err := json.Unmarshal(response.Body.Bytes(), &shelf); err != nil || len(shelf) != 1 || shelf[0].ReadingContext != nil {
		t.Fatal("shelf must not load entry qualification")
	}
	// A missing source keeps detail/recovery available but cannot qualify reuse.
	if _, err := server.standalone.db.Exec("DELETE FROM book_sources WHERE id = ?", source.ID); err != nil {
		t.Fatal(err)
	}
	if read().ReadingContext != nil {
		t.Fatal("missing source qualified cached reading")
	}
}

func TestReaderEntryImportedProviders(t *testing.T) {
	server, closeDB := newWorkflowAPIServer(t)
	defer closeDB()
	for _, provider := range []string{library.TXT, library.EPUB} {
		t.Run(provider, func(t *testing.T) {
			tx, err := server.standalone.db.BeginTx(t.Context(), nil)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback()
			if err := library.InsertTx(t.Context(), tx, library.Item{ID: provider, Provider: provider, ContentRevision: 2, StateVersion: 4}); err != nil {
				t.Fatal(err)
			}
			if err := tx.Commit(); err != nil {
				t.Fatal(err)
			}
			response := performAPIRequest(server, http.MethodGet, "/api/books/"+provider, nil)
			var value libraryBookResponse
			if err := json.Unmarshal(response.Body.Bytes(), &value); err != nil || response.Code != http.StatusOK {
				t.Fatalf("entry: status=%d err=%v", response.Code, err)
			}
			if value.ReadingContext == nil || value.ReadingContext.SourceIdentity != "" || value.ContentRevision != 2 || value.StateVersion != 4 {
				t.Fatalf("imported entry qualification: %+v", value)
			}
		})
	}
}
