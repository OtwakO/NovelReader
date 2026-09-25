package api

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"

	"github.com/otwako/novelreader/internal/book"
	"github.com/otwako/novelreader/internal/library"
	"github.com/otwako/novelreader/internal/reading"
)

func (s *readerAPI) handleGetChapterImage(w http.ResponseWriter, r *http.Request) {
	chapterIndex, err := strconv.Atoi(r.PathValue("idx"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid chapter index")
		return
	}
	imageIndex, err := strconv.Atoi(r.PathValue("imageIdx"))
	if err != nil || imageIndex < 0 {
		writeError(w, http.StatusBadRequest, "invalid image index")
		return
	}
	revision, err := strconv.ParseInt(r.URL.Query().Get("contentRevision"), 10, 64)
	if err != nil || revision < 0 {
		writeError(w, http.StatusBadRequest, "contentRevision is required")
		return
	}
	data, contentType, err := s.reading.BookSource.Image(r.Context(), r.PathValue("id"), revision, chapterIndex, imageIndex, r.URL.Query().Get("bundle"))
	if err != nil {
		switch {
		case errors.Is(err, book.ErrUnsupportedImageDecoder):
			writeErrorCode(w, http.StatusNotImplemented, "chapter_image_decoder_unsupported", "chapter image decoder requires unsupported Android bitmap operations")
		case errors.Is(err, library.ErrStateChanged):
			writeErrorCode(w, http.StatusConflict, "state_changed", "book interpretation or source changed")
		case errors.Is(err, reading.ErrImageUnavailable), errors.Is(err, library.ErrNotFound), errors.Is(err, reading.ErrChapterNotFound), errors.Is(err, reading.ErrSourceNotFound):
			writeErrorCode(w, http.StatusNotFound, "chapter_image_not_found", "chapter image unavailable; reload the chapter")
		default:
			writeErrorCode(w, http.StatusBadGateway, "chapter_image_fetch_failed", "chapter image unavailable")
		}
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "private, no-cache")
	w.Header().Set("Content-Security-Policy", "sandbox; default-src 'none'")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (s *readerAPI) chapterImageHref(bookID string, contentRevision int64, chapterIndex, imageIndex int, bundleID string) string {
	href := "/api/books/" + url.PathEscape(bookID) + "/chapters/" + strconv.Itoa(chapterIndex) + "/images/" + strconv.Itoa(imageIndex) + "?contentRevision=" + strconv.FormatInt(contentRevision, 10)
	href += "&bundle=" + url.QueryEscape(bundleID)
	if s.home != nil {
		href += "&readerGeneration=" + url.QueryEscape(s.home.Generation())
	}
	return href
}
