package txtstore

import (
	"context"
	"errors"
	"os"
	"path"
)

func sameInboxFile(expected, current os.FileInfo) bool {
	return current.Mode().IsRegular() && os.SameFile(expected, current) && expected.Size() == current.Size() && expected.ModTime().Equal(current.ModTime())
}

// copyInbox is the cross-filesystem path after intent is committed. The inbox
// original remains untouched until the managed copy and receipt are complete.
func (s *Store) copyInbox(ctx context.Context, inbox, managed *os.Root, value Receipt, expected os.FileInfo) (Receipt, error) {
	input, err := inbox.Open(value.OriginalName)
	if err != nil {
		return s.failTransfer(ctx, managed, value, err)
	}
	current, err := input.Stat()
	if err == nil && !sameInboxFile(expected, current) {
		err = ErrInboxChanged
	}
	var size int64
	if err == nil {
		size, err = receiveWork(ctx, managed, workPath(value.ID), input)
	}
	if err == nil {
		current, err = input.Stat()
		if err == nil && (size != expected.Size() || !sameInboxFile(expected, current)) {
			err = ErrInboxChanged
		}
	}
	err = errors.Join(err, input.Close())
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
	if err := s.transition(finalCtx, value.ID, Receiving, Received, size, ""); err != nil {
		return value, err
	}
	value.State = Received
	value.Size = size
	current, err = inbox.Lstat(value.OriginalName)
	if err != nil {
		return value, err
	}
	if !sameInboxFile(expected, current) {
		return value, ErrInboxChanged
	}
	if err := inbox.Remove(value.OriginalName); err != nil {
		return value, err
	}
	return value, s.clearInboxClaim(finalCtx, value.ID)
}
