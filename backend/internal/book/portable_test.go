package book

import (
	"bytes"
	"database/sql"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/otwako/novelreader/internal/library"
	"github.com/otwako/novelreader/internal/readerstore"
)

func TestPortableHomeExcludesChapterCacheWithoutChangingReadingState(t *testing.T) {
	ctx := t.Context()
	manager, err := readerstore.NewManager(filepath.Join(t.TempDir(), "data"), 2, library.ReaderSchema(), ReaderSchema())
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	const reader readerstore.UserID = "11111111-1111-4111-8111-111111111111"
	if err := manager.Create(ctx, reader); err != nil {
		t.Fatal(err)
	}
	home, err := manager.Open(ctx, reader)
	if err != nil {
		t.Fatal(err)
	}
	defer home.Close()
	store := NewStore(home.DB())
	if err := store.AddBook(&Book{ID: "book", Name: "Novel", SourceID: "source", SourceURL: "https://source.invalid", BookURL: "https://source.invalid/book"}); err != nil {
		t.Fatal(err)
	}
	chapters := []Chapter{{ID: "chapter", BookID: "book", Index: 0, Title: "Chapter", URL: "https://source.invalid/chapter", Cached: true}}
	if err := store.SaveChapters("book", chapters); err != nil {
		t.Fatal(err)
	}
	before, err := store.GetBook("book")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.UpdateProgress("book", before.ContentRevision, before.StateVersion, 0, 0.5); err != nil {
		t.Fatal(err)
	}
	before, err = store.GetBook("book")
	if err != nil {
		t.Fatal(err)
	}
	// Enough synthetic payload to leave free pages unless staging is compacted.
	payload := strings.Repeat("disposable chapter payload ", 4096)
	entry := CachedChapter{BookID: "book", SourceID: "source", ContentRevision: before.ContentRevision, ChapterIndex: 0, ChapterURL: chapters[0].URL, Title: "Chapter", Paragraphs: []string{payload}}
	if err := store.SaveChapterCache(entry); err != nil {
		t.Fatal(err)
	}

	assertPortable := func(path string) {
		t.Helper()
		db, err := sql.Open("sqlite", filepath.Join(path, readerstore.ReaderDatabaseName))
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close()
		for _, query := range []string{"SELECT COUNT(*) FROM chapter_cache", "PRAGMA freelist_count"} {
			var count int
			if err := db.QueryRow(query).Scan(&count); err != nil || count != 0 {
				t.Fatalf("%s: count=%d, error=%v", query, count, err)
			}
		}
		got, err := NewStore(db).GetBook("book")
		if err != nil || !reflect.DeepEqual(got, before) {
			t.Fatalf("durable book changed: got=%+v, want=%+v, error=%v", got, before, err)
		}
		gotChapters, err := NewStore(db).GetChapters("book")
		wantChapters := append([]Chapter(nil), chapters...)
		wantChapters[0].Cached = false
		if err != nil || !reflect.DeepEqual(gotChapters, wantChapters) {
			t.Fatalf("catalog changed: got=%+v, want=%+v, error=%v", gotChapters, wantChapters, err)
		}
		data, err := os.ReadFile(filepath.Join(path, readerstore.ReaderDatabaseName))
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(data, []byte("disposable chapter payload")) {
			t.Fatal("portable database retains discarded payload bytes")
		}
	}

	snapshot := filepath.Join(t.TempDir(), "snapshot")
	if err := manager.SnapshotHome(ctx, reader, snapshot); err != nil {
		t.Fatal(err)
	}
	assertPortable(snapshot)

	// A compatible older archive can contain caches. Restore strips only its
	// staged copy, not the archive being imported or the current reader home.
	snapshotDB := filepath.Join(snapshot, readerstore.ReaderDatabaseName)
	db, err := sql.Open("sqlite", snapshotDB)
	if err != nil {
		t.Fatal(err)
	}
	if err := NewStore(db).SaveChapterCache(entry); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("UPDATE chapters SET cached=1"); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	stage, err := manager.PrepareReplacement(ctx, reader, snapshotDB, filepath.Join(snapshot, readerstore.FilesDirectory))
	if err != nil {
		t.Fatal(err)
	}
	assertPortable(stage)

	archiveDB, err := sql.Open("sqlite", snapshotDB)
	if err != nil {
		t.Fatal(err)
	}
	defer archiveDB.Close()
	for name, original := range map[string]*sql.DB{"archive": archiveDB, "live home": home.DB()} {
		cached, err := NewStore(original).GetChapterCache("book", "source", 0, entry.ChapterURL, entry.ContentRevision)
		if err != nil || cached == nil || !reflect.DeepEqual(cached.Paragraphs, entry.Paragraphs) {
			t.Fatalf("%s cache changed: error=%v", name, err)
		}
		var flag bool
		if err := original.QueryRow("SELECT cached FROM chapters").Scan(&flag); err != nil || !flag {
			t.Fatalf("%s catalog cache flag changed: %v", name, err)
		}
	}
}
