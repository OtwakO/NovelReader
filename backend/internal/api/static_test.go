package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestServeStaticCoexistsWithAuthenticatedAPIPrefix(t *testing.T) {
	staticDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(staticDir, "index.html"), []byte("app shell"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(staticDir, "assets"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(staticDir, "assets", "index-abc123.js"), []byte("app code"), 0o600); err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "api")
	})
	server := &Server{}
	server.ServeStatic(mux, staticDir, http.FileServer(http.Dir(staticDir)))

	apiResponse := httptest.NewRecorder()
	mux.ServeHTTP(apiResponse, httptest.NewRequest(http.MethodGet, "/api/books", nil))
	if apiResponse.Code != http.StatusOK || apiResponse.Body.String() != "api" {
		t.Fatalf("api status=%d body=%q", apiResponse.Code, apiResponse.Body.String())
	}

	staticResponse := httptest.NewRecorder()
	mux.ServeHTTP(staticResponse, httptest.NewRequest(http.MethodGet, "/reader/book", nil))
	if staticResponse.Code != http.StatusOK || staticResponse.Body.String() != "app shell" {
		t.Fatalf("static status=%d body=%q", staticResponse.Code, staticResponse.Body.String())
	}
	if got := staticResponse.Header().Get("Cache-Control"); got != "no-cache" {
		t.Fatalf("app shell Cache-Control=%q", got)
	}

	assetResponse := httptest.NewRecorder()
	mux.ServeHTTP(assetResponse, httptest.NewRequest(http.MethodGet, "/assets/index-abc123.js", nil))
	if assetResponse.Code != http.StatusOK || assetResponse.Body.String() != "app code" {
		t.Fatalf("asset status=%d body=%q", assetResponse.Code, assetResponse.Body.String())
	}
	if got := assetResponse.Header().Get("Cache-Control"); got != "public, max-age=31536000, immutable" {
		t.Fatalf("asset Cache-Control=%q", got)
	}

	methodResponse := httptest.NewRecorder()
	mux.ServeHTTP(methodResponse, httptest.NewRequest(http.MethodPost, "/reader/book", nil))
	if methodResponse.Code != http.StatusMethodNotAllowed {
		t.Fatalf("static POST status=%d", methodResponse.Code)
	}
}
