package api

import (
	"context"
	"errors"
	"log/slog"

	"github.com/otwako/novelreader/internal/readerstore"
	"github.com/otwako/novelreader/internal/txtimport"
)

const readerRuntimeCapacity = 32

// ReaderHomeCapacity budgets separate foreground, analysis and transfer leases.
// Waiting admission tickets never reserve or open reader homes.
const ReaderHomeCapacity = readerRuntimeCapacity + txtimport.Workers + txtimport.Transfers

func (s *Server) quiesceReader(ctx context.Context, id readerstore.UserID) error {
	// Stop intake first so no new transfers enter while foreground work drains.
	// All owners must drain before replacement/removal; errors retain barriers.
	intakeErr := s.txtAdmission.Quiesce(ctx, id)
	runtimeErr := s.runtimes.quiesce(ctx, id)
	return errors.Join(intakeErr, runtimeErr, s.txtImports.Quiesce(ctx, id))
}

func (s *Server) resumeReader(id readerstore.UserID) {
	s.txtImports.Resume(id)
	s.runtimes.resume(id)
	s.txtAdmission.Resume(id)
}

func (s *Server) forgetReader(id readerstore.UserID) error {
	if err := errors.Join(s.txtAdmission.Forget(id), s.txtImports.Forget(id)); err != nil {
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
