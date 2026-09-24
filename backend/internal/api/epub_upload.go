package api

import (
	"context"
	"errors"
	"log/slog"
	"mime"
	"net/http"
	"time"

	"github.com/otwako/novelreader/internal/auth"
	"github.com/otwako/novelreader/internal/epub"
	"github.com/otwako/novelreader/internal/epubstore"
	"github.com/otwako/novelreader/internal/fileimport"
	"github.com/otwako/novelreader/internal/readerstore"
)

func (s *Server) handleUploadEPUB(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("filename")
	mode := epub.ImageMode(r.URL.Query().Get("imageMode"))
	if mode == "" {
		mode = epub.OriginalImages
	}
	if err := epubstore.ValidateFilename(name); err != nil {
		writeEPUBError(w, err)
		return
	}
	if mode != epub.OriginalImages && mode != epub.OptimizedImages {
		writeEPUBError(w, epub.ErrImagePolicy)
		return
	}
	if r.ContentLength > epub.MaxInputBytes {
		writeEPUBError(w, readerstore.ErrFileTooLarge)
		return
	}
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/octet-stream" {
		writeErrorCode(w, http.StatusUnsupportedMediaType, "epub_invalid_input", "Send one raw file as application/octet-stream")
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
	value, warnings, err := s.receiveEPUBUpload(ctx, account.ID, id, name, mode, w, r)
	release() // Retire only after request/file/home cleanup.
	if err != nil {
		writeEPUBError(w, err)
		return
	}
	writeEPUBAcquired(w, value, warnings)
}

func writeEPUBAcquired(w http.ResponseWriter, value epubstore.ImportReceipt, warnings []string) {
	w.Header().Set("Location", "/api/imports/epub/receipts/"+value.ID)
	writeJSON(w, http.StatusCreated, struct {
		Receipt  epubReceiptResponse `json:"receipt"`
		Warnings []string            `json:"warnings,omitempty"`
	}{epubReceiptDTO(value), warnings})
}

func (s *Server) receiveEPUBUpload(ctx context.Context, reader readerstore.UserID, id, name string, mode epub.ImageMode, w http.ResponseWriter, r *http.Request) (epubstore.ImportReceipt, []string, error) {
	home, err := s.runtimes.readers.Open(ctx, reader)
	if err != nil {
		return epubstore.ImportReceipt{}, nil, err
	}
	r.Body = http.MaxBytesReader(w, r.Body, epub.MaxInputBytes)
	finishBody, err := interruptImportBody(ctx, w, r)
	if err != nil {
		return epubstore.ImportReceipt{}, nil, errors.Join(err, home.Close())
	}
	store := epubstore.NewStore(home.DB(), home.Files())
	receipt, receiveErr := store.Receive(ctx, id, name, r.Body, mode)
	value := epubstore.ImportReceipt{Receipt: receipt}
	var warnings []string
	if receiveErr == nil {
		value, warnings = queueAcquiredEPUB(ctx, store, s.fileImports, reader, receipt)
	}
	cleanupErr := errors.Join(finishBody(), home.Close())
	if receiveErr != nil {
		return value, nil, errors.Join(receiveErr, ctx.Err(), cleanupErr)
	}
	if cleanupErr != nil {
		slog.Warn("EPUB acquired; request cleanup needs attention", "reader_id", reader, "receipt_id", id, "error", cleanupErr)
		warnings = append(warnings, "epub_upload_attention")
	}
	return value, warnings, nil
}

func wakeEPUBPreparation(pool *fileimport.Pool, reader readerstore.UserID, id string) []string {
	if err := pool.Notify(reader); err != nil {
		slog.Warn("EPUB work retained; preparation wake-up failed", "reader_id", reader, "receipt_id", id, "error", err)
		return []string{"epub_preparation_pending"}
	}
	return nil
}

// Initial preparation is shared by browser/inbox acquisition and explicit
// recovery review. A failed queue write leaves a durable, retryable receipt.
func queueAcquiredEPUB(ctx context.Context, store *epubstore.Store, pool *fileimport.Pool, reader readerstore.UserID, receipt epubstore.Receipt) (epubstore.ImportReceipt, []string) {
	value := epubstore.ImportReceipt{Receipt: receipt}
	var warnings []string
	// Acquisition is durable. A disconnect must not lose the short initial queue
	// write; an unsuccessful queue remains an acquired receipt for explicit retry.
	queueCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	attempt, queueErr := store.RetryPreparation(queueCtx, receipt.ID, 0)
	cancel()
	if queueErr != nil {
		slog.Warn("EPUB acquired; preparation queue needs attention", "reader_id", reader, "receipt_id", receipt.ID, "error", queueErr)
		warnings = append(warnings, "epub_preparation_pending")
	} else {
		value.PreparationGeneration, value.PreparationState = attempt.Generation, attempt.State
		value.UpdatedAt = attempt.UpdatedAt
		warnings = wakeEPUBPreparation(pool, reader, receipt.ID)
	}
	return value, warnings
}
