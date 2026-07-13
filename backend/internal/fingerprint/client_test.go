// Regression tests for the fingerprint client contract.
package fingerprint

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/otwako/novelreader/internal/fetcher"
	"github.com/otwako/novelreader/internal/sourceexec"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

type captureFallback struct{ cookie string }

func (c *captureFallback) Get(_ string, headers map[string]string) (*fetcher.Response, error) {
	c.cookie = headers["Cookie"]
	return &fetcher.Response{StatusCode: http.StatusOK, Body: "fallback"}, nil
}
func (c *captureFallback) Post(_ string, _ string, _ string, headers map[string]string) (*fetcher.Response, error) {
	c.cookie = headers["Cookie"]
	return &fetcher.Response{StatusCode: http.StatusOK, Body: "fallback"}, nil
}
func (c *captureFallback) GetContextNoRedirect(_ context.Context, _ string, headers map[string]string) (*fetcher.Response, error) {
	c.cookie = headers["Cookie"]
	return &fetcher.Response{StatusCode: http.StatusOK, Body: "fallback"}, nil
}

func TestScopedClientSendsSourceHeaders(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-Source"); got != "configured" {
			t.Errorf("X-Source=%q", got)
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	base, err := New(Config{Timeout: 5 * time.Second, InsecureSkipVerify: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	session := sourceexec.NewSourceSession()
	session.SetRequestHeaders(map[string]string{"x-source": "configured"})
	if _, err := base.ForSource(session).Get(server.URL, nil); err != nil {
		t.Fatal(err)
	}
}

func TestFallbackReceivesSourceSessionCookie(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	fallback := &captureFallback{}
	base, err := New(Config{Timeout: 5 * time.Second, InsecureSkipVerify: true}, fallback)
	if err != nil {
		t.Fatal(err)
	}
	session := sourceexec.NewSourceSession()
	if err := session.SetCookie(server.URL, "csrf", "token"); err != nil {
		t.Fatal(err)
	}
	client := base.ForSource(session)
	if _, err := client.Get(server.URL, nil); err != nil {
		t.Fatal(err)
	}
	if fallback.cookie != "csrf=token" {
		t.Fatalf("fallback cookie=%q", fallback.cookie)
	}
}

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
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/search" {
			http.Error(w, "blocked", http.StatusForbidden)
			return
		}
		_, _ = w.Write([]byte("fallback"))
	}))
	defer server.Close()

	fallback := fetcher.NewInsecure(3 * time.Second)
	client, err := New(Config{Timeout: 3 * time.Second, InsecureSkipVerify: true}, fallback)
	if err != nil {
		t.Fatal(err)
	}
	response, err := client.Get(server.URL+"/search", nil)
	if err != nil {
		t.Fatal(err)
	}
	if response.Body != "blocked\n" || response.StatusCode != http.StatusForbidden {
		t.Fatalf("response=%+v", response)
	}
}
