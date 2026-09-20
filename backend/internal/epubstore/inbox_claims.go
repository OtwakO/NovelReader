package epubstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// Inbox persistence is part of the EPUB reader-schema contribution. Portable
// copies strip these installation-local claims before validating the copy.
func initializeInboxSchema(tx *sql.Tx) error {
	_, err := tx.Exec(`CREATE TABLE epub_inbox_claims (
 name TEXT PRIMARY KEY,
 receipt_id TEXT NOT NULL UNIQUE
 )`)
	return err
}

// Claims deliberately outlive discarded receipts: forgetting one would silently
// authorize a second import of an unresolved external input.
type InboxClaim struct{ Name, ReceiptID string }

func (s *Store) PendingInbox(ctx context.Context, after string, limit int) ([]InboxClaim, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT name,receipt_id FROM epub_inbox_claims WHERE name>? ORDER BY name LIMIT ?`, after, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []InboxClaim
	for rows.Next() {
		var claim InboxClaim
		if err := rows.Scan(&claim.Name, &claim.ReceiptID); err != nil {
			return nil, err
		}
		result = append(result, claim)
	}
	return result, rows.Err()
}

func (s *Store) GetInboxClaim(ctx context.Context, id string) (InboxClaim, error) {
	var claim InboxClaim
	err := s.db.QueryRowContext(ctx, `SELECT name,receipt_id FROM epub_inbox_claims WHERE receipt_id=?`, id).Scan(&claim.Name, &claim.ReceiptID)
	if errors.Is(err, sql.ErrNoRows) {
		err = ErrNotFound
	}
	return claim, err
}

func (s *Store) recordInboxIntent(ctx context.Context, r Receipt) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `INSERT INTO epub_files(id,original_name,state,size,image_mode,created_at,updated_at) VALUES(?,?,?,?,?,?,?)`, r.ID, r.OriginalName, r.State, r.Size, r.ImageMode, r.CreatedAt, r.UpdatedAt); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO epub_inbox_claims(name,receipt_id) VALUES(?,?)`, r.OriginalName, r.ID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) clearInboxClaim(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM epub_inbox_claims WHERE receipt_id=?`, id)
	return err
}

func preparePortableInbox(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `DELETE FROM epub_inbox_claims`)
	return err
}

func validatePortableInbox(ctx context.Context, tx *sql.Tx) error {
	var exists bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM epub_inbox_claims)`).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("epubstore: portable data contains inbox cleanup authority")
	}
	return nil
}
