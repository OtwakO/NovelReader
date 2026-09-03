package webview

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/otwako/novelreader/internal/sourceexec"
)

// StartInteractive opens a short-lived browser context hydrated from one source session.
func (c *Client) StartInteractive(ctx context.Context, rawURL, title string, viewport InteractiveViewport, session *sourceexec.SourceSession) (InteractiveFrame, error) {
	if c == nil {
		return InteractiveFrame{}, fmt.Errorf("webview: nil client")
	}
	viewport.Width = min(430, max(390, viewport.Width))
	viewport.Height = min(900, max(470, viewport.Height))
	viewport.DeviceScaleFactor = min(3, max(1, viewport.DeviceScaleFactor))
	request := interactiveRequest{URL: rawURL, Viewport: viewport, TimeoutMS: int(c.timeout.Milliseconds())}
	if session != nil {
		if isNetworkBrowserURL(rawURL) {
			request.Headers = session.RequestHeaders()
			for _, cookie := range session.Cookies(rawURL) {
				request.Cookies = append(request.Cookies, toProtocolCookie(cookie, rawURL))
			}
		} else {
			for _, scope := range session.CookieURLs() {
				for _, cookie := range session.Cookies(scope) {
					request.Cookies = append(request.Cookies, toProtocolCookie(cookie, scope))
				}
			}
		}
	}
	var result interactiveResult
	if err := c.interactiveWorker(ctx, http.MethodPost, "/sessions", request, &result); err != nil {
		return InteractiveFrame{}, err
	}
	return result.InteractiveFrame, nil
}

// InteractiveFrame retrieves the current browser image and page metadata.
func (c *Client) InteractiveFrame(ctx context.Context, sessionID string) (InteractiveFrame, error) {
	var result interactiveResult
	if err := c.interactiveWorker(ctx, http.MethodGet, "/sessions/"+sessionID+"/frame", nil, &result); err != nil {
		return InteractiveFrame{}, err
	}
	return result.InteractiveFrame, nil
}

// SendInteractiveInput applies one bounded user-input event and returns the resulting frame.
func (c *Client) SendInteractiveInput(ctx context.Context, sessionID string, input InteractiveInput) (InteractiveFrame, error) {
	var result interactiveResult
	if err := c.interactiveWorker(ctx, http.MethodPost, "/sessions/"+sessionID+"/input", input, &result); err != nil {
		return InteractiveFrame{}, err
	}
	return result.InteractiveFrame, nil
}

// CloseInteractive idempotently closes the worker context and optionally imports its cookies.
func (c *Client) CloseInteractive(ctx context.Context, sessionID, rawURL string, save, returnHTML bool, session *sourceexec.SourceSession) (InteractiveCloseResult, error) {
	var result interactiveResult
	if err := c.interactiveWorker(ctx, http.MethodDelete, "/sessions/"+sessionID, interactiveRequest{Save: save, ReturnHTML: returnHTML}, &result); err != nil {
		return InteractiveCloseResult{}, err
	}
	if save && session != nil && len(result.Cookies) > 0 {
		fallbackURL := result.FinalURL
		if !isNetworkBrowserURL(fallbackURL) {
			fallbackURL = rawURL
		}
		for _, cookie := range result.Cookies {
			cookieURL := protocolCookieScopeURL(cookie, fallbackURL)
			if cookieURL == "" {
				continue
			}
			if err := session.SetCookies(cookieURL, fromProtocolCookies([]protocolCookie{cookie})); err != nil {
				return InteractiveCloseResult{}, fmt.Errorf("webview: sync interactive cookies: %w", err)
			}
		}
	}
	return InteractiveCloseResult{HTML: result.HTML}, nil
}

func protocolCookieScopeURL(cookie protocolCookie, fallbackURL string) string {
	if isNetworkBrowserURL(cookie.URL) {
		return cookie.URL
	}
	domain := strings.TrimPrefix(strings.TrimSpace(cookie.Domain), ".")
	if domain == "" {
		if isNetworkBrowserURL(fallbackURL) {
			return fallbackURL
		}
		return ""
	}
	scheme := "https"
	if !cookie.Secure && isNetworkBrowserURL(fallbackURL) {
		if parsed, err := url.Parse(fallbackURL); err == nil {
			scheme = parsed.Scheme
		}
	}
	scope := (&url.URL{Scheme: scheme, Host: domain, Path: "/"}).String()
	if !isNetworkBrowserURL(scope) {
		return ""
	}
	return scope
}

func isNetworkBrowserURL(rawURL string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(rawURL)), "http://") || strings.HasPrefix(strings.ToLower(strings.TrimSpace(rawURL)), "https://")
}

func (c *Client) interactiveHTTPClient() *http.Client {
	return &http.Client{Timeout: max(c.timeout, 15*time.Second)}
}
