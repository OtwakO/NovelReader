package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/otwako/novelreader/internal/auth"
	"github.com/otwako/novelreader/internal/fileimport"
	"github.com/otwako/novelreader/internal/readerstore"
	"github.com/otwako/novelreader/internal/txtstore"
)

// Acquire one completed inbox file under the same admission/home budget as a
// browser transfer. A scan never authorizes consumption or bypasses journal checks.
func (s *Server) handleAcquireTXTInbox(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("filename")
	if err := txtstore.ValidateFilename(name); err != nil {
		writeTXTError(w, err)
		return
	}
	account, _ := auth.IdentityFromContext(r.Context())
	id := r.PathValue("id")
	ctx, release, err := s.fileAdmission.Begin(r.Context(), account.ID, id)
	if err != nil {
		writeTXTError(w, err)
		return
	}
	defer release()
	value, warnings, err := s.acquireTXTInbox(ctx, account.ID, id, name)
	release()
	if errors.Is(err, txtstore.ErrInboxPending) {
		writeJSON(w, http.StatusConflict, map[string]string{"code": "txt_inbox_pending", "error": "Review the existing inbox claim before importing again", "receiptId": value.ID})
		return
	}
	if err != nil {
		writeTXTError(w, err)
		return
	}
	writeTXTAcquired(w, value, warnings)
}

func (s *Server) acquireTXTInbox(ctx context.Context, reader readerstore.UserID, id, name string) (txtstore.Receipt, []string, error) {
	home, err := s.runtimes.readers.Open(ctx, reader)
	if err != nil {
		return txtstore.Receipt{}, nil, err
	}
	value, acquireErr := txtstore.NewStore(home.DB(), home.Files()).AcquireInbox(ctx, id, name)
	if acquireErr != nil && value.State != txtstore.Received {
		return value, nil, errors.Join(acquireErr, home.Close())
	}
	warnings := wakeTXTAnalysis(s.fileImports, reader, id)
	if acquireErr != nil {
		slog.Warn("TXT acquired; inbox cleanup requires review", "reader_id", reader, "receipt_id", id, "error", acquireErr)
		warnings = append(warnings, "txt_inbox_cleanup_pending")
	}
	if err := home.Close(); err != nil {
		slog.Warn("TXT inbox acquired; home cleanup needs attention", "reader_id", reader, "receipt_id", id, "error", err)
		warnings = append(warnings, "txt_acquisition_attention")
	}
	return value, warnings, nil
}

func wakeTXTAnalysis(pool *fileimport.Pool, reader readerstore.UserID, id string) []string {
	if err := pool.Notify(reader); err != nil {
		slog.Warn("TXT work retained; analysis wake-up failed", "reader_id", reader, "receipt_id", id, "error", err)
		return []string{"txt_analysis_pending"}
	}
	return nil
}
