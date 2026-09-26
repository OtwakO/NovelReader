package api

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/otwako/novelreader/internal/book"
	"github.com/otwako/novelreader/internal/booksource"
	"github.com/otwako/novelreader/internal/readerstore"
)

func TestCoverIdentitySeparatesReadersAndReplacementHomes(t *testing.T) {
	server, sessions, readers, alice, cleanup := newOwnershipServer(t)
	defer cleanup()
	ctx := t.Context()
	result := book.SearchResult{SourceID: "cover-source", SourceURL: "https://cover.test", BookURL: "https://cover.test/book", CoverURL: "data:image/png;base64,aW1hZ2U="}
	for _, reader := range []readerstore.UserID{alice, ownershipBob} {
		home, err := readers.Open(ctx, reader)
		if err != nil {
			t.Fatal(err)
		}
		source := booksource.BookSource{ID: result.SourceID, BookSourceURL: result.SourceURL, BookSourceName: "Cover"}
		if err := booksource.NewStore(home.DB()).Upsert(&source); err != nil {
			t.Fatal(err)
		}
		if err := book.NewStore(home.DB()).AddBook(&book.Book{ID: "cover-book", Name: "Cover", SourceID: source.ID, SourceURL: source.BookSourceURL, BookURL: result.BookURL, CoverURL: result.CoverURL}); err != nil {
			t.Fatal(err)
		}
		if err := home.Close(); err != nil {
			t.Fatal(err)
		}
	}
	urls := func(reader readerstore.UserID) []string {
		t.Helper()
		response := authenticatedOwnershipRequest(t, server, sessions, reader, "/api/books/cover-book")
		var dto libraryBookResponse
		if err := json.Unmarshal(response.Body.Bytes(), &dto); err != nil || response.Code != 200 {
			t.Fatal(response.Code, response.Body.String(), err)
		}
		runtime, release, err := server.runtimes.acquire(ctx, reader)
		if err != nil {
			t.Fatal(err)
		}
		defer release()
		candidate := result
		runtime.api.addCoverDisplayURL(&candidate)
		return []string{dto.CoverDisplayURL, candidate.CoverDisplayURL}
	}
	old := urls(alice)
	bob := urls(ownershipBob)
	for i, href := range old {
		if href == "" || href == bob[i] {
			t.Fatal("cover identity does not separate readers")
		}
		if response := authenticatedOwnershipRequest(t, server, sessions, alice, href); response.Code != 200 || response.Header().Get("Cache-Control") != coverCacheControl {
			t.Fatal("current cover", response.Code, response.Body.String())
		}
		if response := authenticatedOwnershipRequest(t, server, sessions, ownershipBob, href); response.Code != http.StatusNotFound || response.Header().Get("Cache-Control") != "private, no-store" {
			t.Fatal("cross-reader cover", response.Code, response.Body.String())
		}
	}
	// Restore the identical persisted IDs/revisions under a new home generation.
	snapshot := filepath.Join(t.TempDir(), "snapshot")
	if err := readers.SnapshotHome(ctx, alice, snapshot); err != nil {
		t.Fatal(err)
	}
	stage, err := readers.PrepareReplacement(ctx, alice, filepath.Join(snapshot, readerstore.ReaderDatabaseName), filepath.Join(snapshot, readerstore.FilesDirectory))
	if err != nil {
		t.Fatal(err)
	}
	if err := server.quiesceReader(ctx, alice); err != nil {
		t.Fatal(err)
	}
	defer server.resumeReader(alice)
	if err := readers.PublishReplacement(ctx, alice, stage); err != nil {
		t.Fatal(err)
	}
	server.resumeReader(alice)
	fresh := urls(alice)
	for i, href := range old {
		if fresh[i] == href {
			t.Fatal("cover identity survived home replacement")
		}
		if response := authenticatedOwnershipRequest(t, server, sessions, alice, href); response.Code != http.StatusNotFound || response.Header().Get("Cache-Control") != "private, no-store" {
			t.Fatal("old-home cover", response.Code, response.Body.String())
		}
		if response := authenticatedOwnershipRequest(t, server, sessions, alice, fresh[i]); response.Code != 200 {
			t.Fatal("restored cover", response.Code, response.Body.String())
		}
	}
}
