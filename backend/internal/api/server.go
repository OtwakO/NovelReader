// Package api provides the HTTP REST API for NovelReader.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/otwako/novelreader/internal/analyzer"
	"github.com/otwako/novelreader/internal/book"
	"github.com/otwako/novelreader/internal/booksource"
	"github.com/otwako/novelreader/internal/fetcher"
	"github.com/otwako/novelreader/internal/fontstore"
	"github.com/otwako/novelreader/internal/processor"
)

// Server holds API dependencies.
type Server struct {
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
}

// Mux exposes the underlying ServeMux for static file mounting.
func (s *Server) Mux() *http.ServeMux {
	return s.mux
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
) *Server {
	s := &Server{
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
	// Book sources — URLs with slashes can't go in path segments, use query param
	s.mux.HandleFunc("GET /api/sources", s.handleListSources)
	s.mux.HandleFunc("POST /api/sources", s.handleImportSources)
	s.mux.HandleFunc("DELETE /api/sources", s.handleDeleteSource)
	s.mux.HandleFunc("PUT /api/sources", s.handleUpdateSource)

	// Books
	s.mux.HandleFunc("GET /api/books", s.handleListBooks)
	s.mux.HandleFunc("GET /api/books/{id}", s.handleGetBook)
	s.mux.HandleFunc("POST /api/books", s.handleAddBook)
	s.mux.HandleFunc("POST /api/books/enrich", s.handleEnrichBook)
	s.mux.HandleFunc("DELETE /api/books", s.handleDeleteBook)

	// Search
	s.mux.HandleFunc("GET /api/search", s.handleSearch)
	s.mux.HandleFunc("GET /api/search/stream", s.handleSearchStream)

	// Chapters
	s.mux.HandleFunc("GET /api/books/{id}/chapters", s.handleGetChapters)
	s.mux.HandleFunc("GET /api/books/{id}/chapters/{idx}/content", s.handleGetChapterContent)

	// Progress
	s.mux.HandleFunc("PUT /api/books/{id}/progress", s.handleUpdateProgress)

	// Source switching
	s.mux.HandleFunc("PUT /api/books/{id}/source", s.handleSwitchSource)

	// Fonts — IDs are simple UUIDs/timestamps, path-safe
	s.mux.HandleFunc("GET /api/fonts", s.handleListFonts)
	s.mux.HandleFunc("POST /api/fonts", s.handleUploadFont)
	s.mux.HandleFunc("DELETE /api/fonts/{id}", s.handleDeleteFont)
	s.mux.HandleFunc("GET /api/fonts/{id}/file", s.handleGetFontFile)
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
	if err != nil || b == nil {
		writeError(w, http.StatusNotFound, "book not found")
		return
	}
	writeJSON(w, http.StatusOK, b)
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
		ID        string `json:"id"`
		Name      string `json:"name"`
		Author    string `json:"author,omitempty"`
		CoverURL  string `json:"coverUrl,omitempty"`
		Intro     string `json:"intro,omitempty"`
		SourceURL string `json:"sourceUrl"`
		BookURL   string `json:"bookUrl"`
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

	// Create book with available info
	b := &book.Book{
		ID:       req.ID,
		Name:     req.Name,
		Author:   req.Author,
		CoverURL: req.CoverURL,
		Intro:    req.Intro,
		SourceURL: req.SourceURL,
		BookURL:  req.BookURL,
		Origin:   req.SourceURL,
	}

	// Try to enrich from source (non-fatal if fails)
	src, err := s.sourceStore.GetByID(req.SourceURL)
	if err == nil && src != nil {
		enriched, err := s.searcher.GetBookInfo(*src, req.BookURL)
		if err == nil && enriched != nil {
			if enriched.Name != "" {
				b.Name = enriched.Name
			}
			if enriched.Author != "" {
				b.Author = enriched.Author
			}
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
		}
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
	if err != nil || b == nil {
		writeError(w, http.StatusNotFound, "book not found")
		return
	}

	// Return cached chapters if available
	chapters, err := s.bookStore.GetChapters(bookID)
	if err == nil && len(chapters) > 0 {
		writeJSON(w, http.StatusOK, chapters)
		return
	}

	// Fetch TOC from source
	src, err := s.sourceStore.GetByID(b.SourceURL)
	if err != nil || src == nil {
		writeError(w, http.StatusNotFound, "source not found")
		return
	}

	chapters, err = s.searcher.GetChapterList(*src, b.BookURL, b.TocURL)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
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
	if err != nil || b == nil {
		writeError(w, http.StatusNotFound, "book not found")
		return
	}

	chapters, err := s.bookStore.GetChapters(bookID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
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
		writeError(w, http.StatusNotFound, "chapter not found")
		return
	}

	src, err := s.sourceStore.GetByID(b.SourceURL)
	if err != nil || src == nil {
		writeError(w, http.StatusNotFound, "source not found")
		return
	}

	rawContent, contentTitle, err := s.searcher.GetChapterContent(*src, ch.URL)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	displayTitle := ch.Title
	if contentTitle != "" {
		displayTitle = contentTitle
	}

	proc := processor.New(s.processorCfg)
	result := proc.Process(displayTitle, rawContent)

	writeJSON(w, http.StatusOK, result)
}

// --- Progress ---

func (s *Server) handleUpdateProgress(w http.ResponseWriter, r *http.Request) {
	bookID := r.PathValue("id")
	var req struct {
		ChapterIndex int     `json:"chapterIndex"`
		Position     float64 `json:"position"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	defer r.Body.Close()

	if err := s.bookStore.UpdateProgress(bookID, req.ChapterIndex, req.Position); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "saved"})
}

func (s *Server) handleSwitchSource(w http.ResponseWriter, r *http.Request) {
	bookID := r.PathValue("id")
	var req struct {
		SourceURL  string `json:"sourceUrl"`
		BookURL    string `json:"bookUrl"`
		SourceName string `json:"sourceName,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	defer r.Body.Close()

	if req.SourceURL == "" || req.BookURL == "" {
		writeError(w, http.StatusBadRequest, "sourceUrl and bookUrl required")
		return
	}

	// Look up source name if not provided
	sourceName := req.SourceName
	if sourceName == "" {
		if src, err := s.sourceStore.GetByID(req.SourceURL); err == nil && src != nil {
			sourceName = src.BookSourceName
		}
	}

	if err := s.bookStore.SwitchSource(bookID, req.SourceURL, req.BookURL, sourceName); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	b, _ := s.bookStore.GetBook(bookID)
	writeJSON(w, http.StatusOK, b)
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
	path, err := s.fontStore.GetPath(id)
	if err != nil || path == "" {
		writeError(w, http.StatusNotFound, "font not found")
		return
	}

	// ponytail: ServeFile sets Content-Type from extension; don't force woff2.
	http.ServeFile(w, r, path)
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
