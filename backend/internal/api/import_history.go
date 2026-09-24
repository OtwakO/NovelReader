package api

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"sort"

	"github.com/otwako/novelreader/internal/importhistory"
)

type importHistoryItem struct {
	Format  string `json:"format"`
	Status  string `json:"status"`
	Receipt any    `json:"receipt"`
	cursor  importhistory.Cursor
}

func (s *readerAPI) handleImportHistory(w http.ResponseWriter, r *http.Request) {
	limit, err := importPageLimit(r)
	if err != nil {
		writeErrorCode(w, http.StatusBadRequest, "import_invalid_input", "Invalid import history page size")
		return
	}
	format, status := r.URL.Query().Get("format"), r.URL.Query().Get("status")
	validStatus := false
	switch status {
	case "", "processing", "ready", "needs_review", "failed", "added", "removing":
		validStatus = true
	}
	query := importhistory.Query{Status: status, Limit: limit + 1}
	raw := r.URL.Query().Get("before")
	if raw != "" && len(raw) <= 512 {
		var data []byte
		data, err = base64.RawURLEncoding.DecodeString(raw)
		if err == nil {
			query.Before = &importhistory.Cursor{}
			err = json.Unmarshal(data, query.Before)
		}
	}
	validCursor := raw == ""
	if cursor := query.Before; cursor != nil {
		validCursor = cursor.CreatedAt > 0 && cursor.ID != "" && len(cursor.ID) <= 128 &&
			(cursor.Format == "txt" || cursor.Format == "epub")
	}
	if err != nil || !validStatus || !validCursor || (format != "" && format != "txt" && format != "epub") {
		writeErrorCode(w, http.StatusBadRequest, "import_invalid_input", "Invalid import history page or filter")
		return
	}
	items := make([]importHistoryItem, 0, query.Limit*2)
	if format != "epub" {
		receipts, err := s.txtStore.History(r.Context(), query)
		if err != nil {
			writeTXTError(w, err)
			return
		}
		for _, receipt := range receipts {
			items = append(items, importHistoryItem{
				Format: "txt", Status: receipt.Status, Receipt: txtReceiptDTO(receipt.Receipt),
				cursor: importhistory.Cursor{CreatedAt: receipt.CreatedAt, Format: "txt", ID: receipt.ID},
			})
		}
	}
	if format != "txt" {
		receipts, err := s.epubStore.History(r.Context(), query)
		if err != nil {
			writeEPUBError(w, err)
			return
		}
		for _, receipt := range receipts {
			items = append(items, importHistoryItem{
				Format: "epub", Status: receipt.Status, Receipt: epubReceiptDTO(receipt.ImportReceipt),
				cursor: importhistory.Cursor{CreatedAt: receipt.CreatedAt, Format: "epub", ID: receipt.ID},
			})
		}
	}
	// Each provider returns at most limit+1 rows after the SAME total-order cursor.
	// Merge before cutting the page; neither provider's unconsumed rows are skipped.
	sort.Slice(items, func(i, j int) bool {
		a, b := items[i].cursor, items[j].cursor
		if a.CreatedAt != b.CreatedAt {
			return a.CreatedAt > b.CreatedAt
		}
		if a.Format != b.Format {
			return a.Format < b.Format
		}
		return a.ID > b.ID
	})
	next := ""
	if len(items) > limit {
		items = items[:limit]
		data, _ := json.Marshal(items[len(items)-1].cursor)
		next = base64.RawURLEncoding.EncodeToString(data)
	}
	writeJSON(w, http.StatusOK, struct {
		Items      []importHistoryItem `json:"items"`
		NextCursor string              `json:"nextCursor,omitempty"`
	}{items, next})
}
