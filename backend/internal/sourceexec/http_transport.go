// HTTP transport adapter for the shared booksource request contract.
package sourceexec

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/otwako/novelreader/internal/fetcher"
)

// HTTPTransport executes RequestSpec values through the existing HTTP client.
type HTTPTransport struct {
	client *fetcher.Client
}

// NewHTTPTransport wraps a configured fetcher client.
func NewHTTPTransport(client *fetcher.Client) *HTTPTransport {
	return &HTTPTransport{client: client}
}

// Do executes GET and POST requests and retains bodies for all HTTP statuses.
func (t *HTTPTransport) Do(ctx context.Context, spec RequestSpec) (Response, error) {
	if t == nil || t.client == nil {
		return Response{}, fmt.Errorf("sourceexec: HTTP transport has no client")
	}

	method := strings.ToUpper(strings.TrimSpace(spec.Method))
	if method == "" {
		method = http.MethodGet
	}

	headers := cloneHeaders(spec.Headers)
	if spec.Origin != "" {
		if _, exists := headers["Origin"]; !exists {
			headers["Origin"] = spec.Origin
		}
	}

	var (
		resp *fetcher.Response
		err  error
	)
	switch method {
	case http.MethodGet:
		resp, err = t.client.GetContext(ctx, spec.URL, headers, spec.Retry)
	case http.MethodPost:
		if _, exists := headers["Content-Type"]; !exists {
			headers["Content-Type"] = "application/x-www-form-urlencoded"
		}
		resp, err = t.client.PostContext(ctx, spec.URL, spec.Body, headers, spec.Retry)
	default:
		return Response{}, fmt.Errorf("sourceexec: unsupported HTTP method %q", method)
	}
	if err != nil {
		return Response{}, err
	}

	return Response{
		StatusCode: resp.StatusCode,
		Headers:    cloneResponseHeaders(resp.Headers),
		Body:       resp.Body,
		FinalURL:   resp.URL,
		Transport:  "http",
	}, nil
}

func cloneResponseHeaders(headers http.Header) map[string][]string {
	copy := make(map[string][]string, len(headers))
	for key, values := range headers {
		copy[key] = append([]string(nil), values...)
	}
	return copy
}
