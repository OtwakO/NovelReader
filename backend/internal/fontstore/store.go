// Package fontstore manages uploaded font files.
package fontstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/otwako/novelreader/internal/readerstore"
)

// Font represents an uploaded font file.
type Font struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	FileName  string `json:"fileName"`
	FileSize  int64  `json:"fileSize"`
	CreatedAt int64  `json:"createdAt"`
}

// Store handles font persistence and file storage.
type Store struct {
	db    *sql.DB
	files readerstore.FileStore
}

func NewStore(db *sql.DB, files readerstore.FileStore) *Store {
	return &Store{db: db, files: files}
}

// ReaderSchema returns the current font schema module for fresh initialization and validation.
func ReaderSchema() readerstore.ReaderSchema {
	return readerstore.ReaderSchema{Initialize: func(tx *sql.Tx) error { return initSchema(tx) }}
}

type schemaExecutor interface {
	Exec(query string, args ...any) (sql.Result, error)
}

func initSchema(db schemaExecutor) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS fonts (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL UNIQUE,
		file_name TEXT NOT NULL,
		file_size INTEGER NOT NULL,
		created_at INTEGER NOT NULL
	)`)
	if err != nil {
		return fmt.Errorf("fontstore: init: %w", err)
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS font_cleanup (file_name TEXT PRIMARY KEY)`)
	return err
}

// Add publishes a new ID; same-name replacement retires the previous file.
func (s *Store) Add(name, id string, data []byte) (*Font, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	// Acquire the write lock before inspecting/replacing metadata. Cleanup uses
	// the same lock, including when another Store opens for this reader.
	if _, err := tx.Exec(`INSERT OR IGNORE INTO font_cleanup SELECT file_name FROM fonts WHERE name = ?`, name); err != nil {
		return nil, err
	}
	var exists bool
	if err := tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM fonts WHERE id = ?)`, id).Scan(&exists); err != nil {
		return nil, err
	}
	if exists {
		return nil, fmt.Errorf("fontstore: font ID already exists")
	}
	f := &Font{ID: id, Name: name, FileName: id, FileSize: int64(len(data)), CreatedAt: time.Now().UnixMilli()}
	if _, err := tx.Exec(`INSERT INTO fonts (id, name, file_name, file_size, created_at) VALUES (?,?,?,?,?) ON CONFLICT(name) DO UPDATE SET id=excluded.id, file_name=excluded.file_name, file_size=excluded.file_size, created_at=excluded.created_at`, f.ID, f.Name, f.FileName, f.FileSize, f.CreatedAt); err != nil {
		return nil, err
	}
	if err := s.files.WriteFile(data, 0o600, readerstore.FontsDirectory, id); err != nil {
		return nil, errors.Join(fmt.Errorf("fontstore: write: %w", err), s.removeFile(id))
	}
	if err := tx.Commit(); err != nil {
		// Commit errors can be ambiguous: keep the file rather than deleting bytes
		// potentially referenced by committed metadata.
		return nil, fmt.Errorf("fontstore: commit font %s: %w", id, err)
	}
	if err := s.Cleanup(context.Background()); err != nil {
		return f, fmt.Errorf("font saved; obsolete file cleanup pending: %w", err)
	}
	return f, nil
}

// fontColumns keeps SELECT scan order explicit.
var fontColumns = `id, name, file_name, file_size, created_at`

// List returns all fonts.
func (s *Store) List() ([]Font, error) {
	rows, err := s.db.Query(`SELECT ` + fontColumns + ` FROM fonts ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []Font
	for rows.Next() {
		var f Font
		if err := rows.Scan(&f.ID, &f.Name, &f.FileName, &f.FileSize, &f.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, f)
	}
	return list, rows.Err()
}

// Read returns font metadata and bytes by ID.
func (s *Store) Read(id string) (Font, []byte, error) {
	var f Font
	err := s.db.QueryRow(`SELECT `+fontColumns+` FROM fonts WHERE id = ?`, id).Scan(&f.ID, &f.Name, &f.FileName, &f.FileSize, &f.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Font{}, nil, nil
		}
		return Font{}, nil, err
	}
	data, err := s.files.ReadFile(readerstore.FontsDirectory, f.FileName)
	if err != nil {
		return Font{}, nil, err
	}
	return f, data, nil
}

func (s *Store) Delete(id string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`INSERT OR IGNORE INTO font_cleanup SELECT file_name FROM fonts WHERE id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM fonts WHERE id = ?`, id); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	if err := s.Cleanup(context.Background()); err != nil {
		return fmt.Errorf("font deleted; file cleanup pending: %w", err)
	}
	return nil
}
