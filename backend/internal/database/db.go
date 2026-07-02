// Package database provides a shared SQLite connection.
package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite" // pure-Go SQLite driver
)

// Open opens (or creates) a SQLite database at the given path.
// Parent directories are created if missing.
func Open(path string) (*sql.DB, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("database: mkdir: %w", err)
	}

	db, err := sql.Open("sqlite", path+"?_journal_mode=WAL&_busy_timeout=5000&_cache_size=-8000")
	if err != nil {
		return nil, fmt.Errorf("database: open: %w", err)
	}

	// ponytail: WAL mode + small read pool for concurrent readers.
	// SQLite WAL allows concurrent reads + one writer.
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(2)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("database: ping: %w", err)
	}

	return db, nil
}
