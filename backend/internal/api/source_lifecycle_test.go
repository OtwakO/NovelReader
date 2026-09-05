package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/otwako/novelreader/internal/booksource"
	"github.com/otwako/novelreader/internal/readerstore"
	"github.com/otwako/novelreader/internal/sourceprofile"
)

const lifecycleReaderID readerstore.UserID = "11111111-1111-4111-8111-111111111111"

func TestSourceLifecyclePreservesSurvivorsAndDeletesRemovedState(t *testing.T) {
	store, profiles, _, _, closeHome := newSourceLifecycleStores(t)
	defer closeHome()
	ctx := context.Background()
	collection, _, err := store.CreateCollection(ctx, booksource.CreateCollection{Name: "fixture", OriginKind: booksource.CollectionOriginUpload, SyncInterval: booksource.SyncManual}, []*booksource.BookSource{
		{BookSourceURL: "https://keep.test", BookSourceName: "Keep", Enabled: true},
		{BookSourceURL: "https://remove.test", BookSourceName: "Remove", Enabled: true},
	}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	existing, err := store.ListByCollection(t.Context(), collection.ID)
	if err != nil {
		t.Fatal(err)
	}
	keepID, removeID := existing[0].ID, existing[1].ID
	for _, sourceID := range []string{keepID, removeID} {
		if err := profiles.SaveSettings(ctx, sourceID, json.RawMessage(`{"configured":true}`)); err != nil {
			t.Fatal(err)
		}
		if err := profiles.SaveAuthentication(ctx, sourceID, json.RawMessage(`{"token":"secret"}`)); err != nil {
			t.Fatal(err)
		}
	}
	invalidated := make(map[string]bool)
	_, err = replaceSourceCollection(ctx, store, profiles, func(id string) { invalidated[id] = true }, collection.ID, []*booksource.BookSource{
		{BookSourceURL: "https://keep.test", BookSourceName: "Keep updated", Enabled: true},
	}, "fixture.json", "", "", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if profile, err := profiles.Load(ctx, keepID); err != nil || string(profile.Settings) != `{"configured":true}` || string(profile.Authentication) != `{"token":"secret"}` {
		t.Fatalf("survivor profile=%+v error=%v", profile, err)
	}
	if _, err := profiles.Load(ctx, removeID); err != sourceprofile.ErrSourceNotInstalled {
		t.Fatalf("removed source load error=%v", err)
	}
	if !invalidated[keepID] || !invalidated[removeID] {
		t.Fatalf("invalidated=%v", invalidated)
	}
}

func TestDeleteSourceDefinitionRemovesOwnedState(t *testing.T) {
	store, profiles, readerDB, credentialsDB, closeHome := newSourceLifecycleStores(t)
	defer closeHome()
	ctx := context.Background()
	source := &booksource.BookSource{ID: "source-delete", BookSourceURL: "https://delete.test", BookSourceName: "Delete", Enabled: true}
	if err := store.Upsert(source); err != nil {
		t.Fatal(err)
	}
	if err := profiles.SaveSettings(ctx, source.ID, json.RawMessage(`{"configured":true}`)); err != nil {
		t.Fatal(err)
	}
	if err := profiles.SaveAuthentication(ctx, source.ID, json.RawMessage(`{"token":"secret"}`)); err != nil {
		t.Fatal(err)
	}
	invalidated := false
	if err := deleteSourceDefinition(ctx, store, profiles, func(id string) { invalidated = id == source.ID }, source.ID); err != nil {
		t.Fatal(err)
	}
	if !invalidated {
		t.Fatal("source session was not invalidated")
	}
	var profilesCount, authCount int
	if err := readerDB.QueryRow(`SELECT COUNT(*) FROM source_profiles`).Scan(&profilesCount); err != nil {
		t.Fatal(err)
	}
	if err := credentialsDB.QueryRow(`SELECT COUNT(*) FROM source_auth_state`).Scan(&authCount); err != nil {
		t.Fatal(err)
	}
	if profilesCount != 0 || authCount != 0 {
		t.Fatalf("profiles=%d auth=%d", profilesCount, authCount)
	}
}

func newSourceLifecycleStores(t *testing.T) (*booksource.Store, *sourceprofile.Store, *sql.DB, *sql.DB, func()) {
	t.Helper()
	manager, err := readerstore.NewManager(filepath.Join(t.TempDir(), "data"), 1, booksource.ReaderSchema(), sourceprofile.ReaderSchema())
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Create(context.Background(), lifecycleReaderID); err != nil {
		manager.Close()
		t.Fatal(err)
	}
	home, err := manager.Open(context.Background(), lifecycleReaderID)
	if err != nil {
		manager.Close()
		t.Fatal(err)
	}
	closeHome := func() {
		_ = home.Close()
		_ = manager.Close()
	}
	return booksource.NewStore(home.DB()), sourceprofile.NewStore(home.DB(), home.CredentialsDB()), home.DB(), home.CredentialsDB(), closeHome
}

func TestCommittedCollectionInvalidatesBeforeCleanupAndCanReconcileLater(t *testing.T) {
	store, profiles, _, credentials, cleanup := newSourceLifecycleStores(t)
	defer cleanup()
	collection, _, err := store.CreateCollection(t.Context(), booksource.CreateCollection{Name: "Fixture", OriginKind: booksource.CollectionOriginUpload, SyncInterval: booksource.SyncManual}, []*booksource.BookSource{{BookSourceURL: "https://example.test", Enabled: true}}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	sources, err := store.ListByCollection(t.Context(), collection.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := profiles.SaveAuthentication(t.Context(), sources[0].ID, json.RawMessage(`{"token":"synthetic"}`)); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	var invalidated []string
	result, err := replaceSourceCollection(ctx, store, profiles, func(id string) { invalidated = append(invalidated, id); cancel() }, collection.ID, nil, "", "", "", time.Now())
	if !errors.Is(err, context.Canceled) || !strings.Contains(err.Error(), "committed") {
		t.Fatalf("partial outcome not explicit: %v", err)
	}
	if result.Removed != 1 || len(invalidated) != 1 || invalidated[0] != sources[0].ID {
		t.Fatalf("result=%+v invalidated=%v", result, invalidated)
	}
	remaining, err := store.ListByCollection(t.Context(), collection.ID)
	if err != nil || len(remaining) != 0 {
		t.Fatalf("committed removal lost: %v %v", remaining, err)
	}
	if err := profiles.Reconcile(t.Context()); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := credentials.QueryRow(`SELECT COUNT(*) FROM source_auth_state`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("credentials=%d error=%v", count, err)
	}
}
