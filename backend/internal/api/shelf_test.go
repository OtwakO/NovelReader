package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/otwako/novelreader/internal/book"
)

func TestListBooksIncludesStoredCurrentChapterTitle(t *testing.T) {
	server, closeDB := newWorkflowAPIServer(t)
	defer closeDB()

	stored := &book.Book{
		ID: "book-1", Name: "Fixture Novel", Author: "Fixture Author",
		SourceID: "source-test", SourceURL: "https://source.test", BookURL: "https://source.test/book",
	}
	if err := server.bookStore.AddBook(stored); err != nil {
		t.Fatal(err)
	}
	if err := server.bookStore.SaveChapters(stored.ID, []book.Chapter{
		{Index: 0, Title: "Chapter One", URL: "/1"},
		{Index: 1, Title: "Chapter Two", URL: "/2"},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := server.bookStore.UpdateProgress(stored.ID, stored.SourceID, 0, 1, 0.4); err != nil {
		t.Fatal(err)
	}

	response := performAPIRequest(server, http.MethodGet, "/api/books", nil)
	if response.Code != http.StatusOK {
		t.Fatalf("list status=%d body=%s", response.Code, response.Body.String())
	}
	var books []book.Book
	if err := json.Unmarshal(response.Body.Bytes(), &books); err != nil {
		t.Fatal(err)
	}
	if len(books) != 1 || books[0].CurrentChapterTitle != "Chapter Two" {
		t.Fatalf("books=%+v", books)
	}
}
