package sourceprofile

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/otwako/novelreader/internal/readerstore"
)

const (
	maxSettingsBytes       = 256 * 1024
	maxAuthenticationBytes = 1024 * 1024
)

var (
	ErrInvalidDocument    = errors.New("sourceprofile: document must be a JSON object within the size limit")
	ErrSourceNotInstalled = errors.New("sourceprofile: source is not installed")
)

type Profile struct {
	SourceID       string          `json:"sourceId"`
	Settings       json.RawMessage `json:"settings"`
	Authentication json.RawMessage `json:"-"`
}

type Store struct {
	readerDB      *sql.DB
	credentialsDB *sql.DB
}

func ReaderSchema() readerstore.ReaderSchema {
	return readerstore.ReaderSchema{
		Initialize: func(tx *sql.Tx) error {
			_, err := tx.Exec(`CREATE TABLE source_profiles (
				source_id TEXT PRIMARY KEY,
				settings_json TEXT NOT NULL,
				updated_at INTEGER NOT NULL
			)`)
			return err
		},
		InitializeCredentials: func(tx *sql.Tx) error {
			_, err := tx.Exec(`CREATE TABLE source_auth_state (
				source_id TEXT PRIMARY KEY,
				auth_json TEXT NOT NULL,
				updated_at INTEGER NOT NULL
			)`)
			return err
		},
	}
}

func NewStore(readerDB, credentialsDB *sql.DB) *Store {
	return &Store{readerDB: readerDB, credentialsDB: credentialsDB}
}

func (s *Store) Load(ctx context.Context, sourceID string) (Profile, error) {
	if err := s.requireInstalled(ctx, sourceID); err != nil {
		return Profile{}, err
	}
	profile := Profile{SourceID: sourceID, Settings: emptyDocument(), Authentication: emptyDocument()}
	if err := loadDocument(ctx, s.readerDB, `SELECT settings_json FROM source_profiles WHERE source_id = ?`, sourceID, &profile.Settings); err != nil {
		return Profile{}, fmt.Errorf("sourceprofile: load settings: %w", err)
	}
	if err := loadDocument(ctx, s.credentialsDB, `SELECT auth_json FROM source_auth_state WHERE source_id = ?`, sourceID, &profile.Authentication); err != nil {
		return Profile{}, fmt.Errorf("sourceprofile: load authentication: %w", err)
	}
	return profile, nil
}

func (s *Store) SaveSettings(ctx context.Context, sourceID string, settings json.RawMessage) error {
	if err := s.requireInstalled(ctx, sourceID); err != nil {
		return err
	}
	return saveDocument(ctx, s.readerDB, `INSERT INTO source_profiles (source_id, settings_json, updated_at) VALUES (?, ?, ?)
		ON CONFLICT(source_id) DO UPDATE SET settings_json = excluded.settings_json, updated_at = excluded.updated_at`, sourceID, settings, maxSettingsBytes)
}

func (s *Store) SaveAuthentication(ctx context.Context, sourceID string, authentication json.RawMessage) error {
	if err := s.requireInstalled(ctx, sourceID); err != nil {
		return err
	}
	return saveDocument(ctx, s.credentialsDB, `INSERT INTO source_auth_state (source_id, auth_json, updated_at) VALUES (?, ?, ?)
		ON CONFLICT(source_id) DO UPDATE SET auth_json = excluded.auth_json, updated_at = excluded.updated_at`, sourceID, authentication, maxAuthenticationBytes)
}

func (s *Store) ClearAuthentication(ctx context.Context, sourceID string) error {
	if err := s.requireInstalled(ctx, sourceID); err != nil {
		return err
	}
	_, err := s.credentialsDB.ExecContext(ctx, `DELETE FROM source_auth_state WHERE source_id = ?`, sourceID)
	return err
}

func (s *Store) ResetSettings(ctx context.Context, sourceID string) error {
	if err := s.requireInstalled(ctx, sourceID); err != nil {
		return err
	}
	_, err := s.readerDB.ExecContext(ctx, `DELETE FROM source_profiles WHERE source_id = ?`, sourceID)
	return err
}

func (s *Store) Reset(ctx context.Context, sourceID string) error {
	if err := s.ResetSettings(ctx, sourceID); err != nil {
		return err
	}
	return s.ClearAuthentication(ctx, sourceID)
}

func (s *Store) Delete(ctx context.Context, sourceIDs ...string) error {
	for _, sourceID := range sourceIDs {
		if _, err := s.readerDB.ExecContext(ctx, `DELETE FROM source_profiles WHERE source_id = ?`, sourceID); err != nil {
			return fmt.Errorf("sourceprofile: delete settings: %w", err)
		}
		if _, err := s.credentialsDB.ExecContext(ctx, `DELETE FROM source_auth_state WHERE source_id = ?`, sourceID); err != nil {
			return fmt.Errorf("sourceprofile: delete authentication: %w", err)
		}
	}
	return nil
}

// Reconcile removes state whose immutable Source ID is no longer installed.
// It is idempotent and is the crash-recovery path for cross-database deletion.
func (s *Store) Reconcile(ctx context.Context) error {
	if _, err := s.readerDB.ExecContext(ctx, `DELETE FROM source_profiles WHERE source_id NOT IN (SELECT id FROM book_sources)`); err != nil {
		return fmt.Errorf("sourceprofile: reconcile settings: %w", err)
	}
	rows, err := s.readerDB.QueryContext(ctx, `SELECT id FROM book_sources`)
	if err != nil {
		return fmt.Errorf("sourceprofile: list installed sources: %w", err)
	}
	installed := make(map[string]struct{})
	for rows.Next() {
		var sourceID string
		if err := rows.Scan(&sourceID); err != nil {
			_ = rows.Close()
			return fmt.Errorf("sourceprofile: scan installed source: %w", err)
		}
		installed[sourceID] = struct{}{}
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if err := rows.Err(); err != nil {
		return err
	}
	authRows, err := s.credentialsDB.QueryContext(ctx, `SELECT source_id FROM source_auth_state`)
	if err != nil {
		return fmt.Errorf("sourceprofile: list authentication state: %w", err)
	}
	stale := make([]string, 0)
	for authRows.Next() {
		var sourceID string
		if err := authRows.Scan(&sourceID); err != nil {
			_ = authRows.Close()
			return fmt.Errorf("sourceprofile: scan authentication state: %w", err)
		}
		if _, ok := installed[sourceID]; !ok {
			stale = append(stale, sourceID)
		}
	}
	if err := authRows.Close(); err != nil {
		return err
	}
	if err := authRows.Err(); err != nil {
		return err
	}
	if len(stale) == 0 {
		return nil
	}
	tx, err := s.credentialsDB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, sourceID := range stale {
		if _, err := tx.ExecContext(ctx, `DELETE FROM source_auth_state WHERE source_id = ?`, sourceID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) requireInstalled(ctx context.Context, sourceID string) error {
	var exists int
	if err := s.readerDB.QueryRowContext(ctx, `SELECT 1 FROM book_sources WHERE id = ?`, sourceID).Scan(&exists); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrSourceNotInstalled
		}
		return err
	}
	return nil
}

func loadDocument(ctx context.Context, db *sql.DB, query, sourceID string, destination *json.RawMessage) error {
	var value string
	if err := db.QueryRowContext(ctx, query, sourceID).Scan(&value); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return err
	}
	*destination = json.RawMessage(value)
	return nil
}

func saveDocument(ctx context.Context, db *sql.DB, query, sourceID string, document json.RawMessage, limit int) error {
	normalized, err := validateDocument(document, limit)
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, query, sourceID, string(normalized), time.Now().UnixMilli())
	return err
}

func validateDocument(document json.RawMessage, limit int) (json.RawMessage, error) {
	if len(document) == 0 {
		document = emptyDocument()
	}
	if len(document) > limit || !json.Valid(document) {
		return nil, ErrInvalidDocument
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(document, &object); err != nil || object == nil {
		return nil, ErrInvalidDocument
	}
	return document, nil
}

func emptyDocument() json.RawMessage { return json.RawMessage(`{}`) }
