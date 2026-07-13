// Conformance tests for session cookie synchronization through HTTP transport.
package sourceexec

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/otwako/novelreader/internal/fetcher"
)

func TestHTTPTransportPreservesCookiesFromRedirectOrigin(t *testing.T) {
	final := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("final"))
	}))
	defer final.Close()
	finalURL := strings.Replace(final.URL, "127.0.0.1", "localhost", 1)
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/redirect":
			http.SetCookie(w, &http.Cookie{Name: "origin", Value: "kept", Path: "/"})
			http.Redirect(w, r, finalURL, http.StatusFound)
		case "/check":
			if got := r.Header.Get("Cookie"); got != "origin=kept" {
				t.Errorf("redirect-origin Cookie = %q, want origin=kept", got)
			}
			_, _ = w.Write([]byte("check"))
		}
	}))
	defer origin.Close()

	session := NewSourceSession()
	transport := NewHTTPTransportForSession(fetcher.NewWithTimeout(3*time.Second), session)
	if _, err := transport.Do(t.Context(), RequestSpec{URL: origin.URL + "/redirect"}); err != nil {
		t.Fatal(err)
	}
	transport = NewHTTPTransportForSession(fetcher.NewWithTimeout(3*time.Second), session)
	if _, err := transport.Do(t.Context(), RequestSpec{URL: origin.URL + "/check"}); err != nil {
		t.Fatal(err)
	}
}

func TestHTTPTransportSynchronizesCookiesWithSourceSession(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/start":
			http.SetCookie(w, &http.Cookie{Name: "csrf", Value: "fixture", Path: "/"})
			_, _ = w.Write([]byte("start"))
		case "/continue":
			if got := r.Header.Get("Cookie"); got != "csrf=fixture" {
				t.Errorf("Cookie = %q, want csrf=fixture", got)
			}
			_, _ = w.Write([]byte("continue"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	session := NewSourceSession()
	transport := NewHTTPTransportForSession(fetcher.NewWithTimeout(3*time.Second), session)
	if _, err := transport.Do(t.Context(), RequestSpec{URL: server.URL + "/start"}); err != nil {
		t.Fatal(err)
	}
	if got := session.GetCookie(server.URL+"/start", "csrf"); got != "fixture" {
		t.Fatalf("session cookie = %q, want fixture", got)
	}
	if _, err := transport.Do(t.Context(), RequestSpec{URL: server.URL + "/continue"}); err != nil {
		t.Fatal(err)
	}
}
