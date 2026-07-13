// Regression coverage for normal/browser transport routing.
package sourceexec

import (
	"context"
	"testing"
)

type namedTransport string

func (t namedTransport) Do(context.Context, RequestSpec) (Response, error) {
	return Response{Transport: string(t)}, nil
}

func TestRoutingTransportSelectsWebViewOnlyWhenRequested(t *testing.T) {
	router := NewRoutingTransport(namedTransport("http"), namedTransport("webview"))
	for _, test := range []struct {
		name    string
		webView bool
		want    string
	}{
		{name: "normal", want: "http"},
		{name: "browser", webView: true, want: "webview"},
	} {
		t.Run(test.name, func(t *testing.T) {
			response, err := router.Do(context.Background(), RequestSpec{WebView: test.webView})
			if err != nil {
				t.Fatal(err)
			}
			if response.Transport != test.want {
				t.Fatalf("transport=%q, want %q", response.Transport, test.want)
			}
		})
	}
}

func TestRoutingTransportReportsMissingWebViewCapability(t *testing.T) {
	_, err := NewRoutingTransport(namedTransport("http"), nil).Do(context.Background(), RequestSpec{WebView: true})
	if err == nil {
		t.Fatal("expected missing WebView capability error")
	}
}
