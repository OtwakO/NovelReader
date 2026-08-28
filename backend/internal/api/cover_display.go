package api

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/otwako/novelreader/internal/book"
	"github.com/otwako/novelreader/internal/candidate"
)

const maxCoverReferenceBytes = 64 * 1024

var errInvalidCoverReference = errors.New("cover reference is invalid")

type coverReference struct {
	SourceURL string `json:"sourceUrl"`
	BookURL   string `json:"bookUrl"`
	CoverURL  string `json:"coverUrl"`
}

func mustNewCoverReferenceKey() []byte {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		panic(fmt.Sprintf("generate cover reference key: %v", err))
	}
	return key
}

func (s *Server) addCoverDisplayURL(result *book.SearchResult) {
	if result == nil || strings.TrimSpace(result.CoverURL) == "" {
		return
	}
	result.CoverDisplayURL = s.coverDisplayURL(result.SourceURL, result.BookURL, result.CoverURL)
}

func (s *Server) addExploreCoverDisplayURLs(page *book.ExplorePage) {
	if page == nil {
		return
	}
	for index := range page.Books {
		s.addCoverDisplayURL(&page.Books[index])
	}
}

func (s *Server) addCandidateCoverDisplayURL(snapshot *candidate.Snapshot) {
	if snapshot == nil {
		return
	}
	if snapshot.Preview != nil && strings.TrimSpace(snapshot.Preview.Book.CoverURL) != "" {
		preview := &snapshot.Preview.Book
		preview.CoverDisplayURL = s.coverDisplayURL(preview.SourceURL, preview.BookURL, preview.CoverURL)
	}
	addStoredCoverDisplayURL(snapshot.StoredBook)
}

func (s *Server) coverDisplayURL(sourceURL, bookURL, coverURL string) string {
	if s == nil || len(s.coverReferenceKey) == 0 || strings.TrimSpace(sourceURL) == "" || strings.TrimSpace(bookURL) == "" || strings.TrimSpace(coverURL) == "" {
		return ""
	}
	payload, err := json.Marshal(coverReference{SourceURL: sourceURL, BookURL: bookURL, CoverURL: coverURL})
	if err != nil || len(payload) > maxCoverReferenceBytes {
		return ""
	}
	mac := hmac.New(sha256.New, s.coverReferenceKey)
	_, _ = mac.Write(payload)
	signature := mac.Sum(nil)
	return "/api/covers/" + base64.RawURLEncoding.EncodeToString(payload) + "." + base64.RawURLEncoding.EncodeToString(signature)
}

func addStoredCoverDisplayURL(stored *book.Book) {
	if stored == nil || strings.TrimSpace(stored.CoverURL) == "" {
		return
	}
	stored.CoverDisplayURL = storedCoverDisplayURL(stored.ID)
}

func storedCoverDisplayURL(bookID string) string {
	if strings.TrimSpace(bookID) == "" {
		return ""
	}
	return "/api/books/" + url.PathEscape(bookID) + "/cover"
}

func (s *Server) parseCoverReference(value string) (coverReference, error) {
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
	if err := json.Unmarshal(payload, &ref); err != nil || strings.TrimSpace(ref.SourceURL) == "" || strings.TrimSpace(ref.BookURL) == "" || strings.TrimSpace(ref.CoverURL) == "" {
		return coverReference{}, errInvalidCoverReference
	}
	return ref, nil
}

func (s *Server) handleGetCoverDisplay(w http.ResponseWriter, r *http.Request) {
	if s.sourceStore == nil || s.searcher == nil {
		writeError(w, http.StatusServiceUnavailable, "cover service unavailable")
		return
	}
	ref, err := s.parseCoverReference(r.PathValue("reference"))
	if err != nil {
		writeErrorCode(w, http.StatusBadRequest, "invalid_cover_reference", "cover reference is invalid")
		return
	}
	src, err := s.sourceStore.GetByID(ref.SourceURL)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "load source failed")
		return
	}
	if src == nil {
		writeErrorCode(w, http.StatusNotFound, "source_not_found", "book source not found")
		return
	}
	candidate := &book.Book{SourceURL: ref.SourceURL, BookURL: ref.BookURL, CoverURL: ref.CoverURL}
	data, contentType, err := s.searcher.GetBookCover(r.Context(), *src, candidate)
	if err != nil {
		writeErrorCode(w, http.StatusBadGateway, "cover_fetch_failed", "book cover unavailable")
		return
	}
	writeCoverBytes(w, data, contentType)
}

func writeCoverBytes(w http.ResponseWriter, data []byte, contentType string) {
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "private, no-cache")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}
