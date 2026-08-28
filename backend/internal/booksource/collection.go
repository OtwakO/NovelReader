package booksource

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

type CollectionOrigin string

type SyncInterval string

const (
	CollectionOriginUpload CollectionOrigin = "upload"
	CollectionOriginURL    CollectionOrigin = "url"

	SyncManual SyncInterval = "manual"
	SyncDaily  SyncInterval = "daily"
	SyncWeekly SyncInterval = "weekly"
)

var (
	ErrCollectionNotFound = errors.New("booksource: collection not found")
	ErrCollectionConflict = errors.New("booksource: source belongs to another collection")
	ErrCollectionNameUsed = errors.New("booksource: collection name is already used")
)

type Collection struct {
	ID             string           `json:"id"`
	Name           string           `json:"name"`
	OriginKind     CollectionOrigin `json:"originKind"`
	OriginURL      string           `json:"originUrl,omitempty"`
	OriginFilename string           `json:"originFilename,omitempty"`
	SyncInterval   SyncInterval     `json:"syncInterval"`
	SourceCount    int              `json:"sourceCount"`
	LastAttemptAt  *int64           `json:"lastAttemptAt,omitempty"`
	LastSuccessAt  *int64           `json:"lastSuccessAt,omitempty"`
	NextSyncAt     *int64           `json:"nextSyncAt,omitempty"`
	LastError      string           `json:"lastError,omitempty"`
	ETag           string           `json:"-"`
	LastModified   string           `json:"-"`
	CreatedAt      int64            `json:"createdAt"`
	UpdatedAt      int64            `json:"updatedAt"`
}

type ReplaceResult struct {
	Added     int `json:"added"`
	Updated   int `json:"updated"`
	Removed   int `json:"removed"`
	Unchanged int `json:"unchanged"`
	Total     int `json:"total"`
}

type CreateCollection struct {
	Name           string
	OriginKind     CollectionOrigin
	OriginURL      string
	OriginFilename string
	SyncInterval   SyncInterval
	ETag           string
	LastModified   string
}

func NormalizeCollectionName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("booksource: collection name is required")
	}
	if len([]rune(name)) > 100 {
		return "", fmt.Errorf("booksource: collection name is too long")
	}
	for _, r := range name {
		if r < 0x20 || r == 0x7f {
			return "", fmt.Errorf("booksource: collection name contains control characters")
		}
	}
	return name, nil
}

func ValidateSyncInterval(value SyncInterval) error {
	switch value {
	case SyncManual, SyncDaily, SyncWeekly:
		return nil
	default:
		return fmt.Errorf("booksource: invalid sync interval")
	}
}

func NextSyncAt(interval SyncInterval, after time.Time) *int64 {
	var next time.Time
	switch interval {
	case SyncDaily:
		next = after.Add(24 * time.Hour)
	case SyncWeekly:
		next = after.Add(7 * 24 * time.Hour)
	default:
		return nil
	}
	value := next.UnixMilli()
	return &value
}

func (s *Store) ListCollections() ([]Collection, error) {
	rows, err := s.db.Query(`
		SELECT c.id, c.name, c.origin_kind, COALESCE(c.origin_url, ''), COALESCE(c.origin_filename, ''),
			c.sync_interval, c.last_attempt_at, c.last_success_at, c.next_sync_at, c.last_error,
			c.etag, c.last_modified, c.created_at, c.updated_at, COUNT(bs.id)
		FROM book_source_collections c
		LEFT JOIN book_sources bs ON bs.collection_id = c.id
		GROUP BY c.id
		ORDER BY c.name COLLATE NOCASE, c.id`)
	if err != nil {
		return nil, fmt.Errorf("booksource: list collections: %w", err)
	}
	defer rows.Close()
	var collections []Collection
	for rows.Next() {
		collection, err := scanCollection(rows)
		if err != nil {
			return nil, err
		}
		collections = append(collections, collection)
	}
	return collections, rows.Err()
}

func (s *Store) ListDueCollections(now time.Time) ([]Collection, error) {
	rows, err := s.db.Query(`
		SELECT c.id, c.name, c.origin_kind, COALESCE(c.origin_url, ''), COALESCE(c.origin_filename, ''),
			c.sync_interval, c.last_attempt_at, c.last_success_at, c.next_sync_at, c.last_error,
			c.etag, c.last_modified, c.created_at, c.updated_at, COUNT(bs.id)
		FROM book_source_collections c
		LEFT JOIN book_sources bs ON bs.collection_id = c.id
		WHERE c.origin_kind = 'url' AND c.sync_interval != 'manual' AND c.next_sync_at <= ?
		GROUP BY c.id ORDER BY c.next_sync_at, c.id`, now.UnixMilli())
	if err != nil {
		return nil, fmt.Errorf("booksource: list due collections: %w", err)
	}
	defer rows.Close()
	var collections []Collection
	for rows.Next() {
		collection, err := scanCollection(rows)
		if err != nil {
			return nil, err
		}
		collections = append(collections, collection)
	}
	return collections, rows.Err()
}

func (s *Store) GetCollection(id string) (*Collection, error) {
	row := s.db.QueryRow(`
		SELECT c.id, c.name, c.origin_kind, COALESCE(c.origin_url, ''), COALESCE(c.origin_filename, ''),
			c.sync_interval, c.last_attempt_at, c.last_success_at, c.next_sync_at, c.last_error,
			c.etag, c.last_modified, c.created_at, c.updated_at, COUNT(bs.id)
		FROM book_source_collections c
		LEFT JOIN book_sources bs ON bs.collection_id = c.id
		WHERE c.id = ? GROUP BY c.id`, id)
	collection, err := scanCollection(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("booksource: get collection: %w", err)
	}
	return &collection, nil
}

func scanCollection(scanner interface{ Scan(...any) error }) (Collection, error) {
	var collection Collection
	var origin, interval string
	err := scanner.Scan(
		&collection.ID, &collection.Name, &origin, &collection.OriginURL, &collection.OriginFilename,
		&interval, &collection.LastAttemptAt, &collection.LastSuccessAt, &collection.NextSyncAt,
		&collection.LastError, &collection.ETag, &collection.LastModified,
		&collection.CreatedAt, &collection.UpdatedAt, &collection.SourceCount,
	)
	collection.OriginKind = CollectionOrigin(origin)
	collection.SyncInterval = SyncInterval(interval)
	return collection, err
}

func (s *Store) CreateCollection(ctx context.Context, input CreateCollection, sources []*BookSource, now time.Time) (Collection, ReplaceResult, error) {
	name, err := NormalizeCollectionName(input.Name)
	if err != nil {
		return Collection{}, ReplaceResult{}, err
	}
	if input.OriginKind != CollectionOriginUpload && input.OriginKind != CollectionOriginURL {
		return Collection{}, ReplaceResult{}, fmt.Errorf("booksource: invalid collection origin")
	}
	if err := ValidateSyncInterval(input.SyncInterval); err != nil {
		return Collection{}, ReplaceResult{}, err
	}
	id, err := newCollectionID()
	if err != nil {
		return Collection{}, ReplaceResult{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Collection{}, ReplaceResult{}, fmt.Errorf("booksource: begin collection creation: %w", err)
	}
	defer tx.Rollback()
	at := now.UnixMilli()
	_, err = tx.ExecContext(ctx, `INSERT INTO book_source_collections (
		id, name, origin_kind, origin_url, origin_filename, sync_interval,
		last_success_at, next_sync_at, etag, last_modified, created_at, updated_at
	) VALUES (?, ?, ?, NULLIF(?, ''), NULLIF(?, ''), ?, ?, ?, ?, ?, ?, ?)`,
		id, name, string(input.OriginKind), input.OriginURL, input.OriginFilename, string(input.SyncInterval),
		at, NextSyncAt(input.SyncInterval, now), input.ETag, input.LastModified, at, at)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return Collection{}, ReplaceResult{}, ErrCollectionNameUsed
		}
		return Collection{}, ReplaceResult{}, fmt.Errorf("booksource: insert collection: %w", err)
	}
	result, err := replaceCollectionSources(ctx, tx, id, sources, now)
	if err != nil {
		return Collection{}, ReplaceResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return Collection{}, ReplaceResult{}, fmt.Errorf("booksource: commit collection creation: %w", err)
	}
	collection, err := s.GetCollection(id)
	if err != nil || collection == nil {
		return Collection{}, ReplaceResult{}, fmt.Errorf("booksource: read created collection: %w", err)
	}
	return *collection, result, nil
}

func (s *Store) ReplaceCollection(ctx context.Context, id string, sources []*BookSource, originFilename, etag, lastModified string, now time.Time) (ReplaceResult, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ReplaceResult{}, fmt.Errorf("booksource: begin collection replacement: %w", err)
	}
	defer tx.Rollback()
	var interval string
	if err := tx.QueryRowContext(ctx, `SELECT sync_interval FROM book_source_collections WHERE id = ?`, id).Scan(&interval); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ReplaceResult{}, ErrCollectionNotFound
		}
		return ReplaceResult{}, err
	}
	result, err := replaceCollectionSources(ctx, tx, id, sources, now)
	if err != nil {
		return ReplaceResult{}, err
	}
	at := now.UnixMilli()
	_, err = tx.ExecContext(ctx, `UPDATE book_source_collections SET
		origin_filename = CASE WHEN ? = '' THEN origin_filename ELSE ? END,
		last_attempt_at = ?, last_success_at = ?, next_sync_at = ?, last_error = '',
		etag = ?, last_modified = ?, updated_at = ? WHERE id = ?`,
		originFilename, originFilename, at, at, NextSyncAt(SyncInterval(interval), now), etag, lastModified, at, id)
	if err != nil {
		return ReplaceResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return ReplaceResult{}, fmt.Errorf("booksource: commit collection replacement: %w", err)
	}
	return result, nil
}

func replaceCollectionSources(ctx context.Context, tx *sql.Tx, collectionID string, sources []*BookSource, now time.Time) (ReplaceResult, error) {
	for _, source := range sources {
		if source == nil || strings.TrimSpace(source.BookSourceURL) == "" {
			return ReplaceResult{}, fmt.Errorf("booksource: collection contains a source without bookSourceUrl")
		}
	}

	existingRows, err := tx.QueryContext(ctx, `SELECT `+sourceColumns+` FROM book_sources WHERE collection_id = ? ORDER BY custom_order, id`, collectionID)
	if err != nil {
		return ReplaceResult{}, err
	}
	existing, err := scanSources(existingRows)
	closeErr := existingRows.Close()
	if err != nil {
		return ReplaceResult{}, err
	}
	if closeErr != nil {
		return ReplaceResult{}, closeErr
	}

	matches, err := matchCollectionSources(existing, sources)
	if err != nil {
		return ReplaceResult{}, err
	}
	matchedExisting := make(map[int]bool, len(matches))
	result := ReplaceResult{Total: len(sources)}
	for incomingIndex, existingIndex := range matches {
		matchedExisting[existingIndex] = true
		incoming := sources[incomingIndex]
		current := existing[existingIndex]
		incoming.ID = current.ID
		currentJSON, _ := collectionDefinitionJSON(&current)
		incomingJSON, err := collectionDefinitionJSON(incoming)
		if err != nil {
			return ReplaceResult{}, err
		}
		if string(currentJSON) == string(incomingJSON) {
			result.Unchanged++
		} else {
			result.Updated++
		}
	}
	for index, source := range sources {
		if _, matched := matches[index]; !matched {
			id, err := newSourceID()
			if err != nil {
				return ReplaceResult{}, err
			}
			source.ID = id
			result.Added++
		}
		if err := upsertSource(ctx, tx, source, collectionID, now); err != nil {
			return ReplaceResult{}, err
		}
	}
	for index, source := range existing {
		if matchedExisting[index] {
			continue
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM book_sources WHERE id = ? AND collection_id = ?`, source.ID, collectionID); err != nil {
			return ReplaceResult{}, err
		}
		result.Removed++
	}
	return result, nil
}

func matchCollectionSources(existing []BookSource, incoming []*BookSource) (map[int]int, error) {
	matches := make(map[int]int)
	used := make(map[int]bool)
	for incomingIndex, next := range incoming {
		nextJSON, err := collectionDefinitionJSON(next)
		if err != nil {
			return nil, err
		}
		for existingIndex := range existing {
			if used[existingIndex] || existing[existingIndex].BookSourceURL != next.BookSourceURL {
				continue
			}
			currentJSON, _ := collectionDefinitionJSON(&existing[existingIndex])
			if string(currentJSON) == string(nextJSON) {
				matches[incomingIndex] = existingIndex
				used[existingIndex] = true
				break
			}
		}
	}
	for incomingIndex, next := range incoming {
		if _, matched := matches[incomingIndex]; matched {
			continue
		}
		for existingIndex := range existing {
			if !used[existingIndex] && existing[existingIndex].BookSourceURL == next.BookSourceURL {
				matches[incomingIndex] = existingIndex
				used[existingIndex] = true
				break
			}
		}
	}
	return matches, nil
}

func collectionDefinitionJSON(source *BookSource) ([]byte, error) {
	definition := *source
	definition.ID = ""
	definition.CollectionID = ""
	definition.CreatedAt = 0
	definition.UpdatedAt = 0
	return definition.MarshalJSON()
}

func (s *Store) RenameCollection(ctx context.Context, id, name string, now time.Time) error {
	name, err := NormalizeCollectionName(name)
	if err != nil {
		return err
	}
	result, err := s.db.ExecContext(ctx, `UPDATE book_source_collections SET name = ?, updated_at = ? WHERE id = ?`, name, now.UnixMilli(), id)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return ErrCollectionNameUsed
		}
		return err
	}
	count, _ := result.RowsAffected()
	if count == 0 {
		return ErrCollectionNotFound
	}
	return nil
}

func (s *Store) UpdateCollectionSchedule(ctx context.Context, id string, interval SyncInterval, now time.Time) error {
	if err := ValidateSyncInterval(interval); err != nil {
		return err
	}
	result, err := s.db.ExecContext(ctx, `UPDATE book_source_collections SET sync_interval = ?, next_sync_at = ?, updated_at = ? WHERE id = ?`, string(interval), NextSyncAt(interval, now), now.UnixMilli(), id)
	if err != nil {
		return err
	}
	count, _ := result.RowsAffected()
	if count == 0 {
		return ErrCollectionNotFound
	}
	return nil
}

func (s *Store) RecordCollectionSuccess(ctx context.Context, id string, now time.Time) error {
	collection, err := s.GetCollection(id)
	if err != nil {
		return err
	}
	if collection == nil {
		return ErrCollectionNotFound
	}
	at := now.UnixMilli()
	_, err = s.db.ExecContext(ctx, `UPDATE book_source_collections SET last_attempt_at = ?, last_success_at = ?, next_sync_at = ?, last_error = '', updated_at = ? WHERE id = ?`, at, at, NextSyncAt(collection.SyncInterval, now), at, id)
	return err
}

func (s *Store) RecordCollectionFailure(ctx context.Context, id, message string, now time.Time) error {
	if len(message) > 500 {
		message = message[:500]
	}
	retryAt := now.Add(6 * time.Hour).UnixMilli()
	_, err := s.db.ExecContext(ctx, `UPDATE book_source_collections SET last_attempt_at = ?, next_sync_at = ?, last_error = ?, updated_at = ? WHERE id = ?`, now.UnixMilli(), retryAt, message, now.UnixMilli(), id)
	return err
}

func (s *Store) DeleteCollection(ctx context.Context, id string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("booksource: begin collection deletion: %w", err)
	}
	defer tx.Rollback()
	var exists int
	if err := tx.QueryRowContext(ctx, `SELECT 1 FROM book_source_collections WHERE id = ?`, id).Scan(&exists); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrCollectionNotFound
		}
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM book_sources WHERE collection_id = ?`, id); err != nil {
		return fmt.Errorf("booksource: delete collection sources: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM book_source_collections WHERE id = ?`, id); err != nil {
		return fmt.Errorf("booksource: delete collection: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("booksource: commit collection deletion: %w", err)
	}
	return nil
}

func newSourceID() (string, error) {
	return newUUID("source")
}

func newCollectionID() (string, error) {
	return newUUID("collection")
}

func newUUID(kind string) (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", fmt.Errorf("booksource: create %s id: %w", kind, err)
	}
	value[6] = (value[6] & 0x0f) | 0x40
	value[8] = (value[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", value[0:4], value[4:6], value[6:8], value[8:10], value[10:16]), nil
}
