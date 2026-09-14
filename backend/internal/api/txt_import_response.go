package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/otwako/novelreader/internal/library"
	"github.com/otwako/novelreader/internal/txt"
	"github.com/otwako/novelreader/internal/txtimport"
	"github.com/otwako/novelreader/internal/txtstore"
)

func txtControlHandler(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()
		handler(w, r.WithContext(ctx))
	}
}

type txtReceiptResponse struct {
	ID              string         `json:"id"`
	OriginalName    string         `json:"originalName"`
	State           txtstore.State `json:"state"`
	Size            int64          `json:"size"`
	CreatedAt       int64          `json:"createdAt"`
	UpdatedAt       int64          `json:"updatedAt"`
	LibraryID       string         `json:"libraryId,omitempty"`
	AnalysisVersion int64          `json:"analysisVersion"`
	Encoding        txt.Encoding   `json:"encoding"`
	Preset          txt.Preset     `json:"preset"`
	Pattern         string         `json:"pattern,omitempty"`
	HasError        bool           `json:"hasError"`
	ErrorCode       string         `json:"errorCode,omitempty"`
}

func txtReceiptDTO(value txtstore.Receipt) txtReceiptResponse {
	return txtReceiptResponse{ID: value.ID, OriginalName: value.OriginalName, State: value.State,
		Size: value.Size, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt, LibraryID: value.LibraryID,
		AnalysisVersion: value.AnalysisVersion, Encoding: value.Options.Encoding, Preset: value.Options.Preset, Pattern: value.Options.Pattern, HasError: value.Error != "", ErrorCode: txtAnalysisErrorCode(value.Error)}
}

// Older rows contain arbitrary error text. Only known codes cross the API boundary.
func txtAnalysisErrorCode(stored string) string {
	switch stored {
	case "":
		return ""
	case txtstore.AnalysisEncodingRequired, txtstore.AnalysisInvalidEncoding,
		txtstore.AnalysisUnsupportedEncoding, txtstore.AnalysisNoReadableText,
		txtstore.AnalysisNonText, txtstore.AnalysisSectionLimit, txtstore.AnalysisStorageError:
		return stored
	default:
		return txtstore.AnalysisFailedCode
	}
}

// Raw filesystem errors and managed paths are never part of import responses.
func writeTXTError(w http.ResponseWriter, err error) {
	status, code, message := http.StatusInternalServerError, "txt_storage_error", "TXT operation failed; check the receipt before retrying"
	var sizeError *http.MaxBytesError
	switch {
	case errors.Is(err, library.ErrStateChanged):
		status, code, message = http.StatusConflict, "state_changed", "Reading state changed; refresh the impact review before applying"
	case errors.Is(err, txtstore.ErrResumeRequired):
		status, code, message = http.StatusConflict, "txt_resume_required", "Choose a resume section before applying"
	case errors.Is(err, txtstore.ErrInvalidResume):
		status, code, message = http.StatusBadRequest, "txt_invalid_resume", "Resume section is not in the candidate"
	case errors.Is(err, errInboxControlBusy):
		status, code, message = http.StatusTooManyRequests, "txt_inbox_busy", err.Error()
	case errors.Is(err, errInboxProofLimit):
		status, code, message = http.StatusTooManyRequests, "txt_inbox_review_limit", err.Error()
	case errors.Is(err, errInboxProofMissing):
		status, code, message = http.StatusConflict, "txt_inbox_review_expired", err.Error()
	case errors.Is(err, txtstore.ErrInboxChanged), errors.Is(err, txtstore.ErrInboxNotDuplicate):
		status, code, message = http.StatusConflict, "txt_inbox_changed", "Inbox input or managed copy changed; review again before continuing"
	case errors.Is(err, txtstore.ErrInboxEntryMissing):
		status, code, message = http.StatusNotFound, "txt_inbox_missing", "Inbox entry not found"
	case errors.Is(err, txtstore.ErrInboxEntryType):
		status, code, message = http.StatusBadRequest, "txt_invalid_input", "Inbox entry must be a regular TXT file"
	case errors.Is(err, txtimport.ErrAdmissionFull):
		status, code, message = http.StatusTooManyRequests, "txt_intake_busy", "TXT intake queue is full"
	case errors.Is(err, txtimport.ErrPaused), errors.Is(err, txtimport.ErrClosed):
		status, code, message = http.StatusServiceUnavailable, "txt_intake_unavailable", "TXT intake is temporarily unavailable"
	case errors.Is(err, txtimport.ErrTicketNotFound):
		status, code, message = http.StatusNotFound, "txt_ticket_not_found", "Intake ticket is missing or expired; check the receipt before requesting another"
	case errors.Is(err, txtstore.ErrNotFound):
		status, code, message = http.StatusNotFound, "txt_receipt_not_found", "TXT receipt not found"
	case errors.Is(err, txtimport.ErrTicketNotReady), errors.Is(err, txtstore.ErrStateChanged):
		status, code, message = http.StatusConflict, "txt_state_changed", "TXT state changed; refresh before continuing"
	case errors.Is(err, txtstore.ErrInputTooLarge), errors.As(err, &sizeError):
		status, code, message = http.StatusRequestEntityTooLarge, "txt_too_large", "File exceeds the TXT input size limit"
	case errors.Is(err, txt.ErrInvalidPattern):
		status, code, message = http.StatusBadRequest, "txt_invalid_pattern", err.Error()
	case errors.Is(err, txtstore.ErrInvalidFilename), errors.Is(err, txt.ErrInvalidOptions):
		status, code, message = http.StatusBadRequest, "txt_invalid_input", err.Error()
	case errors.Is(err, io.ErrUnexpectedEOF):
		status, code, message = http.StatusBadRequest, "txt_interrupted", "Upload ended before the complete file arrived; check its receipt before retrying"
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		status, code, message = http.StatusRequestTimeout, "txt_interrupted", "TXT operation interrupted; refresh its current status before retrying"
	}
	if status == http.StatusTooManyRequests || status == http.StatusServiceUnavailable {
		w.Header().Set("Retry-After", "2")
	}
	if errors.Is(err, errInboxProofLimit) {
		w.Header().Set("Retry-After", "30")
	}
	if status == http.StatusInternalServerError {
		slog.Warn("TXT import operation failed", "error", err)
	}
	writeErrorCode(w, status, code, message)
}

func decodeTXTRequest(w http.ResponseWriter, r *http.Request, value any) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		writeErrorCode(w, http.StatusBadRequest, "txt_invalid_input", "Invalid TXT request")
		return false
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		writeErrorCode(w, http.StatusBadRequest, "txt_invalid_input", "Expected one JSON object")
		return false
	}
	return true
}

func txtPageLimit(r *http.Request) (int, error) {
	if r.URL.Query().Get("limit") == "" {
		return 50, nil
	}
	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || limit < 1 || limit > 100 {
		return 0, errors.New("limit must be between 1 and 100")
	}
	return limit, nil
}

func writeTXTAcquired(w http.ResponseWriter, value txtstore.Receipt, warnings []string) {
	w.Header().Set("Location", "/api/imports/txt/receipts/"+value.ID)
	writeJSON(w, http.StatusCreated, struct {
		Receipt  txtReceiptResponse `json:"receipt"`
		Warnings []string           `json:"warnings,omitempty"`
	}{txtReceiptDTO(value), warnings})
}
