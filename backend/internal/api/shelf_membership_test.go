package api

import (
	"path/filepath"
	"testing"

	"github.com/otwako/novelreader/internal/book"
	"github.com/otwako/novelreader/internal/database"
)

func TestShelfMembershipAnnotatesNormalizedLogicalBooksAcrossSources(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "reader.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := book.NewStore(db)
	initializeBookAPITestSchema(t, db)
	if err := store.AddBook(&book.Book{ID: "shelf-book", Name: "异度旅社", Author: "远瞳", SourceID: "source-a", SourceURL: "source-a", BookURL: "/a"}); err != nil {
		t.Fatal(err)
	}
	server := newReaderTestServer(&readerRuntime{bookStore: store})
	results := []book.SearchResult{
		{Name: " 异度，旅社 ", Author: "作者：远瞳 著", SourceID: "source-b", BookURL: "/b"},
		{Name: "Other", Author: "Author", SourceID: "source-c", BookURL: "/c"},
	}
	server.standalone.loadShelfMembership().annotate(results)
	if results[0].ShelfBookID != "shelf-book" {
		t.Fatalf("matching result shelfBookId=%q", results[0].ShelfBookID)
	}
	if results[1].ShelfBookID != "" {
		t.Fatalf("unrelated result shelfBookId=%q", results[1].ShelfBookID)
	}
}
