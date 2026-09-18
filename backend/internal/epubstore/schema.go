package epubstore

import "database/sql"

// Acquisition-only schema fragment. Not registered with live reader homes:
// preparation/resource tables and portable hooks must precede epoch activation.
func initializeSchema(tx *sql.Tx) error {
	_, err := tx.Exec(`CREATE TABLE epub_files (
 id TEXT PRIMARY KEY,
 original_name TEXT NOT NULL,
 state TEXT NOT NULL CHECK(state IN ('receiving','finalizing','acquired','failed','removing')),
 size INTEGER NOT NULL DEFAULT 0 CHECK(size >= 0),
 image_mode TEXT NOT NULL CHECK(image_mode IN ('original','optimized')),
 error TEXT NOT NULL DEFAULT '',
 created_at INTEGER NOT NULL,
 updated_at INTEGER NOT NULL
 ); CREATE INDEX idx_epub_files_state ON epub_files(state,id)`)
	return err
}
