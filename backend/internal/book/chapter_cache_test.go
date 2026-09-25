// Chapter-cache tests enforce exact source identity and bounded LRU retention.
package book

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/otwako/novelreader/internal/database"
	"github.com/otwako/novelreader/internal/processor"
)

func TestChapterCacheUsesExactIdentityAndBoundedLRU(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "cache.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := NewStore(db)
	initializeBookTestSchema(t, db)
	for bookIndex := 0; bookIndex < 6; bookIndex++ {
		bookID := fmt.Sprintf("book-%d", bookIndex)
		if err := store.AddBook(&Book{ID: bookID, Name: bookID, SourceID: "source", SourceURL: "source", BookURL: "url"}); err != nil {
			t.Fatal(err)
		}
		for chapter := 0; chapter < 101; chapter++ {
			entry := CachedChapter{CachedAt: 123456, SourceIdentity: "definition", BookContext: map[string]any{"name": "Captured"}, ChapterContext: map[string]any{"url": "chapter"}, BookID: bookID, SourceID: "source", ChapterIndex: chapter, ChapterURL: fmt.Sprintf("url-%d", chapter), Title: "Title", Paragraphs: []string{fmt.Sprintf("content-%d", chapter)}, Blocks: []processor.ProseBlock{{Kind: processor.ProseBlockImage, Src: fmt.Sprintf("image-%d", chapter)}}}
			if err := store.SaveChapterCache(entry); err != nil {
				t.Fatal(err)
			}
		}
	}
	var total int
	if err := db.QueryRow(`SELECT COUNT(*) FROM chapter_cache`).Scan(&total); err != nil || total != 500 {
		t.Fatalf("total=%d err=%v", total, err)
	}
	var perBook int
	if err := db.QueryRow(`SELECT COUNT(*) FROM chapter_cache WHERE book_id = 'book-5'`).Scan(&perBook); err != nil || perBook != 100 {
		t.Fatalf("perBook=%d err=%v", perBook, err)
	}
	cached, err := store.GetChapterCache("book-5", "source", 100, "url-100", 0)
	if err != nil || cached == nil || cached.Paragraphs[0] != "content-100" || len(cached.Blocks) != 1 || cached.Blocks[0].Src != "image-100" {
		t.Fatalf("cached=%+v err=%v", cached, err)
	}
	if cached.CachedAt != 123456 || cached.SourceIdentity != "definition" || cached.BookContext["name"] != "Captured" || cached.ChapterContext["url"] != "chapter" {
		t.Fatalf("lost document metadata: %+v", cached)
	}
	if cached, err := store.GetChapterCache("book-5", "source", 100, "changed-url", 0); err != nil || cached != nil {
		t.Fatalf("changed URL cached=%+v err=%v", cached, err)
	}
	if err := store.DeleteBook("book-5"); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveChapterCache(CachedChapter{BookID: "book-5", SourceID: "source", ChapterIndex: 0, ChapterURL: "late", Paragraphs: []string{"late"}}); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM chapter_cache WHERE book_id = 'book-5'`).Scan(&perBook); err != nil || perBook != 0 {
		t.Fatalf("cache recreated after delete: count=%d err=%v", perBook, err)
	}
}

func TestChapterCacheRejectsSupersededRevision(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "cache.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	initializeBookTestSchema(t, db)
	store := NewStore(db)
	if err := store.AddBook(&Book{ID: "book", Name: "Novel", SourceID: "source", SourceURL: "source", BookURL: "url"}); err != nil {
		t.Fatal(err)
	}
	entry := CachedChapter{BookID: "book", SourceID: "source", ChapterIndex: 0, ChapterURL: "chapter", Paragraphs: []string{"old"}}
	if err := store.SaveChapterCache(entry); err != nil {
		t.Fatal(err)
	}
	// Publishing even an identical URL list creates a new interpretation revision.
	if err := store.SaveChapters("book", []Chapter{{Index: 0, Title: "One", URL: "chapter"}}); err != nil {
		t.Fatal(err)
	}
	if cached, err := store.GetChapterCache("book", "source", 0, "chapter", 0); err != nil || cached != nil {
		t.Fatalf("superseded cache=%+v err=%v", cached, err)
	}
	current := entry
	current.ContentRevision = 1
	current.Paragraphs = []string{"current"}
	if err := store.SaveChapterCache(current); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveChapterCache(entry); err != nil {
		t.Fatal(err)
	}
	cached, err := store.GetChapterCache("book", "source", 0, "chapter", 1)
	if err != nil || cached == nil || cached.Paragraphs[0] != "current" {
		t.Fatalf("late response replaced current cache: %+v err=%v", cached, err)
	}
}
