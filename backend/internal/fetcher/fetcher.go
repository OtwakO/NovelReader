package fetcher

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/html/charset"
	"golang.org/x/text/transform"
)

// Client wraps http.Client with context support and optional cookie jar.
type Client struct {
	httpClient *http.Client
	jar        http.CookieJar
	headers    map[string]string
}

// tunedTransport returns an http.Transport optimized for throughput.
// Higher MaxIdleConnsPerHost benefits reading many chapters from the same source.
func tunedTransport() *http.Transport {
	return &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
	}
}

func newHTTPClient(timeout time.Duration, jar http.CookieJar) *http.Client {
	return &http.Client{
		Timeout:   timeout,
		Transport: tunedTransport(),
		Jar:       jar,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("fetcher: too many redirects")
			}
			return nil
		},
	}
}

// New creates a fetcher Client with a 15s timeout and cookie jar.
func New() *Client {
	return NewWithTimeout(15 * time.Second)
}

// NewWithTimeout creates a fetcher with a custom timeout and cookie jar.
func NewWithTimeout(timeout time.Duration) *Client {
	jar, _ := cookiejar.New(nil)
	return &Client{httpClient: newHTTPClient(timeout, jar), jar: jar}
}

// NewStateless creates a fetcher without a cookie jar.
// Safe for concurrent multi-user use — no cross-user cookie leakage.
func NewStateless(timeout time.Duration) *Client {
	return &Client{httpClient: newHTTPClient(timeout, nil)}
}

// SetHeaders sets default headers for all requests.
func (c *Client) SetHeaders(h map[string]string) { c.headers = h }

// Get performs a GET request with a background context.
func (c *Client) Get(rawURL string, extraHeaders map[string]string) (*Response, error) {
	return c.GetContext(context.Background(), rawURL, extraHeaders)
}

// GetContext performs a GET with context support (cancellation, deadlines).
func (c *Client) GetContext(ctx context.Context, rawURL string, extraHeaders map[string]string) (*Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
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
	// Browser-like defaults for better compatibility with Chinese novel sites
	if _, ok := req.Header["User-Agent"]; !ok {
		req.Header.Set("User-Agent", "Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Mobile Safari/537.36")
	}
	if _, ok := req.Header["Accept"]; !ok {
		req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	}
	if _, ok := req.Header["Accept-Language"]; !ok {
		req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	}
	for k, v := range c.headers {
		if _, ok := req.Header[k]; !ok {
			req.Header.Set(k, v)
		}
	}
	for k, v := range extraHeaders {
		req.Header.Set(k, v)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetcher: do: %w", err)
	}
	defer resp.Body.Close()

	// Read raw bytes, detect charset, decode if needed (handles gbk/gb2312 Chinese sites)
	rawBody, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("fetcher: read body: %w", err)
	}
	enc, name, _ := charset.DetermineEncoding(rawBody, resp.Header.Get("Content-Type"))
	if name != "utf-8" && name != "us-ascii" {
		decoded, _, _ := transform.String(enc.NewDecoder(), string(rawBody))
		rawBody = []byte(decoded)
	}
	body := string(rawBody)
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

func (c *Client) CookieJar() http.CookieJar { return c.jar }

func (c *Client) SetCookies(rawURL string, cookies []*http.Cookie) error {
	if c.jar == nil {
		return nil // stateless client, no-op
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return err
	}
	c.jar.SetCookies(u, cookies)
	return nil
}

func (c *Client) Cookies(rawURL string) []*http.Cookie {
	if c.jar == nil {
		return nil
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil
	}
	return c.jar.Cookies(u)
}
