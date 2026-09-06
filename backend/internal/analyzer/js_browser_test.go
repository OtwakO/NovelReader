package analyzer

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestBrowserUserAgentProvider(t *testing.T) {
	vm := NewJSVM()
	if _, err := vm.Eval(`java.getWebViewUA()`, "", ""); err == nil || !strings.Contains(err.Error(), "not configured") {
		t.Fatalf("missing worker must be explicit: %v", err)
	}
	fork := vm.ForkState()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	vm.SetBrowserUserAgentProvider(func(received context.Context) (string, error) {
		if received != ctx {
			t.Fatal("evaluation context was not forwarded")
		}
		return "Browser-owned UA", nil
	})
	value, err := fork.EvalContext(ctx, `java.getWebViewUA()`, "", "")
	if err != nil || value != "Browser-owned UA" {
		t.Fatalf("value=%v err=%v", value, err)
	}
	vm.SetBrowserUserAgentProvider(func(context.Context) (string, error) { return "", errors.New("worker unavailable") })
	if _, err := fork.EvalContext(ctx, `java.getWebViewUA()`, "", ""); err == nil || !strings.Contains(err.Error(), "worker unavailable") {
		t.Fatalf("worker failure was hidden or stale UA reused: %v", err)
	}
	vm.SetBrowserUserAgentProvider(func(received context.Context) (string, error) {
		cancel()
		return "", received.Err()
	})
	if _, err := fork.EvalContext(ctx, `java.getWebViewUA()`, "", ""); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation was lost: %v", err)
	}
}
