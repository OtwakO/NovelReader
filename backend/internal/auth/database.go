package auth

import (
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

func ensureSystemDatabase(stage, path string) error {
	if info, err := os.Lstat(path); err == nil {
		if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			return ErrInvalidSystemSchema
		}
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("auth: inspect system database: %w", err)
	}

	if info, err := os.Lstat(stage); err == nil {
		if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			return ErrInvalidSystemSchema
		}
		if err := validateSystemDatabase(stage); err != nil {
			if errors.Is(err, ErrNewerSystemSchema) {
				return err
			}
			if err := os.Remove(stage); err != nil {
				return fmt.Errorf("auth: remove incomplete staged system database: %w", err)
			}
			if err := initializeSystemDatabase(stage); err != nil {
				return fmt.Errorf("auth: rebuild staged system database: %w", err)
			}
		}
	} else if errors.Is(err, os.ErrNotExist) {
		if err := initializeSystemDatabase(stage); err != nil {
			return fmt.Errorf("auth: initialize staged system database: %w", err)
		}
	} else {
		return fmt.Errorf("auth: inspect staged system database: %w", err)
	}

	if err := os.Rename(stage, path); err != nil {
		return fmt.Errorf("auth: publish system database: %w", err)
	}
	return nil
}

func initializeSystemDatabase(path string) error {
	db, err := sql.Open("sqlite", sqliteFileURI(path))
	if err != nil {
		return err
	}
	if _, err := db.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		_ = db.Close()
		return err
	}
	tx, err := db.Begin()
	if err != nil {
		_ = db.Close()
		return err
	}
	if _, err := tx.Exec(systemSchema); err != nil {
		_ = tx.Rollback()
		_ = db.Close()
		return err
	}
	if _, err := tx.Exec(`PRAGMA user_version = 1`); err != nil {
		_ = tx.Rollback()
		_ = db.Close()
		return err
	}
	if err := tx.Commit(); err != nil {
		_ = db.Close()
		return err
	}
	if err := db.Close(); err != nil {
		return err
	}
	return os.Chmod(path, 0o600)
}

func openSystemDatabase(path string) (*sql.DB, error) {
	if info, err := os.Lstat(path); err != nil {
		return nil, fmt.Errorf("auth: inspect system database: %w", err)
	} else if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return nil, ErrInvalidSystemSchema
	}
	// Validate through a read-only connection before applying writable pragmas. Newer or malformed
	// databases must remain byte-for-byte untouched when startup refuses them.
	if err := validateSystemDatabase(path); err != nil {
		return nil, err
	}

	query := url.Values{}
	query.Add("_pragma", "journal_mode(WAL)")
	query.Add("_pragma", "busy_timeout(5000)")
	query.Add("_pragma", "cache_size(-8000)")
	query.Add("_pragma", "foreign_keys(ON)")
	query.Add("_txlock", "immediate")
	db, err := sql.Open("sqlite", sqliteFileURI(path)+"?"+query.Encode())
	if err != nil {
		return nil, fmt.Errorf("auth: open system database: %w", err)
	}
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(2)
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, classifySchemaError("open", err)
	}
	return db, nil
}

func validateSystemDatabase(path string) error {
	db, err := sql.Open("sqlite", sqliteFileURI(path)+"?mode=ro")
	if err != nil {
		return classifySchemaError("open read-only", err)
	}
	defer db.Close()
	return validateOpenSystemDatabase(db)
}

func validateOpenSystemDatabase(db *sql.DB) error {
	var version int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		return classifySchemaError("read schema version", err)
	}
	if version > CurrentSystemSchemaVersion {
		return fmt.Errorf("%w: found %d, support %d", ErrNewerSystemSchema, version, CurrentSystemSchemaVersion)
	}
	if version != CurrentSystemSchemaVersion {
		return fmt.Errorf("%w: unsupported version %d", ErrInvalidSystemSchema, version)
	}

	if err := validateDeclaredSchema(db); err != nil {
		return err
	}
	var setupRows int
	if err := db.QueryRow(`SELECT count(*) FROM setup_state WHERE id = 1 AND status IN ('open', 'claimed', 'closed')`).Scan(&setupRows); err != nil {
		return classifySchemaError("validate setup state", err)
	}
	if setupRows != 1 {
		return fmt.Errorf("%w: singleton setup state is missing", ErrInvalidSystemSchema)
	}
	for _, pragma := range []string{"integrity_check", "foreign_key_check"} {
		rows, err := db.Query(`PRAGMA ` + pragma)
		if err != nil {
			return classifySchemaError("run "+pragma, err)
		}
		if err := requireCleanPragma(rows, pragma); err != nil {
			return err
		}
	}
	return nil
}

type schemaObject struct {
	kind      string
	name      string
	tableName string
	sql       string
}

func validateDeclaredSchema(db *sql.DB) error {
	reference, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		return classifySchemaError("open schema reference", err)
	}
	defer reference.Close()
	if _, err := reference.Exec(systemSchema); err != nil {
		return classifySchemaError("build schema reference", err)
	}

	expected, err := declaredSchema(reference)
	if err != nil {
		return err
	}
	actual, err := declaredSchema(db)
	if err != nil {
		return err
	}
	if len(actual) != len(expected) {
		return fmt.Errorf("%w: declared object count is %d, expected %d", ErrInvalidSystemSchema, len(actual), len(expected))
	}
	for index := range expected {
		if actual[index] != expected[index] {
			return fmt.Errorf("%w: declared object %q does not match version %d", ErrInvalidSystemSchema, actual[index].name, CurrentSystemSchemaVersion)
		}
	}
	return nil
}

func declaredSchema(db *sql.DB) ([]schemaObject, error) {
	rows, err := db.Query(`
		SELECT type, name, tbl_name, COALESCE(sql, '')
		FROM sqlite_master
		ORDER BY type, name
	`)
	if err != nil {
		return nil, classifySchemaError("read declared schema", err)
	}
	defer rows.Close()

	var objects []schemaObject
	for rows.Next() {
		var object schemaObject
		if err := rows.Scan(&object.kind, &object.name, &object.tableName, &object.sql); err != nil {
			return nil, classifySchemaError("scan declared schema", err)
		}
		objects = append(objects, object)
	}
	if err := rows.Err(); err != nil {
		return nil, classifySchemaError("read declared schema", err)
	}
	return objects, nil
}

func requireCleanPragma(rows *sql.Rows, pragma string) error {
	defer rows.Close()
	if pragma == "foreign_key_check" {
		if rows.Next() {
			return fmt.Errorf("%w: foreign_key_check reported a violation", ErrInvalidSystemSchema)
		}
		if err := rows.Err(); err != nil {
			return classifySchemaError("read foreign_key_check", err)
		}
		return nil
	}

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return classifySchemaError("read integrity_check", err)
		}
		return fmt.Errorf("%w: integrity_check returned no result", ErrInvalidSystemSchema)
	}
	var result string
	if err := rows.Scan(&result); err != nil {
		return classifySchemaError("read integrity_check", err)
	}
	if result != "ok" {
		return fmt.Errorf("%w: integrity_check reported %s", ErrInvalidSystemSchema, result)
	}
	return nil
}

func classifySchemaError(operation string, err error) error {
	return fmt.Errorf("%w: %s: %v", ErrInvalidSystemSchema, operation, err)
}

func sqliteFileURI(path string) string {
	slashPath := filepath.ToSlash(path)
	if filepath.VolumeName(path) != "" && len(slashPath) >= 2 && slashPath[1] == ':' {
		slashPath = "/" + slashPath
	}
	return (&url.URL{Scheme: "file", Path: slashPath}).String()
}
