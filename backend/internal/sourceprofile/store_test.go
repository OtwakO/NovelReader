package sourceprofile

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"

	"github.com/otwako/novelreader/internal/booksource"
	"github.com/otwako/novelreader/internal/readerstore"
)

const testReaderID readerstore.UserID = "11111111-1111-4111-8111-111111111111"

func TestStoreKeepsOneBoundedProfilePerInstalledSource(t *testing.T) {
	store, _, closeHome := newTestStore(t)
	defer closeHome()
	ctx := context.Background()
	installSource(t, store.readerDB, "source-a")
	if err := store.SaveSettings(ctx, "source-a", json.RawMessage(`{"provider":"番茄"}`)); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveSettings(ctx, "source-a", json.RawMessage(`{"provider":"七猫"}`)); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveAuthentication(ctx, "source-a", json.RawMessage(`{"cookie":"sid=fixture"}`)); err != nil {
		t.Fatal(err)
	}
	profile, err := store.Load(ctx, "source-a")
	if err != nil {
		t.Fatal(err)
	}
	if string(profile.Settings) != `{"provider":"七猫"}` || string(profile.Authentication) != `{"cookie":"sid=fixture"}` {
		t.Fatalf("profile=%+v", profile)
	}
	if _, err := store.Load(ctx, "missing"); !errors.Is(err, ErrSourceNotInstalled) {
		t.Fatalf("load missing source error=%v", err)
	}
	if err := store.SaveSettings(ctx, "missing", json.RawMessage(`{}`)); !errors.Is(err, ErrSourceNotInstalled) {
		t.Fatalf("save missing source error=%v", err)
	}
	if err := store.SaveAuthentication(ctx, "source-a", json.RawMessage(`[]`)); !errors.Is(err, ErrInvalidDocument) {
		t.Fatalf("invalid document error=%v", err)
	}
}

func TestResetScopesPreserveInstalledSourceDefinition(t *testing.T) {
	store, _, closeHome := newTestStore(t)
	defer closeHome()
	ctx := context.Background()
	installSource(t, store.readerDB, "source-a")
	if err := store.SaveSettings(ctx, "source-a", json.RawMessage(`{"variable":"kept"}`)); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveAuthentication(ctx, "source-a", json.RawMessage(`{"loginInfo":{"user":"kept"}}`)); err != nil {
		t.Fatal(err)
	}
	if err := store.ClearAuthentication(ctx, "source-a"); err != nil {
		t.Fatal(err)
	}
	profile, err := store.Load(ctx, "source-a")
	if err != nil || string(profile.Settings) != `{"variable":"kept"}` || string(profile.Authentication) != `{}` {
		t.Fatalf("profile=%+v err=%v", profile, err)
	}
	if err := store.SaveAuthentication(ctx, "source-a", json.RawMessage(`{"loginInfo":{"user":"kept"}}`)); err != nil {
		t.Fatal(err)
	}
	if err := store.ResetSettings(ctx, "source-a"); err != nil {
		t.Fatal(err)
	}
	profile, err = store.Load(ctx, "source-a")
	if err != nil || string(profile.Settings) != `{}` || string(profile.Authentication) == `{}` {
		t.Fatalf("profile=%+v err=%v", profile, err)
	}
	if err := store.Reset(ctx, "source-a"); err != nil {
		t.Fatal(err)
	}
	profile, err = store.Load(ctx, "source-a")
	if err != nil || string(profile.Settings) != `{}` || string(profile.Authentication) != `{}` {
		t.Fatalf("profile=%+v err=%v", profile, err)
	}
}

func TestReconcileDeletesRemovedSourceState(t *testing.T) {
	store, _, closeHome := newTestStore(t)
	defer closeHome()
	ctx := context.Background()
	installSource(t, store.readerDB, "source-a")
	installSource(t, store.readerDB, "source-b")
	for _, sourceID := range []string{"source-a", "source-b"} {
		if err := store.SaveSettings(ctx, sourceID, json.RawMessage(`{"configured":true}`)); err != nil {
			t.Fatal(err)
		}
		if err := store.SaveAuthentication(ctx, sourceID, json.RawMessage(`{"token":"secret"}`)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := store.readerDB.Exec(`DELETE FROM book_sources WHERE id = 'source-a'`); err != nil {
		t.Fatal(err)
	}
	if err := store.Reconcile(ctx); err != nil {
		t.Fatal(err)
	}
	assertRowCount(t, store.readerDB, `SELECT COUNT(*) FROM source_profiles WHERE source_id = 'source-a'`, 0)
	assertRowCount(t, store.credentialsDB, `SELECT COUNT(*) FROM source_auth_state WHERE source_id = 'source-a'`, 0)
	assertRowCount(t, store.readerDB, `SELECT COUNT(*) FROM source_profiles WHERE source_id = 'source-b'`, 1)
	assertRowCount(t, store.credentialsDB, `SELECT COUNT(*) FROM source_auth_state WHERE source_id = 'source-b'`, 1)
}

func TestPortableSnapshotKeepsSettingsAndExcludesAuthentication(t *testing.T) {
	store, manager, closeHome := newTestStore(t)
	ctx := context.Background()
	installSource(t, store.readerDB, "source-a")
	if err := store.SaveSettings(ctx, "source-a", json.RawMessage(`{"provider":"番茄"}`)); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveAuthentication(ctx, "source-a", json.RawMessage(`{"token":"secret"}`)); err != nil {
		t.Fatal(err)
	}
	closeHome()
	destination := filepath.Join(t.TempDir(), "reader-home")
	if err := manager.SnapshotHome(ctx, testReaderID, destination); err != nil {
		t.Fatal(err)
	}
	readerDB := openTestDB(t, filepath.Join(destination, readerstore.ReaderDatabaseName))
	defer readerDB.Close()
	credentialsDB := openTestDB(t, filepath.Join(destination, readerstore.CredentialsDatabaseName))
	defer credentialsDB.Close()
	assertRowCount(t, readerDB, `SELECT COUNT(*) FROM source_profiles WHERE source_id = 'source-a'`, 1)
	assertRowCount(t, credentialsDB, `SELECT COUNT(*) FROM source_auth_state`, 0)
}

func newTestStore(t *testing.T) (*Store, *readerstore.Manager, func()) {
	t.Helper()
	manager, err := readerstore.NewManager(filepath.Join(t.TempDir(), "data"), 1, booksource.ReaderSchema(), ReaderSchema())
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Create(context.Background(), testReaderID); err != nil {
		manager.Close()
		t.Fatal(err)
	}
	home, err := manager.Open(context.Background(), testReaderID)
	if err != nil {
		manager.Close()
		t.Fatal(err)
	}
	closed := false
	closeHome := func() {
		if !closed {
			closed = true
			if err := home.Close(); err != nil {
				t.Errorf("close home: %v", err)
			}
		}
	}
	t.Cleanup(func() {
		closeHome()
		if err := manager.Close(); err != nil {
			t.Errorf("close manager: %v", err)
		}
	})
	return NewStore(home.DB(), home.CredentialsDB()), manager, closeHome
}

func installSource(t *testing.T, db *sql.DB, sourceID string) {
	t.Helper()
	store := booksource.NewStore(db)
	if err := store.Upsert(&booksource.BookSource{ID: sourceID, BookSourceURL: "https://" + sourceID + ".test", BookSourceName: sourceID, Enabled: true}); err != nil {
		t.Fatal(err)
	}
}

func assertRowCount(t *testing.T, db *sql.DB, query string, want int) {
	t.Helper()
	var got int
	if err := db.QueryRow(query).Scan(&got); err != nil || got != want {
		t.Fatalf("count=%d want=%d error=%v", got, want, err)
	}
}

func openTestDB(t *testing.T, path string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	return db
}
