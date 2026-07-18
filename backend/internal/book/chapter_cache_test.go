// Chapter-cache tests enforce exact source identity and bounded LRU retention.
package book

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/otwako/novelreader/internal/database"
)

func TestChapterCacheUsesExactIdentityAndBoundedLRU(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "cache.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := NewStore(db)
	if err := store.Init(); err != nil {
		t.Fatal(err)
	}
	for bookIndex := 0; bookIndex < 6; bookIndex++ {
		bookID := fmt.Sprintf("book-%d", bookIndex)
		if err := store.AddBook(&Book{ID: bookID, Name: bookID, SourceURL: "source", BookURL: "url"}); err != nil {
			t.Fatal(err)
		}
		for chapter := 0; chapter < 101; chapter++ {
			entry := CachedChapter{BookID: bookID, SourceURL: "source", ChapterIndex: chapter, ChapterURL: fmt.Sprintf("url-%d", chapter), Title: "Title", Paragraphs: []string{fmt.Sprintf("content-%d", chapter)}}
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
	cached, err := store.GetChapterCache("book-5", "source", 100, "url-100")
	if err != nil || cached == nil || cached.Paragraphs[0] != "content-100" {
		t.Fatalf("cached=%+v err=%v", cached, err)
	}
	if cached, err := store.GetChapterCache("book-5", "source", 100, "changed-url"); err != nil || cached != nil {
		t.Fatalf("changed URL cached=%+v err=%v", cached, err)
	}
	if err := store.DeleteBook("book-5"); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveChapterCache(CachedChapter{BookID: "book-5", SourceURL: "source", ChapterIndex: 0, ChapterURL: "late", Paragraphs: []string{"late"}}); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM chapter_cache WHERE book_id = 'book-5'`).Scan(&perBook); err != nil || perBook != 0 {
		t.Fatalf("cache recreated after delete: count=%d err=%v", perBook, err)
	}
}
