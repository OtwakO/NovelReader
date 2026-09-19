package api

import (
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/otwako/novelreader/internal/txt"
	"github.com/otwako/novelreader/internal/txtstore"
)

// These bounded review/control operations use the ordinary reader runtime, not
// transfer slots. They neither crawl nor run analysis in an HTTP request.
func (s *readerAPI) registerTXTReceiptRoutes() {
	s.registerTXTReparseRoutes()
	register := func(pattern string, handler http.HandlerFunc) {
		s.mux.HandleFunc(pattern, importControlHandler(handler))
	}
	register("GET /api/imports/txt/receipts", s.handleListTXTReceipts)
	register("GET /api/imports/txt/receipts/{id}", s.handleGetTXTReceipt)
	register("GET /api/imports/txt/receipts/{id}/preview", s.handlePreviewTXTReceipt)
	register("POST /api/imports/txt/receipts/{id}/analysis", s.handleAnalyzeTXTReceipt)
	register("POST /api/imports/txt/receipts/{id}/accept", s.handleAcceptTXTReceipt)
	register("DELETE /api/imports/txt/receipts/{id}", s.handleDiscardTXTReceipt)
}

func (s *readerAPI) handleListTXTReceipts(w http.ResponseWriter, r *http.Request) {
	limit, err := importPageLimit(r)
	state := txtstore.State(r.URL.Query().Get("state"))
	validState := false
	switch state {
	case "", txtstore.Receiving, txtstore.Received, txtstore.Failed, txtstore.Removing, txtstore.Analyzing, txtstore.Ready, txtstore.NeedsReview, txtstore.AnalysisFailed, txtstore.Published:
		validState = true
	}
	after := r.URL.Query().Get("after")
	if err != nil || !validState || len(after) > 128 {
		writeErrorCode(w, http.StatusBadRequest, "txt_invalid_input", "Invalid receipt page or state filter")
		return
	}
	values, err := s.txtStore.List(r.Context(), after, state, limit+1)
	if err != nil {
		writeTXTError(w, err)
		return
	}
	response := struct {
		Items      []txtReceiptResponse `json:"items"`
		NextCursor string               `json:"nextCursor,omitempty"`
	}{Items: make([]txtReceiptResponse, 0)}
	if len(values) > limit {
		values = values[:limit]
		response.NextCursor = values[limit-1].ID
	}
	for _, value := range values {
		response.Items = append(response.Items, txtReceiptDTO(value))
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *readerAPI) handleGetTXTReceipt(w http.ResponseWriter, r *http.Request) {
	value, err := s.txtStore.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		writeTXTError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, txtReceiptDTO(value))
}

func (s *readerAPI) handlePreviewTXTReceipt(w http.ResponseWriter, r *http.Request) {
	version, err := strconv.ParseInt(r.URL.Query().Get("analysisVersion"), 10, 64)
	start := 0
	if r.URL.Query().Has("start") {
		var startErr error
		start, startErr = strconv.Atoi(r.URL.Query().Get("start"))
		if startErr != nil {
			start = -1
		}
	}
	limit, limitErr := importPageLimit(r)
	if err != nil || version < 1 || start < 0 || limitErr != nil {
		writeErrorCode(w, http.StatusBadRequest, "txt_invalid_input", "Expected an analysis version and bounded preview page")
		return
	}
	value, err := s.txtStore.Review(r.Context(), r.PathValue("id"), version, start, limit)
	if err != nil {
		writeTXTError(w, err)
		return
	}
	writeTXTPreview(w, value)
}

func (s *readerAPI) handleAnalyzeTXTReceipt(w http.ResponseWriter, r *http.Request) {
	var input struct {
		AnalysisVersion *int64       `json:"analysisVersion"`
		Encoding        txt.Encoding `json:"encoding"`
		Preset          txt.Preset   `json:"preset"`
		Pattern         string       `json:"pattern"`
	}
	if !decodeTXTRequest(w, r, &input) {
		return
	}
	if input.AnalysisVersion == nil || *input.AnalysisVersion < 0 {
		writeErrorCode(w, http.StatusBadRequest, "txt_invalid_input", "Expected the receipt analysis version")
		return
	}
	if err := s.txtStore.QueueAnalysis(r.Context(), r.PathValue("id"), *input.AnalysisVersion, txt.Options{Encoding: input.Encoding, Preset: input.Preset, Pattern: input.Pattern}); err != nil {
		writeTXTError(w, err)
		return
	}
	warnings := wakeTXTAnalysis(s.fileImports, s.home.ID(), r.PathValue("id"))
	writeJSON(w, http.StatusAccepted, struct {
		Status   string   `json:"status"`
		Warnings []string `json:"warnings,omitempty"`
	}{"queued", warnings})
}

func (s *readerAPI) handleAcceptTXTReceipt(w http.ResponseWriter, r *http.Request) {
	var input struct {
		AnalysisVersion int64  `json:"analysisVersion"`
		Name            string `json:"name"`
		Author          string `json:"author"`
	}
	if !decodeTXTRequest(w, r, &input) {
		return
	}
	if input.AnalysisVersion < 1 || strings.TrimSpace(input.Name) == "" || len(input.Name) > 1024 || len(input.Author) > 512 {
		writeErrorCode(w, http.StatusBadRequest, "txt_invalid_input", "Expected an analysis version, title and optional author")
		return
	}
	item, err := s.txtStore.Accept(r.Context(), r.PathValue("id"), input.AnalysisVersion, input.Name, input.Author)
	if err != nil {
		writeTXTError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, struct {
		LibraryID string `json:"libraryId"`
	}{item.ID})
}

func (s *readerAPI) handleDiscardTXTReceipt(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.fileAdmission.Cancel(r.Context(), s.home.ID(), id); err != nil {
		writeTXTError(w, err)
		return
	}
	pending, err := s.txtStore.DiscardPending(r.Context(), id)
	if pending {
		slog.Warn("TXT receipt discarded; cleanup pending", "reader_id", s.home.ID(), "receipt_id", id, "error", err)
		writeJSON(w, http.StatusOK, map[string]any{"status": "removed", "warnings": []string{"txt_cleanup_pending"}})
		return
	}
	if err != nil {
		writeTXTError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
