// Request transport router for normal and browser-backed source execution.
package sourceexec

import (
	"context"
	"errors"
	"fmt"
)

var ErrWebViewTransportUnavailable = errors.New("sourceexec: WebView transport unavailable")

// RoutingTransport realizes typed in-memory data before selecting browser or normal network execution.
type RoutingTransport struct {
	normal  Transport
	webView Transport
}

// NewRoutingTransport creates a transport policy with an optional browser path.
func NewRoutingTransport(normal, webView Transport) *RoutingTransport {
	return &RoutingTransport{normal: normal, webView: webView}
}

// CloseIdleConnections releases pools owned by ephemeral normal transports.
func (t *RoutingTransport) CloseIdleConnections() {
	if closer, ok := t.normal.(interface{ CloseIdleConnections() }); ok {
		closer.CloseIdleConnections()
	}
}

func (t *RoutingTransport) Do(ctx context.Context, spec RequestSpec) (Response, error) {
	if isTypedDataRequest(spec) {
		return (dataTransport{}).Do(ctx, spec)
	}
	if spec.WebView {
		if t.webView == nil {
			return Response{}, ErrWebViewTransportUnavailable
		}
		return t.webView.Do(ctx, spec)
	}
	if t.normal == nil {
		return Response{}, fmt.Errorf("sourceexec: normal transport unavailable")
	}
	return t.normal.Do(ctx, spec)
}
