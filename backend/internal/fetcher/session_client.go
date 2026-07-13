// Cookie-session adapter for JavaScript HTTP helpers that lack native source scoping.
package fetcher

import (
	"context"
	"net/http"
	"strings"
)

// SessionHTTPClient adds source-session cookies to an existing HTTP client.
type SessionHTTPClient struct {
	base    HTTPClient
	session CookieSession
}

// NewSessionHTTPClient creates a scoped client without changing the wrapped client's jar.
func NewSessionHTTPClient(base HTTPClient, session CookieSession) *SessionHTTPClient {
	return &SessionHTTPClient{base: base, session: session}
}

func (c *SessionHTTPClient) Get(rawURL string, headers map[string]string) (*Response, error) {
	return c.GetContext(context.Background(), rawURL, headers)
}

func (c *SessionHTTPClient) GetContext(ctx context.Context, rawURL string, headers map[string]string, retry ...int) (*Response, error) {
	headers = c.requestHeaders(rawURL, headers)
	var (
		response *Response
		err      error
	)
	if client, ok := c.base.(ContextHTTPClient); ok {
		response, err = client.GetContext(ctx, rawURL, headers, retry...)
	} else {
		response, err = c.base.Get(rawURL, headers)
	}
	if err == nil {
		err = c.syncCookies(rawURL, response)
	}
	return response, err
}

func (c *SessionHTTPClient) GetContextNoRedirect(ctx context.Context, rawURL string, headers map[string]string) (*Response, error) {
	headers = c.requestHeaders(rawURL, headers)
	var (
		response *Response
		err      error
	)
	if client, ok := c.base.(ContextHTTPClient); ok {
		response, err = client.GetContextNoRedirect(ctx, rawURL, headers)
	} else {
		response, err = c.base.Get(rawURL, headers)
	}
	if err == nil {
		err = c.syncCookies(rawURL, response)
	}
	return response, err
}

func (c *SessionHTTPClient) Post(rawURL, contentType, body string, headers map[string]string) (*Response, error) {
	if contentType != "" && !hasSessionHeader(headers, "Content-Type") {
		copy := make(map[string]string, len(headers)+1)
		for key, value := range headers {
			copy[key] = value
		}
		copy["Content-Type"] = contentType
		headers = copy
	}
	return c.PostContext(context.Background(), rawURL, body, headers, 0)
}

func (c *SessionHTTPClient) PostContext(ctx context.Context, rawURL, body string, headers map[string]string, retry int) (*Response, error) {
	headers = c.requestHeaders(rawURL, headers)
	if !hasSessionHeader(headers, "Content-Type") {
		headers["Content-Type"] = "application/x-www-form-urlencoded"
	}
	var (
		response *Response
		err      error
	)
	if client, ok := c.base.(ContextHTTPClient); ok {
		response, err = client.PostContext(ctx, rawURL, body, headers, retry)
	} else {
		response, err = c.base.Post(rawURL, "application/x-www-form-urlencoded", body, headers)
	}
	if err == nil {
		err = c.syncCookies(rawURL, response)
	}
	return response, err
}

func (c *SessionHTTPClient) requestHeaders(rawURL string, headers map[string]string) map[string]string {
	copy := make(map[string]string, len(headers)+1)
	if source, ok := c.session.(interface{ RequestHeaders() map[string]string }); ok {
		for key, value := range source.RequestHeaders() {
			copy[key] = value
		}
	}
	for key, value := range headers {
		for existing := range copy {
			if strings.EqualFold(existing, key) {
				delete(copy, existing)
			}
		}
		copy[key] = value
	}
	if c.session != nil && !hasSessionHeader(copy, "Cookie") {
		if cookie := c.session.CookieHeader(rawURL); cookie != "" {
			copy["Cookie"] = cookie
		}
	}
	return copy
}

func hasSessionHeader(headers map[string]string, name string) bool {
	for key := range headers {
		if strings.EqualFold(key, name) {
			return true
		}
	}
	return false
}

func (c *SessionHTTPClient) syncCookies(requestURL string, response *Response) error {
	if c.session == nil || response == nil {
		return nil
	}
	rawURL := response.URL
	if rawURL == "" {
		rawURL = requestURL
	}
	cookies := (&http.Response{Header: response.Headers}).Cookies()
	return c.session.SetCookies(rawURL, cookies)
}
