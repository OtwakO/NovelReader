package txtstore

import (
	"context"
	"errors"
	"fmt"
	"github.com/otwako/novelreader/internal/library"
	"os"
	"path"
	"time"

	"github.com/otwako/novelreader/internal/txt"
)

// Discard retains a removal record until all owned bytes have been removed.
// It is idempotent. Callers cancel any active transfer before discarding it.
func (s *Store) Discard(ctx context.Context, id string) error {
	_, err := s.discard(ctx, id, removeAny)
	return err
}

// RemovePublication also retries a retained removal record. A true return means
// the publication is hidden but cleanup failed; the error must still be reported.
// Library IDs equal receipt IDs by schema, including after library_id is cleared.
func (s *Store) RemovePublication(ctx context.Context, id string) (cleanupPending bool, err error) {
	return s.discard(ctx, id, removePublished)
}

// DiscardPending cannot remove a publication, including one accepted concurrently.
// The caller first cancels and joins acquisition for this receipt.
func (s *Store) DiscardPending(ctx context.Context, id string) (cleanupPending bool, err error) {
	return s.discard(ctx, id, removePending)
}

type removalScope int

const (
	removeAny removalScope = iota
	removePublished
	removePending
)

func (s *Store) discard(ctx context.Context, id string, scope removalScope) (bool, error) {
	unlock, err := s.files.LockMutation(ctx)
	if err != nil {
		return false, err
	}
	defer unlock()
	value, err := s.Get(ctx, id)
	if errors.Is(err, ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if scope == removePublished && value.LibraryID == "" && value.State != Removing {
		return false, ErrNotFound
	}
	if scope == removePending && (value.LibraryID != "" || value.State == Published) {
		return false, ErrStateChanged
	}
	if err := validateReceiptPath(value); err != nil {
		return false, err
	}
	root, err := s.files.OpenRoot()
	if err != nil {
		return false, err
	}
	defer root.Close()
	if err := s.beginRemoval(ctx, value); err != nil {
		return false, err
	}
	err = s.finishRemoval(ctx, root, value)
	return err != nil, err
}

func (s *Store) finishRemoval(ctx context.Context, root *os.Root, value Receipt) error {
	err := errors.Join(removeIfPresent(root, value.Path), removeIfPresent(root, workPath(value.ID)))
	if err == nil {
		err = removeIfPresent(root, path.Dir(value.Path))
	}
	if err != nil {
		return errors.Join(err, s.transition(ctx, value.ID, Removing, Removing, value.Size, err.Error()))
	}
	_, err = s.db.ExecContext(ctx, `DELETE FROM txt_files WHERE id=? AND state=?`, value.ID, Removing)
	return err
}

// Recover runs with intake quiescent, before workers are admitted. It only visits
// unfinished acquisitions, removals and candidate attempts; section opens never call it.
// It never touches external inbox paths or deletes an unreferenced managed file.
func (s *Store) Recover(ctx context.Context) error {
	unlock, err := s.files.LockMutation(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	root, err := s.files.OpenRoot()
	if err != nil {
		return err
	}
	defer root.Close()
	// Recovery revokes attempts, not requests. Active generations never change.
	if _, err := s.db.ExecContext(ctx, `UPDATE txt_interpretations SET state='queued',error='txtstore: analysis interrupted; retry',updated_at=? WHERE role='candidate' AND state='analyzing'`, time.Now().UnixMilli()); err != nil {
		return err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM txt_files WHERE state IN (?,?) ORDER BY id`, Receiving, Removing)
	if err != nil {
		return err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	err = errors.Join(rows.Err(), rows.Close())
	if err != nil {
		return err
	}
	var failures []error
	for _, id := range ids {
		if err := ctx.Err(); err != nil {
			return errors.Join(append(failures, err)...)
		}
		if err := s.recoverReceipt(ctx, root, id); err != nil {
			failures = append(failures, fmt.Errorf("txtstore: recover receipt %s: %w", id, err))
		}
	}
	return errors.Join(failures...)
}

func (s *Store) recoverReceipt(ctx context.Context, root *os.Root, id string) error {
	value, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	if err := validateReceiptPath(value); err != nil {
		return err
	}
	if value.State == Removing {
		return s.finishRemoval(ctx, root, value)
	}
	info, err := root.Lstat(value.Path)
	if errors.Is(err, os.ErrNotExist) {
		// An incomplete transfer is a per-file failure, not a failed recovery pass.
		_, err := s.recordFailure(ctx, root, value, errInterruptedTransfer)
		return err
	}
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Size() != value.Size || info.Size() > txt.MaxInputBytes {
		_, err := s.recordFailure(ctx, root, value, fmt.Errorf("managed original does not match acquisition intent"))
		return err
	}
	return s.finalizeAcquisition(ctx, id, value.Size)
}

var errInterruptedTransfer = errors.New("txtstore: transfer interrupted; upload the file again or discard this receipt")

// Hide a publication and retain its file-removal record in one transaction.
func (s *Store) beginRemoval(ctx context.Context, value Receipt) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE txt_files SET state=?,error='',updated_at=? WHERE id=? AND COALESCE(library_id,'')=?`, Removing, time.Now().UnixMilli(), value.ID, value.LibraryID)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return ErrStateChanged
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM txt_interpretations WHERE file_id=?`, value.ID); err != nil {
		return err
	}
	if value.LibraryID != "" {
		if err := library.DeleteTx(ctx, tx, value.LibraryID); err != nil {
			return err
		}
	}
	return tx.Commit()
}
