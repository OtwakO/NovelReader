package auth

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/otwako/novelreader/internal/readerstore"
	_ "modernc.org/sqlite"
)

const testUserID readerstore.UserID = "11111111-1111-4111-8111-111111111111"

func TestOpenSystemStoreCreatesVersionedIdentitySchema(t *testing.T) {
	root := prepareTestRoot(t)

	store, err := OpenSystemStore(root)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	if store.Path() != filepath.Join(root, SystemDatabaseName) {
		t.Fatalf("path = %q", store.Path())
	}
	info, err := os.Stat(store.Path())
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("permissions = %o", info.Mode().Perm())
	}
	var version int
	if err := store.db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != CurrentSystemSchemaVersion {
		t.Fatalf("version = %d", version)
	}
	for _, table := range []string{"users", "auth_sessions", "password_reset_tokens", "setup_state", "account_deletions"} {
		var count int
		if err := store.db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Errorf("table %q count = %d", table, count)
		}
	}
	for _, forbidden := range []string{"book_sources", "books", "chapters", "bookmarks", "reading_progress", "fonts"} {
		var count int
		if err := store.db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, forbidden).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Errorf("Reader Data table %q exists in system.db", forbidden)
		}
	}
	var setupStatus string
	if err := store.db.QueryRow(`SELECT status FROM setup_state WHERE id = 1`).Scan(&setupStatus); err != nil {
		t.Fatal(err)
	}
	if setupStatus != "open" {
		t.Fatalf("setup status = %q", setupStatus)
	}
}

func TestSystemSchemaConstrainsRolesAndStatuses(t *testing.T) {
	store := openTestStore(t)
	defer store.Close()

	for _, role := range []string{"owner", "moderator", ""} {
		_, err := store.db.Exec(`
			INSERT INTO users (id, username, username_normalized, role, password_hash, status, created_at, updated_at)
			VALUES (?, 'Alice', ?, ?, 'hash', 'active', 1, 1)
		`, string(testUserID), "alice-"+role, role)
		if err == nil {
			t.Errorf("role %q was accepted", role)
			_, _ = store.db.Exec(`DELETE FROM users`)
		}
	}
	for _, status := range []string{"pending", "deleted", ""} {
		_, err := store.db.Exec(`
			INSERT INTO users (id, username, username_normalized, role, password_hash, status, created_at, updated_at)
			VALUES (?, 'Alice', ?, 'reader', 'hash', ?, 1, 1)
		`, string(testUserID), "alice-"+status, status)
		if err == nil {
			t.Errorf("status %q was accepted", status)
			_, _ = store.db.Exec(`DELETE FROM users`)
		}
	}
}

func TestSystemSchemaRequiresSHA256SizedSessionHashBlob(t *testing.T) {
	store := openTestStore(t)
	defer store.Close()
	insertTestUser(t, store.db, StatusActive)

	for _, tokenHash := range []any{"raw-token-text", []byte("short"), make([]byte, 33)} {
		_, err := store.db.Exec(`
			INSERT INTO auth_sessions (id, user_id, token_hash, created_at, last_seen_at)
			VALUES (random(), ?, ?, 1, 1)
		`, string(testUserID), tokenHash)
		if err == nil {
			t.Fatalf("token hash %#v was accepted", tokenHash)
		}
	}
}

func TestSystemForeignKeysCascadeButDeletionJobSurvivesAccountRemoval(t *testing.T) {
	store := openTestStore(t)
	defer store.Close()
	insertTestUser(t, store.db, StatusActive)

	if _, err := store.db.Exec(`
		INSERT INTO auth_sessions (id, user_id, token_hash, created_at, last_seen_at)
		VALUES ('session-1', ?, zeroblob(32), 1, 1)
	`, string(testUserID)); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec(`
		INSERT INTO account_deletions (id, user_id, status, created_at, updated_at)
		VALUES ('deletion-1', ?, 'pending', 1, 1)
	`, string(testUserID)); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec(`DELETE FROM users WHERE id = ?`, string(testUserID)); err != nil {
		t.Fatal(err)
	}

	var sessions, deletions int
	if err := store.db.QueryRow(`SELECT count(*) FROM auth_sessions`).Scan(&sessions); err != nil {
		t.Fatal(err)
	}
	if err := store.db.QueryRow(`SELECT count(*) FROM account_deletions`).Scan(&deletions); err != nil {
		t.Fatal(err)
	}
	if sessions != 0 || deletions != 1 {
		t.Fatalf("sessions = %d, deletions = %d", sessions, deletions)
	}
}

func TestTransitionAccountStatusAllowsOnlyLegalChanges(t *testing.T) {
	store := openTestStore(t)
	defer store.Close()
	insertTestUser(t, store.db, StatusActive)

	for _, transition := range []struct {
		from AccountStatus
		to   AccountStatus
	}{
		{StatusActive, StatusDisabled},
		{StatusDisabled, StatusActive},
		{StatusActive, StatusDeleting},
		{StatusDisabled, StatusDeleting},
	} {
		setTestUserStatus(t, store.db, transition.from)
		if err := store.TransitionAccountStatus(testUserID, transition.to, 10); err != nil {
			t.Errorf("%s -> %s: %v", transition.from, transition.to, err)
		}
	}

	for _, transition := range []struct {
		from AccountStatus
		to   AccountStatus
	}{
		{StatusActive, StatusActive},
		{StatusDisabled, StatusDisabled},
		{StatusDeleting, StatusActive},
		{StatusDeleting, StatusDisabled},
		{StatusDeleting, StatusDeleting},
		{StatusActive, AccountStatus("unknown")},
	} {
		setTestUserStatus(t, store.db, transition.from)
		err := store.TransitionAccountStatus(testUserID, transition.to, 10)
		if !errors.Is(err, ErrInvalidStatusTransition) {
			t.Errorf("%s -> %s error = %v", transition.from, transition.to, err)
		}
	}
}

func TestTransitionAccountStatusRevokesSessionsAtomically(t *testing.T) {
	store := openTestStore(t)
	defer store.Close()
	insertTestUser(t, store.db, StatusActive)
	if _, err := store.db.Exec(`
		INSERT INTO auth_sessions (id, user_id, token_hash, created_at, last_seen_at)
		VALUES ('session-1', ?, zeroblob(32), 1, 1)
	`, string(testUserID)); err != nil {
		t.Fatal(err)
	}
	if err := store.TransitionAccountStatus(testUserID, StatusDisabled, 10); err != nil {
		t.Fatal(err)
	}
	var sessions int
	if err := store.db.QueryRow(`SELECT count(*) FROM auth_sessions`).Scan(&sessions); err != nil {
		t.Fatal(err)
	}
	if sessions != 0 {
		t.Fatalf("sessions = %d", sessions)
	}

	setTestUserStatus(t, store.db, StatusActive)
	if _, err := store.db.Exec(`
		INSERT INTO auth_sessions (id, user_id, token_hash, created_at, last_seen_at)
		VALUES ('session-2', ?, randomblob(32), 1, 1)
	`, string(testUserID)); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec(`
		CREATE TRIGGER reject_status_session_revocation
		BEFORE DELETE ON auth_sessions
		BEGIN SELECT RAISE(ABORT, 'reject revocation'); END
	`); err != nil {
		t.Fatal(err)
	}
	if err := store.TransitionAccountStatus(testUserID, StatusDeleting, 20); err == nil {
		t.Fatal("status transition succeeded despite failed revocation")
	}
	var status AccountStatus
	if err := store.db.QueryRow(`SELECT status FROM users WHERE id = ?`, string(testUserID)).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != StatusActive {
		t.Fatalf("status after rollback = %q", status)
	}
}

func TestTransitionAccountStatusRejectsMissingAndInvalidUserIDs(t *testing.T) {
	store := openTestStore(t)
	defer store.Close()

	if err := store.TransitionAccountStatus(testUserID, StatusDisabled, 10); !errors.Is(err, ErrAccountNotFound) {
		t.Fatalf("missing account error = %v", err)
	}
	if err := store.TransitionAccountStatus(readerstore.UserID("../escape"), StatusDisabled, 10); !errors.Is(err, readerstore.ErrInvalidUserID) {
		t.Fatalf("invalid ID error = %v", err)
	}
}

func TestOpenSystemStoreRejectsNewerSchemaWithoutModifyingIt(t *testing.T) {
	root := prepareTestRoot(t)
	path := filepath.Join(root, SystemDatabaseName)
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`PRAGMA user_version = 5`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	store, err := OpenSystemStore(root)
	if store != nil {
		store.Close()
	}
	if !errors.Is(err, ErrNewerSystemSchema) {
		t.Fatalf("error = %v", err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatal("newer system database was modified")
	}
}

func TestOpenSystemStoreRejectsVersionOneWithoutModifyingIt(t *testing.T) {
	root := prepareTestRoot(t)
	path := filepath.Join(root, SystemDatabaseName)
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
		CREATE TABLE marker (value TEXT);
		INSERT INTO marker VALUES ('version-one');
		PRAGMA user_version = 1;
	`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	store, err := OpenSystemStore(root)
	if store != nil {
		store.Close()
	}
	if !errors.Is(err, ErrInvalidSystemSchema) {
		t.Fatalf("error = %v", err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatal("version-one system database was modified")
	}
}

func TestOpenSystemStoreRejectsMalformedDatabase(t *testing.T) {
	root := prepareTestRoot(t)
	path := filepath.Join(root, SystemDatabaseName)
	if err := os.WriteFile(path, []byte("not sqlite"), 0o600); err != nil {
		t.Fatal(err)
	}

	store, err := OpenSystemStore(root)
	if store != nil {
		store.Close()
	}
	if !errors.Is(err, ErrInvalidSystemSchema) {
		t.Fatalf("error = %v", err)
	}
	contents, readErr := os.ReadFile(path)
	if readErr != nil || string(contents) != "not sqlite" {
		t.Fatalf("malformed database changed: contents=%q err=%v", contents, readErr)
	}
}

func TestOpenSystemStoreRejectsVersionedDatabaseWithExtraReaderDataWithoutModifyingIt(t *testing.T) {
	root := prepareTestRoot(t)
	path := filepath.Join(root, SystemDatabaseName)
	if err := initializeSystemDatabase(path); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE books (id TEXT PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	store, err := OpenSystemStore(root)
	if store != nil {
		store.Close()
	}
	if !errors.Is(err, ErrInvalidSystemSchema) {
		t.Fatalf("error = %v", err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatal("polluted system database was modified")
	}
}

func TestOpenSystemStoreRejectsHiddenCatalogObjectWithoutWALFiles(t *testing.T) {
	root := prepareTestRoot(t)
	path := filepath.Join(root, SystemDatabaseName)
	if err := initializeSystemDatabase(path); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`PRAGMA writable_schema = ON`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
		INSERT INTO sqlite_master (type, name, tbl_name, rootpage, sql)
		VALUES ('view', 'sqlite_hidden_reader_data', 'sqlite_hidden_reader_data', 0, 'CREATE VIEW sqlite_hidden_reader_data AS SELECT 1')
	`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	store, err := OpenSystemStore(root)
	if store != nil {
		store.Close()
	}
	if !errors.Is(err, ErrInvalidSystemSchema) {
		t.Fatalf("error = %v", err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatal("hidden-catalog system database was modified")
	}
	for _, suffix := range []string{"-wal", "-shm"} {
		if _, err := os.Stat(path + suffix); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("%s unexpectedly exists: %v", suffix, err)
		}
	}
}

func TestOpenSystemStoreRejectsVersionedDatabaseWithWrongColumns(t *testing.T) {
	root := prepareTestRoot(t)
	path := filepath.Join(root, SystemDatabaseName)
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`CREATE TABLE users (id TEXT PRIMARY KEY)`,
		`CREATE TABLE auth_sessions (id TEXT PRIMARY KEY)`,
		`CREATE TABLE password_reset_tokens (token_hash BLOB PRIMARY KEY)`,
		`CREATE TABLE setup_state (id INTEGER PRIMARY KEY)`,
		`CREATE TABLE account_deletions (id TEXT PRIMARY KEY)`,
		`PRAGMA user_version = 2`,
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	store, err := OpenSystemStore(root)
	if store != nil {
		store.Close()
	}
	if !errors.Is(err, ErrInvalidSystemSchema) {
		t.Fatalf("error = %v", err)
	}
}

func TestOpenSystemStoreRejectsMissingRecoverySingleton(t *testing.T) {
	root := prepareTestRoot(t)
	store, err := OpenSystemStore(root)
	if err != nil {
		t.Fatal(err)
	}
	path := store.Path()
	if _, err := store.db.Exec(`DELETE FROM admin_recovery_state`); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = OpenSystemStore(root)
	if store != nil {
		store.Close()
	}
	if !errors.Is(err, ErrInvalidSystemSchema) {
		t.Fatalf("path=%s error=%v", path, err)
	}
}

func TestOpenSystemStorePublishesCompleteInterruptedStage(t *testing.T) {
	root := prepareTestRoot(t)
	stage := filepath.Join(root, systemDatabaseStagingName)
	if err := initializeSystemDatabase(stage); err != nil {
		t.Fatal(err)
	}

	store, err := OpenSystemStore(root)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err := os.Stat(filepath.Join(root, SystemDatabaseName)); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(stage); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("stage still exists: %v", err)
	}
}

func TestOpenSystemStoreRebuildsPartialInterruptedStage(t *testing.T) {
	root := prepareTestRoot(t)
	stage := filepath.Join(root, systemDatabaseStagingName)
	if err := os.WriteFile(stage, []byte("partial"), 0o600); err != nil {
		t.Fatal(err)
	}

	store, err := OpenSystemStore(root)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err := os.Stat(filepath.Join(root, SystemDatabaseName)); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(stage); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("stage still exists: %v", err)
	}
}

func prepareTestRoot(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "data")
	if _, err := readerstore.PrepareRoot(root); err != nil {
		t.Fatal(err)
	}
	return root
}

func openTestStore(t *testing.T) *Store {
	t.Helper()
	store, err := OpenSystemStore(prepareTestRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec(`
		UPDATE setup_state
		SET status = 'closed', proposed_user_id = ?, claimed_at = 1, claim_expires_at = 2, closed_at = 2
		WHERE id = 1 AND status = 'open'
	`, string(testUserID)); err != nil {
		store.Close()
		t.Fatal(err)
	}
	return store
}

func insertTestUser(t *testing.T, db *sql.DB, status AccountStatus) {
	t.Helper()
	_, err := db.Exec(`
		INSERT INTO users (id, username, username_normalized, role, password_hash, status, created_at, updated_at)
		VALUES (?, 'Alice', 'alice', 'reader', 'hash', ?, 1, 1)
	`, string(testUserID), string(status))
	if err != nil {
		t.Fatal(err)
	}
}

func setTestUserStatus(t *testing.T, db *sql.DB, status AccountStatus) {
	t.Helper()
	if _, err := db.Exec(`UPDATE users SET status = ? WHERE id = ?`, string(status), string(testUserID)); err != nil {
		t.Fatal(err)
	}
}
