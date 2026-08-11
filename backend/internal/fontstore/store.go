// Package fontstore manages uploaded font files.
package fontstore

import (
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
	return nil
}

// Add saves a font file and adds a DB record.
func (s *Store) Add(name, id string, data []byte) (*Font, error) {
	if err := s.files.WriteFile(data, 0o600, readerstore.FontsDirectory, id); err != nil {
		return nil, fmt.Errorf("fontstore: write: %w", err)
	}

	f := &Font{
		ID:        id,
		Name:      name,
		FileName:  id,
		FileSize:  int64(len(data)),
		CreatedAt: time.Now().UnixMilli(),
	}

	_, err := s.db.Exec(`INSERT OR REPLACE INTO fonts (id, name, file_name, file_size, created_at) VALUES (?,?,?,?,?)`,
		f.ID, f.Name, f.FileName, f.FileSize, f.CreatedAt)
	if err != nil {
		_ = s.files.Remove(readerstore.FontsDirectory, id) // cleanup file on DB failure
		return nil, fmt.Errorf("fontstore: db: %w", err)
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

// Delete removes a font.
func (s *Store) Delete(id string) error {
	var fileName string
	err := s.db.QueryRow(`SELECT file_name FROM fonts WHERE id = ?`, id).Scan(&fileName)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if fileName != "" {
		_ = s.files.Remove(readerstore.FontsDirectory, fileName)
	}
	_, err = s.db.Exec(`DELETE FROM fonts WHERE id = ?`, id)
	return err
}
