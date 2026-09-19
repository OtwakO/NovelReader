package api

import (
	"log/slog"
	"net/http"
	"strconv"
	"strings"
)

func (s *readerAPI) registerEPUBImportRoutes() {
	register := func(pattern string, handler http.HandlerFunc) {
		s.mux.HandleFunc(pattern, importControlHandler(handler))
	}
	register("GET /api/imports/epub/receipts", s.handleListEPUBReceipts)
	register("GET /api/imports/epub/receipts/{id}", s.handleGetEPUBReceipt)
	register("GET /api/imports/epub/receipts/{id}/preview", s.handlePreviewEPUB)
	register("POST /api/imports/epub/receipts/{id}/retry", s.handleRetryEPUB)
	register("POST /api/imports/epub/receipts/{id}/accept", s.handleAcceptEPUB)
	register("DELETE /api/imports/epub/receipts/{id}", s.handleDiscardEPUB)
}

func (s *readerAPI) handleListEPUBReceipts(w http.ResponseWriter, r *http.Request) {
	limit, err := importPageLimit(r)
	if err != nil {
		writeErrorCode(w, http.StatusBadRequest, "epub_invalid_input", err.Error())
		return
	}
	items, err := s.epubStore.ListImports(r.Context(), r.URL.Query().Get("after"), limit+1)
	if err != nil {
		writeEPUBError(w, err)
		return
	}
	after := ""
	if len(items) > limit {
		items = items[:limit]
		after = items[len(items)-1].ID
	}
	result := make([]epubReceiptResponse, 0, len(items))
	for _, item := range items {
		result = append(result, epubReceiptDTO(item))
	}
	writeJSON(w, http.StatusOK, struct {
		Items      []epubReceiptResponse `json:"items"`
		NextCursor string                `json:"nextCursor,omitempty"`
	}{result, after})
}

func (s *readerAPI) handleGetEPUBReceipt(w http.ResponseWriter, r *http.Request) {
	receipt, err := s.epubStore.GetImport(r.Context(), r.PathValue("id"))
	if err != nil {
		writeEPUBError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, epubReceiptDTO(receipt))
}

func (s *readerAPI) handlePreviewEPUB(w http.ResponseWriter, r *http.Request) {
	generation, err := strconv.ParseInt(r.URL.Query().Get("generation"), 10, 64)
	if err != nil || generation < 1 {
		writeErrorCode(w, http.StatusBadRequest, "epub_invalid_input", "Expected a preparation generation")
		return
	}
	start := 0
	if raw := r.URL.Query().Get("start"); raw != "" {
		start, err = strconv.Atoi(raw)
	}
	if err != nil || start < 0 {
		writeErrorCode(w, http.StatusBadRequest, "epub_invalid_input", "Invalid preview offset")
		return
	}
	limit, err := importPageLimit(r)
	if err != nil {
		writeErrorCode(w, http.StatusBadRequest, "epub_invalid_input", err.Error())
		return
	}
	preview, err := s.epubStore.Review(r.Context(), r.PathValue("id"), generation, start, limit)
	if err != nil {
		writeEPUBError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, epubPreviewDTO(preview))
}

func (s *readerAPI) handleRetryEPUB(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Generation *int64 `json:"generation"`
	}
	if !decodeEPUBRequest(w, r, &body) {
		return
	}
	if body.Generation == nil || *body.Generation < 0 {
		writeErrorCode(w, http.StatusBadRequest, "epub_invalid_input", "Expected the current preparation generation")
		return
	}
	id := r.PathValue("id")
	if _, err := s.epubStore.RetryPreparation(r.Context(), id, *body.Generation); err != nil {
		writeEPUBError(w, err)
		return
	}
	warnings := wakeEPUBPreparation(s.fileImports, s.home.ID(), id)
	receipt, err := s.epubStore.GetImport(r.Context(), id)
	if err != nil {
		writeEPUBError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, struct {
		Receipt  epubReceiptResponse `json:"receipt"`
		Warnings []string            `json:"warnings,omitempty"`
	}{epubReceiptDTO(receipt), warnings})
}

func (s *readerAPI) handleAcceptEPUB(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Generation int64  `json:"generation"`
		Name       string `json:"name"`
		Author     string `json:"author"`
	}
	if !decodeEPUBRequest(w, r, &body) {
		return
	}
	if body.Generation < 1 || strings.TrimSpace(body.Name) == "" || len(body.Name) > 1024 || len(body.Author) > 512 {
		writeErrorCode(w, http.StatusBadRequest, "epub_invalid_input", "Expected a generation and book name")
		return
	}
	result, err := s.epubStore.Accept(r.Context(), r.PathValue("id"), body.Generation, body.Name, body.Author)
	if err != nil {
		writeEPUBError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, struct {
		LibraryID string `json:"libraryId"`
	}{result.ID})
}

func (s *readerAPI) handleDiscardEPUB(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	// Join an active body transfer before touching its durable receipt/files.
	if err := s.fileAdmission.Cancel(r.Context(), s.home.ID(), id); err != nil {
		writeEPUBError(w, err)
		return
	}
	pending, err := s.epubStore.DiscardPending(r.Context(), id)
	if err != nil && !pending {
		writeEPUBError(w, err)
		return
	}
	warnings := []string(nil)
	if pending {
		slog.Warn("EPUB receipt discarded; cleanup pending", "reader_id", s.home.ID(), "receipt_id", id, "error", err)
		warnings = []string{"epub_cleanup_pending"}
	}
	writeJSON(w, http.StatusOK, struct {
		CleanupPending bool     `json:"cleanupPending"`
		Warnings       []string `json:"warnings,omitempty"`
	}{pending, warnings})
}
