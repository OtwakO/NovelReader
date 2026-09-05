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
	"time"
	"unicode/utf8"

	"github.com/otwako/novelreader/internal/book"
)

func (s *readerAPI) handleListBookmarks(w http.ResponseWriter, r *http.Request) {
	bookID := r.PathValue("id")
	stored, err := s.bookStore.GetBook(bookID)
	if err != nil {
		writeErrorCode(w, http.StatusInternalServerError, "storage_error", "failed to load book")
		return
	}
	if stored == nil {
		writeErrorCode(w, http.StatusNotFound, "book_not_found", "book not found")
		return
	}
	marks, err := s.bookStore.ListBookmarks(bookID)
	if err != nil {
		writeErrorCode(w, http.StatusInternalServerError, "storage_error", "failed to load bookmarks")
		return
	}
	writeJSON(w, http.StatusOK, marks)
}

func (s *readerAPI) handleAddBookmark(w http.ResponseWriter, r *http.Request) {
	bookID := r.PathValue("id")
	var req struct {
		ID           string   `json:"id"`
		SourceID     string   `json:"sourceId"`
		StateVersion *int64   `json:"stateVersion"`
		ChapterIndex *int     `json:"chapterIndex"`
		Position     *float64 `json:"position"`
		Note         string   `json:"note"`
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
	if err := decoder.Decode(&struct{}{}); err != io.EOF || !safeBookmarkID(req.ID) || req.SourceID == "" || req.StateVersion == nil || *req.StateVersion < 0 || req.ChapterIndex == nil || *req.ChapterIndex < 0 || req.Position == nil || math.IsNaN(*req.Position) || math.IsInf(*req.Position, 0) || *req.Position < 0 || *req.Position > 1 || !utf8.ValidString(req.Note) || utf8.RuneCountInString(req.Note) > 1000 {
		writeErrorCode(w, http.StatusBadRequest, "invalid_bookmark", "bookmark fields are required and must be valid")
		return
	}
	stored, err := s.bookStore.GetBook(bookID)
	if err != nil {
		writeErrorCode(w, http.StatusInternalServerError, "storage_error", "failed to load book")
		return
	}
	if stored == nil {
		writeErrorCode(w, http.StatusNotFound, "book_not_found", "book not found")
		return
	}
	chapters, err := s.bookStore.GetChapters(bookID)
	if err != nil {
		writeErrorCode(w, http.StatusInternalServerError, "storage_error", "failed to load chapters")
		return
	}
	chapterTitle := ""
	for _, chapter := range chapters {
		if chapter.Index == *req.ChapterIndex && !chapter.IsVolume {
			chapterTitle = chapter.Title
			break
		}
	}
	if chapterTitle == "" {
		writeErrorCode(w, http.StatusBadRequest, "invalid_bookmark", "chapterIndex is not a readable chapter")
		return
	}
	mark := book.Bookmark{ID: req.ID, BookID: bookID, ChapterIndex: *req.ChapterIndex, ChapterTitle: chapterTitle, Position: *req.Position, Note: strings.TrimSpace(req.Note), CreatedAt: time.Now().UnixMilli()}
	if err := s.bookStore.AddBookmark(mark, req.SourceID, *req.StateVersion); err != nil {
		switch {
		case errors.Is(err, book.ErrBookNotFound):
			writeErrorCode(w, http.StatusNotFound, "book_not_found", "book not found")
		case errors.Is(err, book.ErrBookStateChanged):
			writeErrorCode(w, http.StatusConflict, "state_changed", "book state changed before bookmark was saved")
		case errors.Is(err, book.ErrBookmarkConflict):
			writeErrorCode(w, http.StatusConflict, "bookmark_conflict", "bookmark ID already exists with different content")
		default:
			writeErrorCode(w, http.StatusInternalServerError, "storage_error", "failed to save bookmark")
		}
		return
	}
	marks, err := s.bookStore.ListBookmarks(bookID)
	if err != nil {
		writeErrorCode(w, http.StatusInternalServerError, "storage_error", "bookmark saved but reload failed")
		return
	}
	for _, saved := range marks {
		if saved.ID == mark.ID {
			writeJSON(w, http.StatusCreated, saved)
			return
		}
	}
	writeErrorCode(w, http.StatusInternalServerError, "storage_error", "bookmark saved but not found")
}

func (s *readerAPI) handleDeleteBookmark(w http.ResponseWriter, r *http.Request) {
	if err := s.bookStore.DeleteBookmark(r.PathValue("id"), r.PathValue("bookmarkID")); err != nil {
		if errors.Is(err, book.ErrBookmarkNotFound) {
			writeErrorCode(w, http.StatusNotFound, "bookmark_not_found", "bookmark not found")
			return
		}
		writeErrorCode(w, http.StatusInternalServerError, "storage_error", "failed to delete bookmark")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
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
