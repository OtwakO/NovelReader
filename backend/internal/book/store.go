// Package book manages books on the shelf: search results, chapters, content caching.
package book

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/otwako/novelreader/internal/readerstore"
)

var (
	ErrBookNotFound     = errors.New("book: not found")
	ErrBookStateChanged = errors.New("book: state changed")
	ErrInvalidProgress  = errors.New("book: invalid progress")
)

// AltSource is a secondary source for the same book.
type AltSource struct {
	SourceID       string   `json:"sourceId"`
	SourceURL      string   `json:"sourceUrl"`
	BookURL        string   `json:"bookUrl"`
	SourceName     string   `json:"sourceName"`
	SourceGroup    string   `json:"sourceGroup,omitempty"`
	Capabilities   []string `json:"capabilities,omitempty"`
	DiscoveryQuery string   `json:"discoveryQuery,omitempty"`
}

// PreviewBook is source-derived detail data without shelf ownership or progress.
type PreviewBook struct {
	Name             string      `json:"name"`
	Author           string      `json:"author,omitempty"`
	CoverURL         string      `json:"coverUrl,omitempty"`
	CoverDisplayURL  string      `json:"coverDisplayUrl,omitempty"`
	Intro            string      `json:"intro,omitempty"`
	Kind             string      `json:"kind,omitempty"`
	SourceID         string      `json:"sourceId"`
	SourceURL        string      `json:"sourceUrl"`
	BookURL          string      `json:"bookUrl"`
	TocURL           string      `json:"tocUrl,omitempty"`
	Origin           string      `json:"origin,omitempty"`
	LastChapter      string      `json:"lastChapter,omitempty"`
	UpdateTime       string      `json:"updateTime,omitempty"`
	WordCount        string      `json:"wordCount,omitempty"`
	DownloadURLs     []string    `json:"downloadUrls,omitempty"`
	AlternateSources []AltSource `json:"alternateSources,omitempty"`
}

// Book represents a book on the user's shelf.
type Book struct {
	ID              string   `json:"id" db:"id"`
	Name            string   `json:"name" db:"name"`
	Author          string   `json:"author,omitempty" db:"author"`
	CoverURL        string   `json:"coverUrl,omitempty" db:"cover_url"`
	CoverDisplayURL string   `json:"coverDisplayUrl,omitempty" db:"-"`
	Intro           string   `json:"intro,omitempty" db:"intro"`
	Kind            string   `json:"kind,omitempty" db:"kind"`
	SourceID        string   `json:"sourceId" db:"source_id"`
	SourceURL       string   `json:"sourceUrl" db:"source_url"`
	BookURL         string   `json:"bookUrl" db:"book_url"`
	TocURL          string   `json:"tocUrl,omitempty" db:"toc_url"`
	Origin          string   `json:"origin" db:"origin"`
	VariableMap     string   `json:"variableMap,omitempty" db:"variable_map"`
	LastChapter     string   `json:"lastChapter,omitempty" db:"last_chapter"`
	UpdateTime      string   `json:"updateTime,omitempty" db:"update_time"`
	WordCount       string   `json:"wordCount,omitempty" db:"word_count"`
	DownloadURLs    []string `json:"downloadUrls,omitempty" db:"-"`

	DurChapterIndex     int     `json:"durChapterIndex" db:"dur_chapter_index"`
	DurChapterPos       float64 `json:"durChapterPos" db:"dur_chapter_pos"`
	TotalChapterNum     int     `json:"totalChapterNum" db:"total_chapter_num"`
	StateVersion        int64   `json:"stateVersion" db:"state_version"`
	CurrentChapterTitle string  `json:"currentChapterTitle,omitempty" db:"-"`

	ActiveSource     *AltSource  `json:"activeSource,omitempty" db:"-"`
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
	CoverDisplayURL  string      `json:"coverDisplayUrl,omitempty"`
	Intro            string      `json:"intro"`
	Kind             string      `json:"kind"`
	LastChapter      string      `json:"lastChapter"`
	UpdateTime       string      `json:"updateTime"`
	WordCount        string      `json:"wordCount"`
	BookURL          string      `json:"bookUrl"`
	SourceID         string      `json:"sourceId"`
	SourceURL        string      `json:"sourceUrl"`
	SourceName       string      `json:"sourceName"`
	SourceGroup      string      `json:"sourceGroup,omitempty"`
	Capabilities     []string    `json:"capabilities,omitempty"`
	Score            int         `json:"score"`
	ShelfBookID      string      `json:"shelfBookId,omitempty"`
	AlternateSources []AltSource `json:"alternateSources,omitempty"`
}

// ShelfBookIdentity identifies one logical book already stored on the shelf.
type ShelfBookIdentity struct {
	ID     string
	Name   string
	Author string
}

// Store handles book persistence.
type Store struct {
	db      *sql.DB
	mergeMu sync.Mutex
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// ReaderSchema returns the current bookshelf schema module for fresh initialization and validation.
func ReaderSchema() readerstore.ReaderSchema {
	return readerstore.ReaderSchema{Initialize: func(tx *sql.Tx) error { return initSchema(tx) }}
}

type schemaDatabase interface {
	Exec(query string, args ...any) (sql.Result, error)
}

func initSchema(db schemaDatabase) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS books (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			author TEXT DEFAULT '',
			identity_name TEXT NOT NULL DEFAULT '',
			identity_author TEXT NOT NULL DEFAULT '',
			cover_url TEXT DEFAULT '',
			intro TEXT DEFAULT '',
			kind TEXT DEFAULT '',
			source_id TEXT NOT NULL,
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
			state_version INTEGER DEFAULT 0,
			alternate_sources TEXT DEFAULT '[]',
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_books_logical_identity ON books(identity_name, identity_author)`,
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
		`CREATE TABLE IF NOT EXISTS bookmarks (
			id TEXT PRIMARY KEY,
			book_id TEXT NOT NULL,
			chapter_index INTEGER NOT NULL,
			chapter_title TEXT NOT NULL,
			position REAL NOT NULL,
			note TEXT DEFAULT '',
			orphaned INTEGER DEFAULT 0,
			created_at INTEGER NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_bookmarks_book_id ON bookmarks(book_id, created_at DESC)`,
		`CREATE TABLE IF NOT EXISTS chapter_cache (
			book_id TEXT NOT NULL,
			source_id TEXT NOT NULL,
			chapter_index INTEGER NOT NULL,
			chapter_url TEXT NOT NULL,
			title TEXT NOT NULL,
			paragraphs TEXT NOT NULL,
			blocks TEXT NOT NULL DEFAULT '[]',
			cached_at INTEGER NOT NULL,
			last_accessed INTEGER NOT NULL,
			PRIMARY KEY (book_id, source_id, chapter_index)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_chapter_cache_lru ON chapter_cache(last_accessed)`,
		`CREATE INDEX IF NOT EXISTS idx_chapter_cache_book_lru ON chapter_cache(book_id, last_accessed)`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("book: init: %w", err)
		}
	}
	return nil
}

// bookColumns keeps SELECT scan order explicit and deterministic.
var bookColumns = `id, name, author, cover_url, intro, kind,
	source_id, source_url, book_url, toc_url, origin, variable_map,
	last_chapter, update_time, word_count,
	dur_chapter_index, dur_chapter_pos, total_chapter_num, state_version,
	alternate_sources, created_at, updated_at`

// chapterColumns for SELECT queries on the chapters table.
var chapterColumns = `id, book_id, idx, title, url, is_vip, is_volume, is_pay, base_url, tag, word_count, cached`

// NormalizeBookIdentity returns the logical shelf identity for a book.
func NormalizeBookIdentity(name, author string) (string, string) {
	return normalizeIdentityPart(name, false), normalizeIdentityPart(author, true)
}

// ListShelfBookIdentities returns the compact logical identity index used to
// annotate discovery results without exposing full shelf records.
func (s *Store) ListShelfBookIdentities() ([]ShelfBookIdentity, error) {
	rows, err := s.db.Query(`SELECT id, identity_name, identity_author FROM books`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	identities := make([]ShelfBookIdentity, 0)
	for rows.Next() {
		var identity ShelfBookIdentity
		if err := rows.Scan(&identity.ID, &identity.Name, &identity.Author); err != nil {
			return nil, err
		}
		identities = append(identities, identity)
	}
	return identities, rows.Err()
}

func normalizeIdentityPart(value string, author bool) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if author {
		for _, prefix := range []string{"作者：", "作者:", "author：", "author:"} {
			value = strings.TrimSpace(strings.TrimPrefix(value, prefix))
		}
		for _, suffix := range []string{" 著", "著", " 作", "作"} {
			value = strings.TrimSpace(strings.TrimSuffix(value, suffix))
		}
	}
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			return r
		}
		return -1
	}, value)
}

// AddBook inserts a book into the shelf. Internal fixtures that intentionally
// replace an ID retain this low-level behavior; user-facing adds use AddOrMergeBook.
func (s *Store) AddBook(b *Book) error {
	b.Intro = NormalizeDescription(b.Intro)
	now := time.Now().UnixMilli()
	b.CreatedAt = now
	b.UpdatedAt = now
	altJSON, _ := json.Marshal(persistedSourceBindings(b))
	if len(altJSON) == 0 {
		altJSON = []byte("[]")
	}
	identityName, identityAuthor := NormalizeBookIdentity(b.Name, b.Author)
	if identityName == "" {
		return errors.New("book: name is required")
	}
	_, err := s.db.Exec(`INSERT INTO books (
		id, name, author, identity_name, identity_author, cover_url, intro, kind,
		source_id, source_url, book_url, toc_url, origin, variable_map,
		last_chapter, update_time, word_count,
		dur_chapter_index, dur_chapter_pos, total_chapter_num, state_version,
		alternate_sources,
		created_at, updated_at
	) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
	ON CONFLICT(id) DO UPDATE SET
		name=excluded.name, author=excluded.author,
		identity_name=excluded.identity_name, identity_author=excluded.identity_author,
		cover_url=excluded.cover_url, intro=excluded.intro, kind=excluded.kind,
		source_id=excluded.source_id, source_url=excluded.source_url, book_url=excluded.book_url, toc_url=excluded.toc_url,
		origin=excluded.origin, variable_map=excluded.variable_map,
		last_chapter=excluded.last_chapter, update_time=excluded.update_time, word_count=excluded.word_count,
		dur_chapter_index=excluded.dur_chapter_index, dur_chapter_pos=excluded.dur_chapter_pos,
		total_chapter_num=excluded.total_chapter_num, state_version=excluded.state_version,
		alternate_sources=excluded.alternate_sources, updated_at=excluded.updated_at`,
		b.ID, b.Name, b.Author, identityName, identityAuthor, b.CoverURL, b.Intro, b.Kind,
		b.SourceID, b.SourceURL, b.BookURL, b.TocURL, b.Origin, b.VariableMap,
		b.LastChapter, b.UpdateTime, b.WordCount,
		b.DurChapterIndex, b.DurChapterPos, b.TotalChapterNum, b.StateVersion,
		string(altJSON),
		b.CreatedAt, b.UpdatedAt,
	)
	return err
}

// AddOrMergeBook inserts one logical title+author shelf row or merges newly
// discovered source bindings into the existing row without changing its ID,
// current source, reading state, chapters, cache, or bookmarks.
func (s *Store) AddOrMergeBook(candidate *Book) (*Book, bool, error) {
	candidate.Intro = NormalizeDescription(candidate.Intro)
	s.mergeMu.Lock()
	defer s.mergeMu.Unlock()
	identityName, identityAuthor := NormalizeBookIdentity(candidate.Name, candidate.Author)
	if identityName == "" {
		return nil, false, errors.New("book: name is required")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return nil, false, err
	}
	defer tx.Rollback() //nolint:errcheck

	var existing Book
	err = scanBookRow(tx.QueryRow(`SELECT `+bookColumns+` FROM books WHERE identity_name = ? AND identity_author = ?`, identityName, identityAuthor), &existing)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, false, err
	}
	if errors.Is(err, sql.ErrNoRows) {
		now := time.Now().UnixMilli()
		candidate.CreatedAt = now
		candidate.UpdatedAt = now
		candidate.AlternateSources = mergeAlternateSources(candidate.SourceID, candidate.BookURL, candidate.AlternateSources)
		alternateJSON, err := json.Marshal(persistedSourceBindings(candidate))
		if err != nil {
			return nil, false, err
		}
		if _, err := tx.Exec(`INSERT INTO books (
			id, name, author, identity_name, identity_author, cover_url, intro, kind,
			source_id, source_url, book_url, toc_url, origin, variable_map,
			last_chapter, update_time, word_count,
			dur_chapter_index, dur_chapter_pos, total_chapter_num, state_version,
			alternate_sources, created_at, updated_at
		) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			candidate.ID, candidate.Name, candidate.Author, identityName, identityAuthor,
			candidate.CoverURL, candidate.Intro, candidate.Kind,
			candidate.SourceID, candidate.SourceURL, candidate.BookURL, candidate.TocURL, candidate.Origin, candidate.VariableMap,
			candidate.LastChapter, candidate.UpdateTime, candidate.WordCount,
			candidate.DurChapterIndex, candidate.DurChapterPos, candidate.TotalChapterNum, candidate.StateVersion,
			string(alternateJSON), candidate.CreatedAt, candidate.UpdatedAt,
		); err != nil {
			return nil, false, err
		}
		if err := tx.Commit(); err != nil {
			return nil, false, err
		}
		return candidate, true, nil
	}

	discovered := append([]AltSource(nil), existing.AlternateSources...)
	discovered = append(discovered, AltSource{SourceID: candidate.SourceID, SourceURL: candidate.SourceURL, BookURL: candidate.BookURL, SourceName: candidate.Origin})
	discovered = append(discovered, candidate.AlternateSources...)
	existing.AlternateSources = mergeAlternateSources(existing.SourceID, existing.BookURL, discovered)
	alternateJSON, err := json.Marshal(persistedSourceBindings(&existing))
	if err != nil {
		return nil, false, err
	}
	if _, err := tx.Exec(`UPDATE books SET alternate_sources = ?, updated_at = ? WHERE id = ?`, string(alternateJSON), time.Now().UnixMilli(), existing.ID); err != nil {
		return nil, false, err
	}
	if err := tx.Commit(); err != nil {
		return nil, false, err
	}
	return &existing, false, nil
}

// AddOrMergeBookWithChapters persists a new readable book and its verified TOC
// atomically. Existing logical books keep their active source and chapters; the
// validated candidate is merged only as another source binding.
func (s *Store) AddOrMergeBookWithChapters(candidate *Book, chapters []Chapter) (*Book, bool, error) {
	s.mergeMu.Lock()
	defer s.mergeMu.Unlock()
	candidate.Intro = NormalizeDescription(candidate.Intro)
	identityName, identityAuthor := NormalizeBookIdentity(candidate.Name, candidate.Author)
	if identityName == "" {
		return nil, false, errors.New("book: name is required")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return nil, false, err
	}
	defer tx.Rollback() //nolint:errcheck

	var existing Book
	err = scanBookRow(tx.QueryRow(`SELECT `+bookColumns+` FROM books WHERE identity_name = ? AND identity_author = ?`, identityName, identityAuthor), &existing)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, false, err
	}
	if err == nil {
		discovered := append([]AltSource(nil), existing.AlternateSources...)
		discovered = append(discovered, AltSource{SourceID: candidate.SourceID, SourceURL: candidate.SourceURL, BookURL: candidate.BookURL, SourceName: candidate.Origin})
		discovered = append(discovered, candidate.AlternateSources...)
		existing.AlternateSources = mergeAlternateSources(existing.SourceID, existing.BookURL, discovered)
		alternateJSON, marshalErr := json.Marshal(persistedSourceBindings(&existing))
		if marshalErr != nil {
			return nil, false, marshalErr
		}
		if _, err := tx.Exec(`UPDATE books SET alternate_sources = ?, updated_at = ? WHERE id = ?`, string(alternateJSON), time.Now().UnixMilli(), existing.ID); err != nil {
			return nil, false, err
		}
		if err := tx.Commit(); err != nil {
			return nil, false, err
		}
		return &existing, false, nil
	}

	now := time.Now().UnixMilli()
	candidate.CreatedAt = now
	candidate.UpdatedAt = now
	candidate.TotalChapterNum = len(chapters)
	candidate.AlternateSources = mergeAlternateSources(candidate.SourceID, candidate.BookURL, candidate.AlternateSources)
	alternateJSON, err := json.Marshal(persistedSourceBindings(candidate))
	if err != nil {
		return nil, false, err
	}
	if _, err := tx.Exec(`INSERT INTO books (
		id, name, author, identity_name, identity_author, cover_url, intro, kind,
		source_id, source_url, book_url, toc_url, origin, variable_map,
		last_chapter, update_time, word_count,
		dur_chapter_index, dur_chapter_pos, total_chapter_num, state_version,
		alternate_sources, created_at, updated_at
	) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		candidate.ID, candidate.Name, candidate.Author, identityName, identityAuthor,
		candidate.CoverURL, candidate.Intro, candidate.Kind,
		candidate.SourceID, candidate.SourceURL, candidate.BookURL, candidate.TocURL, candidate.Origin, candidate.VariableMap,
		candidate.LastChapter, candidate.UpdateTime, candidate.WordCount,
		candidate.DurChapterIndex, candidate.DurChapterPos, candidate.TotalChapterNum, candidate.StateVersion,
		string(alternateJSON), candidate.CreatedAt, candidate.UpdatedAt,
	); err != nil {
		return nil, false, err
	}
	for index := range chapters {
		chapter := chapters[index]
		chapter.BookID = candidate.ID
		if chapter.ID == "" {
			chapter.ID = fmt.Sprintf("%s_%d", candidate.ID, chapter.Index)
		}
		if _, err := tx.Exec(`INSERT INTO chapters (id, book_id, idx, title, url, is_vip, is_volume, is_pay, base_url, tag, word_count, cached) VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`,
			chapter.ID, chapter.BookID, chapter.Index, chapter.Title, chapter.URL, boolToInt(chapter.IsVip), boolToInt(chapter.IsVolume), boolToInt(chapter.IsPay), chapter.BaseURL, chapter.Tag, chapter.WordCount, boolToInt(chapter.Cached)); err != nil {
			return nil, false, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, false, err
	}
	return candidate, true, nil
}

// ClearBookSources removes discovered alternate bindings without changing the
// active source or reading state.
func (s *Store) ClearBookSources(bookID string) (*Book, error) {
	s.mergeMu.Lock()
	defer s.mergeMu.Unlock()
	result, err := s.db.Exec(`UPDATE books SET alternate_sources = '[]', updated_at = ? WHERE id = ?`, time.Now().UnixMilli(), bookID)
	if err != nil {
		return nil, err
	}
	if affected, err := result.RowsAffected(); err != nil {
		return nil, err
	} else if affected == 0 {
		return nil, ErrBookNotFound
	}
	return s.GetBook(bookID)
}

// MergeBookSources adds source bindings to an existing shelf book without
// switching its active source or touching reading state.
func (s *Store) MergeBookSources(bookID string, sources []AltSource) (*Book, error) {
	s.mergeMu.Lock()
	defer s.mergeMu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback() //nolint:errcheck
	var existing Book
	if err := scanBookRow(tx.QueryRow(`SELECT `+bookColumns+` FROM books WHERE id = ?`, bookID), &existing); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrBookNotFound
		}
		return nil, err
	}
	existing.AlternateSources = mergeAlternateSources(existing.SourceID, existing.BookURL, append(existing.AlternateSources, sources...))
	alternateJSON, err := json.Marshal(persistedSourceBindings(&existing))
	if err != nil {
		return nil, err
	}
	if _, err := tx.Exec(`UPDATE books SET alternate_sources = ?, updated_at = ? WHERE id = ?`, string(alternateJSON), time.Now().UnixMilli(), existing.ID); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &existing, nil
}

func persistedSourceBindings(book *Book) []AltSource {
	bindings := append([]AltSource(nil), book.AlternateSources...)
	if book.ActiveSource != nil {
		bindings = append(bindings, *book.ActiveSource)
	}
	return mergeAlternateSources("", "", bindings)
}

func mergeAlternateSources(currentSourceID, currentBookURL string, sources []AltSource) []AltSource {
	currentKey := currentSourceID + "\n" + currentBookURL
	indexes := make(map[string]int, len(sources))
	merged := make([]AltSource, 0, len(sources))
	for _, source := range sources {
		if source.SourceID == "" || source.BookURL == "" {
			continue
		}
		key := source.SourceID + "\n" + source.BookURL
		if key == currentKey {
			continue
		}
		if index, exists := indexes[key]; exists {
			merged[index] = enrichAlternateSource(merged[index], source)
			continue
		}
		indexes[key] = len(merged)
		merged = append(merged, source)
	}
	return merged
}

func enrichAlternateSource(existing, incoming AltSource) AltSource {
	if incoming.SourceName != "" {
		existing.SourceName = incoming.SourceName
	}
	if incoming.SourceGroup != "" {
		existing.SourceGroup = incoming.SourceGroup
	}
	if len(incoming.Capabilities) > 0 {
		existing.Capabilities = incoming.Capabilities
	}
	if incoming.DiscoveryQuery != "" {
		existing.DiscoveryQuery = incoming.DiscoveryQuery
	}
	return existing
}

// DeleteBook removes a book from the shelf.
func (s *Store) DeleteBook(id string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	if _, err := tx.Exec(`DELETE FROM bookmarks WHERE book_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM chapter_cache WHERE book_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM chapters WHERE book_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM books WHERE id = ?`, id); err != nil {
		return err
	}
	return tx.Commit()
}

// ListBooks returns all books on the shelf.
func (s *Store) ListBooks() ([]Book, error) {
	rows, err := s.db.Query(`SELECT ` + bookColumns + `, COALESCE((SELECT title FROM chapters WHERE book_id = books.id AND idx = books.dur_chapter_index LIMIT 1), '') FROM books ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanShelfBooks(rows)
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
	return s.replaceChapters(bookID, chapters, false)
}

// SaveCatalog atomically replaces a book's complete chapter catalog and count
// only while the active source revision still matches the crawl that produced it.
func (s *Store) SaveCatalog(bookID, sourceID string, stateVersion int64, chapters []Chapter) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	var currentSourceID string
	var currentStateVersion int64
	if err := tx.QueryRow(`SELECT source_id, state_version FROM books WHERE id = ?`, bookID).Scan(&currentSourceID, &currentStateVersion); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrCatalogBookNotFound
		}
		return err
	}
	if currentSourceID != sourceID || currentStateVersion != stateVersion {
		return ErrCatalogSourceChanged
	}
	return replaceChaptersTx(tx, bookID, chapters, true)
}

func (s *Store) replaceChapters(bookID string, chapters []Chapter, updateTotal bool) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	return replaceChaptersTx(tx, bookID, chapters, updateTotal)
}

func replaceChaptersTx(tx *sql.Tx, bookID string, chapters []Chapter, updateTotal bool) error {
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
	if updateTotal {
		result, err := tx.Exec(`UPDATE books SET total_chapter_num = ?, updated_at = ? WHERE id = ?`, len(chapters), time.Now().UnixMilli(), bookID)
		if err != nil {
			return err
		}
		updated, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if updated != 1 {
			return errors.New("book: book not found")
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
func (s *Store) UpdateProgress(bookID, sourceID string, stateVersion int64, chapterIndex int, position float64) (int64, error) {
	if chapterIndex < 0 || math.IsNaN(position) || math.IsInf(position, 0) || position < 0 || position > 1 {
		return 0, ErrInvalidProgress
	}
	result, err := s.db.Exec(`UPDATE books SET dur_chapter_index = ?, dur_chapter_pos = ?, state_version = state_version + 1, updated_at = ? WHERE id = ? AND source_id = ? AND state_version = ?`,
		chapterIndex, position, time.Now().UnixMilli(), bookID, sourceID, stateVersion)
	if err != nil {
		return 0, err
	}
	updated, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	if updated == 0 {
		book, loadErr := s.GetBook(bookID)
		if loadErr != nil {
			return 0, loadErr
		}
		if book == nil {
			return 0, ErrBookNotFound
		}
		return 0, ErrBookStateChanged
	}
	return stateVersion + 1, nil
}

// UpdateTotalChapters updates the total chapter count for a book.
func (s *Store) UpdateTotalChapters(bookID string, total int) error {
	_, err := s.db.Exec(`UPDATE books SET total_chapter_num = ?, updated_at = ? WHERE id = ?`,
		total, time.Now().UnixMilli(), bookID)
	return err
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

func scanShelfBooks(rows *sql.Rows) ([]Book, error) {
	var list []Book
	for rows.Next() {
		b, err := scanBookFromScannerWithCurrentChapter(rows)
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
	return scanBook(s, false)
}

func scanBookFromScannerWithCurrentChapter(s scanner) (*Book, error) {
	return scanBook(s, true)
}

func scanBook(s scanner, includeCurrentChapter bool) (*Book, error) {
	var b Book
	var altSourcesStr string
	destinations := []interface{}{
		&b.ID, &b.Name, &b.Author, &b.CoverURL, &b.Intro, &b.Kind,
		&b.SourceID, &b.SourceURL, &b.BookURL, &b.TocURL, &b.Origin, &b.VariableMap,
		&b.LastChapter, &b.UpdateTime, &b.WordCount,
		&b.DurChapterIndex, &b.DurChapterPos, &b.TotalChapterNum, &b.StateVersion,
		&altSourcesStr,
		&b.CreatedAt, &b.UpdatedAt,
	}
	if includeCurrentChapter {
		destinations = append(destinations, &b.CurrentChapterTitle)
	}
	if err := s.Scan(destinations...); err != nil {
		return nil, err
	}
	if altSourcesStr != "" && altSourcesStr != "[]" {
		var persisted []AltSource
		if err := json.Unmarshal([]byte(altSourcesStr), &persisted); err == nil {
			for index := range persisted {
				source := persisted[index]
				if source.SourceID == b.SourceID && source.BookURL == b.BookURL {
					b.ActiveSource = &source
					continue
				}
				b.AlternateSources = append(b.AlternateSources, source)
			}
		}
	}
	b.Intro = NormalizeDescription(b.Intro)
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

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
