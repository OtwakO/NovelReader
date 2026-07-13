// Request transport router for normal and browser-backed source execution.
package sourceexec

import (
	"context"
	"fmt"
)

// RoutingTransport sends WebView requests to the browser transport and all others to HTTP.
type RoutingTransport struct {
	normal  Transport
	webView Transport
}

// NewRoutingTransport creates a transport policy with an optional browser path.
func NewRoutingTransport(normal, webView Transport) *RoutingTransport {
	return &RoutingTransport{normal: normal, webView: webView}
}

func (t *RoutingTransport) Do(ctx context.Context, spec RequestSpec) (Response, error) {
	if spec.WebView {
		if t.webView == nil {
			return Response{}, fmt.Errorf("sourceexec: WebView transport unavailable")
		}
		return t.webView.Do(ctx, spec)
	}
	if t.normal == nil {
		return Response{}, fmt.Errorf("sourceexec: normal transport unavailable")
	}
	return t.normal.Do(ctx, spec)
}
