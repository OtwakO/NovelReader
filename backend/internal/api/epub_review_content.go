package api

import (
	"net/http"
	"net/url"
	"strconv"

	"github.com/otwako/novelreader/internal/epub"
	"github.com/otwako/novelreader/internal/epubstore"
	"github.com/otwako/novelreader/internal/reading"
)

// Targets belong to the generation on the response envelope, never a library revision.
type epubPreviewEntry struct {
	Label       string              `json:"label"`
	Target      *epub.SectionTarget `json:"target,omitempty"`
	Unavailable bool                `json:"unavailable,omitempty"`
	Children    []epubPreviewEntry  `json:"children"`
}

func previewEntries(entries []epub.ResolvedNavigationEntry) []epubPreviewEntry {
	result := make([]epubPreviewEntry, len(entries))
	for i, entry := range entries {
		result[i] = epubPreviewEntry{Label: entry.Label, Target: entry.Target, Unavailable: entry.Unavailable, Children: previewEntries(entry.Children)}
	}
	return result
}

func previewGeneration(w http.ResponseWriter, r *http.Request) (int64, bool) {
	generation, err := strconv.ParseInt(r.URL.Query().Get("generation"), 10, 64)
	if err != nil || generation < 1 {
		writeErrorCode(w, http.StatusBadRequest, "epub_invalid_input", "Expected a preparation generation")
		return 0, false
	}
	return generation, true
}

func (s *readerAPI) handleEPUBPreviewNavigation(w http.ResponseWriter, r *http.Request) {
	generation, ok := previewGeneration(w, r)
	if !ok {
		return
	}
	navigation, err := s.epubStore.PreviewNavigation(r.Context(), r.PathValue("id"), generation)
	if err != nil {
		writeEPUBError(w, err)
		return
	}
	source := "publication"
	if navigation.Source == "spine" {
		source = "sections"
	}
	writeJSON(w, http.StatusOK, struct {
		Generation int64              `json:"generation"`
		Source     string             `json:"source"`
		Entries    []epubPreviewEntry `json:"entries"`
	}{generation, source, previewEntries(navigation.Entries)})
}

func (s *readerAPI) handleEPUBPreviewSection(w http.ResponseWriter, r *http.Request) {
	generation, ok := previewGeneration(w, r)
	if !ok {
		return
	}
	index, err := strconv.Atoi(r.PathValue("section"))
	if err != nil || index < 0 {
		writeErrorCode(w, http.StatusBadRequest, "epub_invalid_input", "Expected a section index")
		return
	}
	id := r.PathValue("id")
	section, err := s.epubStore.PreviewSection(r.Context(), id, generation, index)
	if err != nil {
		writeEPUBError(w, err)
		return
	}
	document, err := reading.EPUBPreviewDocument(r.Context(), section.Title, section.Section, func(key string) string {
		return "/api/imports/epub/receipts/" + url.PathEscape(id) + "/resources/" + url.PathEscape(section.Resources[key]) + "?generation=" + strconv.FormatInt(generation, 10) + "&reader=" + url.QueryEscape(s.coverCacheScope)
	})
	if err != nil {
		writeEPUBError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, struct {
		Generation int64            `json:"generation"`
		Section    int              `json:"section"`
		Version    int              `json:"version"`
		Document   reading.Document `json:"document"`
	}{generation, index, reading.StructuredDocumentVersion, document})
}

func (s *readerAPI) handleEPUBPreviewResource(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("reader") != s.coverCacheScope {
		writeEPUBError(w, epubstore.ErrNotFound)
		return
	}
	generation, ok := previewGeneration(w, r)
	if !ok {
		return
	}
	data, mediaType, err := s.epubStore.PreviewResource(r.Context(), r.PathValue("id"), generation, r.PathValue("resource"))
	if err != nil {
		writeEPUBError(w, err)
		return
	}
	w.Header().Set("Content-Type", mediaType)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "private, no-store")
	w.Write(data)
}
