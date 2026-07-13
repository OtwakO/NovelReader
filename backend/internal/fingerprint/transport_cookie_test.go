// Regression coverage for redirect-origin cookie propagation.
package fingerprint

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/otwako/novelreader/internal/sourceexec"
)

func TestFingerprintTransportPreservesRedirectOriginCookie(t *testing.T) {
	final := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("final"))
	}))
	defer final.Close()
	finalURL := strings.Replace(final.URL, "127.0.0.1", "localhost", 1)
	origin := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

	session := sourceexec.NewSourceSession()
	first, err := NewTransport(Config{Timeout: 5 * time.Second, InsecureSkipVerify: true}, nil, session)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := first.Do(t.Context(), sourceexec.RequestSpec{URL: origin.URL + "/redirect"}); err != nil {
		t.Fatal(err)
	}
	second, err := NewTransport(Config{Timeout: 5 * time.Second, InsecureSkipVerify: true}, nil, session)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := second.Do(t.Context(), sourceexec.RequestSpec{URL: origin.URL + "/check"}); err != nil {
		t.Fatal(err)
	}
}
