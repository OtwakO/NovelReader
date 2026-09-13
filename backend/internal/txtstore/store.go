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
	"github.com/otwako/novelreader/internal/txt"
)

type State string

const (
	Receiving      State = "receiving"
	Received       State = "received"
	Failed         State = "failed"
	Removing       State = "removing"
	Analyzing      State = "analyzing"
	Ready          State = "ready"
	NeedsReview    State = "needs_review"
	AnalysisFailed State = "analysis_failed"
	Published      State = "published"
)

var (
	ErrNotFound     = errors.New("txtstore: receipt not found")
	ErrStateChanged = errors.New("txtstore: receipt state changed")
)

type Receipt struct {
	ID              string
	OriginalName    string
	Path            string
	State           State
	Size            int64
	Error           string
	CreatedAt       int64
	UpdatedAt       int64
	LibraryID       string
	AnalysisVersion int64
	Options         txt.Options
}

type Store struct {
	db    *sql.DB
	files readerstore.FileStore
}

// NewStore borrows the reader database and confined files. The caller owns the
// reader-home lease and must keep it valid for each operation.
func NewStore(db *sql.DB, files readerstore.FileStore) *Store { return &Store{db: db, files: files} }

func ReaderSchema() readerstore.ReaderSchema {
	return readerstore.ReaderSchema{ValidatePortableFiles: validatePortableFiles, PreparePortable: preparePortable, Initialize: func(tx *sql.Tx) error {
		_, err := tx.Exec(`CREATE TABLE txt_files (
 id TEXT PRIMARY KEY,
 original_name TEXT NOT NULL,
 path TEXT NOT NULL UNIQUE,
 state TEXT NOT NULL CHECK(state IN ('receiving','received','failed','removing','analyzing','ready','needs_review','analysis_failed','published')),
 size INTEGER NOT NULL DEFAULT 0 CHECK(size >= 0),
 error TEXT NOT NULL DEFAULT '',
 library_id TEXT UNIQUE REFERENCES library_items(id) ON DELETE SET NULL CHECK(library_id IS NULL OR library_id=id),
 analysis_version INTEGER NOT NULL DEFAULT 0,
 requested_encoding TEXT NOT NULL DEFAULT '',
 requested_preset TEXT NOT NULL DEFAULT '',
 encoding TEXT NOT NULL DEFAULT '',
 preset TEXT NOT NULL DEFAULT '',
 parser_version INTEGER NOT NULL DEFAULT 0,
 review_reasons TEXT NOT NULL DEFAULT '[]',
 created_at INTEGER NOT NULL,
 updated_at INTEGER NOT NULL
 ); CREATE INDEX idx_txt_files_state ON txt_files(state,id);
 CREATE TABLE txt_inbox_claims (
 name TEXT PRIMARY KEY,
 receipt_id TEXT NOT NULL UNIQUE
 );
 CREATE TABLE txt_sections (
 receipt_id TEXT NOT NULL REFERENCES txt_files(id) ON DELETE CASCADE,
 idx INTEGER NOT NULL,
 title TEXT NOT NULL,
 start_byte INTEGER NOT NULL CHECK(start_byte >= 0),
 end_byte INTEGER NOT NULL CHECK(end_byte > start_byte),
 generated INTEGER NOT NULL,
 PRIMARY KEY(receipt_id,idx)
 )`)
		return err
	}}
}

const receiptColumns = `id, original_name, path, state, size, error, created_at, updated_at, COALESCE(library_id,''), analysis_version, requested_encoding, requested_preset`
const metadataTimeout = 5 * time.Second

func (s *Store) Get(ctx context.Context, id string) (Receipt, error) {
	return scanReceipt(s.db.QueryRowContext(ctx, `SELECT `+receiptColumns+` FROM txt_files WHERE id=?`, id))
}

func scanReceipt(row *sql.Row) (Receipt, error) {
	var value Receipt
	err := row.Scan(&value.ID, &value.OriginalName, &value.Path, &value.State, &value.Size, &value.Error, &value.CreatedAt, &value.UpdatedAt, &value.LibraryID, &value.AnalysisVersion, &value.Options.Encoding, &value.Options.Preset)
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
