package sourceinteraction

import (
	"fmt"

	"github.com/otwako/novelreader/internal/sourceexec"
)

// ExecutionError keeps the private cause server-side while exposing a stable public classification.
type ExecutionError struct {
	Code           string
	Classification sourceexec.FailureClass
	Message        string
	cause          error
}

func (e *ExecutionError) Error() string { return e.Message }
func (e *ExecutionError) Unwrap() error { return e.cause }

func executionError(code, message string, cause error, fallback sourceexec.FailureClass) error {
	return &ExecutionError{
		Code:           code,
		Classification: sourceexec.ClassifyFailure(cause, fallback),
		Message:        message,
		cause:          fmt.Errorf("sourceinteraction: %s: %w", code, cause),
	}
}
