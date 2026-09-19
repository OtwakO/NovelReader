package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/otwako/novelreader/internal/auth"
	"github.com/otwako/novelreader/internal/epub"
	"github.com/otwako/novelreader/internal/fileimport"
	"github.com/otwako/novelreader/internal/txt"
)

func (s *Server) registerFileIntakeRoutes() {
	register := func(pattern string, handler http.HandlerFunc) {
		s.mux.Handle(pattern, s.auth.RequireIdentity(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-store")
			handler(w, r)
		})))
	}
	for _, prefix := range []string{"/api/imports/admission", "/api/imports/txt/admission"} {
		legacyTXT := prefix == "/api/imports/txt/admission"
		handler := importControlHandler(func(w http.ResponseWriter, r *http.Request) { s.handleImportAdmission(w, r, legacyTXT) })
		register("POST "+prefix, handler)
		register("GET "+prefix+"/{id}", handler)
		register("DELETE "+prefix+"/{id}", handler)
	}
	// Acquisition gets its deadline and home capacity from Admission.Begin.
	register("PUT /api/imports/txt/uploads/{id}", s.handleUploadTXT)
	register("POST /api/imports/txt/inbox/acquisitions/{id}", s.handleAcquireTXTInbox)
	register("PUT /api/imports/epub/uploads/{id}", s.handleUploadEPUB)
}

// The TXT alias preserves its response/error contract; tickets are format-neutral.
func (s *Server) handleImportAdmission(w http.ResponseWriter, r *http.Request, legacyTXT bool) {
	account, _ := auth.IdentityFromContext(r.Context())
	var ticket fileimport.AdmissionTicket
	var err error
	switch r.Method {
	case http.MethodPost:
		ticket, err = s.fileAdmission.Request(account.ID)
	case http.MethodGet:
		ticket, err = s.fileAdmission.Status(account.ID, r.PathValue("id"))
	case http.MethodDelete:
		err = s.fileAdmission.Cancel(r.Context(), account.ID, r.PathValue("id"))
	}
	if err != nil {
		if legacyTXT {
			writeTXTError(w, err)
		} else {
			writeImportAdmissionError(w, err)
		}
		return
	}
	if r.Method == http.MethodDelete {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if ticket.State == fileimport.TicketWaiting {
		w.Header().Set("Retry-After", "5")
	}
	response := struct {
		ID            string                 `json:"id"`
		State         fileimport.TicketState `json:"state"`
		ExpiresAt     time.Time              `json:"expiresAt"`
		MaxInputBytes int64                  `json:"maxInputBytes,omitempty"`
		Limits        map[string]int64       `json:"limits,omitempty"`
	}{ID: ticket.ID, State: ticket.State, ExpiresAt: ticket.ExpiresAt}
	if legacyTXT {
		response.MaxInputBytes = txt.MaxInputBytes
	} else {
		response.Limits = map[string]int64{"txt": txt.MaxInputBytes, "epub": epub.MaxInputBytes}
	}
	writeJSON(w, http.StatusOK, response)
}

func writeImportAdmissionError(w http.ResponseWriter, err error) {
	status, code, message, known := importAdmissionError(err)
	if !known {
		status, code, message = http.StatusInternalServerError, "import_storage_error", "Import admission failed"
	}
	if status == http.StatusTooManyRequests || status == http.StatusServiceUnavailable {
		w.Header().Set("Retry-After", "2")
	}
	writeErrorCode(w, status, code, message)
}

func importAdmissionError(err error) (int, string, string, bool) {
	switch {
	case errors.Is(err, fileimport.ErrAdmissionFull):
		return http.StatusTooManyRequests, "import_intake_busy", "Import intake queue is full", true
	case errors.Is(err, fileimport.ErrPaused), errors.Is(err, fileimport.ErrClosed):
		return http.StatusServiceUnavailable, "import_intake_unavailable", "Import intake is temporarily unavailable", true
	case errors.Is(err, fileimport.ErrTicketNotFound):
		return http.StatusNotFound, "import_ticket_not_found", "Intake ticket is missing or expired; check the receipt before requesting another", true
	case errors.Is(err, fileimport.ErrTicketNotReady):
		return http.StatusConflict, "import_state_changed", "Intake state changed; refresh before continuing", true
	}
	return 0, "", "", false
}
