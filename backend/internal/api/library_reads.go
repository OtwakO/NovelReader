package api

import (
	"context"
	"database/sql"
	"net/http"
	"strings"

	"github.com/otwako/novelreader/internal/book"
	"github.com/otwako/novelreader/internal/booksource"
	"github.com/otwako/novelreader/internal/chineseconv"
	"github.com/otwako/novelreader/internal/library"
)

type libraryBookResponse struct {
	library.Item
	CoverDisplayURL string                  `json:"coverDisplayUrl,omitempty"`
	OriginLabel     string                  `json:"originLabel,omitempty"`
	ReadingContext  *readingContextResponse `json:"readingContext,omitempty"`
}

// Single-book entry qualification, not a cached copy of mutable reading state.
type readingContextResponse struct {
	SourceIdentity    string                 `json:"sourceIdentity,omitempty"`
	ChineseConversion chineseconv.Capability `json:"chineseConversion"`
}

// Shared state and native display inputs come from one SQLite snapshot. The
// shelf uses one library query and one native-context query, not per-book reads.
func (s *readerAPI) libraryBooks(ctx context.Context, id string) ([]libraryBookResponse, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var items []library.Item
	if id == "" {
		items, err = library.ListTx(ctx, tx)
	} else {
		var item *library.Item
		item, err = library.GetTx(ctx, tx, id)
		if item != nil {
			items = append(items, *item)
		}
	}
	if err != nil {
		return nil, err
	}
	result := make([]libraryBookResponse, 0, len(items))
	contexts, err := book.ReadDisplayContextsTx(ctx, tx, id)
	if err != nil {
		return nil, err
	}
	var readingContext *readingContextResponse
	if id != "" && len(items) == 1 {
		switch items[0].Provider {
		case library.TXT, library.EPUB:
			readingContext = &readingContextResponse{ChineseConversion: s.conversionCapability()}
		case library.BookSource:
			source, err := booksource.GetByIDTx(ctx, tx, contexts[id].SourceID)
			if err != nil {
				return nil, err
			}
			if source != nil {
				identity, err := source.DefinitionIdentity()
				if err != nil {
					return nil, err
				}
				readingContext = &readingContextResponse{SourceIdentity: identity, ChineseConversion: s.conversionCapability()}
			}
		}
	}
	// No transaction needs to span cover revision enrichment.
	if err := tx.Rollback(); err != nil {
		return nil, err
	}
	revisions := make(map[string]coverCacheRevision)
	if id == "" {
		revisions = s.coverCacheRevisions()
	} else if native, ok := contexts[id]; ok {
		revisions[native.SourceID] = s.coverCacheRevision(native.SourceID)
	}
	for _, item := range items {
		response := libraryBookResponse{Item: item, ReadingContext: readingContext}
		if item.Provider == library.BookSource {
			native := contexts[item.ID]
			response.OriginLabel = native.Origin
			if response.OriginLabel == "" {
				response.OriginLabel = native.SourceURL
			}
			if strings.TrimSpace(item.CoverURL) != "" {
				projection := book.Book{ID: item.ID, CoverURL: item.CoverURL, SourceID: native.SourceID, SourceURL: native.SourceURL, BookURL: native.BookURL, VariableMap: native.VariableMap}
				response.CoverDisplayURL = storedCoverDisplayURL(&projection, revisions[native.SourceID], s.coverIdentityScope())
			}
		}
		if item.Provider == library.EPUB {
			response.OriginLabel = "EPUB"
			response.CoverURL = ""
			if item.CoverURL != "" {
				response.CoverDisplayURL = s.epubCoverDisplayURL(&item)
			}
		}
		result = append(result, response)
	}
	return result, nil
}

func (s *readerAPI) handleListBooks(w http.ResponseWriter, r *http.Request) {
	items, err := s.libraryBooks(r.Context(), "")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "load library failed")
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *readerAPI) handleGetBook(w http.ResponseWriter, r *http.Request) {
	items, err := s.libraryBooks(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "load book failed")
		return
	}
	if len(items) == 0 {
		writeErrorCode(w, http.StatusNotFound, "book_not_found", "book not found")
		return
	}
	writeJSON(w, http.StatusOK, items[0])
}
