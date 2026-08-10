// Package fontstore manages uploaded font files.
package fontstore

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
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
	db      *sql.DB
	dataDir string
}

func NewStore(db *sql.DB, dataDir string) *Store {
	return &Store{db: db, dataDir: dataDir}
}

// ReaderMigration initializes font metadata inside one reader home.
func ReaderMigration() readerstore.ReaderMigration {
	return readerstore.ReaderMigration{Name: "fonts", Apply: func(tx *sql.Tx) error {
		return initSchema(tx)
	}}
}

func (s *Store) Init() error {
	if err := initSchema(s.db); err != nil {
		return err
	}
	return os.MkdirAll(s.fontDir(), 0755)
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

func (s *Store) fontDir() string {
	return filepath.Join(s.dataDir, "fonts")
}

// Add saves a font file and adds a DB record.
func (s *Store) Add(name, id string, data []byte) (*Font, error) {
	fontPath := filepath.Join(s.fontDir(), id)
	if err := os.WriteFile(fontPath, data, 0644); err != nil {
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
		os.Remove(fontPath) // cleanup file on DB failure
		return nil, fmt.Errorf("fontstore: db: %w", err)
	}
	return f, nil
}

// fontColumns for SELECT on the fonts table.
// ponytail: two-column scan, unlikely to migrate, but explicit is safer.
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

// GetPath returns the filesystem path for a font by its ID.
func (s *Store) GetPath(id string) (string, error) {
	var f Font
	err := s.db.QueryRow(`SELECT `+fontColumns+` FROM fonts WHERE id = ?`, id).Scan(&f.ID, &f.Name, &f.FileName, &f.FileSize, &f.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", nil
		}
		return "", err
	}
	return filepath.Join(s.fontDir(), f.FileName), nil
}

// Delete removes a font.
func (s *Store) Delete(id string) error {
	path, err := s.GetPath(id)
	if err != nil {
		return err
	}
	if path != "" {
		os.Remove(path)
	}
	_, err = s.db.Exec(`DELETE FROM fonts WHERE id = ?`, id)
	return err
}
