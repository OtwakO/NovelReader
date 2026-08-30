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

func TestExecutorAppliesBodyJSAfterTransport(t *testing.T) {
	transport := &captureTransport{}
	executor := NewExecutor(analyzer.NewJSVM(), transport)

	response, err := executor.Execute(context.Background(), `https://example.test/content,{"bodyJs":"result.toUpperCase()"}`, "", 1, "https://example.test/")
	if err != nil {
		t.Fatal(err)
	}
	if response.Body != "FIXTURE" {
		t.Fatalf("bodyJs body=%q", response.Body)
	}
}

func TestExecutorRunsAggregateStyleTypedDataRequest(t *testing.T) {
	executor := NewExecutor(analyzer.NewJSVM(), NewRoutingTransport(nil, nil))
	template := `@js:"data:;base64,"+java.base64Encode(JSON.stringify({key:key,page:page}))+","+JSON.stringify({type:"aggregate-search",bodyJs:"java.hexDecodeToString(result)"})`

	response, err := executor.Execute(t.Context(), template, "剑来", 2, "https://example.test/")
	if err != nil {
		t.Fatal(err)
	}
	if response.Body != `{"key":"剑来","page":2}` || response.Transport != "data" {
		t.Fatalf("unexpected aggregate response: %+v", response)
	}
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
