// Source-scoped cookies, variables, and memory used by Legado rule execution.
package sourceexec

import (
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"sync"
)

// SourceSession isolates mutable source state from other sources and users.
type SourceSession struct {
	jar    http.CookieJar
	mu     sync.RWMutex
	vars   map[string]string
	memory map[string]interface{}
}

// NewSourceSession creates an isolated cookie and variable scope.
func NewSourceSession() *SourceSession {
	jar, _ := cookiejar.New(nil)
	return &SourceSession{jar: jar, vars: make(map[string]string), memory: make(map[string]interface{})}
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

// CookieJar returns the session jar for an HTTP transport created for this session.
func (s *SourceSession) CookieJar() http.CookieJar { return s.jar }

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
