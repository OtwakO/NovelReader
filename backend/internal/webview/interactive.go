package webview

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/otwako/novelreader/internal/sourceexec"
)

// StartInteractive opens a short-lived browser context hydrated from one source session.
func (c *Client) StartInteractive(ctx context.Context, rawURL, title string, viewport InteractiveViewport, session *sourceexec.SourceSession) (InteractiveFrame, error) {
	if c == nil {
		return InteractiveFrame{}, fmt.Errorf("webview: nil client")
	}
	viewport.Width = min(1440, max(390, viewport.Width))
	viewport.Height = min(1200, max(480, viewport.Height))
	viewport.DeviceScaleFactor = min(2, max(1, viewport.DeviceScaleFactor))
	request := interactiveRequest{URL: rawURL, Viewport: viewport, TimeoutMS: int(c.timeout.Milliseconds())}
	if session != nil && isNetworkBrowserURL(rawURL) {
		request.Headers = session.RequestHeaders()
		for _, cookie := range session.Cookies(rawURL) {
			request.Cookies = append(request.Cookies, toProtocolCookie(cookie, rawURL))
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
func (c *Client) CloseInteractive(ctx context.Context, sessionID, rawURL string, save bool, session *sourceexec.SourceSession) error {
	var result interactiveResult
	if err := c.interactiveWorker(ctx, http.MethodDelete, "/sessions/"+sessionID, interactiveRequest{Save: save}, &result); err != nil {
		return err
	}
	if save && session != nil && len(result.Cookies) > 0 {
		cookieURL := result.FinalURL
		if !isNetworkBrowserURL(cookieURL) {
			cookieURL = rawURL
		}
		if !isNetworkBrowserURL(cookieURL) {
			return nil
		}
		if err := session.SetCookies(cookieURL, fromProtocolCookies(result.Cookies)); err != nil {
			return fmt.Errorf("webview: sync interactive cookies: %w", err)
		}
	}
	return nil
}

func isNetworkBrowserURL(rawURL string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(rawURL)), "http://") || strings.HasPrefix(strings.ToLower(strings.TrimSpace(rawURL)), "https://")
}

func (c *Client) interactiveHTTPClient() *http.Client {
	return &http.Client{Timeout: max(c.timeout, 15*time.Second)}
}
