// Unified Legado URL expansion and transport execution boundary.
package sourceexec

import (
	"context"
	"fmt"

	"github.com/otwako/novelreader/internal/analyzer"
)

// Executor expands source URL rules and delegates the resulting request to a transport.
type Executor struct {
	jsVM      *analyzer.JSVM
	transport Transport
}

// NewExecutor creates an executor with explicit JavaScript and transport dependencies.
func NewExecutor(jsVM *analyzer.JSVM, transport Transport) *Executor {
	return &Executor{jsVM: jsVM, transport: transport}
}

// Build expands a Legado URL template into a transport-neutral request.
func (e *Executor) Build(template, key string, page int, baseURL string) (RequestSpec, error) {
	if e == nil {
		return RequestSpec{}, fmt.Errorf("sourceexec: nil executor")
	}
	meta, err := analyzer.BuildURL(template, key, page, baseURL, e.jsVM)
	if err != nil {
		return RequestSpec{}, err
	}
	return RequestSpecFromURLMeta(meta), nil
}

// Execute expands and sends one source request.
func (e *Executor) Execute(ctx context.Context, template, key string, page int, baseURL string) (Response, error) {
	if e == nil || e.transport == nil {
		return Response{}, fmt.Errorf("sourceexec: no transport configured")
	}
	spec, err := e.Build(template, key, page, baseURL)
	if err != nil {
		return Response{}, err
	}
	return e.transport.Do(ctx, spec)
}
