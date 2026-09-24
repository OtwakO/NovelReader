package txtstore

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path"
	"time"

	"github.com/otwako/novelreader/internal/inboxfiles"
	"github.com/otwako/novelreader/internal/readerstore"
	"github.com/otwako/novelreader/internal/txt"
)

var ErrInboxPending = errors.New("txtstore: inbox entry has an unresolved claim; review or explicitly retry it")
var ErrInboxChanged = inboxfiles.ErrChanged
var ErrInboxEntryMissing = inboxfiles.ErrMissing
var ErrInboxEntryType = inboxfiles.ErrType

type InboxClaim struct {
	Name      string
	ReceiptID string
}

// PendingInbox reports local claims even when their managed receipt was discarded.
// It never visits or deletes inbox files. Restored databases contain no claims.
func (s *Store) PendingInbox(ctx context.Context, after string, limit int) ([]InboxClaim, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT name,receipt_id FROM txt_inbox_claims WHERE name>? ORDER BY name LIMIT ?`, after, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []InboxClaim
	for rows.Next() {
		var value InboxClaim
		if err := rows.Scan(&value.Name, &value.ReceiptID); err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	return result, rows.Err()
}

func (s *Store) GetInboxClaim(ctx context.Context, id string) (InboxClaim, error) {
	var claim InboxClaim
	err := s.db.QueryRowContext(ctx, `SELECT name,receipt_id FROM txt_inbox_claims WHERE receipt_id=?`, id).Scan(&claim.Name, &claim.ReceiptID)
	if errors.Is(err, sql.ErrNoRows) {
		return claim, ErrNotFound
	}
	return claim, err
}

// AcquireInbox consumes one completed file, recording intent before any move.
// The caller supplies a server-issued ID, as with browser Receive.
// The caller holds a Home lease. Concurrent producers are outside the completed-
// copy contract. An existing claim is never silently replayed or re-imported.
// A received receipt with an error is durable content with unfinished cleanup.
func (s *Store) AcquireInbox(ctx context.Context, id, name string) (Receipt, error) {
	value := Receipt{ID: id, OriginalName: name, State: Receiving, CreatedAt: time.Now().UnixMilli()}
	var err error
	value.Path, err = managedPath(name, value.ID)
	if err != nil {
		return Receipt{}, err
	}
	value.UpdatedAt = value.CreatedAt
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
	// The copy fallback releases this gate while streaming.
	copyInfo, value, err := s.claimInbox(ctx, inbox, managed, value)
	unlock()
	if err != nil || copyInfo == nil {
		return value, err
	}
	return s.copyInbox(ctx, inbox, managed, value, copyInfo)
}

func (s *Store) claimInbox(ctx context.Context, inbox, managed *os.Root, value Receipt) (os.FileInfo, Receipt, error) {
	var existing string
	err := s.db.QueryRowContext(ctx, `SELECT receipt_id FROM txt_inbox_claims WHERE name=?`, value.OriginalName).Scan(&existing)
	if err == nil {
		return nil, Receipt{ID: existing, OriginalName: value.OriginalName}, ErrInboxPending
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, value, err
	}
	info, err := inboxfiles.Inspect(inbox, value.OriginalName, txt.MaxInputBytes)
	if errors.Is(err, readerstore.ErrFileTooLarge) {
		err = errors.Join(ErrInputTooLarge, err)
	}
	if err != nil {
		return nil, value, err
	}
	value.Size = info.Size()
	if err := s.recordInboxIntent(ctx, value); err != nil {
		return nil, value, err
	}
	if err := managed.MkdirAll(path.Dir(value.Path), 0o700); err != nil {
		return nil, value, err
	}
	finalCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), metadataTimeout)
	defer cancel()
	current, err := inbox.Lstat(value.OriginalName)
	if err != nil {
		return nil, value, err
	}
	if !inboxfiles.SameFile(info, current) {
		return nil, value, ErrInboxChanged
	}
	err = s.files.MoveInboxTo(value.OriginalName, value.Path)
	if inboxfiles.IsCrossDevice(err) {
		return info, value, nil
	}
	if err != nil {
		return nil, value, err
	}
	if err := s.finalizeAcquisition(finalCtx, value.ID, value.Size); err != nil {
		return nil, value, err
	}
	stored, err := s.Get(finalCtx, value.ID)
	if err != nil {
		return nil, value, err
	}
	return nil, stored, s.clearInboxClaim(finalCtx, value.ID)
}

func (s *Store) recordInboxIntent(ctx context.Context, value Receipt) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `INSERT INTO txt_files(id,original_name,path,state,size,created_at,updated_at) VALUES(?,?,?,?,?,?,?)`, value.ID, value.OriginalName, value.Path, Receiving, value.Size, value.CreatedAt, value.UpdatedAt); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO txt_inbox_claims(name,receipt_id) VALUES(?,?)`, value.OriginalName, value.ID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) clearInboxClaim(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM txt_inbox_claims WHERE receipt_id=?`, id)
	return err
}
