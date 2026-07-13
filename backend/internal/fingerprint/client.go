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
	config      Config
	session     fetcher.CookieSession
	jar         tlsclient.CookieJar
}

// New creates a fingerprint client. An empty profile selects the dependency's latest profile.
func New(config Config, fallback fetcher.HTTPClient) (*Client, error) {
	return newClient(config, fallback, nil)
}

func newClient(config Config, fallback fetcher.HTTPClient, session fetcher.CookieSession) (*Client, error) {
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
	return &Client{
		fingerprint: primary,
		noRedirect:  noRedirect,
		fallback:    fallback,
		config:      config,
		session:     session,
		jar:         jar,
	}, nil
}

// ForSource creates an isolated fingerprint jar for one source session.
func (c *Client) ForSource(session fetcher.CookieSession) fetcher.HTTPClient {
	if c == nil {
		return nil
	}
	scoped, err := newClient(c.config, c.fallback, session)
	if err != nil {
		return &failedClient{err: err}
	}
	return scoped
}

type failedClient struct{ err error }

func (c *failedClient) Get(string, map[string]string) (*fetcher.Response, error) { return nil, c.err }
func (c *failedClient) Post(string, string, string, map[string]string) (*fetcher.Response, error) {
	return nil, c.err
}
func (c *failedClient) GetContextNoRedirect(context.Context, string, map[string]string) (*fetcher.Response, error) {
	return nil, c.err
}

func (c *Client) Get(rawURL string, headers map[string]string) (*fetcher.Response, error) {
	return c.GetContext(context.Background(), rawURL, headers)
}

func (c *Client) GetContext(ctx context.Context, rawURL string, headers map[string]string, _ ...int) (*fetcher.Response, error) {
	return c.do(ctx, http.MethodGet, rawURL, "", headers, true)
}

func (c *Client) Post(rawURL, contentType, body string, headers map[string]string) (*fetcher.Response, error) {
	if headers == nil {
		headers = map[string]string{}
	}
	if _, ok := headers["Content-Type"]; !ok && contentType != "" {
		headers["Content-Type"] = contentType
	}
	return c.PostContext(context.Background(), rawURL, body, headers, 0)
}

func (c *Client) PostContext(ctx context.Context, rawURL, body string, headers map[string]string, _ int) (*fetcher.Response, error) {
	if headers == nil {
		headers = map[string]string{}
	}
	if _, ok := headers["Content-Type"]; !ok {
		headers["Content-Type"] = "application/x-www-form-urlencoded"
	}
	return c.do(ctx, http.MethodPost, rawURL, body, headers, true)
}

func (c *Client) GetContextNoRedirect(ctx context.Context, rawURL string, headers map[string]string) (*fetcher.Response, error) {
	return c.do(ctx, http.MethodGet, rawURL, "", headers, false)
}

func (c *Client) do(ctx context.Context, method, rawURL, body string, headers map[string]string, followRedirect bool) (*fetcher.Response, error) {
	return c.doWithCharset(ctx, method, rawURL, body, headers, followRedirect, "")
}

func (c *Client) doWithCharset(ctx context.Context, method, rawURL, body string, headers map[string]string, followRedirect bool, responseCharset string) (*fetcher.Response, error) {
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
	if c.session != nil && req.Header.Get("Cookie") == "" {
		if cookie := c.session.CookieHeader(rawURL); cookie != "" {
			req.Header.Set("Cookie", cookie)
		}
	}
	resp, err := client.Do(req)
	if err != nil {
		return c.fallbackRequest(ctx, method, rawURL, body, headers, followRedirect, err)
	}
	result, err := responseWithCharset(resp, responseCharset)
	if err != nil {
		return c.fallbackRequest(ctx, method, rawURL, body, headers, followRedirect, err)
	}
	c.syncSession(result)
	if shouldFallback(result.StatusCode) && c.fallback != nil {
		return c.fallbackRequest(ctx, method, rawURL, body, headers, followRedirect, fmt.Errorf("fingerprint status %d", result.StatusCode))
	}
	return result, nil
}

func (c *Client) fallbackRequest(ctx context.Context, method, rawURL, body string, headers map[string]string, followRedirect bool, cause error) (*fetcher.Response, error) {
	if c.fallback == nil {
		return nil, cause
	}
	if method == http.MethodGet && !followRedirect {
		return c.fallback.GetContextNoRedirect(ctx, rawURL, headers)
	}
	if method == http.MethodGet {
		if client, ok := c.fallback.(fetcher.ContextHTTPClient); ok {
			return client.GetContext(ctx, rawURL, headers)
		}
		return c.fallback.Get(rawURL, headers)
	}
	contentType := "application/x-www-form-urlencoded"
	for key, value := range headers {
		if strings.EqualFold(key, "Content-Type") && value != "" {
			contentType = value
			break
		}
	}
	if client, ok := c.fallback.(fetcher.ContextHTTPClient); ok {
		return client.PostContext(ctx, rawURL, body, headers, 0)
	}
	return c.fallback.Post(rawURL, contentType, body, headers)
}

func response(resp *fhttp.Response) (*fetcher.Response, error) {
	return responseWithCharset(resp, "")
}

func responseWithCharset(resp *fhttp.Response, responseCharset string) (*fetcher.Response, error) {
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		return nil, err
	}
	headers := make(http.Header, len(resp.Header))
	for key, values := range resp.Header {
		headers[key] = append([]string(nil), values...)
	}
	finalURL := ""
	if resp.Request != nil && resp.Request.URL != nil {
		finalURL = resp.Request.URL.String()
	}
	return &fetcher.Response{
		StatusCode: resp.StatusCode,
		Body:       fetcher.DecodeCharset(body, headers.Get("Content-Type"), responseCharset),
		Headers:    headers,
		URL:        finalURL,
	}, nil
}

func (c *Client) syncSession(response *fetcher.Response) {
	if c.session == nil || c.jar == nil || response == nil {
		return
	}
	rawURL := response.URL
	if rawURL == "" {
		return
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return
	}
	if cookies := c.jar.Cookies(parsed); len(cookies) > 0 {
		converted := make([]*http.Cookie, 0, len(cookies))
		for _, cookie := range cookies {
			converted = append(converted, &http.Cookie{
				Name: cookie.Name, Value: cookie.Value, Path: cookie.Path, Domain: cookie.Domain,
				Expires: cookie.Expires, RawExpires: cookie.RawExpires, MaxAge: cookie.MaxAge,
				Secure: cookie.Secure, HttpOnly: cookie.HttpOnly, SameSite: http.SameSite(cookie.SameSite),
			})
		}
		if err := c.session.SetCookies(rawURL, converted); err != nil {
			return
		}
	}
}

func makeHeaders(values map[string]string) fhttp.Header {
	headers := make(fhttp.Header, len(values)+1)
	hasUserAgent := false
	for key, value := range values {
		if strings.EqualFold(key, "User-Agent") {
			hasUserAgent = true
		}
		headers[key] = []string{value}
	}
	if !hasUserAgent {
		headers["User-Agent"] = []string{"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"}
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
