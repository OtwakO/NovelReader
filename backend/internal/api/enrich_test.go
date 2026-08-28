// Shelf persistence regression coverage for merged search alternatives.
package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/otwako/novelreader/internal/book"
	"github.com/otwako/novelreader/internal/database"
)

func TestHandleMergeBookSourcesPreservesCurrentSource(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "books.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	bookStore := book.NewStore(db)
	initializeBookAPITestSchema(t, db)
	if _, _, err := bookStore.AddOrMergeBook(&book.Book{
		ID: "book-1", Name: "Fixture", Author: "Author",
		SourceID: "source-a", SourceURL: "source-a", BookURL: "/a", Origin: "Source A",
	}); err != nil {
		t.Fatal(err)
	}
	server := &Server{bookStore: bookStore}
	request := httptest.NewRequest(http.MethodPost, "/api/books/book-1/sources", bytes.NewBufferString(`{
		"sources":[{"sourceId":"source-b","sourceUrl":"source-b","bookUrl":"/b","sourceName":"Source B"}]
	}`))
	request.SetPathValue("id", "book-1")
	response := httptest.NewRecorder()
	server.handleMergeBookSources(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	stored, err := bookStore.GetBook("book-1")
	if err != nil || stored.SourceURL != "source-a" || stored.Origin != "Source A" || len(stored.AlternateSources) != 1 {
		t.Fatalf("stored=%+v error=%v", stored, err)
	}
	loaded, err := bookStore.GetBook("book-1")
	if err != nil || len(loaded.AlternateSources) != 1 || loaded.AlternateSources[0].SourceURL != "source-b" {
		t.Fatalf("reloaded=%+v error=%v", loaded, err)
	}
}

func TestHandleClearBookSourcesPreservesCurrentSource(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "books.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	bookStore := book.NewStore(db)
	initializeBookAPITestSchema(t, db)
	if _, _, err := bookStore.AddOrMergeBook(&book.Book{
		ID: "book-1", Name: "Fixture", Author: "Author",
		SourceID: "source-a", SourceURL: "source-a", BookURL: "/a", Origin: "Source A",
		AlternateSources: []book.AltSource{{SourceURL: "source-b", BookURL: "/b", SourceName: "Source B"}},
	}); err != nil {
		t.Fatal(err)
	}
	server := &Server{bookStore: bookStore}
	request := httptest.NewRequest(http.MethodDelete, "/api/books/book-1/sources", nil)
	request.SetPathValue("id", "book-1")
	response := httptest.NewRecorder()
	server.handleClearBookSources(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	stored, err := bookStore.GetBook("book-1")
	if err != nil || stored.SourceURL != "source-a" || stored.Origin != "Source A" || len(stored.AlternateSources) != 0 {
		t.Fatalf("stored=%+v error=%v", stored, err)
	}
}
