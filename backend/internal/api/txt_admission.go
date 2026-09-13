package api

import (
	"context"
	"net/http"
	"time"

	"github.com/otwako/novelreader/internal/auth"
	"github.com/otwako/novelreader/internal/txt"
	"github.com/otwako/novelreader/internal/txtimport"
)

func (s *Server) registerTXTIntakeRoutes() {
	register := func(pattern string, handler http.HandlerFunc) {
		s.mux.Handle(pattern, s.auth.RequireIdentity(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-store")
			// Transfers get their deadline from admission; metadata/cancellation
			// requests have a short independent deadline.
			if r.Method != http.MethodPut {
				ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
				defer cancel()
				r = r.WithContext(ctx)
			}
			handler(w, r)
		})))
	}
	register("POST /api/imports/txt/admission", s.handleRequestTXTAdmission)
	register("GET /api/imports/txt/admission/{id}", s.handleGetTXTAdmission)
	register("DELETE /api/imports/txt/admission/{id}", s.handleCancelTXTAdmission)
	register("PUT /api/imports/txt/uploads/{id}", s.handleUploadTXT)
}

func writeTXTAdmission(w http.ResponseWriter, ticket txtimport.AdmissionTicket) {
	if ticket.State == txtimport.TicketWaiting {
		w.Header().Set("Retry-After", "5")
	}
	writeJSON(w, http.StatusOK, struct {
		ID            string                `json:"id"`
		State         txtimport.TicketState `json:"state"`
		ExpiresAt     time.Time             `json:"expiresAt"`
		MaxInputBytes int64                 `json:"maxInputBytes"`
	}{ticket.ID, ticket.State, ticket.ExpiresAt, txt.MaxInputBytes})
}

func (s *Server) handleRequestTXTAdmission(w http.ResponseWriter, r *http.Request) {
	account, _ := auth.IdentityFromContext(r.Context()) // RequireIdentity owns authentication.
	ticket, err := s.txtAdmission.Request(account.ID)
	if err != nil {
		writeTXTError(w, err)
		return
	}
	writeTXTAdmission(w, ticket)
}

func (s *Server) handleGetTXTAdmission(w http.ResponseWriter, r *http.Request) {
	account, _ := auth.IdentityFromContext(r.Context())
	ticket, err := s.txtAdmission.Status(account.ID, r.PathValue("id"))
	if err != nil {
		writeTXTError(w, err)
		return
	}
	writeTXTAdmission(w, ticket)
}

func (s *Server) handleCancelTXTAdmission(w http.ResponseWriter, r *http.Request) {
	account, _ := auth.IdentityFromContext(r.Context())
	if err := s.txtAdmission.Cancel(r.Context(), account.ID, r.PathValue("id")); err != nil {
		writeTXTError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
