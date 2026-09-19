package txtstore

import (
	"context"
	"errors"
	"os"
	"path"

	"github.com/otwako/novelreader/internal/inboxfiles"
	"github.com/otwako/novelreader/internal/readerstore"
	"github.com/otwako/novelreader/internal/txt"
)

// copyInbox is the cross-filesystem path after intent is committed. The inbox
// original remains untouched until the managed copy and receipt are complete.
func (s *Store) copyInbox(ctx context.Context, inbox, managed *os.Root, value Receipt, expected os.FileInfo) (Receipt, error) {
	size, err := inboxfiles.CopyWork(ctx, inbox, managed, value.OriginalName, workPath(value.ID), expected, txt.MaxInputBytes)
	if errors.Is(err, readerstore.ErrFileTooLarge) {
		err = errors.Join(ErrInputTooLarge, err)
	}
	if err != nil {
		return s.failTransfer(ctx, managed, value, err)
	}
	unlock, err := s.files.LockMutation(ctx)
	if err != nil {
		return s.failTransfer(ctx, managed, value, err)
	}
	defer unlock()
	finalCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), metadataTimeout)
	defer cancel()
	// Reserve current receipt ownership before moving completed bytes.
	if err := s.transition(finalCtx, value.ID, Receiving, Receiving, size, ""); err != nil {
		return value, err
	}
	if err := managed.MkdirAll(path.Dir(value.Path), 0o700); err != nil {
		return value, err
	}
	if err := managed.Rename(workPath(value.ID), value.Path); err != nil {
		return value, err
	}
	if err := s.finalizeAcquisition(finalCtx, value.ID, size); err != nil {
		return value, err
	}
	stored, err := s.Get(finalCtx, value.ID)
	if err != nil {
		return value, err
	}
	value = stored
	current, err := inbox.Lstat(value.OriginalName)
	if err != nil {
		return value, err
	}
	if !inboxfiles.SameFile(expected, current) {
		return value, ErrInboxChanged
	}
	if err := inbox.Remove(value.OriginalName); err != nil {
		return value, err
	}
	return value, s.clearInboxClaim(finalCtx, value.ID)
}
