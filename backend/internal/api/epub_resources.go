package api

import (
	"net/http"
	"net/url"
	"strconv"

	"github.com/otwako/novelreader/internal/library"
)

func (s *readerAPI) epubResourceHref(id string, revision int64, resource string) string {
	return "/api/books/" + url.PathEscape(id) + "/epub-resources/" + url.PathEscape(resource) + "?revision=" + strconv.FormatInt(revision, 10) + "&reader=" + url.QueryEscape(s.coverCacheScope)
}

// The outer reader route owns authentication and the home lease. The reader
// scope prevents URLs cached or retained across logout from selecting another home.
func (s *readerAPI) handleEPUBResource(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("reader") != s.coverCacheScope || s.epubStore == nil {
		writeReadingError(w, library.ErrNotFound)
		return
	}
	revision, err := strconv.ParseInt(r.URL.Query().Get("revision"), 10, 64)
	if err != nil || revision <= 0 {
		writeErrorCode(w, http.StatusBadRequest, "invalid_revision", "a positive content revision is required")
		return
	}
	data, mediaType, err := s.epubStore.ReadResource(r.Context(), r.PathValue("id"), revision, r.PathValue("resource"))
	if err != nil {
		writeReadingError(w, err)
		return
	}
	w.Header().Set("Content-Type", mediaType)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	// Never reuse a response without authorization and current publication checks.
	w.Header().Set("Cache-Control", "private, no-store")
	w.Write(data)
}
