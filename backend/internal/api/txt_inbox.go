package api

import (
	"net/http"
	"path"

	"github.com/otwako/novelreader/internal/readerstore"
)

func (s *readerAPI) registerTXTInboxRoutes() {
	register := func(pattern string, handler http.HandlerFunc) {
		s.mux.HandleFunc(pattern, importControlHandler(handler))
	}
	register("GET /api/imports/txt/inbox", s.handleScanTXTInbox)
	register("GET /api/imports/txt/inbox/claims", s.handleListTXTInboxClaims)
	register("POST /api/imports/txt/inbox/claims/{id}/review", s.handleReviewTXTInbox)
	register("POST /api/imports/txt/inbox/reviews/{token}/confirm", s.handleConfirmTXTInbox)
	register("POST /api/imports/txt/inbox/reviews/{token}/release", s.handleReleaseTXTInbox)
	register("DELETE /api/imports/txt/inbox/reviews/{token}", s.handleCancelTXTInboxReview)
}

func txtInboxPage(w http.ResponseWriter, r *http.Request) (string, int, bool) {
	limit, err := importPageLimit(r)
	after := r.URL.Query().Get("after")
	if err != nil || len(after) > 1024 {
		writeErrorCode(w, http.StatusBadRequest, "txt_invalid_input", "Invalid inbox page")
		return "", 0, false
	}
	return after, limit, true
}

func (s *readerAPI) handleScanTXTInbox(w http.ResponseWriter, r *http.Request) {
	after, limit, valid := txtInboxPage(w, r)
	if !valid {
		return
	}
	release, err := s.txtInbox.beginIO()
	if err != nil {
		writeTXTError(w, err)
		return
	}
	defer release()
	entries, err := s.txtStore.ScanInbox(r.Context(), after, limit+1)
	if err != nil {
		writeTXTError(w, err)
		return
	}
	type entry struct {
		Name       string `json:"name"`
		Size       int64  `json:"size"`
		ModifiedAt int64  `json:"modifiedAt"`
		ReceiptID  string `json:"receiptId,omitempty"`
		Problem    string `json:"problem,omitempty"`
	}
	result := struct {
		Items      []entry `json:"items"`
		NextCursor string  `json:"nextCursor,omitempty"`
		Directory  string  `json:"directory"`
	}{Items: make([]entry, 0), Directory: path.Join(readerstore.InboxDirectory, string(s.home.ID()))}
	if len(entries) > limit {
		entries = entries[:limit]
		result.NextCursor = entries[limit-1].Name
	}
	for _, item := range entries {
		result.Items = append(result.Items, entry{item.Name, item.Size, item.ModifiedAt, item.ReceiptID, item.Problem})
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *readerAPI) handleListTXTInboxClaims(w http.ResponseWriter, r *http.Request) {
	after, limit, valid := txtInboxPage(w, r)
	if !valid {
		return
	}
	claims, err := s.txtStore.PendingInbox(r.Context(), after, limit+1)
	if err != nil {
		writeTXTError(w, err)
		return
	}
	type claim struct {
		Name      string `json:"name"`
		ReceiptID string `json:"receiptId"`
	}
	result := struct {
		Items      []claim `json:"items"`
		NextCursor string  `json:"nextCursor,omitempty"`
	}{Items: make([]claim, 0)}
	if len(claims) > limit {
		claims = claims[:limit]
		result.NextCursor = claims[limit-1].Name
	}
	for _, item := range claims {
		result.Items = append(result.Items, claim{item.Name, item.ReceiptID})
	}
	writeJSON(w, http.StatusOK, result)
}
