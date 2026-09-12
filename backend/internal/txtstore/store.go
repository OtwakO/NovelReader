// Package txtstore owns managed TXT originals and their durable lifecycle.
// It does not own library metadata, request handling, or worker scheduling.
package txtstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/otwako/novelreader/internal/readerstore"
)

type State string

const (
	Receiving State = "receiving"
	Received  State = "received"
	Failed    State = "failed"
	Removing  State = "removing"
)

var (
	ErrNotFound     = errors.New("txtstore: receipt not found")
	ErrStateChanged = errors.New("txtstore: receipt state changed")
)

type Receipt struct {
	ID           string
	OriginalName string
	Path         string
	State        State
	Size         int64
	Error        string
	CreatedAt    int64
	UpdatedAt    int64
}

type Store struct {
	db    *sql.DB
	files readerstore.FileStore
}

// NewStore borrows the reader database and confined files. The caller owns the
// reader-home lease and must keep it valid for each operation.
func NewStore(db *sql.DB, files readerstore.FileStore) *Store { return &Store{db: db, files: files} }

func ReaderSchema() readerstore.ReaderSchema {
	return readerstore.ReaderSchema{Initialize: func(tx *sql.Tx) error {
		_, err := tx.Exec(`CREATE TABLE txt_files (
 id TEXT PRIMARY KEY,
 original_name TEXT NOT NULL,
 path TEXT NOT NULL UNIQUE,
 state TEXT NOT NULL CHECK(state IN ('receiving','received','failed','removing')),
 size INTEGER NOT NULL DEFAULT 0 CHECK(size >= 0),
 error TEXT NOT NULL DEFAULT '',
 created_at INTEGER NOT NULL,
 updated_at INTEGER NOT NULL
 ); CREATE INDEX idx_txt_files_state ON txt_files(state,id)`)
		return err
	}}
}

const receiptColumns = `id, original_name, path, state, size, error, created_at, updated_at`

func (s *Store) Get(ctx context.Context, id string) (Receipt, error) {
	var value Receipt
	err := s.db.QueryRowContext(ctx, `SELECT `+receiptColumns+` FROM txt_files WHERE id=?`, id).Scan(&value.ID, &value.OriginalName, &value.Path, &value.State, &value.Size, &value.Error, &value.CreatedAt, &value.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Receipt{}, ErrNotFound
	}
	return value, err
}

func (s *Store) transition(ctx context.Context, id string, from, to State, size int64, message string) error {
	result, err := s.db.ExecContext(ctx, `UPDATE txt_files SET state=?,size=?,error=?,updated_at=? WHERE id=? AND state=?`, to, size, message, time.Now().UnixMilli(), id, from)
	if err != nil {
		return fmt.Errorf("txtstore: update receipt: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return ErrStateChanged
	}
	return nil
}
