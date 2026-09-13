package api

import (
	"context"
	"errors"
	"log/slog"

	"github.com/otwako/novelreader/internal/readerstore"
	"github.com/otwako/novelreader/internal/txtimport"
)

const readerRuntimeCapacity = 32

// ReaderHomeCapacity includes independent active worker homes, not just cached
// API runtimes. Upload intake must add its own bounded allowance when exposed.
const ReaderHomeCapacity = readerRuntimeCapacity + txtimport.Workers

func (s *Server) quiesceReader(ctx context.Context, id readerstore.UserID) error {
	// Gate new HTTP requests before cancelling work. No replacement/removal is
	// allowed unless both owners drained; the caller resumes or retries on error.
	runtimeErr := s.runtimes.quiesce(ctx, id)
	return errors.Join(runtimeErr, s.txtImports.Quiesce(ctx, id))
}

func (s *Server) resumeReader(id readerstore.UserID) {
	s.txtImports.Resume(id)
	s.runtimes.resume(id)
}

func (s *Server) forgetReader(id readerstore.UserID) error {
	if err := s.txtImports.Forget(id); err != nil {
		return err
	}
	// The home has been removed and account admission disabled. Release the
	// API barrier too; a stale request cannot create a missing reader home.
	s.runtimes.resume(id)
	return nil
}

func (s *Server) recoverRestoredTXT(ctx context.Context, id readerstore.UserID) []string {
	if err := txtimport.RecoverHome(ctx, s.runtimes.readers, id); err != nil {
		slog.Warn("Reader data restored; TXT recovery incomplete", "reader_id", id, "error", err)
		return []string{"txt_recovery_incomplete"}
	}
	return nil
}
