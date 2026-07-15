// Regression coverage for persisted chapter context used by later content rules.
package book

import (
	"path/filepath"
	"testing"

	"github.com/otwako/novelreader/internal/database"
)

func TestStoreInitMigratesOldChapterSchema(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "books.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE chapters (id TEXT PRIMARY KEY, book_id TEXT, idx INTEGER, title TEXT, url TEXT, is_vip INTEGER DEFAULT 0, is_volume INTEGER DEFAULT 0, cached INTEGER DEFAULT 0)`); err != nil {
		t.Fatal(err)
	}
	store := NewStore(db)
	if err := store.Init(); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO books (id,name,source_url,book_url,origin,created_at,updated_at) VALUES ('book-1','Book','source','book','source',0,0)`); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveChapters("book-1", []Chapter{{Index: 1, Title: "Chapter", URL: "url", IsPay: true, BaseURL: "toc", Tag: "tag", WordCount: "1"}}); err != nil {
		t.Fatal(err)
	}
	chapters, err := store.GetChapters("book-1")
	if err != nil || len(chapters) != 1 || !chapters[0].IsPay || chapters[0].BaseURL != "toc" {
		t.Fatalf("chapters=%+v err=%v", chapters, err)
	}
}

func TestStorePersistsChapterContext(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "books.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	store := NewStore(db)
	if err := store.Init(); err != nil {
		t.Fatal(err)
	}
	if err := store.AddBook(&Book{ID: "book-1", Name: "Book", SourceURL: "source", BookURL: "book"}); err != nil {
		t.Fatal(err)
	}
	want := []Chapter{{
		Index: 1, Title: "Chapter", URL: "https://example.test/chapter/1", IsVip: true,
		IsVolume: false, IsPay: true, BaseURL: "https://example.test/toc", Tag: "updated", WordCount: "1234",
	}}
	if err := store.SaveChapters("book-1", want); err != nil {
		t.Fatal(err)
	}
	got, err := store.GetChapters("book-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].IsPay != want[0].IsPay || got[0].BaseURL != want[0].BaseURL || got[0].Tag != want[0].Tag || got[0].WordCount != want[0].WordCount {
		t.Fatalf("chapters = %+v, want %+v", got, want)
	}
}
