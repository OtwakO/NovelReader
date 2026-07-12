// Conformance tests for unified URL expansion and transport execution.
package sourceexec

import (
	"context"
	"testing"

	"github.com/otwako/novelreader/internal/analyzer"
)

type captureTransport struct {
	spec RequestSpec
}

func (t *captureTransport) Do(_ context.Context, spec RequestSpec) (Response, error) {
	t.spec = spec
	return Response{StatusCode: 200, Body: "fixture", FinalURL: spec.URL, Transport: "fixture"}, nil
}

func TestExecutorBuildsAndExecutesOneLegadoRequest(t *testing.T) {
	transport := &captureTransport{}
	executor := NewExecutor(analyzer.NewJSVM(), transport)

	response, err := executor.Execute(context.Background(), `https://example.test/search,{"method":"POST","body":"q={{key}}","headers":{"X-Rule":"yes"}}`, "凡人修仙传", 1, "https://example.test/")
	if err != nil {
		t.Fatal(err)
	}
	if response.Body != "fixture" || response.StatusCode != 200 {
		t.Fatalf("unexpected response: %+v", response)
	}
	if transport.spec.Method != "POST" || transport.spec.Body != "q=凡人修仙传" {
		t.Fatalf("template was not expanded: %+v", transport.spec)
	}
	if transport.spec.Headers["X-Rule"] != "yes" {
		t.Fatalf("headers were not forwarded: %+v", transport.spec.Headers)
	}
}
