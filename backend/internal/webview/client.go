// Go client for the optional headless Patchright WebView worker.
package webview

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/otwako/novelreader/internal/sourceexec"
)

const protocolVersion = 1

// Config controls the Patchright worker endpoint and response limit.
type Config struct {
	Endpoint     string
	Timeout      time.Duration
	MaxBodyBytes int64
}

// Client is safe to share; session state is kept by returned scoped transports.
type Client struct {
	endpoint     string
	httpClient   *http.Client
	timeout      time.Duration
	maxBodyBytes int64
}

// NewClient validates and creates a client for one Patchright worker.
func NewClient(config Config) (*Client, error) {
	endpoint := strings.TrimRight(strings.TrimSpace(config.Endpoint), "/")
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Scheme != "http" && parsed.Scheme != "https" || parsed.Host == "" {
		return nil, fmt.Errorf("webview: invalid worker endpoint %q", config.Endpoint)
	}
	if config.Timeout <= 0 {
		config.Timeout = 45 * time.Second
	}
	if config.MaxBodyBytes <= 0 {
		config.MaxBodyBytes = 10 * 1024 * 1024
	}
	return &Client{
		endpoint:     endpoint,
		httpClient:   &http.Client{Timeout: config.Timeout},
		timeout:      config.Timeout,
		maxBodyBytes: config.MaxBodyBytes,
	}, nil
}

// ForSession binds browser cookies and source headers to one source flow.
func (c *Client) ForSession(session *sourceexec.SourceSession) sourceexec.Transport {
	return &sessionTransport{client: c, session: session}
}

// Do executes a request without session cookie synchronization.
func (c *Client) Do(ctx context.Context, spec sourceexec.RequestSpec) (sourceexec.Response, error) {
	return c.do(ctx, spec, nil)
}

type sessionTransport struct {
	client  *Client
	session *sourceexec.SourceSession
}

func (t *sessionTransport) Do(ctx context.Context, spec sourceexec.RequestSpec) (sourceexec.Response, error) {
	if t == nil || t.client == nil {
		return sourceexec.Response{}, fmt.Errorf("webview: nil transport")
	}
	return t.client.do(ctx, spec, t.session)
}

func (c *Client) do(ctx context.Context, spec sourceexec.RequestSpec, session *sourceexec.SourceSession) (sourceexec.Response, error) {
	if c == nil {
		return sourceexec.Response{}, fmt.Errorf("webview: nil client")
	}
	if strings.TrimSpace(spec.DNSIP) != "" {
		return sourceexec.Response{}, fmt.Errorf("webview: dnsIp is unsupported by browser transport")
	}
	headers := sourceexec.MergeHeaders(nil, spec.Headers)
	cookies := []protocolCookie(nil)
	if session != nil {
		headers = sourceexec.MergeHeaders(session.RequestHeaders(), headers)
		if !hasHeader(headers, "Cookie") {
			if cookie := session.CookieHeader(spec.URL); cookie != "" {
				headers["Cookie"] = cookie
			}
		}
		for _, cookie := range session.Cookies(spec.URL) {
			cookies = append(cookies, toProtocolCookie(cookie, spec.URL))
		}
	}
	method := strings.ToUpper(strings.TrimSpace(spec.Method))
	if method == "" {
		method = http.MethodGet
	}
	payload := protocolRequest{
		Version: protocolVersion, URL: spec.URL, Method: method, Body: spec.Body,
		Headers: headers, Cookies: cookies, WebJS: spec.WebJS,
		DelayMS: spec.WebViewDelay, TimeoutMS: int(c.timeout.Milliseconds()),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return sourceexec.Response{}, fmt.Errorf("webview: encode request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint+"/execute", strings.NewReader(string(body)))
	if err != nil {
		return sourceexec.Response{}, fmt.Errorf("webview: create worker request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return sourceexec.Response{}, fmt.Errorf("webview: worker request: %w", err)
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, c.maxBodyBytes+1))
	if err != nil {
		return sourceexec.Response{}, fmt.Errorf("webview: read worker response: %w", err)
	}
	if int64(len(responseBody)) > c.maxBodyBytes {
		return sourceexec.Response{}, fmt.Errorf("webview: worker response exceeds %d bytes", c.maxBodyBytes)
	}
	var result protocolResponse
	if err := json.Unmarshal(responseBody, &result); err != nil {
		return sourceexec.Response{}, fmt.Errorf("webview: decode worker response: %w", err)
	}
	if result.Error != "" {
		return sourceexec.Response{}, fmt.Errorf("webview: %s", result.Error)
	}
	if result.Version != protocolVersion {
		return sourceexec.Response{}, fmt.Errorf("webview: unsupported worker protocol %d", result.Version)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return sourceexec.Response{}, fmt.Errorf("webview: worker status %d", resp.StatusCode)
	}
	finalURL := result.FinalURL
	if finalURL == "" {
		finalURL = spec.URL
	}
	if session != nil {
		if err := session.SetCookies(finalURL, fromProtocolCookies(result.Cookies)); err != nil {
			return sourceexec.Response{}, fmt.Errorf("webview: sync cookies: %w", err)
		}
		session.SetLastURL(finalURL)
	}
	return sourceexec.Response{
		StatusCode: result.StatusCode, Headers: result.Headers, Body: result.Body,
		FinalURL: finalURL, Transport: "webview", RedirectChain: result.RedirectChain,
	}, nil
}

func hasHeader(headers map[string]string, name string) bool {
	for key := range headers {
		if strings.EqualFold(key, name) {
			return true
		}
	}
	return false
}
