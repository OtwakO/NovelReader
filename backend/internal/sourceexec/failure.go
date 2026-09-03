package sourceexec

import (
	"context"
	"errors"
	"net"
)

// FailureClass is the stable, secret-safe family of a source execution failure.
type FailureClass string

const (
	FailureTimeout                FailureClass = "timeout"
	FailureTransport              FailureClass = "transport_failed"
	FailureJavaScriptRuntime      FailureClass = "javascript_runtime"
	FailureInvalidResult          FailureClass = "invalid_result"
	FailureAuthenticationRequired FailureClass = "authentication_required"
)

// ClassifyFailure preserves typed runtime failures and uses fallback for operation-owned meaning.
func ClassifyFailure(err error, fallback FailureClass) FailureClass {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return FailureTimeout
	}
	var networkError net.Error
	if errors.As(err, &networkError) {
		if networkError.Timeout() {
			return FailureTimeout
		}
		return FailureTransport
	}
	return fallback
}
