package api

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/otwako/novelreader/internal/auth"
	backupservice "github.com/otwako/novelreader/internal/backup"
)

func (s *Server) handleBackupExport(w http.ResponseWriter, r *http.Request) {
	account, ok := s.backupIdentity(w, r, auth.BackupExport)
	if !ok {
		return
	}
	createdAt := s.backupsNow()
	filename := backupservice.Filename(account.Username, createdAt)
	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("Content-Disposition", contentDisposition(filename))
	w.Header().Set("Cache-Control", "no-store")
	if _, err := s.backups.Export(r.Context(), account.ID, account.Username, createdAt, w); err != nil {
		// Streaming may already have started; closing the response is safer than appending JSON to an archive.
		return
	}
}

func (s *Server) handlePrepareBackupRestore(w http.ResponseWriter, r *http.Request) {
	account, ok := s.backupIdentity(w, r, auth.BackupRestore)
	if !ok {
		return
	}
	summary, err := s.backups.PrepareRestore(r.Context(), account.ID, http.MaxBytesReader(w, r.Body, 2<<30))
	switch {
	case errors.Is(err, backupservice.ErrRestoreConflict):
		writeErrorCode(w, http.StatusConflict, "restore_conflict", "a restore is already prepared")
	case err != nil:
		writeErrorCode(w, http.StatusBadRequest, "invalid_backup", err.Error())
	default:
		writeJSON(w, http.StatusCreated, summary)
	}
}

func (s *Server) handleGetBackupRestore(w http.ResponseWriter, r *http.Request) {
	account, ok := s.backupIdentity(w, r, auth.BackupRestore)
	if !ok {
		return
	}
	summary, err := s.backups.GetRestore(account.ID, r.PathValue("id"))
	if errors.Is(err, backupservice.ErrRestoreNotFound) {
		writeErrorCode(w, http.StatusNotFound, "restore_not_found", "restore operation not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "restore unavailable")
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

func (s *Server) handleCancelBackupRestore(w http.ResponseWriter, r *http.Request) {
	account, ok := s.backupIdentity(w, r, auth.BackupRestore)
	if !ok {
		return
	}
	if err := s.backups.CancelRestore(account.ID, r.PathValue("id")); err != nil {
		writeErrorCode(w, http.StatusNotFound, "restore_not_found", "restore operation not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleCommitBackupRestore(w http.ResponseWriter, r *http.Request) {
	account, ok := s.backupIdentity(w, r, auth.BackupRestore)
	if !ok {
		return
	}
	result, err := s.backups.CommitRestore(r.Context(), account.ID, r.PathValue("id"))
	if errors.Is(err, backupservice.ErrRestoreNotFound) {
		writeErrorCode(w, http.StatusNotFound, "restore_not_found", "restore operation not found")
		return
	}
	if err != nil {
		w.Header().Set("Retry-After", "1")
		writeErrorCode(w, http.StatusServiceUnavailable, "restore_failed", "Reader Data restore failed; previous data remains active")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) backupIdentity(w http.ResponseWriter, r *http.Request, _ auth.BackupScope) (auth.Account, bool) {
	account, ok := auth.IdentityFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return auth.Account{}, false
	}
	return account, true
}

func contentDisposition(filename string) string {
	fallback := "novelreader-reader-backup.tar.gz"
	return fmt.Sprintf("attachment; filename=%q; filename*=UTF-8''%s", fallback, url.PathEscape(filename))
}

func (s *Server) backupsNow() time.Time { return time.Now().In(time.Local) }
