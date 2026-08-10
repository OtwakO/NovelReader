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

	methodResponse := httptest.NewRecorder()
	mux.ServeHTTP(methodResponse, httptest.NewRequest(http.MethodPost, "/reader/book", nil))
	if methodResponse.Code != http.StatusMethodNotAllowed {
		t.Fatalf("static POST status=%d", methodResponse.Code)
	}
}
