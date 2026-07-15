// Package book manages books on the shelf: search results, chapters, content caching.
package book

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// AltSource is a secondary source for the same book.
type AltSource struct {
	SourceURL  string `json:"sourceUrl"`
	BookURL    string `json:"bookUrl"`
	SourceName string `json:"sourceName"`
}

// Book represents a book on the user's shelf.
type Book struct {
	ID          string `json:"id" db:"id"`
	Name        string `json:"name" db:"name"`
	Author      string `json:"author,omitempty" db:"author"`
	CoverURL    string `json:"coverUrl,omitempty" db:"cover_url"`
	Intro       string `json:"intro,omitempty" db:"intro"`
	Kind        string `json:"kind,omitempty" db:"kind"`
	SourceURL   string `json:"sourceUrl" db:"source_url"`
	BookURL     string `json:"bookUrl" db:"book_url"`
	TocURL      string `json:"tocUrl,omitempty" db:"toc_url"`
	Origin      string `json:"origin" db:"origin"`
	VariableMap string `json:"variableMap,omitempty" db:"variable_map"`
	LastChapter string `json:"lastChapter,omitempty" db:"last_chapter"`
	UpdateTime  string `json:"updateTime,omitempty" db:"update_time"`
	WordCount   string `json:"wordCount,omitempty" db:"word_count"`

	DurChapterIndex int     `json:"durChapterIndex" db:"dur_chapter_index"`
	DurChapterPos   float64 `json:"durChapterPos" db:"dur_chapter_pos"`
	TotalChapterNum int     `json:"totalChapterNum" db:"total_chapter_num"`

	AlternateSources []AltSource `json:"alternateSources,omitempty" db:"alternate_sources"`

	CreatedAt int64 `json:"createdAt" db:"created_at"`
	UpdatedAt int64 `json:"updatedAt" db:"updated_at"`
}

// Chapter represents a single chapter.
type Chapter struct {
	ID        string `json:"id" db:"id"`
	BookID    string `json:"bookId" db:"book_id"`
	Index     int    `json:"index" db:"idx"`
	Title     string `json:"title" db:"title"`
	URL       string `json:"url" db:"url"`
	IsVip     bool   `json:"isVip" db:"is_vip"`
	IsVolume  bool   `json:"isVolume" db:"is_volume"`
	IsPay     bool   `json:"isPay" db:"is_pay"`
	BaseURL   string `json:"baseUrl" db:"base_url"`
	Tag       string `json:"tag,omitempty" db:"tag"`
	WordCount string `json:"wordCount,omitempty" db:"word_count"`
	Cached    bool   `json:"cached" db:"cached"`
}

// SearchResult is a book found by searching a source.
type SearchResult struct {
	Name             string      `json:"name"`
	Author           string      `json:"author"`
	CoverURL         string      `json:"coverUrl"`
	Intro            string      `json:"intro"`
	Kind             string      `json:"kind"`
	LastChapter      string      `json:"lastChapter"`
	UpdateTime       string      `json:"updateTime"`
	WordCount        string      `json:"wordCount"`
	BookURL          string      `json:"bookUrl"`
	SourceURL        string      `json:"sourceUrl"`
	SourceName       string      `json:"sourceName"`
	Score            int         `json:"score"`
	AlternateSources []AltSource `json:"alternateSources,omitempty"`
}

// Store handles book persistence.
type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) Init() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS books (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			author TEXT DEFAULT '',
			cover_url TEXT DEFAULT '',
			intro TEXT DEFAULT '',
			kind TEXT DEFAULT '',
			source_url TEXT NOT NULL,
			book_url TEXT NOT NULL,
			toc_url TEXT DEFAULT '',
			origin TEXT NOT NULL DEFAULT '',
			variable_map TEXT DEFAULT '',
			last_chapter TEXT DEFAULT '',
			update_time TEXT DEFAULT '',
			word_count TEXT DEFAULT '',
			dur_chapter_index INTEGER DEFAULT 0,
			dur_chapter_pos REAL DEFAULT 0,
			total_chapter_num INTEGER DEFAULT 0,
			alternate_sources TEXT DEFAULT '[]',
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS chapters (
			id TEXT PRIMARY KEY,
			book_id TEXT NOT NULL,
			idx INTEGER NOT NULL,
			title TEXT NOT NULL,
			url TEXT NOT NULL,
			is_vip INTEGER DEFAULT 0,
			is_volume INTEGER DEFAULT 0,
			is_pay INTEGER DEFAULT 0,
			base_url TEXT DEFAULT '',
			tag TEXT DEFAULT '',
			word_count TEXT DEFAULT '',
			cached INTEGER DEFAULT 0,
			FOREIGN KEY (book_id) REFERENCES books(id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_chapters_book_id ON chapters(book_id)`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil {
			return fmt.Errorf("book: init: %w", err)
		}
	}
	for _, column := range []struct{ name, definition string }{
		{"is_pay", "INTEGER DEFAULT 0"},
		{"base_url", "TEXT DEFAULT ''"},
		{"tag", "TEXT DEFAULT ''"},
		{"word_count", "TEXT DEFAULT ''"},
	} {
		if err := ensureColumn(s.db, "chapters", column.name, column.definition); err != nil {
			return fmt.Errorf("book: init chapter column %s: %w", column.name, err)
		}
	}
	return nil
}

// bookColumns is the explicit column list for SELECT queries.
// Using explicit columns (not SELECT *) ensures scan order is deterministic
// regardless of whether the DB was created by CREATE TABLE or ALTER TABLE.
var bookColumns = `id, name, author, cover_url, intro, kind,
	source_url, book_url, toc_url, origin, variable_map,
	last_chapter, update_time, word_count,
	dur_chapter_index, dur_chapter_pos, total_chapter_num,
	alternate_sources, created_at, updated_at`

// chapterColumns for SELECT queries on the chapters table.
var chapterColumns = `id, book_id, idx, title, url, is_vip, is_volume, is_pay, base_url, tag, word_count, cached`

// AddBook inserts a book into the shelf.
func (s *Store) AddBook(b *Book) error {
	now := time.Now().UnixMilli()
	b.CreatedAt = now
	b.UpdatedAt = now
	altJSON, _ := json.Marshal(b.AlternateSources)
	if len(altJSON) == 0 {
		altJSON = []byte("[]")
	}
	_, err := s.db.Exec(`INSERT OR REPLACE INTO books (
		id, name, author, cover_url, intro, kind,
		source_url, book_url, toc_url, origin, variable_map,
		last_chapter, update_time, word_count,
		dur_chapter_index, dur_chapter_pos, total_chapter_num,
		alternate_sources,
		created_at, updated_at
	) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		b.ID, b.Name, b.Author, b.CoverURL, b.Intro, b.Kind,
		b.SourceURL, b.BookURL, b.TocURL, b.Origin, b.VariableMap,
		b.LastChapter, b.UpdateTime, b.WordCount,
		b.DurChapterIndex, b.DurChapterPos, b.TotalChapterNum,
		string(altJSON),
		b.CreatedAt, b.UpdatedAt,
	)
	return err
}

// DeleteBook removes a book from the shelf.
func (s *Store) DeleteBook(id string) error {
	_, err := s.db.Exec(`DELETE FROM chapters WHERE book_id = ?`, id)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`DELETE FROM books WHERE id = ?`, id)
	return err
}

// ListBooks returns all books on the shelf.
func (s *Store) ListBooks() ([]Book, error) {
	rows, err := s.db.Query(`SELECT ` + bookColumns + ` FROM books ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanBooks(rows)
}

// GetBook returns a single book by ID.
func (s *Store) GetBook(id string) (*Book, error) {
	row := s.db.QueryRow(`SELECT `+bookColumns+` FROM books WHERE id = ?`, id)
	var b Book
	if err := scanBookRow(row, &b); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &b, nil
}

// SaveChapters replaces all chapters for a book.
func (s *Store) SaveChapters(bookID string, chapters []Chapter) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	// Delete existing chapters
	if _, err := tx.Exec(`DELETE FROM chapters WHERE book_id = ?`, bookID); err != nil {
		return err
	}

	for _, ch := range chapters {
		ch.BookID = bookID
		if ch.ID == "" {
			ch.ID = fmt.Sprintf("%s_%d", bookID, ch.Index)
		}
		_, err := tx.Exec(`INSERT INTO chapters (id, book_id, idx, title, url, is_vip, is_volume, is_pay, base_url, tag, word_count, cached) VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`,
			ch.ID, ch.BookID, ch.Index, ch.Title, ch.URL, boolToInt(ch.IsVip), boolToInt(ch.IsVolume), boolToInt(ch.IsPay), ch.BaseURL, ch.Tag, ch.WordCount, boolToInt(ch.Cached))
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// GetChapters returns all chapters for a book.
func (s *Store) GetChapters(bookID string) ([]Chapter, error) {
	rows, err := s.db.Query(`SELECT `+chapterColumns+` FROM chapters WHERE book_id = ? ORDER BY idx ASC`, bookID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanChapters(rows)
}

// UpdateProgress saves reading progress and total chapter count for a book.
func (s *Store) UpdateProgress(bookID string, chapterIndex int, position float64) error {
	_, err := s.db.Exec(`UPDATE books SET dur_chapter_index = ?, dur_chapter_pos = ?, updated_at = ? WHERE id = ?`,
		chapterIndex, position, time.Now().UnixMilli(), bookID)
	return err
}

// UpdateTotalChapters updates the total chapter count for a book.
func (s *Store) UpdateTotalChapters(bookID string, total int) error {
	_, err := s.db.Exec(`UPDATE books SET total_chapter_num = ?, updated_at = ? WHERE id = ?`,
		total, time.Now().UnixMilli(), bookID)
	return err
}

// SwitchSource swaps the active source for a book and clears chapters/progress.
// The old source becomes an alternate; the new one is promoted to active.
// ponytail: progress is reset on switch — chapter indices differ across sources.
func (s *Store) SwitchSource(bookID, sourceURL, bookURL, sourceName string) error {
	b, err := s.GetBook(bookID)
	if err != nil || b == nil {
		return fmt.Errorf("switch source: book not found")
	}

	// Build new alternates: push current active out, remove the chosen one from alts
	var newAlts []AltSource
	if b.SourceURL != "" {
		newAlts = append(newAlts, AltSource{
			SourceURL:  b.SourceURL,
			BookURL:    b.BookURL,
			SourceName: b.Origin,
		})
	}
	for _, a := range b.AlternateSources {
		if a.SourceURL == sourceURL && a.BookURL == bookURL {
			continue // this one becomes the new active
		}
		newAlts = append(newAlts, a)
	}

	altJSON, _ := json.Marshal(newAlts)
	now := time.Now().UnixMilli()

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.Exec(`UPDATE books SET
		source_url = ?, book_url = ?, origin = ?, toc_url = '',
		dur_chapter_index = 0, dur_chapter_pos = 0, total_chapter_num = 0,
		alternate_sources = ?, updated_at = ?
		WHERE id = ?`,
		sourceURL, bookURL, sourceName, string(altJSON), now, bookID)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`DELETE FROM chapters WHERE book_id = ?`, bookID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// scanBooks scans book rows.
func scanBooks(rows *sql.Rows) ([]Book, error) {
	var list []Book
	for rows.Next() {
		b, err := scanBookFromScanner(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, *b)
	}
	return list, rows.Err()
}

func scanBookRow(row *sql.Row, b *Book) error {
	loaded, err := scanBookFromScanner(row)
	if err != nil {
		return err
	}
	*b = *loaded
	return nil
}

// scanBookFromScanner scans a single book row, handling alternate_sources.
type scanner interface {
	Scan(dest ...interface{}) error
}

func scanBookFromScanner(s scanner) (*Book, error) {
	var b Book
	var altSourcesStr string
	// Scan order MUST match bookColumns (explicit column list, not SELECT *).
	if err := s.Scan(
		&b.ID, &b.Name, &b.Author, &b.CoverURL, &b.Intro, &b.Kind,
		&b.SourceURL, &b.BookURL, &b.TocURL, &b.Origin, &b.VariableMap,
		&b.LastChapter, &b.UpdateTime, &b.WordCount,
		&b.DurChapterIndex, &b.DurChapterPos, &b.TotalChapterNum,
		&altSourcesStr,
		&b.CreatedAt, &b.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if altSourcesStr != "" && altSourcesStr != "[]" {
		json.Unmarshal([]byte(altSourcesStr), &b.AlternateSources)
	}
	return &b, nil
}

func scanChapters(rows *sql.Rows) ([]Chapter, error) {
	var list []Chapter
	for rows.Next() {
		var ch Chapter
		if err := rows.Scan(&ch.ID, &ch.BookID, &ch.Index, &ch.Title, &ch.URL, &ch.IsVip, &ch.IsVolume, &ch.IsPay, &ch.BaseURL, &ch.Tag, &ch.WordCount, &ch.Cached); err != nil {
			return nil, err
		}
		list = append(list, ch)
	}
	return list, rows.Err()
}

func ensureColumn(db *sql.DB, table, column, definition string) error {
	rows, err := db.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, typ string
		var notNull, primaryKey int
		var defaultValue interface{}
		if err := rows.Scan(&cid, &name, &typ, &notNull, &defaultValue, &primaryKey); err != nil {
			return err
		}
		if name == column {
			return rows.Err()
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	_, err = db.Exec(`ALTER TABLE ` + table + ` ADD COLUMN ` + column + ` ` + definition)
	return err
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
