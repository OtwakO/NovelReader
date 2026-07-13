// Regression coverage for browser transport routing at the book workflow boundary.
package book

import (
	"context"
	"testing"

	"github.com/otwako/novelreader/internal/analyzer"
	"github.com/otwako/novelreader/internal/sourceexec"
)

type workflowWebViewTransport struct{}

func (workflowWebViewTransport) Do(context.Context, sourceexec.RequestSpec) (sourceexec.Response, error) {
	return sourceexec.Response{Transport: "webview", Body: "<html>browser</html>"}, nil
}

func TestSearcherRoutesWebViewRequestsToConfiguredTransport(t *testing.T) {
	searcher := NewSearcher(nil, analyzer.NewJSVM(), nil, nil, nil)
	searcher.SetWebViewTransportFactory(func(*sourceexec.SourceSession) sourceexec.Transport {
		return workflowWebViewTransport{}
	})
	transport := searcher.newTransport(nil, sourceexec.NewSourceSession())
	response, err := transport.Do(context.Background(), sourceexec.RequestSpec{WebView: true})
	if err != nil {
		t.Fatal(err)
	}
	if response.Transport != "webview" {
		t.Fatalf("transport=%q", response.Transport)
	}
}
