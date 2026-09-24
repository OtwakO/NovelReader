package epubstore

import (
	"context"
	"errors"
	"io"
	"os"
	"path"
	"time"

	"github.com/otwako/novelreader/internal/epub"
	"github.com/otwako/novelreader/internal/readerstore"
)

// Receive retains original bytes, without interpreting or publishing them. IDs
// are server-issued crypto/rand.Text values. A duplicate ID never overwrites an
// existing original. The caller owns input timeouts and the home lease.
func (s *Store) Receive(ctx context.Context, id, name string, input io.Reader, mode epub.ImageMode) (value Receipt, err error) {
	if err = validateID(id); err != nil {
		return Receipt{}, err
	}
	if err = ValidateFilename(name); err != nil {
		return Receipt{}, err
	}
	mode, err = acquisitionImageMode(mode)
	if err != nil {
		return Receipt{}, err
	}
	root, err := s.files.OpenRoot()
	if err != nil {
		return Receipt{}, err
	}
	defer func() { err = errors.Join(err, root.Close()) }()
	now := time.Now().UnixMilli()
	value = Receipt{ID: id, OriginalName: name, Path: originalPath(id), State: Receiving, ImageMode: mode, CreatedAt: now, UpdatedAt: now}
	_, err = s.db.ExecContext(ctx, `INSERT INTO epub_files(id,original_name,state,image_mode,created_at,updated_at) VALUES(?,?,?,?,?,?)`, id, name, Receiving, mode, now, now)
	if err != nil {
		return Receipt{}, err
	}
	value.Size, err = readerstore.WriteWorkFile(ctx, root, transferPath(id), input, epub.MaxInputBytes)
	if err != nil {
		failed, recordErr := s.recordFailure(ctx, root, value, err)
		return failed, errors.Join(err, recordErr)
	}
	unlock, err := s.files.LockMutation(ctx)
	if err != nil {
		failed, recordErr := s.recordFailure(ctx, root, value, err)
		return failed, errors.Join(err, recordErr)
	}
	defer unlock()
	// Finish the short move/metadata sequence after a client disconnect. Persist
	// complete-transfer intent first; a crash on either side of Rename is recoverable.
	finalCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), metadataTimeout)
	defer cancel()
	if err = s.transition(finalCtx, &value, Finalizing, ""); err != nil {
		return value, err
	}
	if err = s.installOriginal(root, value); err != nil {
		return value, err
	}
	if err = s.transition(finalCtx, &value, Acquired, ""); err != nil {
		return value, err
	}
	return s.Get(finalCtx, id)
}

func (s *Store) installOriginal(root *os.Root, r Receipt) error {
	if err := root.MkdirAll(path.Dir(r.Path), 0700); err != nil {
		return err
	}
	return root.Rename(transferPath(r.ID), r.Path)
}

func (s *Store) recordFailure(ctx context.Context, root *os.Root, r Receipt, cause error) (Receipt, error) {
	cleanupErr := removeTransfer(root, r.ID)
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), metadataTimeout)
	defer cancel()
	message := errors.Join(cause, cleanupErr).Error()
	stateErr := s.transition(cleanupCtx, &r, Failed, message)
	return r, errors.Join(cleanupErr, stateErr)
}

func removeTransfer(root *os.Root, id string) error {
	err := root.Remove(transferPath(id))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func acquisitionImageMode(mode epub.ImageMode) (epub.ImageMode, error) {
	if mode == "" {
		mode = epub.OriginalImages
	}
	if mode != epub.OriginalImages && mode != epub.OptimizedImages {
		return "", epub.ErrImagePolicy
	}
	return mode, nil
}
