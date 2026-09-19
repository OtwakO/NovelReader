package epubstore

import (
	"context"
	"errors"

	"github.com/otwako/novelreader/internal/epub"
)

// Persist only stable categories for new failures. Wrapped paths/parser messages
// still reach the worker's error log, never a receipt response.
func preparationFailureCode(err error) string {
	switch {
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return "epub_preparation_interrupted"
	case errors.Is(err, epub.ErrLimit):
		return "epub_preparation_limit"
	case errors.Is(err, epub.ErrUnsupported):
		return "epub_unsupported_publication"
	case errors.Is(err, epub.ErrArchive), errors.Is(err, epub.ErrPackage), errors.Is(err, epub.ErrReference):
		return "epub_invalid_publication"
	default:
		return "epub_preparation_failed"
	}
}

// PreparationErrorCode also handles older/recovery evidence containing arbitrary
// text. Do not infer categories by parsing error messages.
func PreparationErrorCode(stored string) string {
	switch stored {
	case "", "epub_preparation_interrupted", "epub_preparation_limit", "epub_unsupported_publication", "epub_invalid_publication":
		return stored
	default:
		return "epub_preparation_failed"
	}
}
