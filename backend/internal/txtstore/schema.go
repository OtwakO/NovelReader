package txtstore

import (
	"database/sql"

	"github.com/otwako/novelreader/internal/readerstore"
)

func ReaderSchema() readerstore.ReaderSchema {
	return readerstore.ReaderSchema{ValidatePortableFiles: validatePortableFiles, PreparePortable: preparePortable, Initialize: func(tx *sql.Tx) error {
		_, err := tx.Exec(`CREATE TABLE txt_files (
 id TEXT PRIMARY KEY,
 original_name TEXT NOT NULL,
 path TEXT NOT NULL UNIQUE,
 state TEXT NOT NULL CHECK(state IN ('receiving','acquired','failed','removing')),
 size INTEGER NOT NULL DEFAULT 0 CHECK(size >= 0),
 error TEXT NOT NULL DEFAULT '',
 library_id TEXT UNIQUE REFERENCES library_items(id) ON DELETE SET NULL CHECK(library_id IS NULL OR library_id=id),
 generation INTEGER NOT NULL DEFAULT 0 CHECK(generation >= 0),
 created_at INTEGER NOT NULL,
 updated_at INTEGER NOT NULL
 ); CREATE INDEX idx_txt_files_state ON txt_files(state,id);
 CREATE TABLE txt_inbox_claims (
 name TEXT PRIMARY KEY,
 receipt_id TEXT NOT NULL UNIQUE
 );
 CREATE TABLE txt_interpretations (
 file_id TEXT NOT NULL REFERENCES txt_files(id) ON DELETE CASCADE,
 generation INTEGER NOT NULL CHECK(generation > 0),
 role TEXT NOT NULL CHECK(role IN ('candidate','active')),
 state TEXT NOT NULL CHECK(state IN ('queued','analyzing','ready','needs_review','analysis_failed')),
 requested_encoding TEXT NOT NULL DEFAULT '',
 requested_preset TEXT NOT NULL DEFAULT '',
 requested_pattern TEXT NOT NULL DEFAULT '',
 encoding TEXT NOT NULL DEFAULT '',
 preset TEXT NOT NULL DEFAULT '',
 parser_version INTEGER NOT NULL DEFAULT 0,
 review_reasons TEXT NOT NULL DEFAULT '[]',
 error TEXT NOT NULL DEFAULT '',
 base_content_revision INTEGER NOT NULL DEFAULT 0 CHECK(base_content_revision >= 0),
 queued_at INTEGER NOT NULL,
 updated_at INTEGER NOT NULL,
 PRIMARY KEY(file_id,generation),
 UNIQUE(file_id,role),
 CHECK(role != 'active' OR state IN ('ready','needs_review'))
 );
 CREATE INDEX idx_txt_pending ON txt_interpretations(state,queued_at,file_id) WHERE role='candidate';
 CREATE TABLE txt_sections (
 file_id TEXT NOT NULL,
 generation INTEGER NOT NULL,
 idx INTEGER NOT NULL CHECK(idx >= 0),
 title TEXT NOT NULL,
 start_byte INTEGER NOT NULL CHECK(start_byte >= 0),
 end_byte INTEGER NOT NULL CHECK(end_byte > start_byte),
 generated INTEGER NOT NULL CHECK(generated IN (0,1)),
 PRIMARY KEY(file_id,generation,idx),
 FOREIGN KEY(file_id,generation) REFERENCES txt_interpretations(file_id,generation) ON DELETE CASCADE
 );
 CREATE UNIQUE INDEX idx_txt_section_start ON txt_sections(file_id,generation,start_byte);
 CREATE VIEW txt_receipts AS SELECT f.id,f.original_name,f.path,f.size,f.created_at,
 MAX(f.updated_at,COALESCE(i.updated_at,0)) AS updated_at,f.library_id,
 CASE WHEN f.state != 'acquired' THEN f.state
      WHEN f.library_id IS NOT NULL THEN 'published'
      WHEN i.state='queued' THEN 'received' ELSE i.state END AS state,
 CASE WHEN f.state='acquired' THEN COALESCE(i.error,'') ELSE f.error END AS error,
 COALESCE(i.generation,0) AS analysis_version,
 COALESCE(i.requested_encoding,'') AS requested_encoding,
 COALESCE(i.requested_preset,'') AS requested_preset,
 COALESCE(i.requested_pattern,'') AS requested_pattern
 FROM txt_files f LEFT JOIN txt_interpretations i ON i.file_id=f.id
 AND i.role=CASE WHEN f.library_id IS NULL THEN 'candidate' ELSE 'active' END`)
		return err
	}}
}
