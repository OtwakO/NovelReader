// Optional browser-fingerprint HTTP client used by JavaScript source helpers.
package fingerprint

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	fhttp "github.com/bogdanfinn/fhttp"
	tlsclient "github.com/bogdanfinn/tls-client"
	"github.com/bogdanfinn/tls-client/profiles"
	"github.com/otwako/novelreader/internal/fetcher"
)

// Config controls the optional fingerprint client without exposing tls-client types.
type Config struct {
	Timeout            time.Duration
	Profile            string
	InsecureSkipVerify bool
}

// Client tries a browser profile first and delegates to normal HTTP on rejection/errors.
type Client struct {
	fingerprint tlsclient.HttpClient
	noRedirect  tlsclient.HttpClient
	fallback    fetcher.HTTPClient
}

// New creates a fingerprint client. An empty profile selects the dependency's latest profile.
func New(config Config, fallback fetcher.HTTPClient) (*Client, error) {
	if config.Timeout <= 0 {
		config.Timeout = 15 * time.Second
	}
	profile, ok := profiles.MappedTLSClients[strings.ToLower(strings.TrimSpace(config.Profile))]
	if !ok {
		profile = profiles.DefaultClientProfile
	}
	jar := tlsclient.NewCookieJar()
	options := []tlsclient.HttpClientOption{
		tlsclient.WithTimeoutSeconds(int(config.Timeout / time.Second)),
		tlsclient.WithClientProfile(profile),
		tlsclient.WithCookieJar(jar),
	}
	if config.InsecureSkipVerify {
		options = append(options, tlsclient.WithInsecureSkipVerify())
	}
	primary, err := tlsclient.NewHttpClient(tlsclient.NewNoopLogger(), options...)
	if err != nil {
		return nil, fmt.Errorf("fingerprint: create client: %w", err)
	}
	noRedirectOptions := append([]tlsclient.HttpClientOption{}, options...)
	noRedirectOptions = append(noRedirectOptions, tlsclient.WithNotFollowRedirects())
	noRedirect, err := tlsclient.NewHttpClient(tlsclient.NewNoopLogger(), noRedirectOptions...)
	if err != nil {
		return nil, fmt.Errorf("fingerprint: create redirect client: %w", err)
	}
	return &Client{fingerprint: primary, noRedirect: noRedirect, fallback: fallback}, nil
}

func (c *Client) Get(rawURL string, headers map[string]string) (*fetcher.Response, error) {
	return c.do(context.Background(), http.MethodGet, rawURL, "", headers, true)
}

func (c *Client) Post(rawURL, contentType, body string, headers map[string]string) (*fetcher.Response, error) {
	if headers == nil {
		headers = map[string]string{}
	}
	if _, ok := headers["Content-Type"]; !ok && contentType != "" {
		headers["Content-Type"] = contentType
	}
	return c.do(context.Background(), http.MethodPost, rawURL, body, headers, true)
}

func (c *Client) GetContextNoRedirect(ctx context.Context, rawURL string, headers map[string]string) (*fetcher.Response, error) {
	return c.do(ctx, http.MethodGet, rawURL, "", headers, false)
}

func (c *Client) do(ctx context.Context, method, rawURL, body string, headers map[string]string, followRedirect bool) (*fetcher.Response, error) {
	client := c.fingerprint
	if !followRedirect {
		client = c.noRedirect
	}
	reqBody := io.Reader(nil)
	if body != "" {
		reqBody = bytes.NewBufferString(body)
	}
	normalizedURL := normalizeURL(rawURL)
	req, err := fhttp.NewRequestWithContext(ctx, method, normalizedURL, reqBody)
	if err != nil {
		return c.fallbackRequest(ctx, method, rawURL, body, headers, followRedirect, err)
	}
	req.Header = makeHeaders(headers)
	resp, err := client.Do(req)
	if err != nil {
		return c.fallbackRequest(ctx, method, rawURL, body, headers, followRedirect, err)
	}
	result, err := response(resp)
	if err != nil {
		return c.fallbackRequest(ctx, method, rawURL, body, headers, followRedirect, err)
	}
	if shouldFallback(result.StatusCode) && c.fallback != nil {
		return c.fallbackRequest(ctx, method, rawURL, body, headers, followRedirect, fmt.Errorf("fingerprint status %d", result.StatusCode))
	}
	return result, nil
}

func (c *Client) fallbackRequest(ctx context.Context, method, rawURL, body string, headers map[string]string, noRedirect bool, cause error) (*fetcher.Response, error) {
	if c.fallback == nil {
		return nil, cause
	}
	if method == http.MethodGet && noRedirect {
		return c.fallback.GetContextNoRedirect(ctx, rawURL, headers)
	}
	if method == http.MethodGet {
		return c.fallback.Get(rawURL, headers)
	}
	return c.fallback.Post(rawURL, "application/x-www-form-urlencoded", body, headers)
}

func response(resp *fhttp.Response) (*fetcher.Response, error) {
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	headers := make(http.Header, len(resp.Header))
	for key, values := range resp.Header {
		headers[key] = append([]string(nil), values...)
	}
	return &fetcher.Response{
		StatusCode: resp.StatusCode,
		Body:       fetcher.DecodeCharset(body, headers.Get("Content-Type"), ""),
		Headers:    headers,
		URL:        resp.Request.URL.String(),
	}, nil
}

func makeHeaders(values map[string]string) fhttp.Header {
	headers := make(fhttp.Header, len(values)+1)
	for key, value := range values {
		headers[key] = []string{value}
	}
	return headers
}

func shouldFallback(status int) bool {
	return status == http.StatusBadRequest || status == http.StatusForbidden || status == http.StatusTooManyRequests || status == http.StatusServiceUnavailable
}

func normalizeURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	if parsed.RawQuery != "" {
		if query, queryErr := url.ParseQuery(parsed.RawQuery); queryErr == nil {
			parsed.RawQuery = query.Encode()
		}
	}
	return parsed.String()
}
