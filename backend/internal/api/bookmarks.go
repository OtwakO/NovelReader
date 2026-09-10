// Bookmark handlers validate annotated reader locations against current book state.
package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/otwako/novelreader/internal/library"
)

func (s *readerAPI) handleListBookmarks(w http.ResponseWriter, r *http.Request) {
	bookID := r.PathValue("id")
	stored, err := s.libraryStore.Get(r.Context(), bookID)
	if err != nil {
		writeErrorCode(w, http.StatusInternalServerError, "storage_error", "failed to load book")
		return
	}
	if stored == nil {
		writeErrorCode(w, http.StatusNotFound, "book_not_found", "book not found")
		return
	}
	marks, err := s.libraryStore.GetBookmarks(r.Context(), bookID)
	if err != nil {
		writeErrorCode(w, http.StatusInternalServerError, "storage_error", "failed to load bookmarks")
		return
	}
	writeJSON(w, http.StatusOK, marks)
}

func (s *readerAPI) handleAddBookmark(w http.ResponseWriter, r *http.Request) {
	bookID := r.PathValue("id")
	var req struct {
		ID              string   `json:"id"`
		ContentRevision *int64   `json:"contentRevision"`
		StateVersion    *int64   `json:"stateVersion"`
		ChapterIndex    *int     `json:"chapterIndex"`
		Position        *float64 `json:"position"`
		Note            string   `json:"note"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16384)
	defer r.Body.Close()
	body, err := io.ReadAll(r.Body)
	if err != nil || !utf8.Valid(body) {
		writeErrorCode(w, http.StatusBadRequest, "invalid_bookmark", "invalid bookmark request")
		return
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeErrorCode(w, http.StatusBadRequest, "invalid_bookmark", "invalid bookmark request")
		return
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF || !safeBookmarkID(req.ID) || req.ContentRevision == nil || *req.ContentRevision < 0 || req.StateVersion == nil || *req.StateVersion < 0 || req.ChapterIndex == nil || *req.ChapterIndex < 0 || req.Position == nil || math.IsNaN(*req.Position) || math.IsInf(*req.Position, 0) || *req.Position < 0 || *req.Position > 1 || !utf8.ValidString(req.Note) || utf8.RuneCountInString(req.Note) > 1000 {
		writeErrorCode(w, http.StatusBadRequest, "invalid_bookmark", "bookmark fields are required and must be valid")
		return
	}
	stored, err := s.libraryStore.Get(r.Context(), bookID)
	if err != nil {
		writeErrorCode(w, http.StatusInternalServerError, "storage_error", "failed to load book")
		return
	}
	if stored == nil {
		writeErrorCode(w, http.StatusNotFound, "book_not_found", "book not found")
		return
	}
	if stored.ContentRevision != *req.ContentRevision {
		writeErrorCode(w, http.StatusConflict, "state_changed", "book interpretation changed before bookmark was saved")
		return
	}
	chapter, _, err := s.bookStore.GetChapterWithNext(r.Context(), bookID, *req.ChapterIndex)
	if err != nil {
		writeErrorCode(w, http.StatusInternalServerError, "storage_error", "failed to load chapter")
		return
	}
	if chapter == nil || chapter.IsVolume || chapter.Title == "" {
		writeErrorCode(w, http.StatusBadRequest, "invalid_bookmark", "chapterIndex is not a readable chapter")
		return
	}
	mark := library.Bookmark{ID: req.ID, BookID: bookID, ChapterIndex: *req.ChapterIndex, ChapterTitle: chapter.Title, Position: *req.Position, Note: strings.TrimSpace(req.Note)}
	stateVersion, err := s.libraryStore.AddBookmark(r.Context(), &mark, library.Revision{Content: *req.ContentRevision, State: *req.StateVersion})
	if err != nil {
		switch {
		case errors.Is(err, library.ErrNotFound):
			writeErrorCode(w, http.StatusNotFound, "book_not_found", "book not found")
		case errors.Is(err, library.ErrStateChanged):
			writeErrorCode(w, http.StatusConflict, "state_changed", "book state changed before bookmark was saved")
		case errors.Is(err, library.ErrBookmarkConflict):
			writeErrorCode(w, http.StatusConflict, "bookmark_conflict", "bookmark ID already exists with different content")
		default:
			writeErrorCode(w, http.StatusInternalServerError, "storage_error", "failed to save bookmark")
		}
		return
	}
	writeJSON(w, http.StatusCreated, struct {
		library.Bookmark
		StateVersion int64 `json:"stateVersion"`
	}{Bookmark: mark, StateVersion: stateVersion})
}

func (s *readerAPI) handleDeleteBookmark(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ContentRevision *int64 `json:"contentRevision"`
		StateVersion    *int64 `json:"stateVersion"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeErrorCode(w, http.StatusBadRequest, "invalid_bookmark", "invalid bookmark deletion")
		return
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF || req.ContentRevision == nil || *req.ContentRevision < 0 || req.StateVersion == nil || *req.StateVersion < 0 {
		writeErrorCode(w, http.StatusBadRequest, "invalid_bookmark", "contentRevision and stateVersion are required")
		return
	}
	version, err := s.libraryStore.DeleteBookmark(r.Context(), r.PathValue("id"), r.PathValue("bookmarkID"), library.Revision{Content: *req.ContentRevision, State: *req.StateVersion})
	if err != nil {
		switch {
		case errors.Is(err, library.ErrNotFound):
			writeErrorCode(w, http.StatusNotFound, "bookmark_not_found", "bookmark not found")
		case errors.Is(err, library.ErrStateChanged):
			writeErrorCode(w, http.StatusConflict, "state_changed", "book state changed before bookmark was deleted")
		default:
			writeErrorCode(w, http.StatusInternalServerError, "storage_error", "failed to delete bookmark")
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"status": "deleted", "stateVersion": version})
}

func safeBookmarkID(id string) bool {
	if len(id) == 0 || len(id) > 64 {
		return false
	}
	for _, r := range id {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_') {
			return false
		}
	}
	return true
}
