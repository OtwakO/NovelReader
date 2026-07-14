// Source-execution adapter that keeps fingerprinting outside book workflows.
package fingerprint

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/otwako/novelreader/internal/fetcher"
	"github.com/otwako/novelreader/internal/sourceexec"
)

// Transport executes regular source requests with fingerprint-first failover.
type Transport struct {
	client   *Client
	fallback sourceexec.Transport
	session  *sourceexec.SourceSession
}

// NewTransport creates a fingerprint transport with an injected normal transport fallback.
func NewTransport(config Config, fallback sourceexec.Transport, session *sourceexec.SourceSession) (*Transport, error) {
	var cookieSession fetcher.CookieSession
	if session != nil {
		cookieSession = session
	}
	client, err := newClient(config, nil, cookieSession)
	if err != nil {
		return nil, err
	}
	return &Transport{client: client, fallback: fallback, session: session}, nil
}

// CloseIdleConnections releases this per-source fingerprint pool.
func (t *Transport) CloseIdleConnections() {
	if t != nil && t.client != nil {
		t.client.CloseIdleConnections()
	}
}

// Do implements sourceexec.Transport without exposing tls-client to sourceexec or book.
func (t *Transport) Do(ctx context.Context, spec sourceexec.RequestSpec) (sourceexec.Response, error) {
	if t == nil || t.client == nil {
		return sourceexec.Response{}, fmt.Errorf("fingerprint: transport has no client")
	}
	if strings.TrimSpace(spec.DNSIP) != "" {
		if t.fallback == nil {
			return sourceexec.Response{}, fmt.Errorf("fingerprint: dnsIp requires normal HTTP fallback")
		}
		return t.fallback.Do(ctx, spec)
	}
	method := strings.ToUpper(strings.TrimSpace(spec.Method))
	if method == "" {
		method = http.MethodGet
	}
	headers := cloneHeaders(spec.Headers)
	hasSessionCookie := false
	if t.session != nil && headers["Cookie"] == "" && headers["cookie"] == "" {
		if cookie := t.session.CookieHeader(spec.URL); cookie != "" {
			headers["Cookie"] = cookie
			hasSessionCookie = true
		}
	}
	slog.Debug("fingerprint: source request", "method", method, "url", spec.URL, "sessionCookie", hasSessionCookie)
	if spec.Origin != "" && headers["Origin"] == "" && headers["origin"] == "" {
		headers["Origin"] = spec.Origin
	}
	preparedBody, contentType := sourceexec.PreparePOST(spec.Body, spec.Charset, headers)
	if method == http.MethodPost && !hasHeader(headers, "Content-Type") {
		headers["Content-Type"] = contentType
	}

	var body string
	var err error
	switch method {
	case http.MethodGet, http.MethodHead:
		var response *fetcher.Response
		response, err = t.client.doWithCharset(ctx, method, spec.URL, "", headers, true, spec.Charset)
		slog.Debug("fingerprint: primary response", "url", spec.URL, "status", responseStatus(response), "err", err)
		body = responseBody(response)
		if err == nil {
			return t.finish(ctx, spec, response)
		}
	case http.MethodPost:
		var response *fetcher.Response
		response, err = t.client.doWithCharset(ctx, method, spec.URL, preparedBody, headers, true, spec.Charset)
		slog.Debug("fingerprint: primary response", "url", spec.URL, "status", responseStatus(response), "err", err)
		body = responseBody(response)
		if err == nil {
			return t.finish(ctx, spec, response)
		}
	default:
		return sourceexec.Response{}, fmt.Errorf("fingerprint: unsupported HTTP method %q", method)
	}
	_ = body
	if t.fallback == nil {
		return sourceexec.Response{}, err
	}
	fallbackResponse, fallbackErr := t.fallback.Do(ctx, spec)
	slog.Debug("fingerprint: fallback response", "url", spec.URL, "status", fallbackResponse.StatusCode, "err", fallbackErr)
	return fallbackResponse, fallbackErr
}

func (t *Transport) finish(ctx context.Context, spec sourceexec.RequestSpec, response *fetcher.Response) (sourceexec.Response, error) {
	if response == nil {
		return sourceexec.Response{}, fmt.Errorf("fingerprint: empty response")
	}
	if t.fallback != nil && (shouldFallback(response.StatusCode) || (spec.Retry > 0 && (response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices))) {
		return t.fallback.Do(ctx, spec)
	}
	if t.session != nil {
		stdResponse := &http.Response{Header: response.Headers}
		if err := t.session.SetCookies(response.URL, stdResponse.Cookies()); err != nil {
			return sourceexec.Response{}, fmt.Errorf("fingerprint: sync session cookies: %w", err)
		}
	}
	return sourceexec.Response{
		StatusCode:    response.StatusCode,
		Headers:       cloneResponseHeaders(response.Headers),
		Body:          response.Body,
		FinalURL:      response.URL,
		Transport:     "fingerprint",
		RedirectChain: append([]string(nil), response.RedirectChain...),
	}, nil
}

func responseStatus(response *fetcher.Response) int {
	if response == nil {
		return 0
	}
	return response.StatusCode
}

func responseBody(response *fetcher.Response) string {
	if response == nil {
		return ""
	}
	return response.Body
}

func cloneHeaders(headers map[string]string) map[string]string {
	copy := make(map[string]string, len(headers))
	for key, value := range headers {
		copy[key] = value
	}
	return copy
}

func cloneResponseHeaders(headers http.Header) map[string][]string {
	copy := make(map[string][]string, len(headers))
	for key, values := range headers {
		copy[key] = append([]string(nil), values...)
	}
	return copy
}
