package epubstore

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path"
	"time"

	"github.com/otwako/novelreader/internal/epub"
	"github.com/otwako/novelreader/internal/inboxfiles"
)

var ErrInboxPending = errors.New("epubstore: inbox entry has an unresolved claim; review or release it")

// AcquireInbox consumes a completed file, never prepares or publishes it. The
// caller holds a home lease and shared admission for this server-issued ID.
// Producers must finish copying first. Acquired plus error means cleanup pending.
func (s *Store) AcquireInbox(ctx context.Context, id, name string, mode epub.ImageMode) (Receipt, error) {
	if err := validateID(id); err != nil {
		return Receipt{}, err
	}
	if err := ValidateFilename(name); err != nil {
		return Receipt{}, err
	}
	mode, err := acquisitionImageMode(mode)
	if err != nil {
		return Receipt{}, err
	}
	inbox, err := s.files.OpenInbox()
	if err != nil {
		return Receipt{}, err
	}
	defer inbox.Close()
	managed, err := s.files.OpenRoot()
	if err != nil {
		return Receipt{}, err
	}
	defer managed.Close()
	unlock, err := s.files.LockMutation(ctx)
	if err != nil {
		return Receipt{}, err
	}
	now := time.Now().UnixMilli()
	r := Receipt{ID: id, OriginalName: name, Path: originalPath(id), State: Receiving, ImageMode: mode, CreatedAt: now, UpdatedAt: now}
	info, r, err := s.claimInbox(ctx, inbox, managed, r)
	unlock()
	if err != nil || info == nil {
		return r, err
	}
	return s.copyInbox(ctx, inbox, managed, r, info)
}

func (s *Store) claimInbox(ctx context.Context, inbox, managed *os.Root, r Receipt) (os.FileInfo, Receipt, error) {
	var existing string
	err := s.db.QueryRowContext(ctx, `SELECT receipt_id FROM epub_inbox_claims WHERE name=?`, r.OriginalName).Scan(&existing)
	if err == nil {
		return nil, Receipt{ID: existing, OriginalName: r.OriginalName}, ErrInboxPending
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, r, err
	}
	info, err := inboxfiles.Inspect(inbox, r.OriginalName, epub.MaxInputBytes)
	if err != nil {
		return nil, r, err
	}
	r.Size = info.Size()
	if err = s.recordInboxIntent(ctx, r); err != nil {
		return nil, r, err
	}
	if err = managed.MkdirAll(path.Dir(r.Path), 0700); err != nil {
		return nil, r, err
	}
	finalCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), metadataTimeout)
	defer cancel()
	current, err := inbox.Lstat(r.OriginalName)
	if err != nil {
		return nil, r, err
	}
	if !inboxfiles.SameFile(info, current) {
		return nil, r, inboxfiles.ErrChanged
	}
	// An installed original must always be recoverable, even if metadata fails
	// immediately after rename. Receiving recovery intentionally discards partials.
	if err = s.transition(finalCtx, &r, Finalizing, ""); err != nil {
		return nil, r, err
	}
	err = s.files.MoveInboxTo(r.OriginalName, r.Path)
	if inboxfiles.IsCrossDevice(err) {
		// No bytes moved. A streaming copy is not complete-installation intent yet.
		err = s.transition(finalCtx, &r, Receiving, "")
		return info, r, err
	}
	if err != nil {
		return nil, r, err
	}
	if err = s.transition(finalCtx, &r, Acquired, ""); err != nil {
		return nil, r, err
	}
	return nil, r, s.clearInboxClaim(finalCtx, r.ID)
}

func (s *Store) copyInbox(ctx context.Context, inbox, managed *os.Root, r Receipt, expected os.FileInfo) (Receipt, error) {
	size, err := inboxfiles.CopyWork(ctx, inbox, managed, r.OriginalName, transferPath(r.ID), expected, epub.MaxInputBytes)
	r.Size = size
	if err != nil {
		failed, recordErr := s.recordFailure(ctx, managed, r, err)
		return failed, errors.Join(err, recordErr)
	}
	unlock, err := s.files.LockMutation(ctx)
	if err != nil {
		failed, recordErr := s.recordFailure(ctx, managed, r, err)
		return failed, errors.Join(err, recordErr)
	}
	defer unlock()
	finalCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), metadataTimeout)
	defer cancel()
	if err = s.transition(finalCtx, &r, Finalizing, ""); err != nil {
		return r, err
	}
	if err = s.installOriginal(managed, r); err != nil {
		return r, err
	}
	if err = s.transition(finalCtx, &r, Acquired, ""); err != nil {
		return r, err
	}
	current, err := inbox.Lstat(r.OriginalName)
	if err != nil {
		return r, err
	}
	if !inboxfiles.SameFile(expected, current) {
		return r, inboxfiles.ErrChanged
	}
	if err = inbox.Remove(r.OriginalName); err != nil {
		return r, err
	}
	return r, s.clearInboxClaim(finalCtx, r.ID)
}
