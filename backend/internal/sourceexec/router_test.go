// Regression coverage for normal/browser transport routing.
package sourceexec

import (
	"context"
	"encoding/base64"
	"strings"
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

func TestRoutingTransportExecutesTypedDataWithoutNetwork(t *testing.T) {
	payload := []byte(`{"key":"剑来","page":1}`)
	spec := RequestSpec{
		URL:     "data:;base64," + base64.StdEncoding.EncodeToString(payload),
		Type:    "aggregate-search",
		WebView: true,
	}

	response, err := NewRoutingTransport(nil, nil).Do(t.Context(), spec)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != 200 || response.Body != "7b226b6579223a22e58991e69da5222c2270616765223a317d" {
		t.Fatalf("unexpected typed data response: %+v", response)
	}
	if response.FinalURL != spec.URL || response.Transport != "data" {
		t.Fatalf("typed data metadata lost: %+v", response)
	}
}

func TestRoutingTransportRejectsMalformedAndOversizedTypedData(t *testing.T) {
	router := NewRoutingTransport(namedTransport("http"), nil)
	for _, test := range []struct {
		name string
		url  string
	}{
		{name: "not base64", url: "data:,plain"},
		{name: "invalid base64", url: "data:;base64,%%%"},
		{name: "oversized", url: "data:;base64," + base64.StdEncoding.EncodeToString([]byte(strings.Repeat("x", maxDataPayloadBytes+1)))},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := router.Do(t.Context(), RequestSpec{URL: test.url, Type: "typed"}); err == nil {
				t.Fatal("expected typed data error")
			}
		})
	}
}

func TestRoutingTransportLeavesUntypedDataOnNormalPath(t *testing.T) {
	response, err := NewRoutingTransport(namedTransport("http"), nil).Do(t.Context(), RequestSpec{URL: "data:;base64,WA=="})
	if err != nil {
		t.Fatal(err)
	}
	if response.Transport != "http" {
		t.Fatalf("transport=%q, want normal path", response.Transport)
	}
}
