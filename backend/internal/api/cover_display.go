package api

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/otwako/novelreader/internal/book"
	"github.com/otwako/novelreader/internal/candidate"
	"github.com/otwako/novelreader/internal/sourceprofile"
)

const (
	maxCoverReferenceBytes = 64 * 1024
	coverCacheControl      = "private, max-age=604800"
)

var errInvalidCoverReference = errors.New("cover reference is invalid")

type coverReference struct {
	SourceID  string `json:"sourceId"`
	SourceURL string `json:"sourceUrl"`
	BookURL   string `json:"bookUrl"`
	CoverURL  string `json:"coverUrl"`
	Revision  string `json:"revision"`
}

type coverCacheRevision struct {
	Source  int64
	Profile sourceprofile.CacheRevision
}

func mustNewCoverReferenceKey() []byte {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		panic(fmt.Sprintf("generate cover reference key: %v", err))
	}
	return key
}

func (s *readerAPI) addCoverDisplayURL(result *book.SearchResult) {
	if result == nil || strings.TrimSpace(result.CoverURL) == "" {
		return
	}
	s.addCoverDisplayURLWithRevision(result, s.coverCacheRevision(result.SourceID))
}

func (s *readerAPI) addCoverDisplayURLs(results []book.SearchResult, revision coverCacheRevision) {
	for index := range results {
		result := &results[index]
		if strings.TrimSpace(result.CoverURL) != "" {
			s.addCoverDisplayURLWithRevision(result, revision)
		}
	}
}

func (s *readerAPI) addCoverDisplayURLWithRevision(result *book.SearchResult, revision coverCacheRevision) {
	result.CoverDisplayURL = s.coverDisplayURL(result.SourceID, result.SourceURL, result.BookURL, result.CoverURL, revision)
}

func (s *readerAPI) addExploreCoverDisplayURLs(page *book.ExplorePage) {
	if page == nil {
		return
	}
	s.addCoverDisplayURLs(page.Books, s.coverCacheRevision(page.SourceID))
}

func (s *readerAPI) addCandidateCoverDisplayURL(snapshot *candidate.Snapshot) {
	if snapshot == nil {
		return
	}
	if snapshot.Preview != nil && strings.TrimSpace(snapshot.Preview.Book.CoverURL) != "" {
		preview := &snapshot.Preview.Book
		revision := s.coverCacheRevision(preview.SourceID)
		preview.CoverDisplayURL = s.coverDisplayURL(preview.SourceID, preview.SourceURL, preview.BookURL, preview.CoverURL, revision)
	}
	s.addStoredCoverDisplayURL(snapshot.StoredBook)
}

func (s *readerAPI) coverDisplayURL(sourceID, sourceURL, bookURL, coverURL string, revision coverCacheRevision) string {
	if s == nil || len(s.coverReferenceKey) == 0 || strings.TrimSpace(sourceID) == "" || strings.TrimSpace(sourceURL) == "" || strings.TrimSpace(bookURL) == "" || strings.TrimSpace(coverURL) == "" {
		return ""
	}
	version := coverRevision(s.coverCacheScope, revision, sourceID, sourceURL, bookURL, coverURL)
	payload, err := json.Marshal(coverReference{SourceID: sourceID, SourceURL: sourceURL, BookURL: bookURL, CoverURL: coverURL, Revision: version})
	if err != nil || len(payload) > maxCoverReferenceBytes {
		return ""
	}
	mac := hmac.New(sha256.New, s.coverReferenceKey)
	_, _ = mac.Write(payload)
	signature := mac.Sum(nil)
	return "/api/covers/" + base64.RawURLEncoding.EncodeToString(payload) + "." + base64.RawURLEncoding.EncodeToString(signature)
}

func (s *readerAPI) addStoredCoverDisplayURL(stored *book.Book) {
	if stored == nil || strings.TrimSpace(stored.CoverURL) == "" {
		return
	}
	stored.CoverDisplayURL = storedCoverDisplayURL(stored, s.coverCacheRevision(stored.SourceID), s.coverCacheScope)
}

func (s *readerAPI) addStoredCoverDisplayURLs(books []book.Book) {
	revisions := s.coverCacheRevisions()
	for index := range books {
		stored := &books[index]
		if strings.TrimSpace(stored.CoverURL) != "" {
			stored.CoverDisplayURL = storedCoverDisplayURL(stored, revisions[stored.SourceID], s.coverCacheScope)
		}
	}
}

func storedCoverDisplayURL(stored *book.Book, revision coverCacheRevision, cacheScope string) string {
	if stored == nil || strings.TrimSpace(stored.ID) == "" {
		return ""
	}
	return versionedCoverURL("/api/books/"+url.PathEscape(stored.ID)+"/cover", coverRevision(cacheScope, revision, stored.SourceID, stored.SourceURL, stored.BookURL, stored.CoverURL, stored.VariableMap))
}

func coverRevision(cacheScope string, revision coverCacheRevision, values ...string) string {
	hash := sha256.New()
	_, _ = hash.Write([]byte(cacheScope))
	_, _ = hash.Write([]byte{0})
	_, _ = hash.Write([]byte(strconv.FormatInt(revision.Source, 10)))
	_, _ = hash.Write([]byte{0})
	_, _ = hash.Write([]byte(strconv.FormatInt(revision.Profile.Settings, 10)))
	_, _ = hash.Write([]byte{0})
	_, _ = hash.Write([]byte(strconv.FormatInt(revision.Profile.Authentication, 10)))
	for _, value := range values {
		_, _ = hash.Write([]byte{0})
		_, _ = hash.Write([]byte(value))
	}
	return base64.RawURLEncoding.EncodeToString(hash.Sum(nil)[:12])
}

func (s *readerAPI) coverCacheRevision(sourceID string) coverCacheRevision {
	var revision coverCacheRevision
	if s.sourceStore != nil {
		var err error
		revision.Source, err = s.sourceStore.DefinitionRevision(sourceID)
		if err != nil {
			slog.Warn("cover: source revision unavailable", "sourceId", sourceID, "err", err)
		}
	}
	revision.Profile = s.sourceProfileRevision(sourceID)
	return revision
}

func (s *readerAPI) coverCacheRevisions() map[string]coverCacheRevision {
	revisions := make(map[string]coverCacheRevision)
	if s.sourceStore != nil {
		sourceRevisions, err := s.sourceStore.DefinitionRevisions()
		if err != nil {
			slog.Warn("cover: source revisions incomplete", "err", err)
		}
		for sourceID, sourceRevision := range sourceRevisions {
			revisions[sourceID] = coverCacheRevision{Source: sourceRevision}
		}
	}
	for sourceID, profileRevision := range s.sourceProfileRevisions() {
		revision := revisions[sourceID]
		revision.Profile = profileRevision
		revisions[sourceID] = revision
	}
	return revisions
}

func (s *readerAPI) sourceProfileRevision(sourceID string) sourceprofile.CacheRevision {
	if s.sourceProfiles == nil {
		return sourceprofile.CacheRevision{}
	}
	revision, err := s.sourceProfiles.CacheRevision(sourceID)
	if err != nil {
		slog.Warn("cover: source profile revision unavailable", "sourceId", sourceID, "err", err)
	}
	return revision
}

func (s *readerAPI) sourceProfileRevisions() map[string]sourceprofile.CacheRevision {
	if s.sourceProfiles == nil {
		return nil
	}
	revisions, err := s.sourceProfiles.CacheRevisions()
	if err != nil {
		slog.Warn("cover: source profile revisions incomplete", "err", err)
	}
	return revisions
}

func versionedCoverURL(rawURL, revision string) string {
	if rawURL == "" || revision == "" {
		return rawURL
	}
	return rawURL + "?v=" + url.QueryEscape(revision)
}

func (s *readerAPI) parseCoverReference(value string) (coverReference, error) {
	payloadValue, signatureValue, ok := strings.Cut(value, ".")
	if !ok || payloadValue == "" || signatureValue == "" {
		return coverReference{}, errInvalidCoverReference
	}
	payload, err := base64.RawURLEncoding.DecodeString(payloadValue)
	if err != nil || len(payload) == 0 || len(payload) > maxCoverReferenceBytes {
		return coverReference{}, errInvalidCoverReference
	}
	signature, err := base64.RawURLEncoding.DecodeString(signatureValue)
	if err != nil {
		return coverReference{}, errInvalidCoverReference
	}
	mac := hmac.New(sha256.New, s.coverReferenceKey)
	_, _ = mac.Write(payload)
	if !hmac.Equal(signature, mac.Sum(nil)) {
		return coverReference{}, errInvalidCoverReference
	}
	var ref coverReference
	if err := json.Unmarshal(payload, &ref); err != nil || strings.TrimSpace(ref.SourceID) == "" || strings.TrimSpace(ref.SourceURL) == "" || strings.TrimSpace(ref.BookURL) == "" || strings.TrimSpace(ref.CoverURL) == "" || strings.TrimSpace(ref.Revision) == "" {
		return coverReference{}, errInvalidCoverReference
	}
	return ref, nil
}

func (s *readerAPI) handleGetCoverDisplay(w http.ResponseWriter, r *http.Request) {
	if s.sourceStore == nil || s.searcher == nil {
		writeError(w, http.StatusServiceUnavailable, "cover service unavailable")
		return
	}
	ref, err := s.parseCoverReference(r.PathValue("reference"))
	if err != nil {
		writeErrorCode(w, http.StatusBadRequest, "invalid_cover_reference", "cover reference is invalid")
		return
	}
	src, err := s.sourceStore.GetByID(ref.SourceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "load source failed")
		return
	}
	if src == nil {
		writeErrorCode(w, http.StatusNotFound, "source_not_found", "book source not found")
		return
	}
	candidate := &book.Book{SourceID: ref.SourceID, SourceURL: ref.SourceURL, BookURL: ref.BookURL, CoverURL: ref.CoverURL}
	data, contentType, err := s.searcher.GetBookCover(r.Context(), *src, candidate)
	if err != nil {
		writeErrorCode(w, http.StatusBadGateway, "cover_fetch_failed", "book cover unavailable")
		return
	}
	writeCoverBytes(w, data, contentType)
}

func writeCoverBytes(w http.ResponseWriter, data []byte, contentType string) {
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", coverCacheControl)
	w.Header().Set("Vary", "Cookie")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}
