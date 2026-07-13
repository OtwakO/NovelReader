package fetcher

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log/slog"
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
// If insecure is true, TLS verification is skipped (required for many Chinese novel sites
// that have self-signed or expired certs — same tradeoff legado makes).
func tunedTransport(insecure bool) *http.Transport {
	tlsCfg := &tls.Config{
		InsecureSkipVerify: insecure, // legado uses unsafeTrustManager
	}
	return &http.Transport{
		TLSClientConfig:     tlsCfg,
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
	}
}

func newHTTPClient(timeout time.Duration, jar http.CookieJar, insecure bool) *http.Client {
	return &http.Client{
		Timeout:   timeout,
		Transport: tunedTransport(insecure),
		Jar:       jar,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("fetcher: too many redirects")
			}
			return nil
		},
	}
}

// New creates a fetcher Client with a 15s timeout, cookie jar, and secure TLS.
func New() *Client { return NewWithTimeout(15 * time.Second) }

// NewWithTimeout creates a fetcher with a custom timeout and cookie jar.
func NewWithTimeout(timeout time.Duration) *Client {
	jar, _ := cookiejar.New(nil)
	return &Client{httpClient: newHTTPClient(timeout, jar, false), jar: jar}
}

// NewInsecure creates a fetcher with InsecureSkipVerify (trusts all TLS certs).
// Matches legado's unsafeTrustManager behavior — required for many Chinese novel sites.
func NewInsecure(timeout time.Duration) *Client {
	jar, _ := cookiejar.New(nil)
	slog.Warn("fetcher: created with InsecureSkipVerify — TLS certs are NOT verified")
	return &Client{
		httpClient: newHTTPClient(timeout, jar, true),
		jar:        jar,
	}
}

// NewInsecureStateless creates a fetcher with InsecureSkipVerify but NO cookie jar.
// Use for search / stateless requests where cookie isolation matters.
// All 939 real-world legado sources have enabledCookieJar=false, so this is the correct default.
func NewInsecureStateless(timeout time.Duration) *Client {
	slog.Warn("fetcher: created with InsecureSkipVerify, no cookie jar (stateless)")
	return &Client{
		httpClient: newHTTPClient(timeout, nil, true),
		jar:        nil,
	}
}

// SetHeaders sets default headers for all requests.
func (c *Client) SetHeaders(h map[string]string) { c.headers = h }

// Get performs a GET with a background context.
func (c *Client) Get(rawURL string, extraHeaders map[string]string) (*Response, error) {
	return c.GetContext(context.Background(), rawURL, extraHeaders)
}

// Post performs a POST with a background context.
func (c *Client) Post(rawURL, contentType, body string, extraHeaders map[string]string) (*Response, error) {
	return c.doRequest(context.Background(), "POST", rawURL, body, extraHeaders, 0, "", true)
}

// GetContext performs a GET with context support.
func (c *Client) GetContext(ctx context.Context, rawURL string, extraHeaders map[string]string, retry ...int) (*Response, error) {
	r := 0
	if len(retry) > 0 {
		r = retry[0]
	}
	return c.doRequest(ctx, "GET", rawURL, "", extraHeaders, r, "", true)
}

// GetContextWithCharset performs a GET with an explicit response charset.
func (c *Client) GetContextWithCharset(ctx context.Context, rawURL string, extraHeaders map[string]string, retry int, responseCharset string) (*Response, error) {
	return c.doRequest(ctx, "GET", rawURL, "", extraHeaders, retry, responseCharset, true)
}

// PostContext performs a POST with context support and optional retry.
func (c *Client) PostContext(ctx context.Context, rawURL, body string, extraHeaders map[string]string, retry int) (*Response, error) {
	return c.doRequest(ctx, "POST", rawURL, body, extraHeaders, retry, "", true)
}

// PostContextWithCharset performs a POST with an explicit response charset.
func (c *Client) PostContextWithCharset(ctx context.Context, rawURL, body string, extraHeaders map[string]string, retry int, responseCharset string) (*Response, error) {
	return c.doRequest(ctx, "POST", rawURL, body, extraHeaders, retry, responseCharset, true)
}

// doRequest is the internal request executor with retry support.
// GetContextNoRedirect preserves the first HTTP response for redirect-aware JS rules.
func (c *Client) GetContextNoRedirect(ctx context.Context, rawURL string, extraHeaders map[string]string) (*Response, error) {
	return c.doRequest(ctx, "GET", rawURL, "", extraHeaders, 0, "", false)
}

func (c *Client) doRequest(ctx context.Context, method, rawURL, reqBody string, extraHeaders map[string]string, retry int, responseCharset string, followRedirect bool) (*Response, error) {
	var lastErr error
	for attempt := 0; attempt <= retry; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(time.Duration(attempt) * time.Second):
				// simple backoff: 1s, 2s, 3s...
			}
		}

		var bodyReader io.Reader
		if reqBody != "" {
			bodyReader = strings.NewReader(reqBody)
		}
		req, err := http.NewRequestWithContext(ctx, method, rawURL, bodyReader)
		if err != nil {
			return nil, fmt.Errorf("fetcher: new request: %w", err)
		}

		// Default headers (browser-like for Chinese site compatibility)
		if _, ok := req.Header["User-Agent"]; !ok {
			req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
		}
		if _, ok := req.Header["Accept"]; !ok {
			req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
		}
		if _, ok := req.Header["Accept-Language"]; !ok {
			req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
		}
		if _, ok := req.Header["Cache-Control"]; !ok {
			req.Header.Set("Cache-Control", "no-cache")
		}
		if _, ok := req.Header["Connection"]; !ok {
			req.Header.Set("Connection", "keep-alive")
		}

		// Apply client-level default headers
		for k, v := range c.headers {
			if _, ok := req.Header[k]; !ok {
				req.Header.Set(k, v)
			}
		}
		// Apply per-request headers (override defaults)
		for k, v := range extraHeaders {
			req.Header.Set(k, v)
		}

		httpClient := c.httpClient
		if !followRedirect {
			clone := *c.httpClient
			clone.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
			httpClient = &clone
		}
		resp, err := httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("fetch: %w", err)
			slog.Debug("fetcher: request failed, retrying",
				"url", rawURL[:min(len(rawURL), 80)],
				"attempt", attempt, "retry", retry, "err", err)
			continue
		}

		// Read raw bytes for charset detection
		rawBody, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
		resp.Body.Close()
		if err != nil {
			lastErr = fmt.Errorf("fetcher: read body: %w", err)
			continue
		}

		// Charset detection and decoding
		body := DecodeCharset(rawBody, resp.Header.Get("Content-Type"), responseCharset)

		if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
			if attempt < retry {
				slog.Debug("fetcher: unsuccessful response, retrying",
					"url", rawURL[:min(len(rawURL), 80)],
					"status", resp.StatusCode, "attempt", attempt, "retry", retry)
				continue
			}
		}
		if attempt > 0 {
			slog.Debug("fetcher: retry succeeded",
				"url", rawURL[:min(len(rawURL), 80)],
				"attempt", attempt)
		}

		return &Response{
			StatusCode: resp.StatusCode,
			Body:       body,
			Headers:    resp.Header,
			URL:        resp.Request.URL.String(),
		}, nil
	}

	return nil, fmt.Errorf("fetcher: failed after %d attempts: %w", retry+1, lastErr)
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
		return nil
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

// decodeCharset detects and decodes non-UTF-8 character encodings (gbk, gb2312, etc.).
// Falls back to raw bytes if detection fails.
// DecodeCharset decodes a response using the source-declared charset when present;
// otherwise it follows the HTML metadata/content-type detector.
func DecodeCharset(rawBody []byte, contentType, override string) string {
	enc, name, _ := charset.DetermineEncoding(rawBody, contentType)
	if override = strings.TrimSpace(override); override != "" {
		if declared, declaredName := charset.Lookup(override); declared != nil {
			enc, name = declared, declaredName
		}
	}
	if name != "utf-8" && name != "us-ascii" {
		if decoded, _, err := transform.String(enc.NewDecoder(), string(rawBody)); err == nil {
			return decoded
		}
		slog.Debug("fetcher: charset decode failed, using raw bytes", "charset", name)
	}
	return string(rawBody)
}
