package api

import (
	"net/http"
	"strconv"

	"github.com/otwako/novelreader/internal/library"
	"github.com/otwako/novelreader/internal/txt"
	"github.com/otwako/novelreader/internal/txtstore"
)

type txtOptionsResponse struct {
	Encoding txt.Encoding `json:"encoding"`
	Preset   txt.Preset   `json:"preset"`
	Pattern  string       `json:"pattern,omitempty"`
}

func txtOptionsDTO(value txt.Options) txtOptionsResponse {
	return txtOptionsResponse{value.Encoding, value.Preset, value.Pattern}
}

type txtCandidateResponse struct {
	Generation          int64              `json:"generation"`
	State               txtstore.State     `json:"state"`
	Options             txtOptionsResponse `json:"options"`
	BaseContentRevision int64              `json:"baseContentRevision"`
	HasError            bool               `json:"hasError"`
	ErrorCode           string             `json:"errorCode,omitempty"`
}
type txtReparseResponse struct {
	Name             string                `json:"name"`
	ActiveGeneration int64                 `json:"activeGeneration"`
	ContentRevision  int64                 `json:"contentRevision"`
	StateVersion     int64                 `json:"stateVersion"`
	ActiveOptions    txtOptionsResponse    `json:"activeOptions"`
	Candidate        *txtCandidateResponse `json:"candidate,omitempty"`
}

func (s *readerAPI) registerTXTReparseRoutes() {
	s.mux.HandleFunc("GET /api/books/{id}/txt/reparse", txtControlHandler(s.handleTXTReparseStatus))
	s.mux.HandleFunc("POST /api/books/{id}/txt/reparse", txtControlHandler(s.handlePrepareTXTReparse))
	s.mux.HandleFunc("DELETE /api/books/{id}/txt/reparse", txtControlHandler(s.handleDiscardTXTReparse))
	s.mux.HandleFunc("GET /api/books/{id}/txt/reparse/preview", txtControlHandler(s.handlePreviewTXTReparse))
	s.mux.HandleFunc("GET /api/books/{id}/txt/reparse/impact", txtControlHandler(s.handleTXTReparseImpact))
	s.mux.HandleFunc("POST /api/books/{id}/txt/reparse/apply", txtControlHandler(s.handleApplyTXTReparse))
}
func (s *readerAPI) handleTXTReparseStatus(w http.ResponseWriter, r *http.Request) {
	value, err := s.txtStore.ReparseStatus(r.Context(), r.PathValue("id"))
	if err != nil {
		writeTXTError(w, err)
		return
	}
	response := txtReparseResponse{Name: value.Name, ActiveGeneration: value.ActiveGeneration, ContentRevision: value.ContentRevision, StateVersion: value.StateVersion, ActiveOptions: txtOptionsDTO(value.ActiveOptions)}
	if candidate := value.Candidate; candidate != nil {
		response.Candidate = &txtCandidateResponse{candidate.Generation, candidate.State, txtOptionsDTO(candidate.Options), candidate.BaseContentRevision, candidate.Error != "", txtAnalysisErrorCode(candidate.Error)}
	}
	writeJSON(w, http.StatusOK, response)
}
func (s *readerAPI) handlePrepareTXTReparse(w http.ResponseWriter, r *http.Request) {
	var input struct {
		ContentRevision int64  `json:"contentRevision"`
		Generation      *int64 `json:"generation"`
		txtOptionsResponse
	}
	if !decodeTXTRequest(w, r, &input) {
		return
	}
	if input.ContentRevision < 1 || input.Generation == nil || *input.Generation < 0 {
		writeErrorCode(w, 400, "txt_invalid_input", "Expected current content revision and candidate generation (zero for absence)")
		return
	}
	id := r.PathValue("id")
	generation, err := s.txtStore.QueueReparse(r.Context(), id, input.ContentRevision, *input.Generation, txt.Options{Encoding: input.Encoding, Preset: input.Preset, Pattern: input.Pattern})
	if err != nil {
		writeTXTError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, struct {
		Generation int64    `json:"generation"`
		Warnings   []string `json:"warnings,omitempty"`
	}{generation, wakeTXTAnalysis(s.fileImports, s.home.ID(), id)})
}
func (s *readerAPI) handleDiscardTXTReparse(w http.ResponseWriter, r *http.Request) {
	var input struct {
		ContentRevision int64 `json:"contentRevision"`
		Generation      int64 `json:"generation"`
	}
	if !decodeTXTRequest(w, r, &input) {
		return
	}
	if input.ContentRevision < 1 || input.Generation < 1 {
		writeErrorCode(w, 400, "txt_invalid_input", "Expected current content revision and candidate generation")
		return
	}
	if err := s.txtStore.DiscardReparse(r.Context(), r.PathValue("id"), input.ContentRevision, input.Generation); err != nil {
		writeTXTError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "discarded"})
}
func (s *readerAPI) handlePreviewTXTReparse(w http.ResponseWriter, r *http.Request) {
	generation, err := strconv.ParseInt(r.URL.Query().Get("generation"), 10, 64)
	start := 0
	if r.URL.Query().Has("start") {
		var startErr error
		start, startErr = strconv.Atoi(r.URL.Query().Get("start"))
		if startErr != nil {
			start = -1
		}
	}
	limit, limitErr := txtPageLimit(r)
	if err != nil || generation < 1 || start < 0 || limitErr != nil {
		writeErrorCode(w, 400, "txt_invalid_input", "Expected a candidate generation and bounded preview page")
		return
	}
	value, err := s.txtStore.ReviewReparse(r.Context(), r.PathValue("id"), generation, start, limit)
	if err != nil {
		writeTXTError(w, err)
		return
	}
	writeTXTPreview(w, value)
}
func (s *readerAPI) handleTXTReparseImpact(w http.ResponseWriter, r *http.Request) {
	generation, err := strconv.ParseInt(r.URL.Query().Get("generation"), 10, 64)
	if err != nil || generation < 1 {
		writeErrorCode(w, 400, "txt_invalid_input", "Expected a candidate generation")
		return
	}
	value, err := s.txtStore.ReparseImpact(r.Context(), r.PathValue("id"), generation)
	if err != nil {
		writeTXTError(w, err)
		return
	}
	type location struct {
		ChapterIndex int     `json:"chapterIndex"`
		ChapterTitle string  `json:"chapterTitle"`
		Position     float64 `json:"position"`
	}
	var resume *location
	if value.Resume != nil {
		resume = &location{value.Resume.ChapterIndex, value.Resume.ChapterTitle, value.Resume.Position}
	}
	writeJSON(w, http.StatusOK, struct {
		Generation          int64     `json:"generation"`
		ActiveGeneration    int64     `json:"activeGeneration"`
		ContentRevision     int64     `json:"contentRevision"`
		StateVersion        int64     `json:"stateVersion"`
		TotalSections       int       `json:"totalSections"`
		Resume              *location `json:"resume"`
		PreservedBookmarks  int       `json:"preservedBookmarks"`
		UnresolvedBookmarks int       `json:"unresolvedBookmarks"`
	}{value.Generation, value.ActiveGeneration, value.ContentRevision, value.StateVersion, value.TotalSections, resume, value.PreservedBookmarks, value.UnresolvedBookmarks})
}
func (s *readerAPI) handleApplyTXTReparse(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Generation       int64  `json:"generation"`
		ActiveGeneration int64  `json:"activeGeneration"`
		ContentRevision  int64  `json:"contentRevision"`
		StateVersion     *int64 `json:"stateVersion"`
		ResumeChapter    *int   `json:"resumeChapter"`
	}
	if !decodeTXTRequest(w, r, &input) {
		return
	}
	if input.Generation < 1 || input.ActiveGeneration < 1 || input.ContentRevision < 1 || input.StateVersion == nil || *input.StateVersion < 0 {
		writeErrorCode(w, 400, "txt_invalid_input", "Expected reviewed interpretation and reading-state versions")
		return
	}
	result, err := s.txtStore.ApplyReparse(r.Context(), r.PathValue("id"), txtstore.ApplyReparseRequest{Generation: input.Generation, ActiveGeneration: input.ActiveGeneration, Expected: library.Revision{Content: input.ContentRevision, State: *input.StateVersion}, ResumeChapter: input.ResumeChapter})
	if err != nil {
		writeTXTError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, struct {
		LibraryID       string `json:"libraryId"`
		ContentRevision int64  `json:"contentRevision"`
		StateVersion    int64  `json:"stateVersion"`
		AlreadyApplied  bool   `json:"alreadyApplied"`
	}{result.Item.ID, result.Item.ContentRevision, result.Item.StateVersion, result.AlreadyApplied})
}
