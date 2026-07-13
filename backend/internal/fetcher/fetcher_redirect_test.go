// Conformance test for redirect-aware JavaScript HTTP access.
package fetcher

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClientCanPreserveRedirectResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/start" {
			http.Redirect(w, r, "/result/?searchid=42", http.StatusFound)
			return
		}
		_, _ = w.Write([]byte("result"))
	}))
	defer server.Close()

	response, err := NewWithTimeout(3*time.Second).GetContextNoRedirect(t.Context(), server.URL+"/start", nil)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusFound || response.Headers.Get("Location") != "/result/?searchid=42" {
		t.Fatalf("status=%d location=%q", response.StatusCode, response.Headers.Get("Location"))
	}
}
