package epubstore

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"

	"github.com/otwako/novelreader/internal/epub"
)

var errInterruptedTransfer = errors.New("epubstore: transfer interrupted; upload again or discard the receipt")

// RecoverAcquisitions requires quiescent intake. It settles only persisted
// acquisition/removal intents; it neither scans unknown files nor prepares books.
func (s *Store) RecoverAcquisitions(ctx context.Context) (err error) {
	unlock, err := s.files.LockMutation(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	root, err := s.files.OpenRoot()
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, root.Close()) }()
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM epub_files WHERE state != 'acquired' ORDER BY id`)
	if err != nil {
		return err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			return errors.Join(err, rows.Close())
		}
		ids = append(ids, id)
	}
	if err = errors.Join(rows.Err(), rows.Close()); err != nil {
		return err
	}
	var failures []error
	for _, id := range ids {
		if err = ctx.Err(); err != nil {
			return errors.Join(append(failures, err)...)
		}
		r, getErr := s.Get(ctx, id)
		if getErr == nil {
			getErr = s.recoverAcquisition(ctx, root, r)
		}
		if getErr != nil {
			failures = append(failures, fmt.Errorf("epubstore: recover %s: %w", id, getErr))
		}
	}
	return errors.Join(failures...)
}

func (s *Store) recoverAcquisition(ctx context.Context, root *os.Root, r Receipt) error {
	switch r.State {
	case Removing:
		return s.finishRemoval(ctx, root, r)
	case Failed:
		return removeTransfer(root, r.ID)
	case Receiving:
		_, err := s.recordFailure(ctx, root, r, errInterruptedTransfer)
		return err
	case Finalizing:
		info, err := root.Lstat(r.Path)
		pending := errors.Is(err, os.ErrNotExist)
		if pending {
			info, err = root.Lstat(transferPath(r.ID))
		}
		if errors.Is(err, os.ErrNotExist) {
			_, err = s.recordFailure(ctx, root, r, errInterruptedTransfer)
			return err
		}
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() || info.Size() != r.Size || r.Size > epub.MaxInputBytes {
			_, err = s.recordFailure(ctx, root, r, errors.New("epubstore: original does not match acquisition intent"))
			return err
		}
		if pending {
			if err = s.installOriginal(root, r); err != nil {
				return err
			}
		}
		if err = removeTransfer(root, r.ID); err != nil {
			return err
		}
		return s.transition(ctx, &r, Acquired, "")
	}
	return nil
}

// Discard removes an acquisition, not a library publication. Callers must first
// cancel and join work for this receipt. A failed cleanup retains removal intent.
func (s *Store) Discard(ctx context.Context, id string) error {
	_, err := s.discard(ctx, id, false)
	return err
}

// DiscardPending is safe with live workers: acquisition must be joined by the
// caller, and running/finalizing preparation is rejected under the claim gate.
func (s *Store) DiscardPending(ctx context.Context, id string) (bool, error) {
	return s.discard(ctx, id, true)
}

func (s *Store) discard(ctx context.Context, id string, checkActive bool) (pending bool, err error) {
	unlock, err := s.files.LockMutation(ctx)
	if err != nil {
		return false, err
	}
	defer unlock()
	r, err := s.Get(ctx, id)
	if errors.Is(err, ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if r.LibraryID != "" {
		return false, ErrStateChanged
	}
	if checkActive && r.PreparationGeneration > 0 {
		attempt, err := s.GetPreparation(ctx, id, r.PreparationGeneration)
		if err != nil {
			return false, err
		}
		if attempt.State == PreparationRunning || attempt.State == PreparationFinalizing {
			return false, ErrStateChanged
		}
	}
	root, err := s.files.OpenRoot()
	if err != nil {
		return false, err
	}
	defer func() { err = errors.Join(err, root.Close()) }()
	if err = s.transition(ctx, &r, Removing, ""); err != nil {
		return false, err
	}
	err = s.finishRemoval(ctx, root, r)
	return err != nil, err
}

func (s *Store) finishRemoval(ctx context.Context, root *os.Root, r Receipt) error {
	if err := errors.Join(root.RemoveAll(path.Dir(r.Path)), root.RemoveAll(receiptPreparationWorkPath(r.ID)), removeTransfer(root, r.ID)); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, `DELETE FROM epub_files WHERE id=? AND state='removing'`, r.ID)
	return err
}

// Recover runs only with intake and workers quiescent. Both format-specific
// recovery phases retain their own cleanup and mutation-gate ownership.
func (s *Store) Recover(ctx context.Context) error {
	acquisitionErr := s.RecoverAcquisitions(ctx)
	return errors.Join(acquisitionErr, s.RecoverPreparations(ctx))
}
