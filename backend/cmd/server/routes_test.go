package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestApplicationMuxMountsPublicBoundariesAndDisablesRecoveryWithNotFound(t *testing.T) {
	marker := func(name string) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("X-Handler", name)
			w.WriteHeader(http.StatusNoContent)
		})
	}
	mux := applicationMux(marker("api"), marker("auth"), marker("setup"), marker("recovery"), false)
	for path, want := range map[string]struct {
		status  int
		handler string
	}{
		"/api/auth/login":      {http.StatusNoContent, "auth"},
		"/api/setup/status":    {http.StatusNoContent, "setup"},
		"/api/recovery/status": {http.StatusNotFound, ""},
		"/api/books":           {http.StatusNoContent, "api"},
	} {
		t.Run(path, func(t *testing.T) {
			response := httptest.NewRecorder()
			mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
			if response.Code != want.status || response.Header().Get("X-Handler") != want.handler {
				t.Fatalf("status=%d handler=%q", response.Code, response.Header().Get("X-Handler"))
			}
		})
	}
}

func TestApplicationMuxMountsConfiguredRecovery(t *testing.T) {
	recovery := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux := applicationMux(http.NotFoundHandler(), http.NotFoundHandler(), http.NotFoundHandler(), recovery, true)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/recovery/status", nil))
	if response.Code != http.StatusNoContent {
		t.Fatalf("status=%d", response.Code)
	}
}
