package api

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"

	"github.com/otwako/novelreader/internal/book"
	"github.com/otwako/novelreader/internal/processor"
)

func (s *readerAPI) handleGetChapterImage(w http.ResponseWriter, r *http.Request) {
	if s.bookStore == nil || s.sourceStore == nil || s.searcher == nil {
		writeError(w, http.StatusServiceUnavailable, "chapter image service unavailable")
		return
	}
	chapterIndex, err := strconv.Atoi(r.PathValue("idx"))
	if err != nil {
		writeErrorCode(w, http.StatusNotFound, "chapter_not_found", "chapter not found")
		return
	}
	imageIndex, err := strconv.Atoi(r.PathValue("imageIdx"))
	if err != nil || imageIndex < 0 {
		writeErrorCode(w, http.StatusNotFound, "chapter_image_not_found", "chapter image not found")
		return
	}
	storedBook, chapter, _, err := s.bookStore.GetChapterSnapshot(r.Context(), r.PathValue("id"), chapterIndex)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "load chapters failed")
		return
	}
	if storedBook == nil {
		writeErrorCode(w, http.StatusNotFound, "book_not_found", "book not found")
		return
	}
	revision, err := strconv.ParseInt(r.URL.Query().Get("contentRevision"), 10, 64)
	if err != nil || revision < 0 {
		writeError(w, http.StatusBadRequest, "contentRevision is required")
		return
	}
	if revision != storedBook.ContentRevision {
		writeErrorCode(w, http.StatusConflict, "state_changed", "book interpretation changed")
		return
	}
	if chapter == nil {
		writeErrorCode(w, http.StatusNotFound, "chapter_not_found", "chapter not found")
		return
	}
	cached, err := s.bookStore.GetChapterCache(storedBook.ID, storedBook.SourceID, chapter.Index, chapter.URL, storedBook.ContentRevision)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "load chapter images failed")
		return
	}
	imageURL := cachedImageURL(cached, imageIndex)
	if imageURL == "" {
		writeErrorCode(w, http.StatusNotFound, "chapter_image_not_found", "chapter image not found")
		return
	}
	source, err := s.sourceStore.GetByID(storedBook.SourceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "load source failed")
		return
	}
	if source == nil {
		writeErrorCode(w, http.StatusNotFound, "source_not_found", "book source not found")
		return
	}
	data, contentType, err := s.searcher.GetChapterImage(r.Context(), *source, storedBook, chapter, imageURL)
	if errors.Is(err, book.ErrUnsupportedImageDecoder) {
		writeErrorCode(w, http.StatusNotImplemented, "chapter_image_decoder_unsupported", "chapter image decoder requires unsupported Android bitmap operations")
		return
	}
	if err != nil {
		writeErrorCode(w, http.StatusBadGateway, "chapter_image_fetch_failed", "chapter image unavailable")
		return
	}
	if !s.validateChapterSnapshot(w, r, storedBook) {
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "private, no-cache")
	w.Header().Set("Content-Security-Policy", "sandbox; default-src 'none'")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func cachedImageURL(cached *book.CachedChapter, requested int) string {
	if cached == nil {
		return ""
	}
	imageIndex := 0
	for _, block := range cached.Blocks {
		if block.Kind != processor.ProseBlockImage {
			continue
		}
		if imageIndex == requested {
			return block.Src
		}
		imageIndex++
	}
	return ""
}

func (s *readerAPI) chapterImageHref(bookID string, contentRevision int64, chapterIndex, imageIndex int) string {
	href := "/api/books/" + url.PathEscape(bookID) + "/chapters/" + strconv.Itoa(chapterIndex) + "/images/" + strconv.Itoa(imageIndex) + "?contentRevision=" + strconv.FormatInt(contentRevision, 10)
	if s.home != nil {
		href += "&readerGeneration=" + url.QueryEscape(s.home.Generation())
	}
	return href
}

func (s *readerAPI) validateChapterSnapshot(w http.ResponseWriter, r *http.Request, snapshot *book.Book) bool {
	current, err := s.bookStore.IsChapterSnapshotCurrent(r.Context(), snapshot)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "validate chapter interpretation failed")
		return false
	}
	if !current {
		writeErrorCode(w, http.StatusConflict, "state_changed", "book interpretation changed while loading content")
		return false
	}
	return true
}
