package api

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/otwako/novelreader/internal/book"
	"github.com/otwako/novelreader/internal/processor"
)

func (s *readerAPI) handleGetChapterImage(w http.ResponseWriter, r *http.Request) {
	if s.bookStore == nil || s.sourceStore == nil || s.searcher == nil {
		writeError(w, http.StatusServiceUnavailable, "chapter image service unavailable")
		return
	}
	storedBook, err := s.bookStore.GetBook(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "load book failed")
		return
	}
	if storedBook == nil {
		writeErrorCode(w, http.StatusNotFound, "book_not_found", "book not found")
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
	chapters, err := s.bookStore.GetChapters(storedBook.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "load chapters failed")
		return
	}
	var chapter *book.Chapter
	for index := range chapters {
		if chapters[index].Index == chapterIndex {
			chapter = &chapters[index]
			break
		}
	}
	if chapter == nil {
		writeErrorCode(w, http.StatusNotFound, "chapter_not_found", "chapter not found")
		return
	}
	cached, err := s.bookStore.GetChapterCache(storedBook.ID, storedBook.SourceID, chapter.Index, chapter.URL)
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
