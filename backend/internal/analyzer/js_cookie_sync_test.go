// Conformance test for cookies shared between staged JavaScript requests and source execution.
package analyzer_test

import (
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/otwako/novelreader/internal/analyzer"
	"github.com/otwako/novelreader/internal/fetcher"
)

type cookieState struct{ jar http.CookieJar }

func newCookieState() *cookieState {
	jar, _ := cookiejar.New(nil)
	return &cookieState{jar: jar}
}
func (s *cookieState) GetCookie(rawURL, key string) string {
	for _, cookie := range s.jar.Cookies(mustURL(rawURL)) {
		if cookie.Name == key {
			return cookie.Value
		}
	}
	return ""
}
func (s *cookieState) CookieHeader(rawURL string) string {
	values := s.jar.Cookies(mustURL(rawURL))
	out := ""
	for i, cookie := range values {
		if i > 0 {
			out += "; "
		}
		out += cookie.Name + "=" + cookie.Value
	}
	return out
}
func (s *cookieState) SetCookie(rawURL, key, value string) error {
	s.jar.SetCookies(mustURL(rawURL), []*http.Cookie{{Name: key, Value: value}})
	return nil
}
func (s *cookieState) SetCookies(rawURL string, cookies []*http.Cookie) error {
	s.jar.SetCookies(mustURL(rawURL), cookies)
	return nil
}
func (*cookieState) RemoveCookies(string) error    { return nil }
func (*cookieState) GetVariable(string) string     { return "" }
func (*cookieState) PutVariable(string, string)    {}
func (*cookieState) GetMemory(string) interface{}  { return nil }
func (*cookieState) PutMemory(string, interface{}) {}
func mustURL(raw string) *url.URL                  { u, _ := url.Parse(raw); return u }

func TestJavaAjaxStoresResponseCookiesInSourceSession(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/set" {
			http.SetCookie(w, &http.Cookie{Name: "session", Value: "abc", Path: "/"})
			return
		}
		if _, err := r.Cookie("session"); err == nil {
			_, _ = w.Write([]byte("yes"))
			return
		}
		_, _ = w.Write([]byte("no"))
	}))
	defer server.Close()

	vm := analyzer.NewJSVM()
	vm.SetFetcher(fetcher.NewInsecureStateless(3 * time.Second))
	value, err := vm.Eval(`java.ajax("`+server.URL+`/set"); java.ajax("`+server.URL+`/check")`, "", server.URL,
		map[string]interface{}{"sourceState": newCookieState()})
	if err != nil {
		t.Fatal(err)
	}
	if got := analyzer.ToString(value); got != "yes" {
		t.Fatalf("value=%q", got)
	}
}
