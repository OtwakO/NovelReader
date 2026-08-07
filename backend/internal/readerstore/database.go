package readerstore

import (
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
)

const InitialDatabaseVersion = 1

var ErrNewerDatabaseSchema = errors.New("readerstore: database schema is newer than supported")

func initializeHomeDatabase(path string) error {
	db, err := sql.Open("sqlite", sqliteFileURI(path))
	if err != nil {
		return err
	}
	if _, err := db.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, InitialDatabaseVersion)); err != nil {
		_ = db.Close()
		return err
	}
	return db.Close()
}

func openHomeDatabase(path string) (*sql.DB, error) {
	query := url.Values{}
	query.Add("_pragma", "journal_mode(WAL)")
	query.Add("_pragma", "busy_timeout(5000)")
	query.Add("_pragma", "cache_size(-8000)")
	dsn := sqliteFileURI(path) + "?" + query.Encode()

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("readerstore: open database: %w", err)
	}
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(2)
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("readerstore: ping database: %w", err)
	}
	var version int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("readerstore: read database schema version: %w", err)
	}
	if version > InitialDatabaseVersion {
		_ = db.Close()
		return nil, fmt.Errorf("%w: found %d, support %d", ErrNewerDatabaseSchema, version, InitialDatabaseVersion)
	}
	if version < InitialDatabaseVersion {
		_ = db.Close()
		return nil, fmt.Errorf("readerstore: unsupported database schema version %d", version)
	}
	return db, nil
}

func validateHomeDatabase(path string) error {
	db, err := sql.Open("sqlite", sqliteFileURI(path)+"?mode=ro")
	if err != nil {
		return err
	}
	defer db.Close()
	var version int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		return err
	}
	if version > InitialDatabaseVersion {
		return fmt.Errorf("%w: found %d, support %d", ErrNewerDatabaseSchema, version, InitialDatabaseVersion)
	}
	if version < InitialDatabaseVersion {
		return fmt.Errorf("readerstore: unsupported database schema version %d", version)
	}
	return nil
}

func sqliteFileURI(path string) string {
	slashPath := filepath.ToSlash(path)
	if filepath.VolumeName(path) != "" && len(slashPath) >= 2 && slashPath[1] == ':' {
		slashPath = "/" + slashPath
	}
	return (&url.URL{Scheme: "file", Path: slashPath}).String()
}
