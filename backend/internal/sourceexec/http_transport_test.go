// Conformance tests for the HTTP implementation of the source transport contract.
package sourceexec

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/otwako/novelreader/internal/fetcher"
)

func TestHTTPTransportRecordsRedirectChain(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/start" {
			http.Redirect(w, r, "/final", http.StatusFound)
			return
		}
		_, _ = w.Write([]byte("final"))
	}))
	defer server.Close()

	response, err := NewHTTPTransport(fetcher.NewWithTimeout(3*time.Second)).Do(t.Context(), RequestSpec{URL: server.URL + "/start"})
	if err != nil {
		t.Fatal(err)
	}
	if response.FinalURL != server.URL+"/final" || len(response.RedirectChain) != 1 || response.RedirectChain[0] != server.URL+"/final" {
		t.Fatalf("redirect chain=%+v final=%q", response.RedirectChain, response.FinalURL)
	}
}

func TestHTTPTransportPreservesRequestAndNonSuccessResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if got := r.Header.Get("X-Source"); got != "fixture" {
			t.Errorf("X-Source = %q, want fixture", got)
		}
		if got := r.Header.Get("Content-Type"); got != "application/x-www-form-urlencoded" {
			t.Errorf("Content-Type = %q, want form content type", got)
		}
		w.Header().Set("X-Final", "yes")
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte("usable diagnostic body"))
	}))
	defer server.Close()

	transport := NewHTTPTransport(fetcher.NewWithTimeout(3 * time.Second))
	response, err := transport.Do(t.Context(), RequestSpec{
		URL:     server.URL + "/search",
		Method:  http.MethodPost,
		Body:    "q=fixture",
		Headers: map[string]string{"X-Source": "fixture"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusTeapot || response.Body != "usable diagnostic body" {
		t.Fatalf("response status/body lost: %+v", response)
	}
	if response.FinalURL != server.URL+"/search" || response.Transport != "http" {
		t.Fatalf("response URL/transport lost: %+v", response)
	}
	if response.Headers["X-Final"][0] != "yes" {
		t.Fatalf("response headers lost: %+v", response.Headers)
	}
}
