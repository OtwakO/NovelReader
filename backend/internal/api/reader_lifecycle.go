package api

import (
	"context"
	"errors"
	"log/slog"

	"github.com/otwako/novelreader/internal/fileimport"
	"github.com/otwako/novelreader/internal/readerstore"
)

const readerRuntimeCapacity = 32

// ReaderHomeCapacity budgets separate foreground, analysis and transfer leases.
// Waiting admission tickets never reserve or open reader homes.
const ReaderHomeCapacity = readerRuntimeCapacity + fileimport.Workers + fileimport.Transfers

func (s *Server) quiesceReader(ctx context.Context, id readerstore.UserID) error {
	// Stop intake first so no new transfers enter while foreground work drains.
	// All owners must drain before replacement/removal; errors retain barriers.
	intakeErr := s.fileAdmission.Quiesce(ctx, id)
	runtimeErr := s.runtimes.quiesce(ctx, id)
	if runtimeErr == nil {
		s.services.fileInbox.invalidate(id)
	}
	return errors.Join(intakeErr, runtimeErr, s.fileImports.Quiesce(ctx, id))
}

func (s *Server) resumeReader(id readerstore.UserID) {
	s.fileImports.Resume(id)
	s.runtimes.resume(id)
	s.fileAdmission.Resume(id)
}

func (s *Server) forgetReader(id readerstore.UserID) error {
	if err := errors.Join(s.fileAdmission.Forget(id), s.fileImports.Forget(id)); err != nil {
		return err
	}
	s.services.fileInbox.invalidate(id)
	// The home has been removed and account admission disabled. Release the
	// API barrier too; a stale request cannot create a missing reader home.
	s.runtimes.resume(id)
	return nil
}

func (s *Server) recoverRestoredImports(ctx context.Context, id readerstore.UserID) []string {
	if err := fileimport.RecoverHome(ctx, s.runtimes.readers, id); err != nil {
		slog.Warn("Reader data restored; import recovery incomplete", "reader_id", id, "error", err)
		var warnings []string
		if errors.Is(err, fileimport.ErrTXTRecovery) {
			warnings = append(warnings, "txt_recovery_incomplete")
		}
		if errors.Is(err, fileimport.ErrEPUBRecovery) {
			warnings = append(warnings, "epub_recovery_incomplete")
		}
		if len(warnings) == 0 {
			warnings = append(warnings, "import_recovery_incomplete")
		}
		return warnings
	}
	return nil
}
