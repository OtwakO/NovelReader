package api

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/otwako/novelreader/internal/epub"
	"github.com/otwako/novelreader/internal/epubstore"
	"github.com/otwako/novelreader/internal/imageproc"
	"github.com/otwako/novelreader/internal/inboxfiles"
	"github.com/otwako/novelreader/internal/readerstore"
)

type epubReceiptResponse struct {
	ID               string                     `json:"id"`
	OriginalName     string                     `json:"originalName"`
	State            epubstore.AcquisitionState `json:"state"`
	Size             int64                      `json:"size"`
	CreatedAt        int64                      `json:"createdAt"`
	UpdatedAt        int64                      `json:"updatedAt"`
	LibraryID        string                     `json:"libraryId,omitempty"`
	Generation       int64                      `json:"generation"`
	PreparationState epubstore.PreparationState `json:"preparationState,omitempty"`
	ImageMode        epub.ImageMode             `json:"imageMode"`
	HasError         bool                       `json:"hasError"`
	ErrorCode        string                     `json:"errorCode,omitempty"`
	Notices          []string                   `json:"notices,omitempty"`
}

func epubReceiptDTO(value epubstore.ImportReceipt) epubReceiptResponse {
	code := epubstore.PreparationErrorCode(value.PreparationError)
	if value.Error != "" {
		code = "epub_acquisition_failed"
	}
	result := epubReceiptResponse{ID: value.ID, OriginalName: value.OriginalName, State: value.State, Size: value.Size,
		CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt, LibraryID: value.LibraryID, Generation: value.PreparationGeneration,
		PreparationState: value.PreparationState, ImageMode: value.ImageMode, HasError: code != "", ErrorCode: code}
	// This is a capability notice before work, not a content warning. Ready review
	// uses persisted actual backend evidence, including after a restore elsewhere.
	if value.ImageMode == epub.OptimizedImages && value.State == epubstore.Acquired &&
		(value.PreparationGeneration == 0 || value.PreparationState == epubstore.PreparationQueued || value.PreparationState == epubstore.PreparationRunning) && imageproc.EncoderBackend() == "portable" {
		result.Notices = []string{"epub_portable_encoder"}
	}
	return result
}

func writeEPUBError(w http.ResponseWriter, err error) {
	if _, _, _, known := importAdmissionError(err); known {
		writeImportAdmissionError(w, err)
		return
	}
	status, code, message := http.StatusInternalServerError, "epub_storage_error", "EPUB operation failed; check the receipt before retrying"
	var sizeError *http.MaxBytesError
	switch {
	case errors.Is(err, errInboxControlBusy):
		status, code, message = http.StatusTooManyRequests, "epub_inbox_busy", err.Error()
	case errors.Is(err, errInboxProofLimit):
		status, code, message = http.StatusTooManyRequests, "epub_inbox_review_limit", err.Error()
	case errors.Is(err, errInboxProofMissing):
		status, code, message = http.StatusConflict, "epub_inbox_review_expired", err.Error()
	case errors.Is(err, inboxfiles.ErrChanged), errors.Is(err, epubstore.ErrInboxNotDuplicate):
		status, code, message = http.StatusConflict, "epub_inbox_changed", "Inbox input or managed copy changed; review again before continuing"
	case errors.Is(err, inboxfiles.ErrMissing):
		status, code, message = http.StatusNotFound, "epub_inbox_missing", "Inbox entry not found"
	case errors.Is(err, inboxfiles.ErrType):
		status, code, message = http.StatusBadRequest, "epub_invalid_input", "Inbox entry must be a regular EPUB file"
	case errors.Is(err, epubstore.ErrNotFound):
		status, code, message = http.StatusNotFound, "epub_receipt_not_found", "EPUB receipt not found"
	case errors.Is(err, epubstore.ErrStateChanged):
		status, code, message = http.StatusConflict, "epub_state_changed", "EPUB state changed or preparation is active; refresh before continuing"
	case errors.Is(err, readerstore.ErrFileTooLarge), errors.As(err, &sizeError):
		status, code, message = http.StatusRequestEntityTooLarge, "epub_too_large", "File exceeds the EPUB input size limit"
	case errors.Is(err, epubstore.ErrInvalidFilename), errors.Is(err, epub.ErrImagePolicy):
		status, code, message = http.StatusBadRequest, "epub_invalid_input", "Expected an EPUB filename and original or optimized image mode"
	case errors.Is(err, io.ErrUnexpectedEOF):
		status, code, message = http.StatusBadRequest, "epub_interrupted", "Upload ended before the complete file arrived; check its receipt before retrying"
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		status, code, message = http.StatusRequestTimeout, "epub_interrupted", "EPUB operation interrupted; refresh its current status before retrying"
	}
	if status == http.StatusTooManyRequests {
		w.Header().Set("Retry-After", "2")
	}
	if errors.Is(err, errInboxProofLimit) {
		w.Header().Set("Retry-After", "30")
	}
	if status == http.StatusInternalServerError {
		slog.Warn("EPUB import operation failed", "error", err)
	}
	writeErrorCode(w, status, code, message)
}

func decodeEPUBRequest(w http.ResponseWriter, r *http.Request, value any) bool {
	return decodeImportRequest(w, r, value, "epub_invalid_input", "Invalid EPUB request")
}
