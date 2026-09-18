package epubstore

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/otwako/novelreader/internal/epub"
	"github.com/otwako/novelreader/internal/readerstore"
)

type AcquisitionState string

const (
	Receiving       AcquisitionState = "receiving"
	Finalizing      AcquisitionState = "finalizing"
	Acquired        AcquisitionState = "acquired"
	Failed          AcquisitionState = "failed"
	Removing        AcquisitionState = "removing"
	metadataTimeout                  = 5 * time.Second
)

var (
	ErrNotFound     = errors.New("epubstore: receipt not found")
	ErrStateChanged = errors.New("epubstore: receipt state changed")
)

// Receipt describes original acquisition, not preparation or shelf admission.
type Receipt struct {
	ID, OriginalName, Path string
	State                  AcquisitionState
	Size                   int64
	ImageMode              epub.ImageMode
	Error                  string
	CreatedAt, UpdatedAt   int64
}

type Store struct {
	db    *sql.DB
	files readerstore.FileStore
}

// NewStore borrows the reader database/files. Keep the home lease open throughout
// every operation. Worker scheduling and library state belong to their owners.
func NewStore(db *sql.DB, files readerstore.FileStore) *Store { return &Store{db: db, files: files} }

func (s *Store) Get(ctx context.Context, id string) (Receipt, error) {
	var r Receipt
	err := s.db.QueryRowContext(ctx, `SELECT id,original_name,state,size,image_mode,error,created_at,updated_at FROM epub_files WHERE id=?`, id).Scan(&r.ID, &r.OriginalName, &r.State, &r.Size, &r.ImageMode, &r.Error, &r.CreatedAt, &r.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Receipt{}, ErrNotFound
	}
	if err != nil {
		return Receipt{}, err
	}
	// IDs also cross the eventual portable-restore boundary. Derive paths rather
	// than trusting a stored pathname that could identify another feature's data.
	if err = validateID(r.ID); err != nil {
		return Receipt{}, err
	}
	r.Path = originalPath(r.ID)
	return r, nil
}

// Update the caller's snapshot only after the guarded database write succeeds.
func (s *Store) transition(ctx context.Context, r *Receipt, to AcquisitionState, message string) error {
	now := time.Now().UnixMilli()
	result, err := s.db.ExecContext(ctx, `UPDATE epub_files SET state=?,size=?,error=?,updated_at=? WHERE id=? AND state=?`, to, r.Size, message, now, r.ID, r.State)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrStateChanged
	}
	r.State, r.Error, r.UpdatedAt = to, message, now
	return nil
}
