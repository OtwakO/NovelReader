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

type testCookieSession struct{ jar http.CookieJar }

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
