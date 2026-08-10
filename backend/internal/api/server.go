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
	"sync"
	"time"

	"github.com/otwako/novelreader/internal/analyzer"
	"github.com/otwako/novelreader/internal/auth"
	"github.com/otwako/novelreader/internal/book"
	"github.com/otwako/novelreader/internal/booksource"
	"github.com/otwako/novelreader/internal/fetcher"
	"github.com/otwako/novelreader/internal/fontstore"
	"github.com/otwako/novelreader/internal/processor"
	"github.com/otwako/novelreader/internal/readerstore"
)

// Server holds API dependencies.
type Server struct {
	db           *sql.DB
	sourceStore  *booksource.Store
	bookStore    *book.Store
	searcher     *book.Searcher
	fontStore    *fontstore.Store
	fetcher      *fetcher.Client
	jsVM         *analyzer.JSVM
	cache        *analyzer.CacheManager
	processorCfg processor.Config
	dataDir      string
	mux          *http.ServeMux
	auth         *auth.HTTPHandler
	runtimes     *readerRuntimeManager
	health       interface{ PingContext(context.Context) error }
}

// Mux exposes the underlying ServeMux for static file mounting.
func (s *Server) Mux() *http.ServeMux {
	return s.mux
}

// Close releases cached reader-home leases owned by the authenticated server.
func (s *Server) Close() error {
	if s == nil || s.runtimes == nil {
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
		db:           db,
		sourceStore:  sourceStore,
		bookStore:    bookStore,
		searcher:     searcher,
		fontStore:    fontStore,
		fetcher:      fetcher,
		jsVM:         jsVM,
		cache:        cache,
		processorCfg: processorCfg,
		dataDir:      dataDir,
		mux:          http.NewServeMux(),
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
	s.mux.HandleFunc("DELETE /api/sources", s.handleDeleteSource)
	s.mux.HandleFunc("PUT /api/sources", s.handleUpdateSource)

	// Books
	s.mux.HandleFunc("GET /api/books", s.handleListBooks)
	s.mux.HandleFunc("GET /api/books/{id}", s.handleGetBook)
	s.mux.HandleFunc("GET /api/books/{id}/cover", s.handleGetBookCover)
	s.mux.HandleFunc("POST /api/books", s.handleAddBook)
	s.mux.HandleFunc("POST /api/books/enrich", s.handleEnrichBook)
	s.mux.HandleFunc("DELETE /api/books", s.handleDeleteBook)

	// Search
	s.mux.HandleFunc("GET /api/search", s.handleSearch)
	s.mux.HandleFunc("GET /api/search/stream", s.handleSearchStream)

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
}

// NewAuthenticatedServer creates the production Reader Data boundary.
func NewAuthenticatedServer(authHandler *auth.HTTPHandler, readers *readerstore.Manager, rootSearcher *book.Searcher, jsVM *analyzer.JSVM, limits book.SearcherLimits, processorCfg processor.Config, health interface{ PingContext(context.Context) error }) *Server {
	s := &Server{
		fetcher: rootSearcherFetcher(rootSearcher), jsVM: jsVM, cache: analyzer.NewCacheManager(),
		processorCfg: processorCfg, mux: http.NewServeMux(), auth: authHandler, health: health,
	}
	s.runtimes = newReaderRuntimeManager(readers, rootSearcher, jsVM, limits, 32, limits.SessionTTL)
	authHandler.ConfigureDeletionQuiescer(readers, s.runtimes.quiesce)
	s.registerAuthenticatedRoutes()
	return s
}

func rootSearcherFetcher(searcher *book.Searcher) *fetcher.Client {
	return searcher.SharedFetcher()
}

func (s *Server) registerAuthenticatedRoutes() {
	s.mux.HandleFunc("GET /api/healthz", s.handleHealth)
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
		requestServer.db = runtime.home.DB()
		requestServer.sourceStore = runtime.sourceStore
		requestServer.bookStore = runtime.bookStore
		requestServer.searcher = runtime.searcher
		requestServer.fontStore = runtime.fontStore
		requestServer.registerRoutesWithoutHealth()
		requestServer.mux.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), readerHomeContextKey{}, runtime.home)))
	})))
}

type readerHomeContextKey struct{}

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
	writeJSON(w, http.StatusOK, sources)
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
	rawURL := r.URL.Query().Get("url")
	if rawURL == "" {
		writeError(w, http.StatusBadRequest, "missing query param url")
		return
	}
	if err := s.sourceStore.Delete(rawURL); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (s *Server) handleUpdateSource(w http.ResponseWriter, r *http.Request) {
	rawURL := r.URL.Query().Get("url")
	if rawURL == "" {
		writeError(w, http.StatusBadRequest, "missing query param url")
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
	src.BookSourceURL = rawURL
	if err := s.sourceStore.Upsert(src); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
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
	src, err := s.sourceStore.GetByID(b.SourceURL)
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
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "private, no-cache")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (s *Server) handleAddBook(w http.ResponseWriter, r *http.Request) {
	var b book.Book
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	defer r.Body.Close()

	if err := s.bookStore.AddBook(&b); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, b)
}

func (s *Server) handleEnrichBook(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID               string           `json:"id"`
		Name             string           `json:"name"`
		Author           string           `json:"author,omitempty"`
		CoverURL         string           `json:"coverUrl,omitempty"`
		Intro            string           `json:"intro,omitempty"`
		SourceURL        string           `json:"sourceUrl"`
		BookURL          string           `json:"bookUrl"`
		AlternateSources []book.AltSource `json:"alternateSources,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	defer r.Body.Close()

	if req.ID == "" || req.SourceURL == "" || req.BookURL == "" {
		writeError(w, http.StatusBadRequest, "id, sourceUrl, and bookUrl are required")
		return
	}

	// Check if book already exists
	existing, err := s.bookStore.GetBook(req.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "load book failed")
		return
	}
	if existing != nil {
		writeJSON(w, http.StatusOK, existing)
		return
	}

	// Create book with available info from request
	b := &book.Book{
		ID:               req.ID,
		Name:             req.Name,
		Author:           req.Author,
		CoverURL:         req.CoverURL,
		Intro:            req.Intro,
		SourceURL:        req.SourceURL,
		BookURL:          req.BookURL,
		Origin:           req.SourceURL,
		AlternateSources: req.AlternateSources,
	}

	// Try to enrich from source
	src, err := s.sourceStore.GetByID(req.SourceURL)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "load source failed")
		return
	}
	if src == nil {
		// An unimported source keeps the existing minimal-book fallback.
		if err := s.bookStore.AddBook(b); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, b)
		return
	}

	// Fetch and enrich book info from source while preserving search identity unless the source permits renaming.
	enriched, err := s.searcher.GetBookInfoForBook(*src, b, req.BookURL)
	if err != nil {
		writeCrawlError(w, "book_info", err)
		return
	}

	// Merge ALL enriched fields into book (overwrite with enriched data)
	if enriched.CoverURL != "" {
		b.CoverURL = enriched.CoverURL
	}
	if enriched.Intro != "" {
		b.Intro = enriched.Intro
	}
	if enriched.LastChapter != "" {
		b.LastChapter = enriched.LastChapter
	}
	if enriched.TocURL != "" {
		b.TocURL = enriched.TocURL
	}
	if enriched.Kind != "" {
		b.Kind = enriched.Kind
	}
	if enriched.WordCount != "" {
		b.WordCount = enriched.WordCount
	}
	if enriched.UpdateTime != "" {
		b.UpdateTime = enriched.UpdateTime
	}

	if err := s.bookStore.AddBook(b); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, b)
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

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		writeError(w, http.StatusBadRequest, "missing query param q")
		return
	}

	results, err := s.searcher.Search(query)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if results == nil {
		results = []book.SearchResult{}
	}
	// Merge same books, sort by relevance
	results = book.MergeAndSort(query, results)
	writeJSON(w, http.StatusOK, results)
}

func (s *Server) handleSearchStream(w http.ResponseWriter, r *http.Request) {
	if _, batched := r.URL.Query()["batchSize"]; batched {
		s.handleSearchBatchStream(w, r)
		return
	}

	query := r.URL.Query().Get("q")
	if query == "" {
		writeError(w, http.StatusBadRequest, "missing query param q")
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	writeSSE := func(v interface{}) {
		b, _ := json.Marshal(v)
		fmt.Fprintf(w, "data: %s\n\n", b)
		flusher.Flush()
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	// Collect all results for cross-source merge at the end
	var allResults []book.SearchResult
	var mu sync.Mutex

	total := 0
	sourcesDone := 0
	err := s.searcher.SearchStream(ctx, query, func(src booksource.BookSource, results []book.SearchResult, e error) {
		sourcesDone++
		if e != nil {
			writeSSE(map[string]interface{}{
				"type":    "error",
				"source":  src.BookSourceName,
				"message": e.Error(),
			})
			return
		}
		total += len(results)
		mu.Lock()
		allResults = append(allResults, results...)
		mu.Unlock()
		writeSSE(map[string]interface{}{
			"type":   "results",
			"source": src.BookSourceName,
			"data":   results,
		})
	})

	// Merge same books from different sources, sort by relevance
	merged := book.MergeAndSort(query, allResults)

	writeSSE(map[string]interface{}{
		"type":        "done",
		"total":       total,
		"sourcesDone": sourcesDone,
		"merged":      merged,
		"error":       err != nil && err != context.Canceled,
	})
}

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
	src, err := s.sourceStore.GetByID(b.SourceURL)
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

	src, err := s.sourceStore.GetByID(b.SourceURL)
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
		SourceURL    *string  `json:"sourceUrl"`
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
	if req.SourceURL == nil || *req.SourceURL == "" || req.StateVersion == nil || *req.StateVersion < 0 || req.ChapterIndex == nil || req.Position == nil || *req.ChapterIndex < 0 || math.IsNaN(*req.Position) || math.IsInf(*req.Position, 0) || *req.Position < 0 || *req.Position > 1 {
		writeErrorCode(w, http.StatusBadRequest, "invalid_progress", "sourceUrl, stateVersion, chapterIndex, and position are required and must be valid")
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
	if storedBook.SourceURL != *req.SourceURL || storedBook.StateVersion != *req.StateVersion {
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
	stateVersion, err := s.bookStore.UpdateProgress(bookID, *req.SourceURL, *req.StateVersion, chapterIndex, position)
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
	if err := decoder.Decode(&struct{}{}); err != io.EOF || req.SourceURL == "" || req.BookURL == "" {
		writeErrorCode(w, http.StatusBadRequest, "invalid_source_switch", "sourceUrl and bookUrl are required")
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
		if alternate.SourceURL == req.SourceURL && alternate.BookURL == req.BookURL {
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

	src, err := s.sourceStore.GetByID(req.SourceURL)
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

// ServeStatic serves the Svelte frontend build output.
// Should be mounted at / with a fallback to index.html for SPA routing.
func (s *Server) ServeStatic(mux *http.ServeMux, staticDir string, fs http.Handler) {
	mux.Handle("GET /", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := filepath.Join(staticDir, r.URL.Path)
		if _, err := os.Stat(path); err == nil {
			fs.ServeHTTP(w, r)
			return
		}
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
