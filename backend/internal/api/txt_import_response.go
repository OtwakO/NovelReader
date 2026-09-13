package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/otwako/novelreader/internal/txt"
	"github.com/otwako/novelreader/internal/txtimport"
	"github.com/otwako/novelreader/internal/txtstore"
)

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
	HasError        bool           `json:"hasError"`
}

func txtReceiptDTO(value txtstore.Receipt) txtReceiptResponse {
	return txtReceiptResponse{ID: value.ID, OriginalName: value.OriginalName, State: value.State,
		Size: value.Size, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt, LibraryID: value.LibraryID,
		AnalysisVersion: value.AnalysisVersion, Encoding: value.Options.Encoding, Preset: value.Options.Preset, HasError: value.Error != ""}
}

// Raw filesystem errors and managed paths are never part of import responses.
func writeTXTError(w http.ResponseWriter, err error) {
	status, code, message := http.StatusInternalServerError, "txt_storage_error", "TXT operation failed; check the receipt before retrying"
	var sizeError *http.MaxBytesError
	switch {
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
	case errors.Is(err, txtstore.ErrInvalidFilename), errors.Is(err, txt.ErrInvalidOptions):
		status, code, message = http.StatusBadRequest, "txt_invalid_input", err.Error()
	case errors.Is(err, io.ErrUnexpectedEOF):
		status, code, message = http.StatusBadRequest, "txt_interrupted", "Upload ended before the complete file arrived; check its receipt before retrying"
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		status, code, message = http.StatusRequestTimeout, "txt_interrupted", "TXT operation interrupted; check its receipt before retrying"
	}
	if status == http.StatusTooManyRequests || status == http.StatusServiceUnavailable {
		w.Header().Set("Retry-After", "2")
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
