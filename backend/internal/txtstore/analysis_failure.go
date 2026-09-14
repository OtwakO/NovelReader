package txtstore

import (
	"errors"

	"github.com/otwako/novelreader/internal/txt"
)

// Stable codes are stored in the existing interpretation error column. Raw
// failures still return to the worker; they must never become response text.
const (
	AnalysisEncodingRequired    = "txt_encoding_required"
	AnalysisInvalidEncoding     = "txt_invalid_encoding"
	AnalysisUnsupportedEncoding = "txt_unsupported_encoding"
	AnalysisNoReadableText      = "txt_no_readable_text"
	AnalysisNonText             = "txt_non_text"
	AnalysisSectionLimit        = "txt_section_limit"
	AnalysisStorageError        = "txt_storage_error"
	AnalysisFailedCode          = "txt_analysis_failed"
)

var errAnalysisStorage = errors.New("txtstore: analysis storage failure")

func analysisFailureCode(err error) string {
	switch {
	case errors.Is(err, errAnalysisStorage):
		return AnalysisStorageError
	case errors.Is(err, txt.ErrEncodingRequired):
		return AnalysisEncodingRequired
	case errors.Is(err, txt.ErrInvalidEncoding):
		return AnalysisInvalidEncoding
	case errors.Is(err, txt.ErrUnsupportedEncoding):
		return AnalysisUnsupportedEncoding
	case errors.Is(err, txt.ErrNoReadableText):
		return AnalysisNoReadableText
	case errors.Is(err, txt.ErrNonText):
		return AnalysisNonText
	case errors.Is(err, txt.ErrSectionLimit):
		return AnalysisSectionLimit
	default:
		return AnalysisFailedCode
	}
}
