package sourceexec

import (
	"context"
	"errors"
	"testing"
)

func TestClassifyFailureUsesTypedRuntimeErrorsAndOperationFallback(t *testing.T) {
	if got := ClassifyFailure(context.DeadlineExceeded, FailureJavaScriptRuntime); got != FailureTimeout {
		t.Fatalf("deadline classification=%q", got)
	}
	if got := ClassifyFailure(errors.New("syntax detail"), FailureJavaScriptRuntime); got != FailureJavaScriptRuntime {
		t.Fatalf("fallback classification=%q", got)
	}
}
