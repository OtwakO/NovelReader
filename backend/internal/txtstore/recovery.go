package txtstore

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"

	"github.com/otwako/novelreader/internal/txt"
)

// Discard retains a removal record until all owned bytes have been removed.
// It is idempotent. Callers cancel any active transfer before discarding it.
func (s *Store) Discard(ctx context.Context, id string) error {
	unlock, err := s.files.LockMutation(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	value, err := s.Get(ctx, id)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := validateReceiptPath(value); err != nil {
		return err
	}
	root, err := s.files.OpenRoot()
	if err != nil {
		return err
	}
	defer root.Close()
	if err := s.transition(ctx, id, value.State, Removing, value.Size, ""); err != nil {
		return err
	}
	return s.finishRemoval(ctx, root, value)
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
// unfinished acquisition/removal records; ordinary section opens never call it.
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
	return s.transition(ctx, id, Receiving, Received, value.Size, "")
}

var errInterruptedTransfer = errors.New("txtstore: transfer interrupted; upload the file again or discard this receipt")
