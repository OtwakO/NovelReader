package book

import (
	"path/filepath"
	"sync"
	"testing"

	"github.com/otwako/novelreader/internal/database"
)

func TestStoreAddOrMergeBookUsesNormalizedTitleAuthorIdentity(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "books.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := NewStore(db)
	initializeBookTestSchema(t, db)
	first := &Book{
		ID: "book-first", Name: "异度旅社", Author: "远瞳",
		SourceID: "source-a", SourceURL: "source-a", BookURL: "/a", Origin: "Source A",
		DurChapterIndex: 50, DurChapterPos: 0.4, StateVersion: 7,
		AlternateSources: []AltSource{{SourceID: "source-b", SourceURL: "source-b", BookURL: "/b", SourceName: "Source B"}},
	}
	stored, created, err := store.AddOrMergeBook(first)
	if err != nil || !created || stored.ID != first.ID {
		t.Fatalf("first=%+v created=%v error=%v", stored, created, err)
	}
	duplicate := &Book{
		ID: "book-duplicate", Name: " 异度，旅社 ", Author: "作者：远瞳",
		SourceID: "source-c", SourceURL: "source-c", BookURL: "/c", Origin: "Source C",
		AlternateSources: []AltSource{
			{SourceID: "source-b", SourceURL: "source-b", BookURL: "/b", SourceName: "Source B duplicate"},
			{SourceID: "source-d", SourceURL: "source-d", BookURL: "/d", SourceName: "Source D"},
		},
	}
	stored, created, err = store.AddOrMergeBook(duplicate)
	if err != nil || created {
		t.Fatalf("duplicate=%+v created=%v error=%v", stored, created, err)
	}
	if stored.ID != first.ID || stored.SourceURL != first.SourceURL || stored.BookURL != first.BookURL {
		t.Fatalf("logical book identity/current source changed: %+v", stored)
	}
	if stored.DurChapterIndex != 50 || stored.DurChapterPos != 0.4 || stored.StateVersion != 7 {
		t.Fatalf("reading state changed: %+v", stored)
	}
	if len(stored.AlternateSources) != 3 {
		t.Fatalf("alternates=%+v", stored.AlternateSources)
	}
	books, err := store.ListBooks()
	if err != nil || len(books) != 1 {
		t.Fatalf("books=%+v error=%v", books, err)
	}
}

func TestStoreMergeBookSourcesPreservesDiscoveryQuery(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "books.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := NewStore(db)
	initializeBookTestSchema(t, db)
	if err := store.AddBook(&Book{ID: "book-a", Name: "Fixture", Author: "Author", SourceID: "aggregate", SourceURL: "aggregate", BookURL: "/current"}); err != nil {
		t.Fatal(err)
	}
	stored, err := store.MergeBookSources("book-a", []AltSource{{SourceID: "aggregate", SourceURL: "aggregate", BookURL: "/alternate", SourceName: "Aggregate", DiscoveryQuery: "Fixture@provider"}})
	if err != nil || len(stored.AlternateSources) != 1 || stored.AlternateSources[0].DiscoveryQuery != "Fixture@provider" {
		t.Fatalf("stored=%+v error=%v", stored, err)
	}
	reloaded, err := store.GetBook("book-a")
	if err != nil || len(reloaded.AlternateSources) != 1 || reloaded.AlternateSources[0].DiscoveryQuery != "Fixture@provider" {
		t.Fatalf("reloaded=%+v error=%v", reloaded, err)
	}
}

func TestStoreMergeBookSourcesEnrichesExistingBinding(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "books.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := NewStore(db)
	initializeBookTestSchema(t, db)
	if err := store.AddBook(&Book{ID: "book-a", Name: "Fixture", Author: "Author", SourceID: "aggregate", SourceURL: "aggregate", BookURL: "/current", AlternateSources: []AltSource{{SourceID: "aggregate", SourceURL: "aggregate", BookURL: "/alternate", SourceName: "Aggregate"}}}); err != nil {
		t.Fatal(err)
	}
	stored, err := store.MergeBookSources("book-a", []AltSource{{SourceID: "aggregate", SourceURL: "aggregate", BookURL: "/alternate", SourceName: "Aggregate", DiscoveryQuery: "Fixture@provider"}})
	if err != nil || len(stored.AlternateSources) != 1 || stored.AlternateSources[0].DiscoveryQuery != "Fixture@provider" {
		t.Fatalf("stored=%+v error=%v", stored, err)
	}
}

func TestStoreClearBookSourcesPreservesActiveBindingMetadata(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "books.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := NewStore(db)
	initializeBookTestSchema(t, db)
	active := &AltSource{SourceID: "aggregate", SourceURL: "aggregate", BookURL: "/current", SourceName: "Aggregate", DiscoveryQuery: "Fixture@current"}
	if err := store.AddBook(&Book{ID: "book-a", Name: "Fixture", Author: "Author", SourceID: active.SourceID, SourceURL: active.SourceURL, BookURL: active.BookURL, Origin: active.SourceName, ActiveSource: active, AlternateSources: []AltSource{{SourceID: "aggregate", SourceURL: "aggregate", BookURL: "/alternate", SourceName: "Aggregate"}}}); err != nil {
		t.Fatal(err)
	}
	stored, err := store.ClearBookSources("book-a")
	if err != nil || stored.ActiveSource == nil || stored.ActiveSource.DiscoveryQuery != "Fixture@current" || len(stored.AlternateSources) != 0 {
		t.Fatalf("stored=%+v error=%v", stored, err)
	}
	reloaded, err := store.GetBook("book-a")
	if err != nil || reloaded.ActiveSource == nil || reloaded.ActiveSource.DiscoveryQuery != "Fixture@current" || len(reloaded.AlternateSources) != 0 {
		t.Fatalf("reloaded=%+v error=%v", reloaded, err)
	}
}

func TestStoreLowLevelAddCannotReplaceAnotherLogicalBookID(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "books.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := NewStore(db)
	initializeBookTestSchema(t, db)
	if err := store.AddBook(&Book{ID: "book-a", Name: "Fixture", Author: "Author", SourceID: "a", SourceURL: "a", BookURL: "/a"}); err != nil {
		t.Fatal(err)
	}
	if err := store.AddBook(&Book{ID: "book-b", Name: "Fixture", Author: "Author", SourceID: "b", SourceURL: "b", BookURL: "/b"}); err == nil {
		t.Fatal("low-level add replaced a different logical book ID")
	}
	books, err := store.ListBooks()
	if err != nil || len(books) != 1 || books[0].ID != "book-a" {
		t.Fatalf("books=%+v error=%v", books, err)
	}
}

func TestStoreConcurrentLogicalBookAddsConverge(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "books.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := NewStore(db)
	initializeBookTestSchema(t, db)
	var wait sync.WaitGroup
	errorsFound := make(chan error, 2)
	for _, candidate := range []*Book{
		{ID: "book-a", Name: "Fixture", Author: "Author", SourceID: "a", SourceURL: "a", BookURL: "/a", Origin: "A"},
		{ID: "book-b", Name: "Fixture", Author: "Author", SourceID: "b", SourceURL: "b", BookURL: "/b", Origin: "B"},
	} {
		wait.Add(1)
		go func(candidate *Book) {
			defer wait.Done()
			_, _, err := store.AddOrMergeBook(candidate)
			errorsFound <- err
		}(candidate)
	}
	wait.Wait()
	close(errorsFound)
	for err := range errorsFound {
		if err != nil {
			t.Fatal(err)
		}
	}
	books, err := store.ListBooks()
	if err != nil || len(books) != 1 || len(books[0].AlternateSources) != 1 {
		t.Fatalf("books=%+v error=%v", books, err)
	}
}

func TestNormalizeBookIdentity(t *testing.T) {
	name, author := NormalizeBookIdentity(" 异度，旅社 ", "作者： 远瞳 著")
	if name != "异度旅社" || author != "远瞳" {
		t.Fatalf("identity=(%q,%q)", name, author)
	}
}
