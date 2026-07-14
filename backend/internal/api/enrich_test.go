// Shelf persistence regression coverage for merged search alternatives.
package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/otwako/novelreader/internal/book"
	"github.com/otwako/novelreader/internal/booksource"
	"github.com/otwako/novelreader/internal/database"
)

func TestHandleEnrichBookPreservesAlternateSourcesOnFallback(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "books.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	bookStore := book.NewStore(db)
	if err := bookStore.Init(); err != nil {
		t.Fatal(err)
	}
	sourceStore := booksource.NewStore(db)
	if err := sourceStore.Init(); err != nil {
		t.Fatal(err)
	}
	server := &Server{bookStore: bookStore, sourceStore: sourceStore}

	request := httptest.NewRequest(http.MethodPost, "/api/books/enrich", bytes.NewBufferString(`{
		"id":"book-1","name":"Fixture","sourceUrl":"https://primary.test","bookUrl":"/primary",
		"alternateSources":[{"sourceUrl":"https://alternate.test","bookUrl":"/alternate","sourceName":"Alternate"}]
	}`))
	response := httptest.NewRecorder()
	server.handleEnrichBook(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	stored, err := bookStore.GetBook("book-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(stored.AlternateSources) != 1 || stored.AlternateSources[0].SourceURL != "https://alternate.test" {
		t.Fatalf("alternate sources were not preserved: %+v", stored.AlternateSources)
	}
}
