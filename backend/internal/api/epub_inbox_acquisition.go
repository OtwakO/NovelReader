package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/otwako/novelreader/internal/auth"
	"github.com/otwako/novelreader/internal/epub"
	"github.com/otwako/novelreader/internal/epubstore"
	"github.com/otwako/novelreader/internal/readerstore"
)

func (s *Server) handleAcquireEPUBInbox(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("filename")
	mode := epub.ImageMode(r.URL.Query().Get("imageMode"))
	if err := epubstore.ValidateFilename(name); err != nil {
		writeEPUBError(w, err)
		return
	}
	if mode != "" && mode != epub.OriginalImages && mode != epub.OptimizedImages {
		writeEPUBError(w, epub.ErrImagePolicy)
		return
	}
	account, _ := auth.IdentityFromContext(r.Context())
	id := r.PathValue("id")
	ctx, release, err := s.fileAdmission.Begin(r.Context(), account.ID, id)
	if err != nil {
		writeEPUBError(w, err)
		return
	}
	defer release()
	value, warnings, err := s.acquireEPUBInbox(ctx, account.ID, id, name, mode)
	release()
	if errors.Is(err, epubstore.ErrInboxPending) {
		writeJSON(w, http.StatusConflict, map[string]string{"code": "epub_inbox_pending", "error": "Review the existing inbox claim before importing again", "receiptId": value.ID})
		return
	}
	if err != nil {
		writeEPUBError(w, err)
		return
	}
	writeEPUBAcquired(w, value, warnings)
}

func (s *Server) acquireEPUBInbox(ctx context.Context, reader readerstore.UserID, id, name string, mode epub.ImageMode) (epubstore.ImportReceipt, []string, error) {
	home, err := s.runtimes.readers.Open(ctx, reader)
	if err != nil {
		return epubstore.ImportReceipt{}, nil, err
	}
	store := epubstore.NewStore(home.DB(), home.Files())
	receipt, acquireErr := store.AcquireInbox(ctx, id, name, mode)
	if acquireErr != nil && receipt.State != epubstore.Acquired {
		return epubstore.ImportReceipt{Receipt: receipt}, nil, errors.Join(acquireErr, home.Close())
	}
	value, warnings := queueAcquiredEPUB(ctx, store, s.fileImports, reader, receipt)
	if acquireErr != nil {
		slog.Warn("EPUB acquired; inbox cleanup requires review", "reader_id", reader, "receipt_id", id, "error", acquireErr)
		warnings = append(warnings, "epub_inbox_cleanup_pending")
	}
	if err := home.Close(); err != nil {
		slog.Warn("EPUB inbox acquired; home cleanup needs attention", "reader_id", reader, "receipt_id", id, "error", err)
		warnings = append(warnings, "epub_acquisition_attention")
	}
	return value, warnings, nil
}
