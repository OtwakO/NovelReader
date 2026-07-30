// Source-scoped cookies, variables, login headers, and memory used by Legado rule execution.
package sourceexec

import (
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/otwako/novelreader/internal/analyzer"
)

// SourceSession isolates mutable source state from other sources and users.
type SourceSession struct {
	jar         http.CookieJar
	mu          sync.RWMutex
	vars        map[string]string
	memory      map[string]interface{}
	headers     map[string]string
	loginHeader string
	loginCookie string
	lastURL     string
}

// NewSourceSession creates an isolated cookie and variable scope.
func NewSourceSession() *SourceSession {
	jar, _ := cookiejar.New(nil)
	return &SourceSession{jar: jar, vars: make(map[string]string), memory: make(map[string]interface{}), headers: make(map[string]string)}
}

// SetCookie stores one cookie for a source URL.
func (s *SourceSession) SetCookie(rawURL, name, value string) error {
	u, err := parseSessionURL(rawURL)
	if err != nil {
		return err
	}
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("sourceexec: empty cookie name")
	}
	s.jar.SetCookies(u, []*http.Cookie{{Name: name, Value: value, Path: "/"}})
	return nil
}

// GetCookie returns one cookie value, or the empty string when absent.
func (s *SourceSession) GetCookie(rawURL, name string) string {
	u, err := parseSessionURL(rawURL)
	if err != nil {
		return ""
	}
	for _, cookie := range s.jar.Cookies(u) {
		if cookie.Name == name {
			return cookie.Value
		}
	}
	return ""
}

// CookieHeader returns cookies in HTTP request-header form.
func (s *SourceSession) CookieHeader(rawURL string) string {
	u, err := parseSessionURL(rawURL)
	if err != nil {
		return ""
	}
	cookies := s.jar.Cookies(u)
	values := make([]string, 0, len(cookies))
	for _, cookie := range cookies {
		values = append(values, cookie.Name+"="+cookie.Value)
	}
	s.mu.RLock()
	loginCookie := s.loginCookie
	s.mu.RUnlock()
	if loginCookie != "" {
		return loginCookie
	}
	return strings.Join(values, "; ")
}

// Cookies exposes a copy of the cookies applicable to a URL for transport sync.
func (s *SourceSession) Cookies(rawURL string) []*http.Cookie {
	u, err := parseSessionURL(rawURL)
	if err != nil {
		return nil
	}
	return append([]*http.Cookie(nil), s.jar.Cookies(u)...)
}

// RemoveCookies clears cookies applicable to a source URL.
func (s *SourceSession) RemoveCookies(rawURL string) error {
	u, err := parseSessionURL(rawURL)
	if err != nil {
		return err
	}
	cookies := s.jar.Cookies(u)
	for _, cookie := range cookies {
		cookie.MaxAge = -1
		cookie.Expires = time.Unix(1, 0)
	}
	s.jar.SetCookies(u, cookies)
	return nil
}

// SetCookies imports cookies received from a transport into this session.
func (s *SourceSession) SetCookies(rawURL string, cookies []*http.Cookie) error {
	u, err := parseSessionURL(rawURL)
	if err != nil {
		return err
	}
	s.jar.SetCookies(u, cookies)
	return nil
}

// CookieJar returns the session jar for an HTTP transport created for this session.
func (s *SourceSession) CookieJar() http.CookieJar { return s.jar }

// SetLastURL records the final URL of the latest staged source response.
func (s *SourceSession) SetLastURL(rawURL string) {
	s.mu.Lock()
	s.lastURL = rawURL
	s.mu.Unlock()
}

// LastURL returns the final URL of the latest staged source response.
func (s *SourceSession) LastURL() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastURL
}

// SetRequestHeaders stores source-level defaults for staged and JavaScript requests.
func (s *SourceSession) SetRequestHeaders(headers map[string]string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.headers = make(map[string]string, len(headers))
	for key, value := range headers {
		s.headers[key] = value
	}
}

// RequestHeaders returns source defaults overlaid by stored login headers.
func (s *SourceSession) RequestHeaders() map[string]string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	headers := make(map[string]string, len(s.headers))
	for key, value := range s.headers {
		headers[key] = value
	}
	if login, ok := analyzer.ParseLenientStringMap(s.loginHeader); ok {
		for key, value := range login {
			for existing := range headers {
				if strings.EqualFold(existing, key) {
					delete(headers, existing)
				}
			}
			headers[key] = value
		}
	}
	return headers
}

// SetLoginHeader stores login headers and synchronizes any Cookie entry with the source cookie jar.
func (s *SourceSession) SetLoginHeader(header string) {
	s.mu.Lock()
	s.loginHeader = header
	s.mu.Unlock()
	login, ok := analyzer.ParseLenientStringMap(header)
	if !ok {
		return
	}
	for key, value := range login {
		if strings.EqualFold(key, "Cookie") {
			s.mu.Lock()
			s.loginCookie = value
			s.mu.Unlock()
			return
		}
	}
}

// LoginHeader returns the source login-header JSON, or an empty string when absent.
func (s *SourceSession) LoginHeader() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.loginHeader
}

// PutVariable stores a persistent source variable.
func (s *SourceSession) PutVariable(key, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.vars[key] = value
}

// GetVariable returns a persistent source variable.
func (s *SourceSession) GetVariable(key string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.vars[key]
}

// PutMemory stores request-flow memory that may contain non-string JS values.
func (s *SourceSession) PutMemory(key string, value interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.memory[key] = value
}

// GetMemory returns request-flow memory.
func (s *SourceSession) GetMemory(key string) interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.memory[key]
}

func parseSessionURL(rawURL string) (*url.URL, error) {
	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("sourceexec: invalid session URL %q", rawURL)
	}
	return u, nil
}
