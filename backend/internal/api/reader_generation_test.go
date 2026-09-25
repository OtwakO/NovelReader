package api

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/otwako/novelreader/internal/auth"
	"github.com/otwako/novelreader/internal/book"
	"github.com/otwako/novelreader/internal/readerstore"
)

func TestReaderGenerationRejectsStaleRequestsAfterReplacement(t *testing.T) {
	server, sessions, readers, id, cleanup := newOwnershipServer(t)
	defer cleanup()
	ctx := t.Context()
	home, err := readers.Open(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if err := book.NewStore(home.DB()).AddBook(&book.Book{ID: "book", Name: "Preserved", SourceURL: "source", BookURL: "book"}); err != nil {
		t.Fatal(err)
	}
	oldGeneration := home.Generation()
	home.Close()
	credential, err := sessions.Create(ctx, id, time.Now().Unix())
	if err != nil {
		t.Fatal(err)
	}
	request := func(method, path, generation string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, nil)
		req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: credential.Token})
		if generation != "" {
			req.Header.Set(readerGenerationHeader, generation)
		}
		response := httptest.NewRecorder()
		server.ServeHTTP(response, req)
		return response
	}
	initial := request(http.MethodGet, "/api/books/book", "")
	if initial.Code != http.StatusOK || initial.Header().Get(readerGenerationHeader) != oldGeneration {
		t.Fatalf("initial identity response: %d %s", initial.Code, initial.Body.String())
	}
	snapshot := filepath.Join(t.TempDir(), "snapshot")
	if err := readers.SnapshotHome(ctx, id, snapshot); err != nil {
		t.Fatal(err)
	}
	stage, err := readers.PrepareReplacement(ctx, id, filepath.Join(snapshot, readerstore.ReaderDatabaseName), filepath.Join(snapshot, readerstore.FilesDirectory))
	if err != nil {
		t.Fatal(err)
	}
	if err := server.quiesceReader(ctx, id); err != nil {
		t.Fatal(err)
	}
	defer server.resumeReader(id)
	if err := readers.PublishReplacement(ctx, id, stage); err != nil {
		t.Fatal(err)
	}
	server.resumeReader(id)
	stale := request(http.MethodDelete, "/api/books/book", oldGeneration)
	newGeneration := stale.Header().Get(readerGenerationHeader)
	if stale.Code != http.StatusConflict || newGeneration == "" || newGeneration == oldGeneration {
		t.Fatalf("stale mutation accepted: %d %s", stale.Code, stale.Body.String())
	}
	for _, header := range []string{"", newGeneration} {
		image := request(http.MethodGet, "/api/books/book/chapters/0/images/0?contentRevision=0&readerGeneration="+oldGeneration, header)
		if image.Code != http.StatusNotFound {
			t.Fatalf("stale image reference accepted: %d", image.Code)
		}
	}
	fresh := request(http.MethodGet, "/api/books/book", newGeneration)
	if fresh.Code != http.StatusOK {
		t.Fatalf("stale delete changed restored data: %d %s", fresh.Code, fresh.Body.String())
	}
}
