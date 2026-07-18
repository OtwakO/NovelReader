// Progress API tests validate resume positions against stored readable chapters.
package api

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/otwako/novelreader/internal/book"
	"github.com/otwako/novelreader/internal/database"
	"github.com/otwako/novelreader/internal/processor"
)

func TestProgressAPIValidatesBookChapterAndPosition(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "progress.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := book.NewStore(db)
	if err := store.Init(); err != nil {
		t.Fatal(err)
	}
	if err := store.AddBook(&book.Book{ID: "book-1", Name: "Book", SourceURL: "source", BookURL: "book"}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveChapters("book-1", []book.Chapter{
		{Index: 0, Title: "One", URL: "one"},
		{Index: 1, Title: "Volume", URL: "", IsVolume: true},
		{Index: 2, Title: "Two", URL: "two"},
	}); err != nil {
		t.Fatal(err)
	}
	server := NewServer(nil, store, nil, nil, nil, nil, nil, processor.Config{}, t.TempDir(), db)

	response := performAPIRequest(server, http.MethodPut, "/api/books/book-1/progress", []byte(`{"sourceUrl":"source","stateVersion":0,"chapterIndex":2,"position":0.6}`))
	if response.Code != http.StatusOK {
		t.Fatalf("valid status=%d body=%s", response.Code, response.Body.String())
	}
	stored, err := store.GetBook("book-1")
	if err != nil || stored.DurChapterIndex != 2 || stored.DurChapterPos != 0.6 {
		t.Fatalf("stored=%+v err=%v", stored, err)
	}

	for _, test := range []struct {
		bookID string
		body   string
		status int
	}{
		{"missing", `{"sourceUrl":"source","stateVersion":0,"chapterIndex":0,"position":0}`, http.StatusNotFound},
		{"book-1", `{"sourceUrl":"old","stateVersion":1,"chapterIndex":0,"position":0}`, http.StatusConflict},
		{"book-1", `{"sourceUrl":"source","stateVersion":0,"chapterIndex":0,"position":0}`, http.StatusConflict},
		{"book-1", `{"sourceUrl":"source","stateVersion":1,"chapterIndex":-1,"position":0}`, http.StatusBadRequest},
		{"book-1", `{"sourceUrl":"source","stateVersion":1,"chapterIndex":1,"position":0}`, http.StatusBadRequest},
		{"book-1", `{"sourceUrl":"source","stateVersion":1,"chapterIndex":9,"position":0}`, http.StatusBadRequest},
		{"book-1", `{"sourceUrl":"source","stateVersion":1,"chapterIndex":0,"position":1.1}`, http.StatusBadRequest},
		{"book-1", `{}`, http.StatusBadRequest},
		{"book-1", `{]`, http.StatusBadRequest},
	} {
		response := performAPIRequest(server, http.MethodPut, "/api/books/"+test.bookID+"/progress", []byte(test.body))
		if response.Code != test.status {
			var payload map[string]interface{}
			_ = json.Unmarshal(response.Body.Bytes(), &payload)
			t.Fatalf("book=%s body=%s status=%d want=%d response=%v", test.bookID, test.body, response.Code, test.status, payload)
		}
	}
}
