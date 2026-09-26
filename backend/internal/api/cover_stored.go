package api

import (
	"context"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/otwako/novelreader/internal/book"
	"github.com/otwako/novelreader/internal/library"
)

func (s *readerAPI) epubCoverVersion(item *library.Item) string {
	return coverRevision(s.coverIdentityScope(), coverCacheRevision{}, string(item.Provider), item.ID, strconv.FormatInt(item.ContentRevision, 10), item.CoverURL)
}

func (s *readerAPI) epubCoverDisplayURL(item *library.Item) string {
	return versionedCoverURL("/api/books/"+url.PathEscape(item.ID)+"/cover", s.epubCoverVersion(item))
}

// Metadata only: the same inputs qualify both display URLs and delivered bytes.
type storedCover struct {
	item    *library.Item
	native  *book.Book
	version string
}

func (s *readerAPI) loadStoredCover(ctx context.Context, id string) (*storedCover, error) {
	item, err := s.libraryStore.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if item == nil || strings.TrimSpace(item.CoverURL) == "" {
		return nil, library.ErrNotFound
	}
	cover := &storedCover{item: item}
	switch item.Provider {
	case library.EPUB:
		cover.version = s.epubCoverVersion(item)
	case library.BookSource:
		cover.native, err = s.bookStore.GetBook(id)
		if err != nil {
			return nil, err
		}
		if cover.native == nil {
			return nil, library.ErrNotFound
		}
		revision, err := s.readCoverCacheRevision(cover.native.SourceID)
		if err != nil {
			return nil, err
		}
		cover.version = storedCoverVersion(cover.native, revision, s.coverIdentityScope())
	default:
		return nil, library.ErrNotFound
	}
	return cover, nil
}

func (s *readerAPI) handleGetBookCover(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "private, no-store")
	cover, err := s.loadStoredCover(r.Context(), r.PathValue("id"))
	if err != nil {
		writeReadingError(w, err)
		return
	}
	if r.URL.Query().Get("v") != cover.version {
		writeReadingError(w, library.ErrNotFound)
		return
	}
	var data []byte
	var contentType string
	switch cover.item.Provider {
	case library.EPUB:
		if s.epubStore == nil {
			writeError(w, http.StatusServiceUnavailable, "cover service unavailable")
			return
		}
		data, contentType, err = s.epubStore.ReadResource(r.Context(), cover.item.ID, cover.item.ContentRevision, cover.item.CoverURL)
		if err != nil {
			writeReadingError(w, err)
			return
		}
	case library.BookSource:
		if s.searcher == nil {
			writeError(w, http.StatusServiceUnavailable, "cover service unavailable")
			return
		}
		src, err := s.sourceStore.GetByID(cover.native.SourceID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "load source failed")
			return
		}
		if src == nil {
			writeErrorCode(w, http.StatusNotFound, "source_not_found", "book source not found")
			return
		}
		data, contentType, err = s.searcher.GetBookCover(r.Context(), *src, cover.native)
		if err != nil {
			slog.Warn("cover: fetch failed", "bookId", cover.item.ID, "sourceId", src.ID, "err", err)
			writeErrorCode(w, http.StatusBadGateway, "cover_fetch_failed", "book cover unavailable")
			return
		}
	}
	// Source/profile edits or a publication change during acquisition must not
	// publish different inputs under the requested long-lived cache identity.
	current, err := s.loadStoredCover(r.Context(), cover.item.ID)
	if err != nil {
		writeReadingError(w, err)
		return
	}
	if current.version != cover.version {
		writeReadingError(w, library.ErrNotFound)
		return
	}
	writeCoverBytes(w, data, contentType)
}
