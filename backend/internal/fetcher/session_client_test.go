// Regression coverage for source-session cookies used by JavaScript HTTP helpers.
package fetcher

import (
	"context"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

type testCookieSession struct {
	jar     http.CookieJar
	headers map[string]string
}

func (s *testCookieSession) RequestHeaders() map[string]string { return s.headers }

func (s *testCookieSession) CookieHeader(rawURL string) string {
	u, _ := url.Parse(rawURL)
	cookies := s.jar.Cookies(u)
	if len(cookies) == 0 {
		return ""
	}
	return cookies[0].Name + "=" + cookies[0].Value
}

func (s *testCookieSession) SetCookies(rawURL string, cookies []*http.Cookie) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return err
	}
	s.jar.SetCookies(u, cookies)
	return nil
}

func TestSessionHTTPClientPreservesPostContentType(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("Content-Type=%q", got)
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	jar, _ := cookiejar.New(nil)
	client := NewSessionHTTPClient(NewInsecureStateless(3*time.Second), &testCookieSession{jar: jar})
	if _, err := client.Post(server.URL, "application/json", "{}", nil); err != nil {
		t.Fatal(err)
	}
}

func TestSessionHTTPClientAppliesSessionHeadersToGetAndPost(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "login" {
			t.Errorf("Authorization=%q", got)
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	jar, _ := cookiejar.New(nil)
	client := NewSessionHTTPClient(NewInsecureStateless(3*time.Second), &testCookieSession{jar: jar, headers: map[string]string{"Authorization": "login"}})
	if _, err := client.GetContext(t.Context(), server.URL, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Post(server.URL, "application/json", "{}", nil); err != nil {
		t.Fatal(err)
	}
}

func TestSessionHTTPClientCarriesCookiesAcrossRequests(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/set":
			http.SetCookie(w, &http.Cookie{Name: "source", Value: "ok", Path: "/"})
		case "/check":
			if got := r.Header.Get("Cookie"); got != "source=ok" {
				t.Errorf("Cookie=%q", got)
			}
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	session := &testCookieSession{jar: jar}
	client := NewSessionHTTPClient(NewInsecureStateless(3*time.Second), session)
	if _, err := client.GetContext(context.Background(), server.URL+"/set", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := client.GetContext(context.Background(), server.URL+"/check", nil); err != nil {
		t.Fatal(err)
	}
}
