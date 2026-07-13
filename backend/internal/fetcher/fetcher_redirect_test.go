// Conformance test for redirect-aware JavaScript HTTP access.
package fetcher

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestStatelessCloneSharesTransportWithoutCookies(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/set" {
			http.SetCookie(w, &http.Cookie{Name: "private", Value: "yes", Path: "/"})
		}
		if r.URL.Path == "/check" && r.Header.Get("Cookie") != "" {
			t.Errorf("stateless clone sent Cookie=%q", r.Header.Get("Cookie"))
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	original := NewWithTimeout(3 * time.Second)
	clone := original.StatelessClone()
	if clone == nil || clone.httpClient.Transport != original.httpClient.Transport {
		t.Fatal("clone did not share the connection transport")
	}
	if _, err := original.Get(server.URL+"/set", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := clone.Get(server.URL+"/check", nil); err != nil {
		t.Fatal(err)
	}
}

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
