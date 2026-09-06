package analyzer

import (
	"context"
	"errors"
)

// SetBrowserUserAgentProvider supplies browser-owned identity without coupling
// rule evaluation to the worker protocol. Reader forks share this process service.
func (vm *JSVM) SetBrowserUserAgentProvider(provider func(context.Context) (string, error)) {
	vm.executor.mu.Lock()
	vm.executor.browserUserAgent = provider
	vm.executor.mu.Unlock()
}

func (vm *JSVM) getBrowserUserAgent(ctx context.Context) (string, error) {
	vm.executor.mu.Lock()
	provider := vm.executor.browserUserAgent
	vm.executor.mu.Unlock()
	if provider == nil {
		return "", errors.New("getWebViewUA: browser worker is not configured")
	}
	return provider(ctx)
}
