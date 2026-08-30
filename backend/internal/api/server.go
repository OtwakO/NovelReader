// Package api provides the HTTP REST API for NovelReader.
package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"log/slog"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/otwako/novelreader/internal/analyzer"
	"github.com/otwako/novelreader/internal/auth"
	backupservice "github.com/otwako/novelreader/internal/backup"
	"github.com/otwako/novelreader/internal/book"
	"github.com/otwako/novelreader/internal/booksource"
	"github.com/otwako/novelreader/internal/candidate"
	"github.com/otwako/novelreader/internal/chineseconv"
	"github.com/otwako/novelreader/internal/fetcher"
	"github.com/otwako/novelreader/internal/fontstore"
	"github.com/otwako/novelreader/internal/processor"
	"github.com/otwako/novelreader/internal/readerstore"
	"github.com/otwako/novelreader/internal/sourceprofile"
)

// Server holds API dependencies.
type Server struct {
	db                  *sql.DB
	sourceStore         *booksource.Store
	bookStore           *book.Store
	searcher            *book.Searcher
	fontStore           *fontstore.Store
	sourceProfiles      *sourceprofile.Store
	fetcher             *fetcher.Client
	jsVM                *analyzer.JSVM
	cache               *analyzer.CacheManager
	processorCfg        processor.Config
	dataDir             string
	mux                 *http.ServeMux
	auth                *auth.HTTPHandler
	runtimes            *readerRuntimeManager
	runtime             *readerRuntime
	health              interface{ PingContext(context.Context) error }
	webViewProbe        interface{ Probe(context.Context) error }
	chineseConversion   chineseconv.Service
	candidateOperations *candidate.Manager
	coverReferenceKey   []byte
	collectionLoader    *booksource.RemoteLoader
	collectionScheduler *sourceCollectionScheduler
	backups             *backupservice.Service
}

// Mux exposes the underlying ServeMux for static file mounting.
func (s *Server) Mux() *http.ServeMux {
	return s.mux
}

// Close releases cached reader-home leases owned by the authenticated server.
func (s *Server) Close() error {
	if s == nil {
		return nil
	}
	if s.candidateOperations != nil {
		s.candidateOperations.Close()
	}
	if s.backups != nil {
		if err := s.backups.Close(); err != nil {
			return err
		}
	}
	if s.collectionScheduler != nil {
		s.collectionScheduler.Close()
	}
	if s.chineseConversion != nil {
		if err := s.chineseConversion.Close(); err != nil {
			return err
		}
	}
	if s.runtimes == nil {
		return nil
	}
	return s.runtimes.Close()
}

// NewServer creates and wires up the API server.
func NewServer(
	sourceStore *booksource.Store,
	bookStore *book.Store,
	searcher *book.Searcher,
	fontStore *fontstore.Store,
	fetcher *fetcher.Client,
	jsVM *analyzer.JSVM,
	cache *analyzer.CacheManager,
	processorCfg processor.Config,
	dataDir string,
	db *sql.DB,
) *Server {
	s := &Server{
		db:                  db,
		sourceStore:         sourceStore,
		bookStore:           bookStore,
		searcher:            searcher,
		fontStore:           fontStore,
		fetcher:             fetcher,
		jsVM:                jsVM,
		cache:               cache,
		processorCfg:        processorCfg,
		dataDir:             dataDir,
		mux:                 http.NewServeMux(),
		candidateOperations: candidate.NewManager(candidate.DefaultPolicy()),
		coverReferenceKey:   mustNewCoverReferenceKey(),
		collectionLoader:    booksource.NewRemoteLoader(),
	}
	s.registerRoutes()
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Recover from panics to avoid crashing the entire server
	defer func() {
		if rec := recover(); rec != nil {
			slog.Error("server: panic recovered",
				"path", r.URL.Path,
				"method", r.Method,
				"panic", fmt.Sprintf("%v", rec))
			writeError(w, http.StatusInternalServerError, "internal server error")
		}
	}()
	s.mux.ServeHTTP(w, r)
}

func (s *Server) registerRoutes() {
	s.mux.HandleFunc("GET /api/healthz", s.handleHealth)
	s.registerRoutesWithoutHealth()
}

func (s *Server) registerRoutesWithoutHealth() {
	// Book sources — URLs with slashes can't go in path segments, use query param
	s.mux.HandleFunc("GET /api/sources", s.handleListSources)
	s.mux.HandleFunc("POST /api/sources", s.handleImportSources)
	s.mux.HandleFunc("GET /api/source-collections", s.handleListSourceCollections)
	s.mux.HandleFunc("POST /api/source-collections/upload", s.handleCreateUploadCollection)
	s.mux.HandleFunc("POST /api/source-collections/url", s.handleCreateURLCollection)
	s.mux.HandleFunc("PATCH /api/source-collections/{id}", s.handleUpdateSourceCollection)
	s.mux.HandleFunc("POST /api/source-collections/{id}/replace", s.handleReplaceUploadCollection)
	s.mux.HandleFunc("POST /api/source-collections/{id}/sync", s.handleSyncSourceCollection)
	s.mux.HandleFunc("DELETE /api/source-collections/{id}", s.handleDeleteSourceCollection)
	s.mux.HandleFunc("DELETE /api/sources", s.handleDeleteSource)
	s.mux.HandleFunc("PUT /api/sources", s.handleUpdateSource)

	// Books
	s.mux.HandleFunc("GET /api/books", s.handleListBooks)
	s.mux.HandleFunc("GET /api/books/{id}", s.handleGetBook)
	s.mux.HandleFunc("GET /api/books/{id}/cover", s.handleGetBookCover)
	s.mux.HandleFunc("GET /api/covers/{reference}", s.handleGetCoverDisplay)
	s.mux.HandleFunc("POST /api/candidate-resolutions", s.handleStartCandidateResolution)
	s.mux.HandleFunc("GET /api/candidate-resolutions/{id}", s.handleGetCandidateResolution)
	s.mux.HandleFunc("GET /api/candidate-resolutions/{id}/events", s.handleStreamCandidateResolution)
	s.mux.HandleFunc("DELETE /api/candidate-resolutions/{id}", s.handleCancelCandidateResolution)
	s.mux.HandleFunc("POST /api/candidate-resolutions/{id}/shelve", s.handleCommitCandidateResolution)
	s.mux.HandleFunc("POST /api/books/{id}/sources", s.handleMergeBookSources)
	s.mux.HandleFunc("DELETE /api/books/{id}/sources", s.handleClearBookSources)
	s.mux.HandleFunc("DELETE /api/books", s.handleDeleteBook)

	// Search
	s.mux.HandleFunc("GET /api/search/stream", s.handleSearchBatchStream)

	// Explore
	s.mux.HandleFunc("GET /api/explore/sources", s.handleExploreSources)
	s.mux.HandleFunc("POST /api/explore/catalog", s.handleExploreCatalog)
	s.mux.HandleFunc("POST /api/explore/control", s.handleExploreControl)
	s.mux.HandleFunc("POST /api/explore/page", s.handleExplorePage)

	// Chapters
	s.mux.HandleFunc("GET /api/books/{id}/chapters", s.handleGetChapters)
	s.mux.HandleFunc("GET /api/books/{id}/chapters/{idx}/content", s.handleGetChapterContent)
	s.mux.HandleFunc("GET /api/books/{id}/chapters/{idx}/images/{imageIdx}", s.handleGetChapterImage)

	// Progress
	s.mux.HandleFunc("PUT /api/books/{id}/progress", s.handleUpdateProgress)

	// Source switching and bookmarks
	s.mux.HandleFunc("PUT /api/books/{id}/source", s.handleSwitchSource)
	s.mux.HandleFunc("GET /api/books/{id}/bookmarks", s.handleListBookmarks)
	s.mux.HandleFunc("POST /api/books/{id}/bookmarks", s.handleAddBookmark)
	s.mux.HandleFunc("DELETE /api/books/{id}/bookmarks/{bookmarkID}", s.handleDeleteBookmark)

	// Fonts — IDs are simple UUIDs/timestamps, path-safe
	s.mux.HandleFunc("GET /api/fonts", s.handleListFonts)
	s.mux.HandleFunc("POST /api/fonts", s.handleUploadFont)
	s.mux.HandleFunc("DELETE /api/fonts/{id}", s.handleDeleteFont)
	s.mux.HandleFunc("GET /api/fonts/{id}/file", s.handleGetFontFile)
	s.mux.HandleFunc("GET /api/system/webview-status", s.handleWebViewStatus)
	s.mux.HandleFunc("GET /api/system/chinese-conversion", s.handleChineseConversionCapability)
	s.mux.HandleFunc("POST /api/system/chinese-conversion", s.handleChineseConversion)
}

// NewAuthenticatedServer creates the production Reader Data boundary.
func NewAuthenticatedServer(authHandler *auth.HTTPHandler, readers *readerstore.Manager, dataRoot string, rootSearcher *book.Searcher, jsVM *analyzer.JSVM, limits book.SearcherLimits, processorCfg processor.Config, health interface{ PingContext(context.Context) error }, webViewProbe interface{ Probe(context.Context) error }, conversion chineseconv.Service) (*Server, error) {
	s := &Server{
		fetcher: rootSearcherFetcher(rootSearcher), jsVM: jsVM, cache: analyzer.NewCacheManager(),
		processorCfg: processorCfg, mux: http.NewServeMux(), auth: authHandler, health: health, webViewProbe: webViewProbe, chineseConversion: conversion,
		candidateOperations: candidate.NewManager(candidate.DefaultPolicy()),
		coverReferenceKey:   mustNewCoverReferenceKey(),
		collectionLoader:    booksource.NewRemoteLoader(),
	}
	s.runtimes = newReaderRuntimeManager(readers, rootSearcher, jsVM, limits, 32, limits.SessionTTL)
	backups, err := backupservice.NewService(readers, dataRoot, s.runtimes.quiesce, s.runtimes.resume)
	if err != nil {
		_ = s.runtimes.Close()
		return nil, fmt.Errorf("initialize backup service: %w", err)
	}
	s.backups = backups
	s.collectionScheduler = newSourceCollectionScheduler(s.runtimes, s.collectionLoader, authHandler.ListActiveReaderIDs)
	s.collectionScheduler.Start()
	authHandler.ConfigureDeletionQuiescer(readers, s.runtimes.quiesce)
	s.registerAuthenticatedRoutes()
	return s, nil
}

func rootSearcherFetcher(searcher *book.Searcher) *fetcher.Client {
	return searcher.SharedFetcher()
}

func (s *Server) registerAuthenticatedRoutes() {
	s.mux.HandleFunc("GET /api/healthz", s.handleHealth)
	s.mux.Handle("GET /api/backups/export", s.auth.RequireBackupScope(auth.BackupExport, http.HandlerFunc(s.handleBackupExport)))
	s.mux.Handle("POST /api/backups/restores", s.auth.RequireBackupScope(auth.BackupRestore, http.HandlerFunc(s.handlePrepareBackupRestore)))
	s.mux.Handle("GET /api/backups/restores/{id}", s.auth.RequireBackupScope(auth.BackupRestore, http.HandlerFunc(s.handleGetBackupRestore)))
	s.mux.Handle("DELETE /api/backups/restores/{id}", s.auth.RequireBackupScope(auth.BackupRestore, http.HandlerFunc(s.handleCancelBackupRestore)))
	s.mux.Handle("POST /api/backups/restores/{id}/commit", s.auth.RequireBackupScope(auth.BackupRestore, http.HandlerFunc(s.handleCommitBackupRestore)))
	s.mux.Handle("/api/", s.auth.RequireIdentity(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		account, ok := auth.IdentityFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "authentication required")
			return
		}
		runtime, release, err := s.runtimes.acquire(r.Context(), account.ID)
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, "reader storage unavailable")
			return
		}
		defer release()
		requestServer := *s
		requestServer.mux = http.NewServeMux()
		requestServer.runtime = runtime
		requestServer.db = runtime.home.DB()
		requestServer.sourceStore = runtime.sourceStore
		requestServer.bookStore = runtime.bookStore
		requestServer.searcher = runtime.searcher
		requestServer.fontStore = runtime.fontStore
		requestServer.sourceProfiles = runtime.sourceProfiles
		requestServer.registerRoutesWithoutHealth()
		requestServer.mux.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), readerHomeContextKey{}, runtime.home)))
	})))
}

type readerHomeContextKey struct{}

func (s *Server) deleteSourceSession(sourceID string) {
	if s.searcher != nil {
		s.searcher.DeleteSourceSession(sourceID)
	}
}

// --- Book Sources ---

func (s *Server) handleListSources(w http.ResponseWriter, r *http.Request) {
	sources, err := s.sourceStore.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if sources == nil {
		sources = []booksource.BookSource{}
	}
	responses, err := sourceManagementResponses(sources)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, responses)
}

func (s *Server) handleImportSources(w http.ResponseWriter, r *http.Request) {
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

func (s *Server) handleDeleteSource(w http.ResponseWriter, r *http.Request) {
	sourceID := r.URL.Query().Get("id")
	if sourceID == "" {
		writeError(w, http.StatusBadRequest, "missing query param id")
		return
	}
	if err := deleteSourceDefinition(r.Context(), s.sourceStore, s.sourceProfiles, s.deleteSourceSession, sourceID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (s *Server) handleUpdateSource(w http.ResponseWriter, r *http.Request) {
	sourceID := r.URL.Query().Get("id")
	if sourceID == "" {
		writeError(w, http.StatusBadRequest, "missing query param id")
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	defer r.Body.Close()

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
	s.deleteSourceSession(sourceID)
	writeJSON(w, http.StatusOK, src)
}

// --- Books ---

func (s *Server) handleListBooks(w http.ResponseWriter, r *http.Request) {
	books, err := s.bookStore.ListBooks()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if books == nil {
		books = []book.Book{}
	}
	for index := range books {
		addStoredCoverDisplayURL(&books[index])
	}
	writeJSON(w, http.StatusOK, books)
}

func (s *Server) handleGetBook(w http.ResponseWriter, r *http.Request) {
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
	addStoredCoverDisplayURL(b)
	writeJSON(w, http.StatusOK, b)
}

func (s *Server) handleGetBookCover(w http.ResponseWriter, r *http.Request) {
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

func (s *Server) handleMergeBookSources(w http.ResponseWriter, r *http.Request) {
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
	addStoredCoverDisplayURL(stored)
	writeJSON(w, http.StatusOK, stored)
}

func (s *Server) handleClearBookSources(w http.ResponseWriter, r *http.Request) {
	stored, err := s.bookStore.ClearBookSources(r.PathValue("id"))
	if errors.Is(err, book.ErrBookNotFound) {
		writeErrorCode(w, http.StatusNotFound, "book_not_found", "book not found")
		return
	}
	if err != nil {
		writeErrorCode(w, http.StatusInternalServerError, "storage_error", "failed to clear book sources")
		return
	}
	addStoredCoverDisplayURL(stored)
	writeJSON(w, http.StatusOK, stored)
}

func (s *Server) handleDeleteBook(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if err := s.bookStore.DeleteBook(id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// --- Search ---

// --- Chapters ---

func (s *Server) handleGetChapters(w http.ResponseWriter, r *http.Request) {
	bookID := r.PathValue("id")
	b, err := s.bookStore.GetBook(bookID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "load book failed")
		return
	}
	if b == nil {
		writeErrorCode(w, http.StatusNotFound, "book_not_found", "book not found")
		return
	}

	// Return cached chapters if available.
	chapters, err := s.bookStore.GetChapters(bookID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "load chapters failed")
		return
	}
	if len(chapters) > 0 {
		writeJSON(w, http.StatusOK, chapters)
		return
	}

	// Fetch TOC from source.
	src, err := s.sourceStore.GetByID(b.SourceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "load source failed")
		return
	}
	if src == nil {
		writeErrorCode(w, http.StatusNotFound, "source_not_found", "source not found")
		return
	}

	chapters, err = s.searcher.GetChapterListForBook(*src, b, b.TocURL)
	if err != nil {
		writeCrawlError(w, "toc", err)
		return
	}

	// Generate unique chapter IDs using bookID (the unique book identifier)
	// instead of bookURL (which can be the same across different book entries)
	for i := range chapters {
		chapters[i].ID = fmt.Sprintf("%s_%d", bookID, i)
	}

	if err := s.bookStore.SaveChapters(bookID, chapters); err != nil {
		log.Printf("api: save chapters: %v", err)
		// non-fatal: return fetched chapters anyway
	}

	// Update total chapter count on the book
	if err := s.bookStore.UpdateTotalChapters(bookID, len(chapters)); err != nil {
		log.Printf("api: update chapter count: %v", err)
	}

	writeJSON(w, http.StatusOK, chapters)
}

func (s *Server) handleGetChapterContent(w http.ResponseWriter, r *http.Request) {
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
	rawContent, contentTitle, err := s.searcher.GetChapterContentForBook(*src, b, ch, next)
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

	writeJSON(w, http.StatusOK, chapterContentResponse{Title: result.Title, Paragraphs: result.Paragraphs, Blocks: responseBlocks(result.Blocks)})
}

// --- Progress ---

func (s *Server) handleUpdateProgress(w http.ResponseWriter, r *http.Request) {
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

func (s *Server) handleSwitchSource(w http.ResponseWriter, r *http.Request) {
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
	addStoredCoverDisplayURL(switched)
	writeJSON(w, http.StatusOK, map[string]interface{}{"book": switched, "mapping": mapping})
}

// --- Fonts ---

func (s *Server) handleListFonts(w http.ResponseWriter, r *http.Request) {
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

func (s *Server) handleUploadFont(w http.ResponseWriter, r *http.Request) {
	// Parse multipart form (max 20MB)
	if err := r.ParseMultipartForm(20 << 20); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "missing file field")
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	name := r.FormValue("name")
	if name == "" {
		name = strings.TrimSuffix(header.Filename, filepath.Ext(header.Filename))
	}

	id := fmt.Sprintf("%d", TimeNowMillis()) // ponytail: simple timestamp ID
	f, err := s.fontStore.Add(name, id, data)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, f)
}

func (s *Server) handleDeleteFont(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.fontStore.Delete(id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (s *Server) handleGetFontFile(w http.ResponseWriter, r *http.Request) {
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

// ServeStatic serves the frontend build output.
// Should be mounted at / with a fallback to index.html for SPA routing.
func (s *Server) ServeStatic(mux *http.ServeMux, staticDir string, fs http.Handler) {
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		path := filepath.Join(staticDir, r.URL.Path)
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			if strings.HasPrefix(r.URL.Path, "/assets/") {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			} else {
				w.Header().Set("Cache-Control", "no-cache")
			}
			fs.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeFile(w, r, filepath.Join(staticDir, "index.html"))
	}))
}

// --- Helpers ---

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// ponytail: simple millis, non-atomic. Fine for single-server.
var TimeNowMillis = func() int64 {
	return time.Now().UnixMilli()
}
