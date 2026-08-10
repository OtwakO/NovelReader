package readerstore

import (
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
)

const InitialDatabaseVersion = 1

var (
	ErrNewerDatabaseSchema = errors.New("readerstore: database schema is newer than supported")
	ErrMigrationOrder      = errors.New("readerstore: reader migration order does not match the database")
)

type ReaderMigration struct {
	Name  string
	Apply func(*sql.Tx) error
}

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
	return db, nil
}

func applyReaderMigrations(db *sql.DB, migrations []ReaderMigration) error {
	targetVersion := InitialDatabaseVersion + len(migrations)
	var version int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		return fmt.Errorf("readerstore: read reader schema version: %w", err)
	}
	if version > targetVersion {
		return fmt.Errorf("%w: found %d, support %d", ErrNewerDatabaseSchema, version, targetVersion)
	}
	if version < InitialDatabaseVersion {
		return fmt.Errorf("readerstore: unsupported reader schema version %d", version)
	}

	applied, err := appliedReaderMigrations(db)
	if err != nil {
		return err
	}
	if version != InitialDatabaseVersion+len(applied) || len(applied) > len(migrations) {
		return ErrMigrationOrder
	}
	for index, name := range applied {
		if migrations[index].Name != name {
			return ErrMigrationOrder
		}
	}
	for index := len(applied); index < len(migrations); index++ {
		migration := migrations[index]
		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("readerstore: begin migration %q: %w", migration.Name, err)
		}
		if _, err := tx.Exec(`CREATE TABLE IF NOT EXISTS readerstore_migrations (sequence INTEGER PRIMARY KEY, name TEXT NOT NULL UNIQUE)`); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("readerstore: initialize migration registry: %w", err)
		}
		if err := migration.Apply(tx); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("readerstore: apply migration %q: %w", migration.Name, err)
		}
		if _, err := tx.Exec(`INSERT INTO readerstore_migrations (sequence, name) VALUES (?, ?)`, index+1, migration.Name); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("readerstore: record migration %q: %w", migration.Name, err)
		}
		if _, err := tx.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, InitialDatabaseVersion+index+1)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("readerstore: version migration %q: %w", migration.Name, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("readerstore: commit migration %q: %w", migration.Name, err)
		}
	}
	return nil
}

func appliedReaderMigrations(db *sql.DB) ([]string, error) {
	var tableCount int
	if err := db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = 'readerstore_migrations'`).Scan(&tableCount); err != nil {
		return nil, fmt.Errorf("readerstore: inspect migration registry: %w", err)
	}
	if tableCount == 0 {
		return nil, nil
	}
	rows, err := db.Query(`SELECT sequence, name FROM readerstore_migrations ORDER BY sequence`)
	if err != nil {
		return nil, fmt.Errorf("readerstore: read migration registry: %w", err)
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var sequence int
		var name string
		if err := rows.Scan(&sequence, &name); err != nil {
			return nil, fmt.Errorf("readerstore: scan migration registry: %w", err)
		}
		if sequence != len(names)+1 {
			return nil, ErrMigrationOrder
		}
		names = append(names, name)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("readerstore: read migration registry: %w", err)
	}
	return names, nil
}

func validateHomeDatabase(path string, minimumVersion, maximumVersion int) error {
	db, err := sql.Open("sqlite", sqliteFileURI(path)+"?mode=ro")
	if err != nil {
		return err
	}
	defer db.Close()
	var version int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		return err
	}
	if version > maximumVersion {
		return fmt.Errorf("%w: found %d, support %d", ErrNewerDatabaseSchema, version, maximumVersion)
	}
	if version < minimumVersion {
		return fmt.Errorf("readerstore: unsupported database schema version %d; expected at least %d", version, minimumVersion)
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
