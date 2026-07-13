// Conformance tests for fingerprint-first transport selection.
package fingerprint

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/otwako/novelreader/internal/fetcher"
	"github.com/otwako/novelreader/internal/sourceexec"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

func TestClientDecodesExplicitResponseCharset(t *testing.T) {
	encoded, _, err := transform.String(simplifiedchinese.GBK.NewEncoder(), "搜索结果")
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=gbk")
		_, _ = w.Write([]byte(encoded))
	}))
	defer server.Close()

	client, err := New(Config{Timeout: 5 * time.Second, InsecureSkipVerify: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := client.doWithCharset(t.Context(), http.MethodGet, server.URL, "", nil, true, "gbk")
	if err != nil {
		t.Fatal(err)
	}
	if response.Body != "搜索结果" {
		t.Fatalf("body=%q", response.Body)
	}
}

func TestScopedClientsDoNotShareCookies(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/set" {
			http.SetCookie(w, &http.Cookie{Name: "source", Value: "a", Path: "/"})
			return
		}
		_, _ = w.Write([]byte(r.Header.Get("Cookie")))
	}))
	defer server.Close()

	base, err := New(Config{Timeout: 5 * time.Second, InsecureSkipVerify: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	a := base.ForSource(sourceexec.NewSourceSession())
	b := base.ForSource(sourceexec.NewSourceSession())
	if _, err := a.Get(server.URL+"/set", nil); err != nil {
		t.Fatal(err)
	}
	response, err := b.Get(server.URL+"/echo", nil)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(response.Body, "source=a") {
		t.Fatalf("cookie leaked between source clients: %q", response.Body)
	}
}

func TestScopedClientPreservesRedirectCookiesAcrossNewTransport(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redirect" {
			http.SetCookie(w, &http.Cookie{Name: "redirected", Value: "yes", Path: "/"})
			http.Redirect(w, r, "/final", http.StatusFound)
			return
		}
		_, _ = w.Write([]byte(r.Header.Get("Cookie")))
	}))
	defer server.Close()

	base, err := New(Config{Timeout: 5 * time.Second, InsecureSkipVerify: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	session := sourceexec.NewSourceSession()
	first := base.ForSource(session)
	response, err := first.Get(server.URL+"/redirect", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(response.Body, "redirected=yes") {
		t.Fatalf("redirect response cookies=%q", response.Body)
	}
	second := base.ForSource(session)
	response, err = second.Get(server.URL+"/final", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(response.Body, "redirected=yes") {
		t.Fatalf("new client lost redirect cookie: %q", response.Body)
	}
}

func TestClientFallsBackToNormalHTTPAfterFingerprintRejection(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			http.Error(w, "fingerprint rejected", http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte("success"))
	}))
	defer server.Close()

	client, err := New(Config{Timeout: 5 * time.Second, InsecureSkipVerify: true}, fetcher.NewInsecure(5*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	response, err := client.Get(server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK || response.Body != "success" {
		t.Fatalf("status=%d body=%q calls=%d", response.StatusCode, response.Body, calls.Load())
	}
	if calls.Load() != 2 {
		t.Fatalf("calls=%d, want 2", calls.Load())
	}
}
