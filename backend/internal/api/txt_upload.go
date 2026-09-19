package api

import (
	"context"
	"errors"
	"log/slog"
	"mime"
	"net/http"
	"time"

	"github.com/otwako/novelreader/internal/auth"
	"github.com/otwako/novelreader/internal/readerstore"
	"github.com/otwako/novelreader/internal/txt"
	"github.com/otwako/novelreader/internal/txtstore"
)

func (s *Server) handleUploadTXT(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("filename")
	if err := txtstore.ValidateFilename(name); err != nil {
		writeTXTError(w, err)
		return
	}
	if r.ContentLength > txt.MaxInputBytes {
		writeTXTError(w, txtstore.ErrInputTooLarge)
		return
	}
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/octet-stream" {
		writeErrorCode(w, http.StatusUnsupportedMediaType, "txt_invalid_input", "Send one raw file as application/octet-stream")
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
	value, warnings, err := s.receiveTXTUpload(ctx, account.ID, id, name, w, r)
	// Retire the grant after cleanup, before a client can enqueue its next file.
	release()
	if err != nil {
		writeTXTError(w, err)
		return
	}
	writeTXTAcquired(w, value, warnings)
}

func (s *Server) receiveTXTUpload(ctx context.Context, readerID readerstore.UserID, id, name string, w http.ResponseWriter, r *http.Request) (txtstore.Receipt, []string, error) {
	home, err := s.runtimes.readers.Open(ctx, readerID)
	if err != nil {
		return txtstore.Receipt{}, nil, err
	}
	r.Body = http.MaxBytesReader(w, r.Body, txt.MaxInputBytes)
	finishBody, err := interruptTXTBody(ctx, w, r)
	if err != nil {
		return txtstore.Receipt{}, nil, errors.Join(err, home.Close())
	}
	value, receiveErr := txtstore.NewStore(home.DB(), home.Files()).Receive(ctx, id, name, r.Body)
	var warnings []string
	if receiveErr == nil {
		warnings = wakeTXTAnalysis(s.fileImports, readerID, id)
	}
	cleanupErr := errors.Join(finishBody(), home.Close())
	if receiveErr != nil {
		// A disconnected client already knows id and can inspect durable outcome.
		return value, nil, errors.Join(receiveErr, ctx.Err(), cleanupErr)
	}
	if cleanupErr != nil {
		slog.Warn("TXT acquired; request cleanup needs attention", "reader_id", readerID, "receipt_id", id, "error", cleanupErr)
		warnings = append(warnings, "txt_upload_attention")
	}
	return value, warnings, nil
}

// A context check before Read cannot interrupt an already blocked HTTP body read.
// Set a read deadline before closing; Close alone can wait on Read's HTTP/1 lock.
// Join the callback before resetting the deadline so it cannot affect keep-alive.
func interruptTXTBody(ctx context.Context, w http.ResponseWriter, r *http.Request) (func() error, error) {
	controller := http.NewResponseController(w)
	deadline, _ := ctx.Deadline() // Admission.Begin always supplies a deadline.
	setDeadline := func(at time.Time) error {
		err := controller.SetReadDeadline(at)
		if errors.Is(err, http.ErrNotSupported) {
			return nil
		} // In-memory handlers have no socket.
		return err
	}
	if err := setDeadline(deadline); err != nil {
		return nil, err
	}
	done := make(chan struct{})
	var interruptErr error
	stop := context.AfterFunc(ctx, func() {
		interruptErr = errors.Join(setDeadline(time.Now()), r.Body.Close())
		close(done)
	})
	return func() error {
		if !stop() {
			<-done
		}
		return errors.Join(interruptErr, setDeadline(time.Time{}))
	}, nil
}
