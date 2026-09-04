package api

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/otwako/novelreader/internal/book"
	"github.com/otwako/novelreader/internal/booksource"
	"github.com/otwako/novelreader/internal/processor"
	"github.com/otwako/novelreader/internal/sourceprofile"
	_ "modernc.org/sqlite"
)

func TestStoredBookCoverUsesSourceHeadersAndDecodeScript(t *testing.T) {
	encoded := []byte{0x0f, 0x2a, 0x3d, 0x28, 0x3b, 0x3e, 0x3f, 0x17, 0x3b, 0x3d, 0x3f}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Cover-Token") != "fixture-token" {
			http.Error(w, "missing source header", http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(encoded)
	}))
	defer upstream.Close()

	server, closeDB := newWorkflowAPIServer(t)
	defer closeDB()
	src := booksource.BookSource{
		BookSourceURL:  upstream.URL,
		BookSourceName: "decoded cover fixture",
		Header:         `{"X-Cover-Token":"fixture-token"}`,
		JSLib:          `function decodeCover(bytes) { return bytes.map(function(value) { return value ^ 90; }); }`,
		CoverDecodeJS:  `decodeCover(result)`,
	}
	if err := server.sourceStore.Upsert(&src); err != nil {
		t.Fatal(err)
	}
	b := book.Book{ID: "cover-book", Name: "Cover Fixture", SourceID: src.ID, SourceURL: src.BookSourceURL, BookURL: upstream.URL + "/book", CoverURL: upstream.URL + "/cover"}
	if err := server.bookStore.AddBook(&b); err != nil {
		t.Fatal(err)
	}

	response := performAPIRequest(server, http.MethodGet, "/api/books/cover-book/cover", nil)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if got, want := response.Body.Bytes(), []byte("UpgradeMage"); !bytes.Equal(got, want) {
		t.Fatalf("decoded cover=%v, want %v", got, want)
	}
	if contentType := response.Header().Get("Content-Type"); contentType != "text/plain; charset=utf-8" {
		t.Fatalf("content-type=%q", contentType)
	}
	if cacheControl := response.Header().Get("Cache-Control"); cacheControl != "private, max-age=604800" {
		t.Fatalf("cache-control=%q", cacheControl)
	}
	if vary := response.Header().Get("Vary"); vary != "Cookie" {
		t.Fatalf("vary=%q", vary)
	}
}

func TestStoredBookCoverAppliesURLScopedHeaders(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Referer") != "https://reader.fixture/book" {
			http.Error(w, "missing URL header", http.StatusForbidden)
			return
		}
		_, _ = w.Write([]byte("cover"))
	}))
	defer upstream.Close()

	server, closeDB := newWorkflowAPIServer(t)
	defer closeDB()
	src := booksource.BookSource{BookSourceURL: upstream.URL, BookSourceName: "URL header fixture"}
	if err := server.sourceStore.Upsert(&src); err != nil {
		t.Fatal(err)
	}
	coverURL := upstream.URL + `/cover,{"headers":{"Referer":"https://reader.fixture/book"}}`
	b := book.Book{ID: "header-cover", Name: "Header", SourceID: src.ID, SourceURL: src.BookSourceURL, BookURL: upstream.URL + "/book", CoverURL: coverURL}
	if err := server.bookStore.AddBook(&b); err != nil {
		t.Fatal(err)
	}

	response := performAPIRequest(server, http.MethodGet, "/api/books/header-cover/cover", nil)
	if response.Code != http.StatusOK || response.Body.String() != "cover" {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestStoredBookCoverPreservesOriginalBytesWhenDecoderReturnsNull(t *testing.T) {
	original := []byte{0xff, 0xd8, 0xff, 0xdb, 0x00, 0x43}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write(original)
	}))
	defer upstream.Close()

	server, closeDB := newWorkflowAPIServer(t)
	defer closeDB()
	src := booksource.BookSource{BookSourceURL: upstream.URL, BookSourceName: "null decoder", CoverDecodeJS: `null`}
	if err := server.sourceStore.Upsert(&src); err != nil {
		t.Fatal(err)
	}
	b := book.Book{ID: "null-cover", Name: "Null", SourceID: src.ID, SourceURL: src.BookSourceURL, BookURL: upstream.URL + "/book", CoverURL: upstream.URL + "/cover"}
	if err := server.bookStore.AddBook(&b); err != nil {
		t.Fatal(err)
	}

	response := performAPIRequest(server, http.MethodGet, "/api/books/null-cover/cover", nil)
	if response.Code != http.StatusOK || !bytes.Equal(response.Body.Bytes(), original) {
		t.Fatalf("status=%d body=%v", response.Code, response.Body.Bytes())
	}
}

func TestStoredBookCoverDoesNotAcceptTargetURL(t *testing.T) {
	server, closeDB := newWorkflowAPIServer(t)
	defer closeDB()

	response := performAPIRequest(server, http.MethodGet, "/api/books/missing/cover?url=http://127.0.0.1/private", nil)
	if response.Code != http.StatusNotFound || !bytes.Contains(response.Body.Bytes(), []byte("book_not_found")) {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestCandidateCoverReferenceFetchesHTTPImageThroughSameOriginEndpoint(t *testing.T) {
	image := []byte{0xff, 0xd8, 0xff, 0xdb, 0x00, 0x43}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/cover.jpg" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write(image)
	}))
	defer upstream.Close()

	server, closeDB := newWorkflowAPIServer(t)
	defer closeDB()
	src := booksource.BookSource{BookSourceURL: upstream.URL, BookSourceName: "HTTP cover fixture"}
	if err := server.sourceStore.Upsert(&src); err != nil {
		t.Fatal(err)
	}
	result := book.SearchResult{SourceID: src.ID, SourceURL: src.BookSourceURL, BookURL: upstream.URL + "/book", CoverURL: upstream.URL + "/cover.jpg"}
	server.addCoverDisplayURL(&result)
	if !strings.HasPrefix(result.CoverDisplayURL, "/api/covers/") || result.CoverDisplayURL == result.CoverURL {
		t.Fatalf("display URL=%q, want opaque same-origin API URL", result.CoverDisplayURL)
	}

	response := performAPIRequest(server, http.MethodGet, result.CoverDisplayURL, nil)
	if response.Code != http.StatusOK || !bytes.Equal(response.Body.Bytes(), image) {
		t.Fatalf("status=%d body=%v", response.Code, response.Body.Bytes())
	}
	if got := response.Header().Get("Content-Type"); got != "image/jpeg" {
		t.Fatalf("content-type=%q", got)
	}
}

func TestStoredBookResponsesUseVersionedSameOriginCoverURL(t *testing.T) {
	db, err := sql.Open("sqlite", "file:cover-version?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	initializeBookAndSourceAPITestSchema(t, db)
	sourceStore := booksource.NewStore(db)
	bookStore := book.NewStore(db)
	server := NewServer(sourceStore, bookStore, nil, nil, nil, nil, nil, processor.Config{}, "", db)
	source := booksource.BookSource{BookSourceURL: "https://source.test", BookSourceName: "Display source"}
	if err := server.sourceStore.Upsert(&source); err != nil {
		t.Fatal(err)
	}
	stored := &book.Book{ID: "display-book", Name: "Display", SourceID: source.ID, SourceURL: source.BookSourceURL, BookURL: "https://source.test/book", CoverURL: "http://images.test/cover.jpg"}
	if err := server.bookStore.AddBook(stored); err != nil {
		t.Fatal(err)
	}
	var firstURL string
	for _, endpoint := range []string{"/api/books", "/api/books/display-book"} {
		response := performAPIRequest(server, http.MethodGet, endpoint, nil)
		if response.Code != http.StatusOK {
			t.Fatalf("endpoint=%s status=%d body=%s", endpoint, response.Code, response.Body.String())
		}
		var payload interface{}
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		var coverURL string
		switch value := payload.(type) {
		case []interface{}:
			coverURL = value[0].(map[string]interface{})["coverDisplayUrl"].(string)
		case map[string]interface{}:
			coverURL = value["coverDisplayUrl"].(string)
		}
		parsed, err := url.Parse(coverURL)
		if err != nil || parsed.Path != "/api/books/display-book/cover" || parsed.Query().Get("v") == "" {
			t.Fatalf("endpoint=%s coverDisplayUrl=%q", endpoint, coverURL)
		}
		if firstURL == "" {
			firstURL = coverURL
		} else if coverURL != firstURL {
			t.Fatalf("cover URL differs between responses: %q != %q", coverURL, firstURL)
		}
	}

	if _, err := db.Exec(`UPDATE book_sources SET updated_at = updated_at + 1 WHERE id = ?`, source.ID); err != nil {
		t.Fatal(err)
	}
	source.Header = `{"Referer":"https://source.test/book"}`
	if _, err := db.Exec(`UPDATE book_sources SET header = ? WHERE id = ?`, source.Header, source.ID); err != nil {
		t.Fatal(err)
	}
	response := performAPIRequest(server, http.MethodGet, "/api/books/display-book", nil)
	var updated book.Book
	if err := json.Unmarshal(response.Body.Bytes(), &updated); err != nil {
		t.Fatal(err)
	}
	if updated.CoverDisplayURL == firstURL {
		t.Fatalf("cover URL did not change after source replacement: %q", firstURL)
	}
}

func TestCandidateCoverURLChangesWithSourceRevision(t *testing.T) {
	server, closeDB := newWorkflowAPIServer(t)
	defer closeDB()
	result := book.SearchResult{SourceID: "source-test", SourceURL: "https://source.test", BookURL: "https://source.test/book", CoverURL: "https://images.test/cover.jpg"}
	first := server.coverDisplayURL(result.SourceID, result.SourceURL, result.BookURL, result.CoverURL, coverCacheRevision{Source: 1})
	second := server.coverDisplayURL(result.SourceID, result.SourceURL, result.BookURL, result.CoverURL, coverCacheRevision{Source: 2})
	if first == second {
		t.Fatalf("candidate cover URL did not change with source revision: %q", first)
	}
}

func TestCandidateCoverURLChangesWithSourceProfileRevision(t *testing.T) {
	server, closeDB := newWorkflowAPIServer(t)
	defer closeDB()
	result := book.SearchResult{SourceID: "source-test", SourceURL: "https://source.test", BookURL: "https://source.test/book", CoverURL: "https://images.test/cover.jpg"}
	first := server.coverDisplayURL(result.SourceID, result.SourceURL, result.BookURL, result.CoverURL, coverCacheRevision{Source: 1, Profile: sourceprofile.CacheRevision{Settings: 1, Authentication: 3}})
	second := server.coverDisplayURL(result.SourceID, result.SourceURL, result.BookURL, result.CoverURL, coverCacheRevision{Source: 1, Profile: sourceprofile.CacheRevision{Settings: 2, Authentication: 3}})
	if first == second {
		t.Fatalf("candidate cover URL did not change with source profile revision: %q", first)
	}
}

func TestStoredCoverURLChangesWithSourceProfileRevision(t *testing.T) {
	stored := &book.Book{ID: "profile-book", SourceID: "source", SourceURL: "https://source.test", BookURL: "https://source.test/book", CoverURL: "https://images.test/cover.jpg"}
	first := storedCoverDisplayURL(stored, coverCacheRevision{Source: 42, Profile: sourceprofile.CacheRevision{Settings: 5, Authentication: 8}}, "reader")
	second := storedCoverDisplayURL(stored, coverCacheRevision{Source: 42, Profile: sourceprofile.CacheRevision{Settings: 5, Authentication: 9}}, "reader")
	if first == second {
		t.Fatalf("cover URL did not change with source profile revision: %q", first)
	}
}

func TestStoredCoverVersionIsScopedPerReader(t *testing.T) {
	stored := &book.Book{ID: "shared-book", SourceID: "source", SourceURL: "https://source.test", BookURL: "https://source.test/book", CoverURL: "https://images.test/cover.jpg"}
	aliceURL := storedCoverDisplayURL(stored, coverCacheRevision{Source: 42}, "reader-alice")
	bobURL := storedCoverDisplayURL(stored, coverCacheRevision{Source: 42}, "reader-bob")
	if aliceURL == bobURL {
		t.Fatalf("reader-scoped cover URLs match: %q", aliceURL)
	}
}

func TestCandidateCoverReferenceRejectsTampering(t *testing.T) {
	server, closeDB := newWorkflowAPIServer(t)
	defer closeDB()
	result := book.SearchResult{SourceID: "source-test", SourceURL: "https://source.test", BookURL: "https://source.test/book", CoverURL: "http://images.test/cover.jpg"}
	server.addCoverDisplayURL(&result)
	parsed, err := url.Parse(result.CoverDisplayURL)
	if err != nil {
		t.Fatal(err)
	}
	parsed.Path += "x"
	response := performAPIRequest(server, http.MethodGet, parsed.String(), nil)
	if response.Code != http.StatusBadRequest || !bytes.Contains(response.Body.Bytes(), []byte("invalid_cover_reference")) {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}
