// Explore handlers expose typed per-source navigation without source rules or actions.
package api

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/otwako/novelreader/internal/book"
	"github.com/otwako/novelreader/internal/sourceinteraction"
)

const maxExploreRequestBytes = 32 * 1024

type exploreErrorResponse struct {
	Code      string `json:"code"`
	Stage     string `json:"stage"`
	Severity  string `json:"severity"`
	Retryable bool   `json:"retryable"`
	Message   string `json:"message"`
	NextPage  int    `json:"nextPage,omitempty"`
}

func (s *Server) handleExploreSources(w http.ResponseWriter, _ *http.Request) {
	sources, err := s.searcher.ExploreSources()
	if err != nil {
		writeExploreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, sources)
}

func (s *Server) handleExploreCatalog(w http.ResponseWriter, r *http.Request) {
	var request struct {
		SourceID string `json:"sourceId"`
	}
	if !decodeExploreJSON(w, r, &request) {
		return
	}
	if strings.TrimSpace(request.SourceID) == "" {
		writeError(w, http.StatusBadRequest, "sourceId is required")
		return
	}
	catalog, err := s.searcher.OpenExplore(r.Context(), request.SourceID)
	if err != nil {
		writeExploreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, catalog)
}

func (s *Server) handleExploreControl(w http.ResponseWriter, r *http.Request) {
	var request book.ExploreControlRequest
	if !decodeExploreJSON(w, r, &request) {
		return
	}
	if strings.TrimSpace(request.SessionID) == "" || strings.TrimSpace(request.ControlID) == "" {
		writeError(w, http.StatusBadRequest, "sessionId and controlId are required")
		return
	}
	catalog, err := s.searcher.UpdateExploreControl(r.Context(), request)
	if err != nil {
		writeExploreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, catalog)
}

func (s *Server) handleExploreAction(w http.ResponseWriter, r *http.Request) {
	var request book.ExploreActionRequest
	if !decodeExploreJSON(w, r, &request) {
		return
	}
	if strings.TrimSpace(request.SessionID) == "" || strings.TrimSpace(request.EntryID) == "" {
		writeError(w, http.StatusBadRequest, "sessionId and entryId are required")
		return
	}
	result, err := s.searcher.ExecuteExploreAction(r.Context(), request)
	if err != nil {
		writeExploreError(w, err)
		return
	}
	effects := make([]sourceinteraction.Effect, len(result.Effects))
	for index, effect := range result.Effects {
		effects[index] = sourceinteraction.Effect{Type: effect.Type, Message: effect.Message, URL: effect.URL, Title: effect.Title, Await: effect.Await}
	}
	effects = sourceinteraction.RegisterBrowserRequests(effects, s.browserSessions)
	writeJSON(w, http.StatusOK, map[string]interface{}{"sourceId": result.SourceID, "effects": effects})
}

func (s *Server) handleExplorePage(w http.ResponseWriter, r *http.Request) {
	var request book.ExplorePageRequest
	if !decodeExploreJSON(w, r, &request) {
		return
	}
	if strings.TrimSpace(request.SessionID) == "" || strings.TrimSpace(request.CategoryID) == "" || request.Page < 1 {
		writeError(w, http.StatusBadRequest, "sessionId, categoryId, and a positive page are required")
		return
	}
	page, err := s.searcher.GetExplorePage(r.Context(), request)
	if err != nil {
		writeExploreError(w, err)
		return
	}
	s.loadShelfMembership().annotate(page.Books)
	s.addExploreCoverDisplayURLs(&page)
	writeJSON(w, http.StatusOK, page)
}

func decodeExploreJSON(w http.ResponseWriter, r *http.Request, destination interface{}) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxExploreRequestBytes)
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeError(w, http.StatusRequestEntityTooLarge, "request body is too large")
		} else {
			writeError(w, http.StatusBadRequest, "invalid request body")
		}
		return false
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return false
	}
	return true
}

func writeExploreError(w http.ResponseWriter, err error) {
	var exploreErr *book.ExploreError
	if !errors.As(err, &exploreErr) {
		slog.Error("api: unexpected Explore failure")
		writeJSON(w, http.StatusInternalServerError, exploreErrorResponse{
			Code: "internal_error", Stage: "internal", Severity: "error", Message: "Explore request failed",
		})
		return
	}
	slog.Warn("api: Explore request failed", "code", exploreErr.Code, "stage", exploreErr.Stage, "retryable", exploreErr.Retryable)
	writeJSON(w, exploreHTTPStatus(exploreErr.Code), exploreErrorResponse{
		Code: exploreErr.Code, Stage: exploreErr.Stage, Severity: "error", Retryable: exploreErr.Retryable,
		Message: exploreErr.Message, NextPage: exploreErr.ExpectedPage,
	})
}

func exploreHTTPStatus(code string) int {
	switch code {
	case "source_unavailable", "session_not_found", "invalid_session", "control_not_found", "invalid_category", "action_not_found":
		return http.StatusNotFound
	case "page_conflict", "page_exhausted":
		return http.StatusConflict
	case "invalid_control_value", "invalid_control_type":
		return http.StatusUnprocessableEntity
	case "unsupported_control_type", "unsupported_capability":
		return http.StatusNotImplemented
	case "result_capacity_exceeded":
		return http.StatusInsufficientStorage
	case "session_create_failed":
		return http.StatusServiceUnavailable
	case "category_cancelled", "rate_limit_cancelled", "capacity_cancelled":
		return http.StatusGatewayTimeout
	case "category_script_failed", "category_parse_failed", "control_action_failed", "action_failed", "request_build_failed",
		"transport_failed", "response_transform_failed", "http_status", "result_rule_failed":
		return http.StatusBadGateway
	default:
		return http.StatusInternalServerError
	}
}
