// HTTP transport adapter for the shared booksource request contract.
package sourceexec

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/otwako/novelreader/internal/fetcher"
)

// HTTPTransport executes RequestSpec values through the existing HTTP client.
type HTTPTransport struct {
	client  *fetcher.Client
	session *SourceSession
}

// NewHTTPTransport wraps a configured fetcher client without source session sync.
func NewHTTPTransport(client *fetcher.Client) *HTTPTransport {
	return &HTTPTransport{client: client}
}

// NewHTTPTransportForSession binds one HTTP client and cookie session together.
// Stateless clients may share their connection pool; jar-backed clients must remain isolated.
func NewHTTPTransportForSession(client *fetcher.Client, session *SourceSession) *HTTPTransport {
	return &HTTPTransport{client: client, session: session}
}

// Do executes GET and POST requests and retains bodies for all HTTP statuses.
func (t *HTTPTransport) Do(ctx context.Context, spec RequestSpec) (Response, error) {
	if t == nil || t.client == nil {
		return Response{}, fmt.Errorf("sourceexec: HTTP transport has no client")
	}

	method := strings.ToUpper(strings.TrimSpace(spec.Method))
	if method == "" {
		method = http.MethodGet
	}

	if t.session != nil {
		if err := t.client.SetCookies(spec.URL, t.session.Cookies(spec.URL)); err != nil {
			return Response{}, fmt.Errorf("sourceexec: sync session cookies before request: %w", err)
		}
	}

	headers := cloneHeaders(spec.Headers)
	if t.session != nil && t.client.CookieJar() == nil && !hasHeader(headers, "Cookie") {
		if cookie := t.session.CookieHeader(spec.URL); cookie != "" {
			headers["Cookie"] = cookie
		}
	}
	if spec.Origin != "" && !hasHeader(headers, "Origin") {
		headers["Origin"] = spec.Origin
	}

	var (
		resp *fetcher.Response
		err  error
	)
	switch method {
	case http.MethodGet:
		if spec.DNSIP != "" {
			resp, err = t.client.GetContextWithCharsetAndDNSIP(ctx, spec.URL, headers, spec.Retry, spec.Charset, spec.DNSIP)
		} else {
			resp, err = t.client.GetContextWithCharset(ctx, spec.URL, headers, spec.Retry, spec.Charset)
		}
	case http.MethodHead:
		if spec.DNSIP != "" {
			resp, err = t.client.HeadContextWithCharsetAndDNSIP(ctx, spec.URL, headers, spec.Retry, spec.DNSIP)
		} else {
			resp, err = t.client.HeadContextWithCharset(ctx, spec.URL, headers, spec.Retry)
		}
	case http.MethodPost:
		body, contentType := PreparePOST(spec.Body, spec.Charset, headers)
		if !hasHeader(headers, "Content-Type") {
			headers["Content-Type"] = contentType
		}
		if spec.DNSIP != "" {
			resp, err = t.client.PostContextWithCharsetAndDNSIP(ctx, spec.URL, body, headers, spec.Retry, spec.Charset, spec.DNSIP)
		} else {
			resp, err = t.client.PostContextWithCharset(ctx, spec.URL, body, headers, spec.Retry, spec.Charset)
		}
	default:
		return Response{}, fmt.Errorf("sourceexec: unsupported HTTP method %q", method)
	}
	if err != nil {
		return Response{}, err
	}
	if t.session != nil {
		cookieURLs := []string{spec.URL}
		if resp.URL != "" && resp.URL != spec.URL {
			cookieURLs = append(cookieURLs, resp.URL)
		}
		responseCookies := (&http.Response{Header: resp.Headers}).Cookies()
		for _, cookieURL := range cookieURLs {
			cookies := mergeCookies(responseCookies, t.client.Cookies(cookieURL))
			if err := t.session.SetCookies(cookieURL, cookies); err != nil {
				return Response{}, fmt.Errorf("sourceexec: sync session cookies after response: %w", err)
			}
		}
	}

	return Response{
		StatusCode:    resp.StatusCode,
		Headers:       cloneResponseHeaders(resp.Headers),
		Body:          resp.Body,
		FinalURL:      resp.URL,
		Transport:     "http",
		RedirectChain: append([]string(nil), resp.RedirectChain...),
	}, nil
}

func mergeCookies(first, second []*http.Cookie) []*http.Cookie {
	cookies := append([]*http.Cookie(nil), first...)
	seen := make(map[string]struct{}, len(cookies))
	for _, cookie := range cookies {
		seen[cookie.Name+"\x00"+cookie.Domain+"\x00"+cookie.Path] = struct{}{}
	}
	for _, cookie := range second {
		key := cookie.Name + "\x00" + cookie.Domain + "\x00" + cookie.Path
		if _, exists := seen[key]; !exists {
			cookies = append(cookies, cookie)
			seen[key] = struct{}{}
		}
	}
	return cookies
}

func hasHeader(headers map[string]string, name string) bool {
	for key := range headers {
		if strings.EqualFold(key, name) {
			return true
		}
	}
	return false
}

func cloneResponseHeaders(headers http.Header) map[string][]string {
	copy := make(map[string][]string, len(headers))
	for key, values := range headers {
		copy[key] = append([]string(nil), values...)
	}
	return copy
}
