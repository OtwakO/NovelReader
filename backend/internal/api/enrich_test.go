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

func TestHandleEnrichBookMergesNormalizedTitleAuthorIntoExistingShelfBook(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "books.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	bookStore := book.NewStore(db)
	initializeBookAndSourceAPITestSchema(t, db)
	sourceStore := booksource.NewStore(db)
	server := &Server{bookStore: bookStore, sourceStore: sourceStore}

	for _, body := range []string{
		`{"id":"book-1","name":"异度旅社","author":"远瞳","sourceUrl":"source-a","bookUrl":"/a"}`,
		`{"id":"book-2","name":" 异度，旅社 ","author":"作者：远瞳","sourceUrl":"source-b","bookUrl":"/b"}`,
	} {
		request := httptest.NewRequest(http.MethodPost, "/api/books/enrich", bytes.NewBufferString(body))
		response := httptest.NewRecorder()
		server.handleEnrichBook(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
		}
	}
	books, err := bookStore.ListBooks()
	if err != nil || len(books) != 1 {
		t.Fatalf("books=%+v error=%v", books, err)
	}
	if books[0].ID != "book-1" || len(books[0].AlternateSources) != 1 || books[0].AlternateSources[0].SourceURL != "source-b" {
		t.Fatalf("merged book=%+v", books[0])
	}
}

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
		SourceURL: "source-a", BookURL: "/a", Origin: "Source A",
	}); err != nil {
		t.Fatal(err)
	}
	server := &Server{bookStore: bookStore}
	request := httptest.NewRequest(http.MethodPost, "/api/books/book-1/sources", bytes.NewBufferString(`{
		"sources":[{"sourceUrl":"source-b","bookUrl":"/b","sourceName":"Source B"}]
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
}

func TestHandleEnrichBookPreservesAlternateSourcesOnFallback(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "books.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	bookStore := book.NewStore(db)
	initializeBookAndSourceAPITestSchema(t, db)
	sourceStore := booksource.NewStore(db)
	server := &Server{bookStore: bookStore, sourceStore: sourceStore}

	request := httptest.NewRequest(http.MethodPost, "/api/books/enrich", bytes.NewBufferString(`{
		"id":"book-1","name":"Fixture","sourceName":"Primary Source","sourceUrl":"https://primary.test","bookUrl":"/primary",
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
	if stored.Origin != "Primary Source" {
		t.Fatalf("current source title=%q", stored.Origin)
	}
	if len(stored.AlternateSources) != 1 || stored.AlternateSources[0].SourceURL != "https://alternate.test" {
		t.Fatalf("alternate sources were not preserved: %+v", stored.AlternateSources)
	}
}
