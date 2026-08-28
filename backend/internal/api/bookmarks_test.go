// Bookmark API tests enforce current readable locations and idempotent client IDs.
package api

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/otwako/novelreader/internal/book"
	"github.com/otwako/novelreader/internal/database"
	"github.com/otwako/novelreader/internal/processor"
)

func TestBookmarkAPIAddsListsAndDeletesValidatedNotes(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "bookmarks.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := book.NewStore(db)
	initializeBookAPITestSchema(t, db)
	if err := store.AddBook(&book.Book{ID: "book", Name: "Book", SourceID: "source", SourceURL: "source", BookURL: "url"}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveChapters("book", []book.Chapter{{Index: 0, Title: "Chapter", URL: "chapter"}}); err != nil {
		t.Fatal(err)
	}
	server := NewServer(nil, store, nil, nil, nil, nil, nil, processor.Config{}, t.TempDir(), db)

	body := []byte(`{"id":"mark-1","sourceId":"source","stateVersion":0,"chapterIndex":0,"position":0.5,"note":"  note  "}`)
	for range 2 {
		response := performAPIRequest(server, http.MethodPost, "/api/books/book/bookmarks", body)
		if response.Code != http.StatusCreated {
			t.Fatalf("add status=%d body=%s", response.Code, response.Body.String())
		}
	}
	response := performAPIRequest(server, http.MethodGet, "/api/books/book/bookmarks", nil)
	var marks []book.Bookmark
	if err := json.Unmarshal(response.Body.Bytes(), &marks); err != nil || len(marks) != 1 || marks[0].Note != "note" || marks[0].ChapterTitle != "Chapter" {
		t.Fatalf("marks=%+v err=%v body=%s", marks, err, response.Body.String())
	}
	unicodeNote, _ := json.Marshal(map[string]interface{}{"id": "unicode", "sourceId": "source", "stateVersion": 0, "chapterIndex": 0, "position": 0, "note": strings.Repeat("书", 1000)})
	if response := performAPIRequest(server, http.MethodPost, "/api/books/book/bookmarks", unicodeNote); response.Code != http.StatusCreated {
		t.Fatalf("unicode note status=%d body=%s", response.Code, response.Body.String())
	}
	malformed := append([]byte(`{"id":"bad","sourceId":"source","stateVersion":0,"chapterIndex":0,"position":0,"note":"`), 0xff)
	malformed = append(malformed, []byte(`"}`)...)
	if response := performAPIRequest(server, http.MethodPost, "/api/books/book/bookmarks", malformed); response.Code != http.StatusBadRequest {
		t.Fatalf("malformed UTF-8 status=%d body=%s", response.Code, response.Body.String())
	}
	tooLong, _ := json.Marshal(map[string]interface{}{"id": "long", "sourceId": "source", "stateVersion": 0, "chapterIndex": 0, "position": 0, "note": strings.Repeat("x", 1001)})
	if response := performAPIRequest(server, http.MethodPost, "/api/books/book/bookmarks", tooLong); response.Code != http.StatusBadRequest {
		t.Fatalf("long note status=%d body=%s", response.Code, response.Body.String())
	}
	if response := performAPIRequest(server, http.MethodDelete, "/api/books/book/bookmarks/mark-1", nil); response.Code != http.StatusOK {
		t.Fatalf("delete status=%d body=%s", response.Code, response.Body.String())
	}
	if response := performAPIRequest(server, http.MethodDelete, "/api/books/book/bookmarks/mark-1", nil); response.Code != http.StatusNotFound {
		t.Fatalf("missing delete status=%d body=%s", response.Code, response.Body.String())
	}
}
