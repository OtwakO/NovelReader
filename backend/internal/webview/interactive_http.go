package webview

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

func (c *Client) interactiveWorker(ctx context.Context, method, path string, payload any, result *interactiveResult) error {
	body := ""
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("webview: encode interactive request: %w", err)
		}
		body = string(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.endpoint+path, strings.NewReader(body))
	if err != nil {
		return fmt.Errorf("webview: create interactive request: %w", err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.interactiveHTTPClient().Do(req)
	if err != nil {
		return fmt.Errorf("webview: interactive request: %w", err)
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, c.maxBodyBytes+1))
	if err != nil {
		return fmt.Errorf("webview: read interactive response: %w", err)
	}
	if int64(len(responseBody)) > c.maxBodyBytes {
		return fmt.Errorf("webview: interactive response exceeds %d bytes", c.maxBodyBytes)
	}
	if err := json.Unmarshal(responseBody, result); err != nil {
		return fmt.Errorf("webview: decode interactive response: %w", err)
	}
	if result.Error != "" {
		return fmt.Errorf("webview: %s", result.Error)
	}
	if result.Version != protocolVersion {
		return fmt.Errorf("webview: unsupported worker protocol %d", result.Version)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("webview: interactive worker status %d", resp.StatusCode)
	}
	return nil
}
