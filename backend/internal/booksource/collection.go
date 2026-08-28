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
		last_success_at, next_sync_at, created_at, updated_at
	) VALUES (?, ?, ?, NULLIF(?, ''), NULLIF(?, ''), ?, ?, ?, ?, ?)`,
		id, name, string(input.OriginKind), input.OriginURL, input.OriginFilename, string(input.SyncInterval),
		at, NextSyncAt(input.SyncInterval, now), at, at)
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
	incoming := make(map[string]*BookSource, len(sources))
	for _, source := range sources {
		if source == nil || strings.TrimSpace(source.BookSourceURL) == "" {
			return ReplaceResult{}, fmt.Errorf("booksource: collection contains a source without bookSourceUrl")
		}
		if _, exists := incoming[source.BookSourceURL]; exists {
			return ReplaceResult{}, fmt.Errorf("booksource: duplicate bookSourceUrl %q", source.BookSourceURL)
		}
		incoming[source.BookSourceURL] = source
		var owner sql.NullString
		err := tx.QueryRowContext(ctx, `SELECT collection_id FROM book_sources WHERE id = ?`, source.BookSourceURL).Scan(&owner)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return ReplaceResult{}, err
		}
		if owner.Valid && owner.String != collectionID {
			return ReplaceResult{}, fmt.Errorf("%w: %s", ErrCollectionConflict, source.BookSourceURL)
		}
		if !owner.Valid && err == nil {
			return ReplaceResult{}, fmt.Errorf("%w: %s", ErrCollectionConflict, source.BookSourceURL)
		}
	}

	existingRows, err := tx.QueryContext(ctx, `SELECT id, source_json FROM book_sources WHERE collection_id = ?`, collectionID)
	if err != nil {
		return ReplaceResult{}, err
	}
	existing := map[string]string{}
	for existingRows.Next() {
		var id, raw string
		if err := existingRows.Scan(&id, &raw); err != nil {
			existingRows.Close()
			return ReplaceResult{}, err
		}
		existing[id] = raw
	}
	if err := existingRows.Close(); err != nil {
		return ReplaceResult{}, err
	}

	result := ReplaceResult{Total: len(sources)}
	for id, raw := range existing {
		if _, ok := incoming[id]; !ok {
			if _, err := tx.ExecContext(ctx, `DELETE FROM book_sources WHERE id = ? AND collection_id = ?`, id, collectionID); err != nil {
				return ReplaceResult{}, err
			}
			result.Removed++
		} else if incoming[id].sourceJSON == raw {
			result.Unchanged++
		} else {
			result.Updated++
		}
	}
	for id := range incoming {
		if _, ok := existing[id]; !ok {
			result.Added++
		}
	}
	for _, source := range sources {
		if err := upsertSource(ctx, tx, source, collectionID, now); err != nil {
			return ReplaceResult{}, err
		}
	}
	return result, nil
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

func (s *Store) RecordCollectionFailure(ctx context.Context, id, message string, now time.Time) error {
	if len(message) > 500 {
		message = message[:500]
	}
	_, err := s.db.ExecContext(ctx, `UPDATE book_source_collections SET last_attempt_at = ?, last_error = ?, updated_at = ? WHERE id = ?`, now.UnixMilli(), message, now.UnixMilli(), id)
	return err
}

func (s *Store) DeleteCollection(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM book_source_collections WHERE id = ?`, id)
	if err != nil {
		return err
	}
	count, _ := result.RowsAffected()
	if count == 0 {
		return ErrCollectionNotFound
	}
	return nil
}

func newCollectionID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", fmt.Errorf("booksource: create collection id: %w", err)
	}
	value[6] = (value[6] & 0x0f) | 0x40
	value[8] = (value[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", value[0:4], value[4:6], value[6:8], value[8:10], value[10:16]), nil
}
