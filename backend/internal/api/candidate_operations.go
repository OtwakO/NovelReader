package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/otwako/novelreader/internal/auth"
	"github.com/otwako/novelreader/internal/candidate"
)

func (s *readerAPI) handleStartCandidateResolution(w http.ResponseWriter, r *http.Request) {
	var input candidate.Input
	if !decodeCandidateOperationJSON(w, r, &input, 1<<20) {
		return
	}
	ownerID := candidateOperationOwner(r)
	runtime, err := s.acquireCandidateRuntime(r, ownerID)
	if err != nil {
		writeErrorCode(w, http.StatusServiceUnavailable, "reader_storage_unavailable", "reader storage unavailable")
		return
	}
	snapshot, err := s.candidateOperations.Start(ownerID, input, runtime)
	if err != nil {
		if runtime.Release != nil {
			runtime.Release()
		}
		writeErrorCode(w, http.StatusBadRequest, "invalid_candidate_operation", err.Error())
		return
	}
	s.addCandidateCoverDisplayURL(&snapshot)
	writeJSON(w, http.StatusAccepted, snapshot)
}

func (s *readerAPI) handleGetCandidateResolution(w http.ResponseWriter, r *http.Request) {
	snapshot, ok := s.candidateOperations.Get(candidateOperationOwner(r), r.PathValue("id"))
	if !ok {
		writeErrorCode(w, http.StatusNotFound, "candidate_operation_not_found", "candidate resolution not found")
		return
	}
	s.addCandidateCoverDisplayURL(&snapshot)
	writeJSON(w, http.StatusOK, snapshot)
}

func (s *readerAPI) handleStreamCandidateResolution(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeErrorCode(w, http.StatusInternalServerError, "streaming_not_supported", "streaming not supported")
		return
	}
	updates, unsubscribe, ok := s.candidateOperations.Subscribe(candidateOperationOwner(r), r.PathValue("id"))
	if !ok {
		writeErrorCode(w, http.StatusNotFound, "candidate_operation_not_found", "candidate resolution not found")
		return
	}
	defer unsubscribe()
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	for {
		select {
		case <-r.Context().Done():
			return
		case snapshot, open := <-updates:
			if !open {
				return
			}
			s.addCandidateCoverDisplayURL(&snapshot)
			payload, err := json.Marshal(snapshot)
			if err != nil {
				return
			}
			_, _ = fmt.Fprintf(w, "data: %s\n\n", payload)
			flusher.Flush()
			if terminalCandidateState(snapshot.State) {
				return
			}
		}
	}
}

func (s *readerAPI) handleCancelCandidateResolution(w http.ResponseWriter, r *http.Request) {
	if !s.candidateOperations.Cancel(candidateOperationOwner(r), r.PathValue("id")) {
		writeErrorCode(w, http.StatusNotFound, "candidate_operation_not_found", "candidate resolution not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *readerAPI) handleCommitCandidateResolution(w http.ResponseWriter, r *http.Request) {
	var request struct {
		BookID string `json:"bookId"`
	}
	if !decodeCandidateOperationJSON(w, r, &request, 8<<10) {
		return
	}
	snapshot, err := s.candidateOperations.Commit(candidateOperationOwner(r), r.PathValue("id"), request.BookID)
	if err != nil {
		status := http.StatusConflict
		code := "candidate_not_verified"
		switch {
		case errors.Is(err, candidate.ErrOperationNotFound):
			status = http.StatusNotFound
			code = "candidate_operation_not_found"
		case errors.Is(err, candidate.ErrInvalidBookID):
			status = http.StatusBadRequest
			code = "invalid_candidate_commit"
		}
		writeErrorCode(w, status, code, err.Error())
		return
	}
	status := http.StatusOK
	if snapshot.Created {
		status = http.StatusCreated
	}
	s.addCandidateCoverDisplayURL(&snapshot)
	writeJSON(w, status, snapshot)
}

func (s *readerAPI) acquireCandidateRuntime(r *http.Request, ownerID string) (candidate.Runtime, error) {
	if s.runtimes == nil {
		return candidate.Runtime{Sources: s.sourceStore, Books: s.bookStore, Searcher: s.searcher}, nil
	}
	account, ok := auth.IdentityFromContext(r.Context())
	if !ok || string(account.ID) != ownerID {
		return candidate.Runtime{}, fmt.Errorf("candidate owner is unavailable")
	}
	runtime, release, err := s.runtimes.acquire(r.Context(), account.ID)
	if err != nil {
		return candidate.Runtime{}, err
	}
	return candidate.Runtime{Sources: runtime.sourceStore, Books: runtime.bookStore, Searcher: runtime.searcher, Release: release}, nil
}

func candidateOperationOwner(r *http.Request) string {
	if account, ok := auth.IdentityFromContext(r.Context()); ok {
		return string(account.ID)
	}
	return "local"
}

func terminalCandidateState(state candidate.State) bool {
	switch state {
	case candidate.StateCommitted, candidate.StateExhausted, candidate.StateCancelled, candidate.StateFailed:
		return true
	default:
		return false
	}
}

func decodeCandidateOperationJSON(w http.ResponseWriter, r *http.Request, target any, limit int64) bool {
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeErrorCode(w, http.StatusBadRequest, "invalid_candidate_request", "invalid candidate request")
		return false
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		writeErrorCode(w, http.StatusBadRequest, "invalid_candidate_request", "invalid candidate request")
		return false
	}
	return true
}
