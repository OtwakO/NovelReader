// Unified Legado URL expansion and transport execution boundary.
package sourceexec

import (
	"context"
	"fmt"

	"github.com/otwako/novelreader/internal/analyzer"
)

// Executor expands source URL rules and delegates the resulting request to a transport.
type Executor struct {
	jsVM        *analyzer.JSVM
	transport   Transport
	sourceState analyzer.SourceState
}

// NewExecutor creates an executor with explicit JavaScript and transport dependencies.
func NewExecutor(jsVM *analyzer.JSVM, transport Transport) *Executor {
	return &Executor{jsVM: jsVM, transport: transport}
}

// NewExecutorWithSession binds one source session to URL and JS evaluation.
func NewExecutorWithSession(jsVM *analyzer.JSVM, transport Transport, session *SourceSession) *Executor {
	return &Executor{jsVM: jsVM, transport: transport, sourceState: session}
}

// Build expands a Legado URL template into a transport-neutral request.
func (e *Executor) Build(template, key string, page int, baseURL string) (RequestSpec, error) {
	return e.BuildContext(context.Background(), template, key, page, baseURL)
}

// BuildContext expands a URL while preserving the caller's cancellation context.
func (e *Executor) BuildContext(ctx context.Context, template, key string, page int, baseURL string) (RequestSpec, error) {
	if e == nil {
		return RequestSpec{}, fmt.Errorf("sourceexec: nil executor")
	}
	meta, err := analyzer.BuildURLWithContext(ctx, template, key, page, baseURL, e.jsVM, e.sourceState)
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
	spec, err := e.BuildContext(ctx, template, key, page, baseURL)
	if err != nil {
		return Response{}, err
	}
	return e.transport.Do(ctx, spec)
}
