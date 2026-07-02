// Package fetcher provides an HTTP client with cookie/header management for book source requests.
package fetcher

import (
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"
)

// Client wraps http.Client with book-source-friendly defaults.
type Client struct {
	httpClient *http.Client
	jar        http.CookieJar
	headers    map[string]string
}

// New creates a fetcher Client with cookie jar support.
func New() *Client {
	jar, _ := cookiejar.New(nil)
	return &Client{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Jar:     jar,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 5 {
					return fmt.Errorf("fetcher: too many redirects")
				}
				// Preserve auth headers on redirect
				if len(via) > 0 {
					for k, v := range via[0].Header {
						req.Header[k] = v
					}
				}
				return nil
			},
		},
		jar: jar,
	}
}

// SetHeaders sets default headers for all requests.
func (c *Client) SetHeaders(h map[string]string) {
	c.headers = h
}

// Get performs a GET request.
func (c *Client) Get(rawURL string, extraHeaders map[string]string) (*Response, error) {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("fetcher: new request: %w", err)
	}
	return c.do(req, extraHeaders)
}

// Post performs a POST request with a body.
func (c *Client) Post(rawURL, contentType, body string, extraHeaders map[string]string) (*Response, error) {
	req, err := http.NewRequest(http.MethodPost, rawURL, strings.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("fetcher: new request: %w", err)
	}
	req.Header.Set("Content-Type", contentType)
	return c.do(req, extraHeaders)
}

func (c *Client) do(req *http.Request, extraHeaders map[string]string) (*Response, error) {
	// Set default User-Agent if not already set
	if _, ok := req.Header["User-Agent"]; !ok {
		req.Header.Set("User-Agent", "Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Mobile Safari/537.36")
	}
	// Apply default headers
	for k, v := range c.headers {
		if _, ok := req.Header[k]; !ok {
			req.Header.Set(k, v)
		}
	}
	// Apply extra headers (override defaults)
	for k, v := range extraHeaders {
		req.Header.Set(k, v)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetcher: do: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024)) // 10MB limit
	if err != nil {
		return nil, fmt.Errorf("fetcher: read body: %w", err)
	}

	return &Response{
		StatusCode: resp.StatusCode,
		Body:       string(body),
		Headers:    resp.Header,
		URL:        resp.Request.URL.String(),
	}, nil
}

// Response wraps an HTTP response.
type Response struct {
	StatusCode int
	Body       string
	Headers    http.Header
	URL        string
}

// CookieJar returns the underlying cookie jar for direct manipulation.
func (c *Client) CookieJar() http.CookieJar {
	return c.jar
}

// SetCookies sets cookies for a given URL.
func (c *Client) SetCookies(rawURL string, cookies []*http.Cookie) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return err
	}
	c.jar.SetCookies(u, cookies)
	return nil
}

// Cookies returns stored cookies for a given URL.
func (c *Client) Cookies(rawURL string) []*http.Cookie {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil
	}
	return c.jar.Cookies(u)
}

// ponytail: single user-agent string, no rotation. Add when a source blocks it.
// ponytail: 10MB body limit. Add streaming for very large pages when needed.
