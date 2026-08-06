package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/otwako/novelreader/internal/book"
	"github.com/otwako/novelreader/internal/booksource"
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
	b := book.Book{ID: "cover-book", Name: "Cover Fixture", SourceURL: src.BookSourceURL, BookURL: upstream.URL + "/book", CoverURL: upstream.URL + "/cover"}
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
	b := book.Book{ID: "header-cover", Name: "Header", SourceURL: src.BookSourceURL, BookURL: upstream.URL + "/book", CoverURL: coverURL}
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
	b := book.Book{ID: "null-cover", Name: "Null", SourceURL: src.BookSourceURL, BookURL: upstream.URL + "/book", CoverURL: upstream.URL + "/cover"}
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
