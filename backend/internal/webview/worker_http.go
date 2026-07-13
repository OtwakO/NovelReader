// Worker HTTP exchange with bounded backpressure retries.
package webview

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const workerBusyRetries = 3

func (c *Client) executeWorker(ctx context.Context, payload protocolRequest) (protocolResponse, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return protocolResponse{}, fmt.Errorf("webview: encode request: %w", err)
	}
	for attempt := 0; ; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint+"/execute", strings.NewReader(string(body)))
		if err != nil {
			return protocolResponse{}, fmt.Errorf("webview: create worker request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := c.httpClient.Do(req)
		if err != nil {
			return protocolResponse{}, fmt.Errorf("webview: worker request: %w", err)
		}
		responseBody, readErr := io.ReadAll(io.LimitReader(resp.Body, c.maxBodyBytes+1))
		resp.Body.Close()
		if readErr != nil {
			return protocolResponse{}, fmt.Errorf("webview: read worker response: %w", readErr)
		}
		if int64(len(responseBody)) > c.maxBodyBytes {
			return protocolResponse{}, fmt.Errorf("webview: worker response exceeds %d bytes", c.maxBodyBytes)
		}
		var result protocolResponse
		if err := json.Unmarshal(responseBody, &result); err != nil {
			return protocolResponse{}, fmt.Errorf("webview: decode worker response: %w", err)
		}
		if resp.StatusCode == http.StatusServiceUnavailable && result.Error == "browser worker is busy" && attempt < workerBusyRetries {
			if err := waitForWorkerCapacity(ctx, attempt); err != nil {
				return protocolResponse{}, err
			}
			continue
		}
		if result.Error != "" {
			return protocolResponse{}, fmt.Errorf("webview: %s", result.Error)
		}
		if result.Version != protocolVersion {
			return protocolResponse{}, fmt.Errorf("webview: unsupported worker protocol %d", result.Version)
		}
		if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
			return protocolResponse{}, fmt.Errorf("webview: worker status %d", resp.StatusCode)
		}
		return result, nil
	}
}

func waitForWorkerCapacity(ctx context.Context, attempt int) error {
	delay := time.Duration(100*(1<<attempt)) * time.Millisecond
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return fmt.Errorf("webview: waiting for worker capacity: %w", ctx.Err())
	case <-timer.C:
		return nil
	}
}
