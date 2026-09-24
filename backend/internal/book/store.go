// Package book manages books on the shelf: search results, chapters, content caching.
package book

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/otwako/novelreader/internal/library"
	"github.com/otwako/novelreader/internal/readerstore"
)

var (
	ErrBookNotFound     = library.ErrNotFound
	ErrBookStateChanged = library.ErrStateChanged
	ErrInvalidProgress  = library.ErrInvalidProgress
)

// AltSource is a complete source binding for the same logical book.
// It may occupy either the active or alternate role.
type AltSource struct {
	VariableMap    string   `json:"variableMap,omitempty"` // alternate binding snapshot; active variables live on Book
	SourceID       string   `json:"sourceId"`
	SourceURL      string   `json:"sourceUrl"`
	BookURL        string   `json:"bookUrl"`
	SourceName     string   `json:"sourceName"`
	SourceGroup    string   `json:"sourceGroup,omitempty"`
	Capabilities   []string `json:"capabilities,omitempty"`
	DiscoveryQuery string   `json:"discoveryQuery,omitempty"`
	LastChapter    string   `json:"lastChapter,omitempty"`
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
	Provider        string   `json:"provider"`
	ContentRevision int64    `json:"contentRevision"`
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

	CreatedAt  int64 `json:"createdAt" db:"created_at"`
	UpdatedAt  int64 `json:"updatedAt" db:"updated_at"`
	LastReadAt int64 `json:"lastReadAt" db:"-"`
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
	VariableMap      string      `json:"variableMap,omitempty"` // serialized per-result rule variables
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
	return readerstore.ReaderSchema{
		Initialize: func(tx *sql.Tx) error { return initSchema(tx) },
		PreparePortable: func(ctx context.Context, tx *sql.Tx) error {
			// Catalog identities are durable; fetched content and availability flags are not.
			if _, err := tx.ExecContext(ctx, `DELETE FROM chapter_cache`); err != nil {
				return fmt.Errorf("book: remove portable chapter cache: %w", err)
			}
			if _, err := tx.ExecContext(ctx, `UPDATE chapters SET cached=0 WHERE cached<>0`); err != nil {
				return fmt.Errorf("book: clear portable chapter cache flags: %w", err)
			}
			return nil
		},
	}
}

type schemaDatabase interface {
	Exec(query string, args ...any) (sql.Result, error)
}

func initSchema(db schemaDatabase) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS books (
			id TEXT PRIMARY KEY REFERENCES library_items(id) ON DELETE CASCADE,
			identity_name TEXT NOT NULL DEFAULT '',
			identity_author TEXT NOT NULL DEFAULT '',
			source_id TEXT NOT NULL,
			source_url TEXT NOT NULL,
			book_url TEXT NOT NULL,
			toc_url TEXT DEFAULT '',
			origin TEXT NOT NULL DEFAULT '',
			variable_map TEXT DEFAULT '',
			alternate_sources TEXT DEFAULT '[]'
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
			FOREIGN KEY (book_id) REFERENCES books(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_chapters_book_id ON chapters(book_id, idx)`,
		`CREATE TABLE IF NOT EXISTS chapter_cache (
			book_id TEXT NOT NULL,
			content_revision INTEGER NOT NULL,
			source_id TEXT NOT NULL,
			chapter_index INTEGER NOT NULL,
			chapter_url TEXT NOT NULL,
			title TEXT NOT NULL,
			paragraphs TEXT NOT NULL,
			blocks TEXT NOT NULL DEFAULT '[]',
			cached_at INTEGER NOT NULL,
			last_accessed INTEGER NOT NULL,
			PRIMARY KEY (book_id, source_id, chapter_index),
			FOREIGN KEY (book_id) REFERENCES books(id) ON DELETE CASCADE
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
var bookColumns = `id, source_id, source_url, book_url, toc_url, origin, variable_map, alternate_sources`

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

func (s *Store) AddBook(b *Book) error {
	b.Intro = NormalizeDescription(b.Intro)
	b.CreatedAt, b.UpdatedAt = time.Now().UnixMilli(), time.Now().UnixMilli()
	state := bindingStateFromBook(b)
	applyBindingState(b, state)
	bindingJSON, err := encodeBindingState(state)
	if err != nil {
		return err
	}
	identityName, identityAuthor := NormalizeBookIdentity(b.Name, b.Author)
	if identityName == "" {
		return errors.New("book: name is required")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	b.Provider = library.BookSource
	if err := library.PutTx(context.Background(), tx, b.libraryItem()); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO books (id, identity_name, identity_author, source_id, source_url, book_url, toc_url, origin, variable_map, alternate_sources)
		VALUES (?,?,?,?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET identity_name=excluded.identity_name, identity_author=excluded.identity_author,
		source_id=excluded.source_id, source_url=excluded.source_url, book_url=excluded.book_url, toc_url=excluded.toc_url,
		origin=excluded.origin, variable_map=excluded.variable_map, alternate_sources=excluded.alternate_sources`,
		b.ID, identityName, identityAuthor, b.SourceID, b.SourceURL, b.BookURL, b.TocURL, b.Origin, b.VariableMap, bindingJSON); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) AddOrMergeBook(candidate *Book) (*Book, bool, error) {
	return s.addOrMergeBook(candidate, nil)
}

func (s *Store) AddOrMergeBookWithChapters(candidate *Book, chapters []Chapter) (*Book, bool, error) {
	return s.addOrMergeBook(candidate, chapters)
}

func (s *Store) addOrMergeBook(candidate *Book, chapters []Chapter) (*Book, bool, error) {
	if candidate == nil {
		return nil, false, errors.New("book: candidate is nil")
	}
	s.mergeMu.Lock()
	defer s.mergeMu.Unlock()
	identityName, identityAuthor := NormalizeBookIdentity(candidate.Name, candidate.Author)
	if identityName == "" {
		return nil, false, errors.New("book: name is required")
	}
	existing, err := s.getBookByIdentity(identityName, identityAuthor)
	if err != nil {
		return nil, false, err
	}
	candidateState := bindingStateFromBook(candidate)
	if existing != nil {
		stored, err := s.updateBindingStateLocked(existing.ID, func(state bindingState) (bindingState, error) {
			state = state.upsert(candidateState.Active)
			for _, binding := range candidateState.Alternates {
				state = state.upsert(binding)
			}
			return state, nil
		})
		return stored, false, err
	}
	candidate.Intro = NormalizeDescription(candidate.Intro)
	candidate.CreatedAt = time.Now().UnixMilli()
	candidate.UpdatedAt = candidate.CreatedAt
	candidate.Provider = library.BookSource
	if len(chapters) > 0 {
		candidate.TotalChapterNum = len(chapters)
		candidate.CurrentChapterTitle = chapterTitleAt(chapters, candidate.DurChapterIndex)
	}
	applyBindingState(candidate, candidateState)
	bindingJSON, err := encodeBindingState(candidateState)
	if err != nil {
		return nil, false, err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return nil, false, err
	}
	defer tx.Rollback()
	if err := library.InsertTx(context.Background(), tx, candidate.libraryItem()); err != nil {
		return nil, false, err
	}
	if _, err := tx.Exec(`INSERT INTO books (id, identity_name, identity_author, source_id, source_url, book_url, toc_url, origin, variable_map, alternate_sources) VALUES (?,?,?,?,?,?,?,?,?,?)`,
		candidate.ID, identityName, identityAuthor, candidate.SourceID, candidate.SourceURL, candidate.BookURL, candidate.TocURL, candidate.Origin, candidate.VariableMap, bindingJSON); err != nil {
		return nil, false, err
	}
	if len(chapters) > 0 {
		if err := replaceChaptersTx(tx, candidate.ID, chapters, false); err != nil {
			return nil, false, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, false, err
	}
	stored, err := s.GetBook(candidate.ID)
	return stored, true, err
}

// ClearBookSources removes discovered alternate bindings without changing the
// active source or reading state.
func (s *Store) ClearBookSources(bookID string) (*Book, error) {
	return s.updateBindingState(bookID, func(state bindingState) (bindingState, error) {
		return state.clearAlternates(), nil
	})
}

// MergeBookSources adds source bindings to an existing shelf book without
// switching its active source or touching reading state.
func (s *Store) MergeBookSources(bookID string, sources []AltSource) (*Book, error) {
	return s.updateBindingState(bookID, func(state bindingState) (bindingState, error) {
		for _, source := range sources {
			state = state.upsert(source)
		}
		return state, nil
	})
}

func (s *Store) updateBindingState(bookID string, mutate func(bindingState) (bindingState, error)) (*Book, error) {
	s.mergeMu.Lock()
	defer s.mergeMu.Unlock()
	return s.updateBindingStateLocked(bookID, mutate)
}

func (s *Store) updateBindingStateLocked(bookID string, mutate func(bindingState) (bindingState, error)) (*Book, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	// Reserve the write before reading state shared with catalog/source changes.
	if _, err := tx.Exec(`UPDATE books SET source_id=source_id WHERE id=?`, bookID); err != nil {
		return nil, err
	}
	stored, err := readBookTx(context.Background(), tx, bookID)
	if err != nil {
		return nil, err
	}
	if stored == nil {
		return nil, ErrBookNotFound
	}
	state, err := mutate(bindingStateFromBook(stored))
	if err != nil {
		return nil, err
	}
	encoded, err := encodeBindingState(state)
	if err != nil {
		return nil, err
	}
	if _, err := tx.Exec(`UPDATE books SET alternate_sources=? WHERE id=?`, encoded, stored.ID); err != nil {
		return nil, err
	}
	if err := library.TouchTx(context.Background(), tx, stored.ID); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	applyBindingState(stored, state)
	return stored, nil
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
	if incoming.VariableMap != "" {
		existing.VariableMap = incoming.VariableMap
	}
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
	if incoming.LastChapter != "" {
		existing.LastChapter = incoming.LastChapter
	}
	return existing
}

func (s *Store) DeleteBook(id string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := library.DeleteTx(context.Background(), tx, id); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) ListBooks() ([]Book, error) {
	tx, err := s.db.BeginTx(context.Background(), &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	items, err := library.ListTx(context.Background(), tx)
	if err != nil {
		return nil, err
	}
	rows, err := tx.Query(`SELECT ` + bookColumns + ` FROM books`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	bindings, err := scanBooks(rows)
	if err != nil {
		return nil, err
	}
	byID := make(map[string]Book, len(bindings))
	for _, b := range bindings {
		byID[b.ID] = b
	}
	result := make([]Book, 0, len(bindings))
	for _, item := range items {
		if b, ok := byID[item.ID]; ok {
			b.applyLibraryItem(item)
			result = append(result, b)
		}
	}
	return result, nil
}

func (s *Store) GetBook(id string) (*Book, error) {
	tx, err := s.db.BeginTx(context.Background(), &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	return readBookTx(context.Background(), tx, id)
}

// SaveChapters replaces all chapters for a book.
func (s *Store) SaveChapters(bookID string, chapters []Chapter) error {
	return s.replaceChapters(bookID, chapters, true)
}

func (s *Store) SaveCatalog(bookID, sourceID string, contentRevision int64, chapters []Chapter) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// Reserve the writer before reading the shared interpretation guard.
	result, err := tx.Exec(`UPDATE books SET source_id=source_id WHERE id=? AND source_id=?`, bookID, sourceID)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	item, err := library.GetTx(context.Background(), tx, bookID)
	if err != nil {
		return err
	}
	if item == nil {
		return ErrBookNotFound
	}
	if count == 0 || item.ContentRevision != contentRevision {
		return ErrCatalogSourceChanged
	}
	if err := replaceChaptersTx(tx, bookID, chapters, true); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) replaceChapters(bookID string, chapters []Chapter, updateTotal bool) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	if err := replaceChaptersTx(tx, bookID, chapters, updateTotal); err != nil {
		return err
	}
	return tx.Commit()
}

// replaceChaptersTx leaves commit/rollback to the use case that owns tx.
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
		item, err := library.GetTx(context.Background(), tx, bookID)
		if err != nil {
			return err
		}
		if item == nil {
			return ErrBookNotFound
		}
		return library.PublishCatalogTx(context.Background(), tx, bookID, item.ContentRevision, len(chapters), chapterTitleAt(chapters, item.DurChapterIndex))
	}

	return nil
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

// UpdateProgress resolves the BookSource chapter label before the shared CAS.
// The request/use case validates readability; library owns location/state writes.
func (s *Store) UpdateProgress(bookID string, contentRevision, stateVersion int64, chapterIndex int, position float64) (int64, error) {
	var title string
	err := s.db.QueryRow(`SELECT title FROM chapters WHERE book_id=? AND idx=? LIMIT 1`, bookID, chapterIndex).Scan(&title)
	if err != nil && err != sql.ErrNoRows {
		return 0, err
	}
	return library.NewStore(s.db).UpdateProgress(context.Background(), bookID,
		library.Revision{Content: contentRevision, State: stateVersion},
		library.Location{ChapterIndex: chapterIndex, Position: position, ChapterTitle: title})
}

func (s *Store) UpdateTotalChapters(bookID string, total int) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := library.UpdateTotalTx(context.Background(), tx, bookID, total); err != nil {
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

func scanBookRow(row scanner, b *Book) error {
	var bindingJSON string
	if err := row.Scan(&b.ID, &b.SourceID, &b.SourceURL, &b.BookURL, &b.TocURL, &b.Origin, &b.VariableMap, &bindingJSON); err != nil {
		return err
	}
	state, err := decodeBindingState(bindingJSON, b)
	if err != nil {
		return err
	}
	applyBindingState(b, state)
	b.Intro = NormalizeDescription(b.Intro)
	return nil
}

// scanBookFromScanner scans a single book row, handling alternate_sources.
type scanner interface {
	Scan(dest ...interface{}) error
}

func scanBookFromScanner(row scanner) (*Book, error) {
	b := &Book{}
	if err := scanBookRow(row, b); err != nil {
		return nil, err
	}
	return b, nil
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
