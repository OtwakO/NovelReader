// Package api provides the HTTP REST API for NovelReader.
package api

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/otwako/novelreader/internal/book"
	"github.com/otwako/novelreader/internal/booksource"
	"github.com/otwako/novelreader/internal/fontstore"
	"github.com/otwako/novelreader/internal/processor"
	"github.com/otwako/novelreader/internal/sourceinteraction"
	"github.com/otwako/novelreader/internal/sourceprofile"
)

func (s *readerAPI) deleteSourceSession(sourceID string) {
	if s.searcher != nil {
		s.searcher.DeleteSourceSession(sourceID)
	}
}

// --- Book Sources ---

func (s *readerAPI) handleListSources(w http.ResponseWriter, r *http.Request) {
	sources, err := s.sourceStore.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if sources == nil {
		sources = []booksource.BookSource{}
	}
	writeJSON(w, http.StatusOK, sourceManagementSummaries(sources))
}

func (s *readerAPI) handleGetSource(w http.ResponseWriter, r *http.Request) {
	source, err := s.sourceStore.GetByID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if source == nil {
		writeError(w, http.StatusNotFound, "source not found")
		return
	}
	response, err := sourceManagementResponseFor(*source)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, response)
}

type sourcePreferenceUpdateRequest struct {
	Enabled        *bool `json:"enabled"`
	EnabledExplore *bool `json:"enabledExplore"`
}

func (s *readerAPI) handleUpdateSourcePreferences(w http.ResponseWriter, r *http.Request) {
	var request sourcePreferenceUpdateRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid source preference update")
		return
	}
	if request.Enabled == nil && request.EnabledExplore == nil {
		writeError(w, http.StatusBadRequest, "source preference update is empty")
		return
	}
	source, err := s.sourceStore.UpdatePreferences(r.PathValue("id"), request.Enabled, request.EnabledExplore)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if source == nil {
		writeError(w, http.StatusNotFound, "source not found")
		return
	}
	s.closeSourceRuntime(source.ID)
	writeJSON(w, http.StatusOK, sourceManagementSummaries([]booksource.BookSource{*source})[0])
}

func (s *readerAPI) handleImportSources(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 50*1024*1024))
	if err != nil {
		writeError(w, http.StatusBadRequest, "read body: "+err.Error())
		return
	}
	defer r.Body.Close()

	sources, err := booksource.ImportSources(body)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	count, err := s.sourceStore.ImportBatch(sources)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"imported": count,
		"total":    len(sources),
	})
}

func (s *readerAPI) closeSourceRuntime(sourceID string) {
	invalidateSourceRuntime(s.searcher, s.browserSessions, sourceID)
}

func (s *readerAPI) handleDeleteSource(w http.ResponseWriter, r *http.Request) {
	sourceID := r.URL.Query().Get("id")
	if sourceID == "" {
		writeError(w, http.StatusBadRequest, "missing query param id")
		return
	}
	if err := deleteSourceDefinition(r.Context(), s.sourceStore, s.sourceProfiles, s.closeSourceRuntime, sourceID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (s *readerAPI) handleSourceInteraction(w http.ResponseWriter, r *http.Request) {
	if s.sourceInteractions == nil {
		writeError(w, http.StatusNotImplemented, "source interaction is unavailable")
		return
	}
	view, err := s.sourceInteractions.Describe(r.Context(), r.PathValue("id"))
	if err != nil {
		if errors.Is(err, sourceprofile.ErrSourceNotInstalled) {
			writeError(w, http.StatusNotFound, "book source not found")
			return
		}
		writeSourceInteractionError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (s *readerAPI) handleSourceInteractionAction(w http.ResponseWriter, r *http.Request) {
	if s.sourceInteractions == nil {
		writeError(w, http.StatusNotImplemented, "source interaction is unavailable")
		return
	}
	var request sourceinteraction.ActionRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 256*1024)).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid action request")
		return
	}
	result, err := s.sourceInteractions.Act(r.Context(), r.PathValue("id"), request)
	if err != nil {
		switch {
		case errors.Is(err, sourceprofile.ErrSourceNotInstalled):
			writeError(w, http.StatusNotFound, "book source not found")
		case errors.Is(err, sourceinteraction.ErrStaleRevision):
			writeError(w, http.StatusConflict, err.Error())
		case errors.Is(err, sourceinteraction.ErrActionNotFound):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeSourceInteractionError(w, err)
		}
		return
	}
	s.deleteSourceSession(r.PathValue("id"))
	result.Effects = sourceinteraction.RegisterBrowserRequests(result.Effects, s.browserSessions)
	writeJSON(w, http.StatusOK, result)
}

func writeSourceInteractionError(w http.ResponseWriter, err error) {
	var executionErr *sourceinteraction.ExecutionError
	if errors.As(err, &executionErr) {
		slog.Warn("api: source interaction failed", "code", executionErr.Code, "classification", executionErr.Classification)
		writeJSON(w, http.StatusBadGateway, map[string]interface{}{
			"code": executionErr.Code, "classification": executionErr.Classification, "error": executionErr.Message,
		})
		return
	}
	writeError(w, http.StatusUnprocessableEntity, err.Error())
}

func (s *readerAPI) handleSourceInteractionResetLogin(w http.ResponseWriter, r *http.Request) {
	s.handleSourceInteractionReset(w, r, sourceinteraction.ResetLogin)
}

func (s *readerAPI) handleSourceInteractionResetSettings(w http.ResponseWriter, r *http.Request) {
	s.handleSourceInteractionReset(w, r, sourceinteraction.ResetSettings)
}

func (s *readerAPI) handleSourceInteractionResetAll(w http.ResponseWriter, r *http.Request) {
	s.handleSourceInteractionReset(w, r, sourceinteraction.ResetAll)
}

func (s *readerAPI) handleSourceInteractionReset(w http.ResponseWriter, r *http.Request, scope sourceinteraction.ResetScope) {
	if s.sourceInteractions == nil {
		writeError(w, http.StatusNotImplemented, "source interaction is unavailable")
		return
	}
	view, err := s.sourceInteractions.Reset(r.Context(), r.PathValue("id"), scope)
	if err != nil {
		if errors.Is(err, sourceprofile.ErrSourceNotInstalled) {
			writeError(w, http.StatusNotFound, "book source not found")
			return
		}
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	s.closeSourceRuntime(r.PathValue("id"))
	writeJSON(w, http.StatusOK, view)
}

func (s *readerAPI) handleUpdateSource(w http.ResponseWriter, r *http.Request) {
	sourceID := r.URL.Query().Get("id")
	if sourceID == "" {
		writeError(w, http.StatusBadRequest, "missing query param id")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, booksource.MaxCollectionDocumentBytes)
	defer r.Body.Close()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeError(w, http.StatusRequestEntityTooLarge, "source document exceeds 50 MiB")
		} else {
			writeError(w, http.StatusBadRequest, err.Error())
		}
		return
	}

	src, err := booksource.NewFromJSON(body)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	src.ID = sourceID
	if err := s.sourceStore.Upsert(src); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.closeSourceRuntime(sourceID)
	writeJSON(w, http.StatusOK, src)
}

// --- Books ---

func (s *readerAPI) handleListBooks(w http.ResponseWriter, r *http.Request) {
	books, err := s.bookStore.ListBooks()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if books == nil {
		books = []book.Book{}
	}
	s.addStoredCoverDisplayURLs(books)
	writeJSON(w, http.StatusOK, books)
}

func (s *readerAPI) handleGetBook(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	b, err := s.bookStore.GetBook(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "load book failed")
		return
	}
	if b == nil {
		writeErrorCode(w, http.StatusNotFound, "book_not_found", "book not found")
		return
	}
	s.addStoredCoverDisplayURL(b)
	writeJSON(w, http.StatusOK, b)
}

func (s *readerAPI) handleGetBookCover(w http.ResponseWriter, r *http.Request) {
	if s.bookStore == nil || s.sourceStore == nil || s.searcher == nil {
		writeError(w, http.StatusServiceUnavailable, "cover service unavailable")
		return
	}
	b, err := s.bookStore.GetBook(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "load book failed")
		return
	}
	if b == nil {
		writeErrorCode(w, http.StatusNotFound, "book_not_found", "book not found")
		return
	}
	if b.CoverURL == "" {
		writeErrorCode(w, http.StatusNotFound, "cover_not_found", "book cover not found")
		return
	}
	src, err := s.sourceStore.GetByID(b.SourceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "load source failed")
		return
	}
	if src == nil {
		writeErrorCode(w, http.StatusNotFound, "source_not_found", "book source not found")
		return
	}
	data, contentType, err := s.searcher.GetBookCover(r.Context(), *src, b)
	if err != nil {
		slog.Warn("cover: fetch failed", "bookId", b.ID, "source", b.SourceURL, "err", err)
		writeErrorCode(w, http.StatusBadGateway, "cover_fetch_failed", "book cover unavailable")
		return
	}
	writeCoverBytes(w, data, contentType)
}

func (s *readerAPI) handleMergeBookSources(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Sources []book.AltSource `json:"sources"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 256<<10)
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeErrorCode(w, http.StatusBadRequest, "invalid_book_sources", "invalid book sources request")
		return
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF || len(req.Sources) == 0 {
		writeErrorCode(w, http.StatusBadRequest, "invalid_book_sources", "at least one source is required")
		return
	}
	stored, err := s.bookStore.MergeBookSources(r.PathValue("id"), req.Sources)
	if errors.Is(err, book.ErrBookNotFound) {
		writeErrorCode(w, http.StatusNotFound, "book_not_found", "book not found")
		return
	}
	if err != nil {
		writeErrorCode(w, http.StatusInternalServerError, "storage_error", "failed to merge book sources")
		return
	}
	s.addStoredCoverDisplayURL(stored)
	writeJSON(w, http.StatusOK, stored)
}

func (s *readerAPI) handleClearBookSources(w http.ResponseWriter, r *http.Request) {
	stored, err := s.bookStore.ClearBookSources(r.PathValue("id"))
	if errors.Is(err, book.ErrBookNotFound) {
		writeErrorCode(w, http.StatusNotFound, "book_not_found", "book not found")
		return
	}
	if err != nil {
		writeErrorCode(w, http.StatusInternalServerError, "storage_error", "failed to clear book sources")
		return
	}
	s.addStoredCoverDisplayURL(stored)
	writeJSON(w, http.StatusOK, stored)
}

func (s *readerAPI) handleDeleteBook(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if s.catalogs != nil {
		s.catalogs.Invalidate(id)
	}
	if err := s.bookStore.DeleteBook(id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// --- Search ---

// --- Chapters ---

func (s *readerAPI) handleGetChapters(w http.ResponseWriter, r *http.Request) {
	if s.catalogs == nil {
		writeErrorCode(w, http.StatusServiceUnavailable, "catalog_unavailable", "catalog synchronization is unavailable")
		return
	}
	writeCatalogResult(w, s.catalogs.Get(r.PathValue("id")))
}

func (s *readerAPI) handleRetryChapters(w http.ResponseWriter, r *http.Request) {
	if s.catalogs == nil {
		writeErrorCode(w, http.StatusServiceUnavailable, "catalog_unavailable", "catalog synchronization is unavailable")
		return
	}
	writeCatalogResult(w, s.catalogs.Retry(r.PathValue("id")))
}

func writeCatalogResult(w http.ResponseWriter, result book.CatalogResult) {
	switch result.State {
	case book.CatalogReady:
		writeJSON(w, http.StatusOK, result.Chapters)
	case book.CatalogSyncing:
		w.Header().Set("Retry-After", "1")
		writeJSON(w, http.StatusAccepted, map[string]string{"state": string(book.CatalogSyncing)})
	case book.CatalogFailed:
		switch result.Failure {
		case book.CatalogFailureBookNotFound:
			writeErrorCode(w, http.StatusNotFound, "book_not_found", "book not found")
		case book.CatalogFailureSourceNotFound:
			writeErrorCode(w, http.StatusNotFound, "source_not_found", "source not found")
		case book.CatalogFailureStorage:
			writeErrorCode(w, http.StatusInternalServerError, "storage_error", "catalog storage unavailable")
		case book.CatalogFailureUpstream:
			if result.Err != nil {
				writeCrawlError(w, "toc", result.Err)
				return
			}
			writeErrorCode(w, http.StatusBadGateway, "catalog_sync_failed", "catalog synchronization failed")
		default:
			writeErrorCode(w, http.StatusBadGateway, "catalog_sync_failed", "catalog synchronization failed")
		}
	default:
		writeErrorCode(w, http.StatusInternalServerError, "catalog_state_invalid", "invalid catalog synchronization state")
	}
}

func (s *readerAPI) handleGetChapterContent(w http.ResponseWriter, r *http.Request) {
	bookID := r.PathValue("id")
	idx := r.PathValue("idx")

	b, err := s.bookStore.GetBook(bookID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "load book failed")
		return
	}
	if b == nil {
		writeErrorCode(w, http.StatusNotFound, "book_not_found", "book not found")
		return
	}

	chapters, err := s.bookStore.GetChapters(bookID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "load chapters failed")
		return
	}

	var ch *book.Chapter
	for i := range chapters {
		if fmt.Sprint(chapters[i].Index) == idx {
			ch = &chapters[i]
			break
		}
	}
	if ch == nil {
		writeErrorCode(w, http.StatusNotFound, "chapter_not_found", "chapter not found")
		return
	}

	src, err := s.sourceStore.GetByID(b.SourceID)
	if err != nil {
		if s.writeChapterCacheFallback(w, b, ch) {
			return
		}
		writeError(w, http.StatusInternalServerError, "load source failed")
		return
	}
	if src == nil {
		if s.writeChapterCacheFallback(w, b, ch) {
			return
		}
		writeErrorCode(w, http.StatusNotFound, "source_not_found", "source not found")
		return
	}

	var next *book.Chapter
	for i := range chapters {
		if &chapters[i] == ch && i+1 < len(chapters) {
			next = &chapters[i+1]
			break
		}
	}
	rawContent, contentTitle, err := s.searcher.GetChapterContentForBookContext(r.Context(), *src, b, ch, next)
	if err == nil && strings.TrimSpace(rawContent) == "" {
		err = errors.New("content: empty extraction")
	}
	if err != nil {
		if s.writeChapterCacheFallback(w, b, ch) {
			return
		}
		writeCrawlError(w, "content", err)
		return
	}

	displayTitle := ch.Title
	if contentTitle != "" {
		displayTitle = contentTitle
	}

	proc := processor.New(s.processorCfg)
	result := proc.Process(displayTitle, rawContent)
	s.saveChapterCache(b, ch, result)

	writeJSON(w, http.StatusOK, newChapterContentResponse(b.ID, ch.Index, result.Title, result.Paragraphs, result.Blocks, false))
}

// --- Progress ---

func (s *readerAPI) handleUpdateProgress(w http.ResponseWriter, r *http.Request) {
	bookID := r.PathValue("id")
	var req struct {
		SourceID     *string  `json:"sourceId"`
		StateVersion *int64   `json:"stateVersion"`
		ChapterIndex *int     `json:"chapterIndex"`
		Position     *float64 `json:"position"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeErrorCode(w, http.StatusBadRequest, "invalid_progress", "invalid progress request")
		return
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		writeErrorCode(w, http.StatusBadRequest, "invalid_progress", "invalid progress request")
		return
	}
	if req.SourceID == nil || *req.SourceID == "" || req.StateVersion == nil || *req.StateVersion < 0 || req.ChapterIndex == nil || req.Position == nil || *req.ChapterIndex < 0 || math.IsNaN(*req.Position) || math.IsInf(*req.Position, 0) || *req.Position < 0 || *req.Position > 1 {
		writeErrorCode(w, http.StatusBadRequest, "invalid_progress", "sourceId, stateVersion, chapterIndex, and position are required and must be valid")
		return
	}
	chapterIndex, position := *req.ChapterIndex, *req.Position
	storedBook, err := s.bookStore.GetBook(bookID)
	if err != nil {
		writeErrorCode(w, http.StatusInternalServerError, "storage_error", "failed to load book")
		return
	}
	if storedBook == nil {
		writeErrorCode(w, http.StatusNotFound, "book_not_found", "book not found")
		return
	}
	if storedBook.SourceID != *req.SourceID || storedBook.StateVersion != *req.StateVersion {
		writeErrorCode(w, http.StatusConflict, "state_changed", "book state changed before progress was saved")
		return
	}
	chapters, err := s.bookStore.GetChapters(bookID)
	if err != nil {
		writeErrorCode(w, http.StatusInternalServerError, "storage_error", "failed to load chapters")
		return
	}
	validChapter := false
	for _, chapter := range chapters {
		if chapter.Index == chapterIndex && !chapter.IsVolume {
			validChapter = true
			break
		}
	}
	if !validChapter {
		writeErrorCode(w, http.StatusBadRequest, "invalid_progress", "chapterIndex is not a readable chapter")
		return
	}
	stateVersion, err := s.bookStore.UpdateProgress(bookID, *req.SourceID, *req.StateVersion, chapterIndex, position)
	if err != nil {
		if errors.Is(err, book.ErrBookNotFound) {
			writeErrorCode(w, http.StatusNotFound, "book_not_found", "book not found")
			return
		}
		if errors.Is(err, book.ErrBookStateChanged) {
			writeErrorCode(w, http.StatusConflict, "state_changed", "book state changed before progress was saved")
			return
		}
		writeErrorCode(w, http.StatusInternalServerError, "storage_error", "failed to save progress")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"status": "saved", "stateVersion": stateVersion})
}

func (s *readerAPI) handleSwitchSource(w http.ResponseWriter, r *http.Request) {
	bookID := r.PathValue("id")
	var req struct {
		SourceID   string `json:"sourceId"`
		SourceURL  string `json:"sourceUrl"`
		BookURL    string `json:"bookUrl"`
		SourceName string `json:"sourceName,omitempty"` // accepted for older clients; the imported source is authoritative
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeErrorCode(w, http.StatusBadRequest, "invalid_source_switch", "invalid source switch request")
		return
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF || req.SourceID == "" || req.SourceURL == "" || req.BookURL == "" {
		writeErrorCode(w, http.StatusBadRequest, "invalid_source_switch", "sourceId, sourceUrl, and bookUrl are required")
		return
	}

	current, err := s.bookStore.GetBook(bookID)
	if err != nil {
		writeErrorCode(w, http.StatusInternalServerError, "storage_error", "failed to load book")
		return
	}
	if current == nil {
		writeErrorCode(w, http.StatusNotFound, "book_not_found", "book not found")
		return
	}
	isAlternate := false
	for _, alternate := range current.AlternateSources {
		if alternate.SourceID == req.SourceID && alternate.BookURL == req.BookURL {
			isAlternate = true
			break
		}
	}
	if !isAlternate {
		writeErrorCode(w, http.StatusBadRequest, "source_not_alternate", "source is not an alternate for this book")
		return
	}

	currentChapters, err := s.bookStore.GetChapters(bookID)
	if err != nil {
		writeErrorCode(w, http.StatusInternalServerError, "storage_error", "failed to load chapters")
		return
	}
	currentTitle := ""
	for _, chapter := range currentChapters {
		if chapter.Index == current.DurChapterIndex && !chapter.IsVolume {
			currentTitle = chapter.Title
			break
		}
	}

	src, err := s.sourceStore.GetByID(req.SourceID)
	if err != nil {
		writeErrorCode(w, http.StatusInternalServerError, "storage_error", "failed to load source")
		return
	}
	if src == nil {
		writeErrorCode(w, http.StatusNotFound, "source_not_found", "source not found")
		return
	}
	target, err := s.searcher.GetBookInfoForBook(*src, &book.Book{
		Name: current.Name, Author: current.Author,
	}, req.BookURL)
	if err != nil {
		writeCrawlError(w, "book_info", err)
		return
	}
	target.ID = bookID
	target.SourceID = req.SourceID
	target.SourceURL = req.SourceURL
	target.BookURL = req.BookURL
	target.Origin = src.BookSourceName
	targetChapters, err := s.searcher.GetChapterListForBook(*src, target, target.TocURL)
	if err != nil {
		writeCrawlError(w, "toc", err)
		return
	}
	chapterIndex, mapping := book.MigrateChapterIndex(targetChapters, currentTitle, current.DurChapterIndex)
	if chapterIndex < 0 {
		writeErrorCode(w, http.StatusBadGateway, "source_toc_empty", "target source has no readable chapters")
		return
	}
	if s.catalogs != nil {
		s.catalogs.Invalidate(bookID)
	}
	if err := s.bookStore.SwitchSource(bookID, current.StateVersion, *target, targetChapters, chapterIndex, current.DurChapterPos); err != nil {
		if errors.Is(err, book.ErrBookStateChanged) {
			writeErrorCode(w, http.StatusConflict, "state_changed", "reading position changed during source validation; try again")
			return
		}
		if errors.Is(err, book.ErrSourceNotAlternate) {
			writeErrorCode(w, http.StatusConflict, "source_changed", "book sources changed during switch")
			return
		}
		writeErrorCode(w, http.StatusInternalServerError, "storage_error", "failed to switch source")
		return
	}
	switched, err := s.bookStore.GetBook(bookID)
	if err != nil {
		writeErrorCode(w, http.StatusInternalServerError, "storage_error", "source switched but reload failed")
		return
	}
	s.addStoredCoverDisplayURL(switched)
	writeJSON(w, http.StatusOK, map[string]interface{}{"book": switched, "mapping": mapping})
}

// --- Fonts ---

func (s *readerAPI) handleListFonts(w http.ResponseWriter, r *http.Request) {
	fonts, err := s.fontStore.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if fonts == nil {
		fonts = []fontstore.Font{}
	}
	writeJSON(w, http.StatusOK, fonts)
}

func (s *readerAPI) handleUploadFont(w http.ResponseWriter, r *http.Request) {
	const maxFontBytes = 20 << 20
	r.Body = http.MaxBytesReader(w, r.Body, maxFontBytes+(1<<20))
	defer r.Body.Close()
	defer func() {
		if r.MultipartForm != nil {
			if err := r.MultipartForm.RemoveAll(); err != nil {
				slog.Warn("font upload temporary-file cleanup failed", "error", err)
			}
		}
	}()
	if err := r.ParseMultipartForm(maxFontBytes); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeError(w, http.StatusRequestEntityTooLarge, "font upload request is too large")
		} else {
			writeError(w, http.StatusBadRequest, err.Error())
		}
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "missing file field")
		return
	}
	defer file.Close()
	// FileHeader.Size is measured by the bounded multipart parser, not supplied by the client.
	if header.Size > maxFontBytes {
		writeError(w, http.StatusRequestEntityTooLarge, "font file exceeds 20 MiB")
		return
	}

	data, err := io.ReadAll(file)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	name := r.FormValue("name")
	if name == "" {
		name = strings.TrimSuffix(header.Filename, filepath.Ext(header.Filename))
	}

	id := rand.Text()
	f, err := s.fontStore.Add(name, id, data)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, f)
}

func (s *readerAPI) handleDeleteFont(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.fontStore.Delete(id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (s *readerAPI) handleGetFontFile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	font, data, err := s.fontStore.Read(id)
	if err != nil || data == nil {
		writeError(w, http.StatusNotFound, "font not found")
		return
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf(`inline; filename=%q`, font.Name))
	w.Header().Set("Content-Type", http.DetectContentType(data))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// --- Static files (frontend) ---
