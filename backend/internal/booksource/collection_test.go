package booksource

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestCollectionReplacementIsAuthoritativeAndTransactional(t *testing.T) {
	store := newCollectionStore(t)
	now := time.Unix(100, 0)
	collection, first, err := store.CreateCollection(context.Background(), CreateCollection{
		Name: "Main Sources", OriginKind: CollectionOriginUpload, OriginFilename: "sources-a.json", SyncInterval: SyncManual,
	}, []*BookSource{collectionSource(t, "https://one", "One"), collectionSource(t, "https://two", "Two")}, now)
	if err != nil {
		t.Fatal(err)
	}
	if first.Added != 2 || collection.SourceCount != 2 {
		t.Fatalf("unexpected creation result: %#v %#v", first, collection)
	}

	one, err := store.GetByID("https://one")
	if err != nil {
		t.Fatal(err)
	}
	one.BookSourceName = "Local edit"
	if err := store.Upsert(one); err != nil {
		t.Fatal(err)
	}

	result, err := store.ReplaceCollection(context.Background(), collection.ID, []*BookSource{
		collectionSource(t, "https://one", "Upstream replacement"),
		collectionSource(t, "https://three", "Three"),
	}, "sources-b.json", "", "", now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if result.Added != 1 || result.Updated != 1 || result.Removed != 1 || result.Total != 2 {
		t.Fatalf("unexpected replacement result: %#v", result)
	}
	one, _ = store.GetByID("https://one")
	if one.BookSourceName != "Upstream replacement" || one.CollectionID != collection.ID {
		t.Fatalf("source was not authoritatively replaced: %#v", one)
	}
	if removed, _ := store.GetByID("https://two"); removed != nil {
		t.Fatalf("missing upstream source was retained: %#v", removed)
	}
}

func TestCollectionRejectsExclusiveOwnershipConflictWithoutPartialChanges(t *testing.T) {
	store := newCollectionStore(t)
	now := time.Unix(100, 0)
	first, _, err := store.CreateCollection(context.Background(), CreateCollection{
		Name: "First", OriginKind: CollectionOriginUpload, SyncInterval: SyncManual,
	}, []*BookSource{collectionSource(t, "https://owned", "Owned")}, now)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = store.CreateCollection(context.Background(), CreateCollection{
		Name: "Second", OriginKind: CollectionOriginUpload, SyncInterval: SyncManual,
	}, []*BookSource{collectionSource(t, "https://new", "New"), collectionSource(t, "https://owned", "Conflict")}, now)
	if !errors.Is(err, ErrCollectionConflict) {
		t.Fatalf("expected ownership conflict, got %v", err)
	}
	if source, _ := store.GetByID("https://new"); source != nil {
		t.Fatalf("conflicted import partially wrote source: %#v", source)
	}
	owned, _ := store.GetByID("https://owned")
	if owned.CollectionID != first.ID || owned.BookSourceName != "Owned" {
		t.Fatalf("existing ownership changed: %#v", owned)
	}
}

func TestCollectionRenameAndDeletePreserveSimpleOwnershipModel(t *testing.T) {
	store := newCollectionStore(t)
	collection, _, err := store.CreateCollection(context.Background(), CreateCollection{
		Name: "Original", OriginKind: CollectionOriginURL, OriginURL: "https://example.test/sources.json", SyncInterval: SyncManual,
	}, []*BookSource{collectionSource(t, "https://one", "One")}, time.Unix(100, 0))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.RenameCollection(context.Background(), collection.ID, " Renamed ", time.Unix(200, 0)); err != nil {
		t.Fatal(err)
	}
	got, _ := store.GetCollection(collection.ID)
	if got.Name != "Renamed" {
		t.Fatalf("rename was not persisted: %#v", got)
	}
	if err := store.DeleteCollection(context.Background(), collection.ID); err != nil {
		t.Fatal(err)
	}
	if source, _ := store.GetByID("https://one"); source != nil {
		t.Fatalf("collection delete did not remove owned source: %#v", source)
	}
}

func newCollectionStore(t *testing.T) *Store {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+t.Name()+"?mode=memory&cache=shared&_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	initializeBookSourceTestSchema(t, db)
	return NewStore(db)
}

func collectionSource(t *testing.T, url, name string) *BookSource {
	t.Helper()
	source, err := NewFromJSON([]byte(`{"bookSourceUrl":"` + url + `","bookSourceName":"` + name + `","enabled":true}`))
	if err != nil {
		t.Fatal(err)
	}
	return source
}
