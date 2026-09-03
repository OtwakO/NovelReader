package sourceprofile

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"sort"
	"strings"
)

const (
	maxRuntimeCookieScopes = 128
	maxRuntimeCookieHeader = 64 * 1024
)

var ErrInvalidRuntimeCookies = errors.New("sourceprofile: invalid runtime cookies")

// RuntimeCookie is one editable cookie header scoped to an HTTP(S) URL.
type RuntimeCookie struct {
	Scope  string `json:"scope"`
	Header string `json:"header"`
}

// RuntimeCookies returns a stable copy of one source's stored runtime cookies.
func (s *Store) RuntimeCookies(ctx context.Context, sourceID string) ([]RuntimeCookie, error) {
	profile, err := s.Load(ctx, sourceID)
	if err != nil {
		return nil, err
	}
	authentication := DecodeAuthentication(profile.Authentication)
	cookies := make([]RuntimeCookie, 0, len(authentication.Cookies))
	for scope, header := range authentication.Cookies {
		cookies = append(cookies, RuntimeCookie{Scope: scope, Header: header})
	}
	sort.Slice(cookies, func(i, j int) bool { return cookies[i].Scope < cookies[j].Scope })
	return cookies, nil
}

// ReplaceRuntimeCookies validates and replaces only the cookie portion of source authentication.
func (s *Store) ReplaceRuntimeCookies(ctx context.Context, sourceID string, cookies []RuntimeCookie) error {
	if len(cookies) > maxRuntimeCookieScopes {
		return ErrInvalidRuntimeCookies
	}
	normalized := make(map[string]string, len(cookies))
	for _, cookie := range cookies {
		scope, header, err := normalizeRuntimeCookie(cookie)
		if err != nil {
			return err
		}
		if _, exists := normalized[scope]; exists {
			return ErrInvalidRuntimeCookies
		}
		normalized[scope] = header
	}

	profile, err := s.Load(ctx, sourceID)
	if err != nil {
		return err
	}
	authentication := DecodeAuthentication(profile.Authentication)
	authentication.Cookies = normalized
	document, err := json.Marshal(authentication)
	if err != nil {
		return err
	}
	return s.SaveAuthentication(ctx, sourceID, document)
}

func normalizeRuntimeCookie(cookie RuntimeCookie) (string, string, error) {
	scope := strings.TrimSpace(cookie.Scope)
	parsed, err := url.Parse(scope)
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", "", ErrInvalidRuntimeCookies
	}

	header := strings.TrimSpace(cookie.Header)
	if header == "" || len(header) > maxRuntimeCookieHeader {
		return "", "", ErrInvalidRuntimeCookies
	}
	pairs := strings.Split(header, ";")
	normalizedPairs := make([]string, 0, len(pairs))
	for _, pair := range pairs {
		parts := strings.SplitN(strings.TrimSpace(pair), "=", 2)
		if len(parts) != 2 {
			return "", "", ErrInvalidRuntimeCookies
		}
		item := http.Cookie{Name: strings.TrimSpace(parts[0]), Value: strings.TrimSpace(parts[1])}
		if item.Valid() != nil {
			return "", "", ErrInvalidRuntimeCookies
		}
		normalizedPairs = append(normalizedPairs, item.Name+"="+item.Value)
	}
	return parsed.String(), strings.Join(normalizedPairs, "; "), nil
}
