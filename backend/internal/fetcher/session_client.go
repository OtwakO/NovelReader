// Cookie-session adapter for JavaScript HTTP helpers that lack native source scoping.
package fetcher

import (
	"context"
	"fmt"
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

func (c *SessionHTTPClient) GetBytesContext(ctx context.Context, rawURL string, headers map[string]string, maxBytes int64) (*Response, error) {
	headers = c.requestHeaders(rawURL, headers)
	client, ok := c.base.(interface {
		GetBytesContext(context.Context, string, map[string]string, int64) (*Response, error)
	})
	if !ok {
		return nil, fmt.Errorf("fetcher: binary GET is not supported by the wrapped client")
	}
	response, err := client.GetBytesContext(ctx, rawURL, headers, maxBytes)
	if err == nil {
		err = c.syncCookies(rawURL, response)
	}
	return response, err
}

func (c *SessionHTTPClient) GetContextWithCharset(ctx context.Context, rawURL string, headers map[string]string, retry int, responseCharset string) (*Response, error) {
	headers = c.requestHeaders(rawURL, headers)
	var (
		response *Response
		err      error
	)
	if client, ok := c.base.(interface {
		GetContextWithCharset(context.Context, string, map[string]string, int, string) (*Response, error)
	}); ok {
		response, err = client.GetContextWithCharset(ctx, rawURL, headers, retry, responseCharset)
	} else if client, ok := c.base.(ContextHTTPClient); ok {
		response, err = client.GetContext(ctx, rawURL, headers, retry)
	} else {
		response, err = c.base.Get(rawURL, headers)
	}
	if err == nil {
		err = c.syncCookies(rawURL, response)
	}
	return response, err
}

func (c *SessionHTTPClient) HeadContextWithCharset(ctx context.Context, rawURL string, headers map[string]string, retry int) (*Response, error) {
	headers = c.requestHeaders(rawURL, headers)
	client, ok := c.base.(interface {
		HeadContextWithCharset(context.Context, string, map[string]string, int) (*Response, error)
	})
	if !ok {
		return nil, fmt.Errorf("fetcher: HEAD is not supported by the wrapped client")
	}
	response, err := client.HeadContextWithCharset(ctx, rawURL, headers, retry)
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

func (c *SessionHTTPClient) PostContextWithCharset(ctx context.Context, rawURL, body string, headers map[string]string, retry int, responseCharset string) (*Response, error) {
	headers = c.requestHeaders(rawURL, headers)
	if !hasSessionHeader(headers, "Content-Type") {
		headers["Content-Type"] = "application/x-www-form-urlencoded"
	}
	var (
		response *Response
		err      error
	)
	if client, ok := c.base.(interface {
		PostContextWithCharset(context.Context, string, string, map[string]string, int, string) (*Response, error)
	}); ok {
		response, err = client.PostContextWithCharset(ctx, rawURL, body, headers, retry, responseCharset)
	} else if client, ok := c.base.(ContextHTTPClient); ok {
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
	if session, ok := c.session.(interface {
		SetResponseCookies(string, []*http.Cookie) error
	}); ok {
		return session.SetResponseCookies(rawURL, cookies)
	}
	return c.session.SetCookies(rawURL, cookies)
}
