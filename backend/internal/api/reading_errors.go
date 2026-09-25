package api

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/otwako/novelreader/internal/epubstore"
	"github.com/otwako/novelreader/internal/library"
	"github.com/otwako/novelreader/internal/reading"
	"github.com/otwako/novelreader/internal/txtstore"
)

func writeReadingError(w http.ResponseWriter, err error) {
	var crawl *reading.CrawlError
	switch {
	case errors.Is(err, library.ErrNotFound), errors.Is(err, txtstore.ErrNotFound), errors.Is(err, epubstore.ErrNotFound):
		writeErrorCode(w, http.StatusNotFound, "book_not_found", "book not found")
	case errors.Is(err, library.ErrStateChanged), errors.Is(err, epubstore.ErrStateChanged):
		writeErrorCode(w, http.StatusConflict, "state_changed", "book interpretation or reading state changed")
	case errors.Is(err, reading.ErrChapterNotFound):
		writeErrorCode(w, http.StatusNotFound, "chapter_not_found", "chapter not found")
	case errors.Is(err, reading.ErrSourceNotFound):
		writeErrorCode(w, http.StatusNotFound, "source_not_found", "source not found")
	case errors.Is(err, reading.ErrUnsupportedProvider):
		writeErrorCode(w, http.StatusNotImplemented, "provider_not_supported", "reading is not available for this publication provider")
	case errors.Is(err, reading.ErrImageUnavailable):
		writeErrorCode(w, http.StatusServiceUnavailable, "chapter_images_unavailable", "chapter images could not be prepared; retry loading the chapter")
	case errors.As(err, &crawl):
		writeCrawlError(w, crawl.Stage, crawl.Err)
	default:
		slog.Warn("reading storage operation failed", "error", err)
		writeErrorCode(w, http.StatusInternalServerError, "storage_error", "reading storage unavailable")
	}
}
