package booksource

import (
	"context"
	"database/sql"
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

	one := collectionSourceByURL(t, store, collection.ID, "https://one")
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
	one = collectionSourceByURL(t, store, collection.ID, "https://one")
	if one.BookSourceName != "Upstream replacement" || one.CollectionID != collection.ID {
		t.Fatalf("source was not authoritatively replaced: %#v", one)
	}
	if removed := collectionSourceByURLOrNil(t, store, collection.ID, "https://two"); removed != nil {
		t.Fatalf("missing upstream source was retained: %#v", removed)
	}
}

func TestCollectionsAllowIndependentDefinitionsWithTheSameURL(t *testing.T) {
	store := newCollectionStore(t)
	now := time.Unix(100, 0)
	first, _, err := store.CreateCollection(context.Background(), CreateCollection{
		Name: "First", OriginKind: CollectionOriginUpload, SyncInterval: SyncManual,
	}, []*BookSource{collectionSource(t, "https://shared", "First definition")}, now)
	if err != nil {
		t.Fatal(err)
	}
	second, result, err := store.CreateCollection(context.Background(), CreateCollection{
		Name: "Second", OriginKind: CollectionOriginUpload, SyncInterval: SyncManual,
	}, []*BookSource{collectionSource(t, "https://shared", "Second definition")}, now)
	if err != nil {
		t.Fatal(err)
	}
	if result.Added != 1 {
		t.Fatalf("unexpected second import: %#v", result)
	}
	firstSource := collectionSourceByURL(t, store, first.ID, "https://shared")
	secondSource := collectionSourceByURL(t, store, second.ID, "https://shared")
	if firstSource.ID == secondSource.ID || firstSource.BookSourceName == secondSource.BookSourceName {
		t.Fatalf("duplicate URL definitions were not independent: first=%#v second=%#v", firstSource, secondSource)
	}
}

func TestCollectionReplacementPreservesDuplicateSourceIDsByExactDefinitionThenOrder(t *testing.T) {
	store := newCollectionStore(t)
	now := time.Unix(100, 0)
	collection, _, err := store.CreateCollection(context.Background(), CreateCollection{
		Name: "Duplicates", OriginKind: CollectionOriginUpload, SyncInterval: SyncManual,
	}, []*BookSource{
		collectionSource(t, "https://shared", "Alpha"),
		collectionSource(t, "https://shared", "Beta"),
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	before, err := store.ListByCollection(collection.ID)
	if err != nil {
		t.Fatal(err)
	}
	beforeByName := make(map[string]string, len(before))
	for _, source := range before {
		beforeByName[source.BookSourceName] = source.ID
	}
	alphaID, betaID := beforeByName["Alpha"], beforeByName["Beta"]

	result, err := store.ReplaceCollection(context.Background(), collection.ID, []*BookSource{
		collectionSource(t, "https://shared", "Beta"),
		collectionSource(t, "https://shared", "Gamma"),
	}, "duplicates.json", "", "", now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if result.Added != 0 || result.Removed != 0 || result.Total != 2 {
		t.Fatalf("unexpected replacement membership result: %#v", result)
	}
	after, err := store.ListByCollection(collection.ID)
	if err != nil {
		t.Fatal(err)
	}
	byName := make(map[string]string, len(after))
	for _, source := range after {
		byName[source.BookSourceName] = source.ID
	}
	if byName["Beta"] != betaID || byName["Gamma"] != alphaID {
		t.Fatalf("source IDs did not follow exact-then-order matching: before=%#v after=%#v", before, after)
	}
}

func TestCollectionReplacementUsesStoredDocumentOrderWhenEveryDuplicateChanges(t *testing.T) {
	store := newCollectionStore(t)
	now := time.Unix(100, 0)
	collection, _, err := store.CreateCollection(context.Background(), CreateCollection{
		Name: "Changed duplicates", OriginKind: CollectionOriginUpload, SyncInterval: SyncManual,
	}, []*BookSource{
		collectionSource(t, "https://shared", "Alpha"),
		collectionSource(t, "https://shared", "Beta"),
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	before, err := store.ListByCollection(collection.ID)
	if err != nil {
		t.Fatal(err)
	}
	alphaID, betaID := before[0].ID, before[1].ID

	if _, err := store.ReplaceCollection(context.Background(), collection.ID, []*BookSource{
		collectionSource(t, "https://shared", "Gamma"),
		collectionSource(t, "https://shared", "Delta"),
	}, "changed.json", "", "", now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	after, err := store.ListByCollection(collection.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after[0].BookSourceName != "Gamma" || after[0].ID != alphaID || after[1].BookSourceName != "Delta" || after[1].ID != betaID {
		t.Fatalf("changed duplicates did not follow stored document order: before=%#v after=%#v", before, after)
	}
}

func TestCollectionReplacementKeepsExactIdentityAcrossDocumentReorder(t *testing.T) {
	store := newCollectionStore(t)
	now := time.Unix(100, 0)
	collection, _, err := store.CreateCollection(context.Background(), CreateCollection{
		Name: "Reordered duplicates", OriginKind: CollectionOriginUpload, SyncInterval: SyncManual,
	}, []*BookSource{
		collectionSource(t, "https://shared", "Alpha"),
		collectionSource(t, "https://shared", "Beta"),
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	before, err := store.ListByCollection(collection.ID)
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]string{before[0].BookSourceName: before[0].ID, before[1].BookSourceName: before[1].ID}

	if _, err := store.ReplaceCollection(context.Background(), collection.ID, []*BookSource{
		collectionSource(t, "https://shared", "Beta"),
		collectionSource(t, "https://shared", "Alpha"),
	}, "reordered.json", "", "", now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	after, err := store.ListByCollection(collection.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after[0].BookSourceName != "Beta" || after[0].ID != byName["Beta"] || after[1].BookSourceName != "Alpha" || after[1].ID != byName["Alpha"] {
		t.Fatalf("exact identities did not survive document reorder: before=%#v after=%#v", before, after)
	}
}

func TestCollectionSourceEditPreservesDocumentPosition(t *testing.T) {
	store := newCollectionStore(t)
	now := time.Unix(100, 0)
	collection, _, err := store.CreateCollection(context.Background(), CreateCollection{
		Name: "Edited collection", OriginKind: CollectionOriginUpload, SyncInterval: SyncManual,
	}, []*BookSource{
		collectionSource(t, "https://one", "One"),
		collectionSource(t, "https://two", "Two"),
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	before, err := store.ListByCollection(collection.ID)
	if err != nil {
		t.Fatal(err)
	}
	before[0].BookSourceName = "Locally edited"
	before[0].collectionPosition = nil // Simulate a management payload that cannot carry internal ownership order.
	if err := store.Upsert(before[0]); err != nil {
		t.Fatal(err)
	}
	after, err := store.ListByCollection(collection.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after[0].ID != before[0].ID || after[0].BookSourceName != "Locally edited" || after[1].BookSourceName != "Two" {
		t.Fatalf("individual edit changed collection document order: before=%#v after=%#v", before, after)
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
	if source := collectionSourceByURLOrNil(t, store, collection.ID, "https://one"); source != nil {
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

func collectionSourceByURL(t *testing.T, store *Store, collectionID, sourceURL string) *BookSource {
	t.Helper()
	source := collectionSourceByURLOrNil(t, store, collectionID, sourceURL)
	if source == nil {
		t.Fatalf("source %q not found in collection %q", sourceURL, collectionID)
	}
	return source
}

func collectionSourceByURLOrNil(t *testing.T, store *Store, collectionID, sourceURL string) *BookSource {
	t.Helper()
	sources, err := store.ListByCollection(collectionID)
	if err != nil {
		t.Fatal(err)
	}
	for _, source := range sources {
		if source.BookSourceURL == sourceURL {
			return source
		}
	}
	return nil
}

func collectionSource(t *testing.T, url, name string) *BookSource {
	t.Helper()
	source, err := NewFromJSON([]byte(`{"bookSourceUrl":"` + url + `","bookSourceName":"` + name + `","enabled":true}`))
	if err != nil {
		t.Fatal(err)
	}
	return source
}

func TestCapabilityTagsIncludeSourceFeatures(t *testing.T) {
	source := collectionSource(t, "https://source", "Capabilities")
	source.SearchURL = `https://source/search,{"webView":true}`
	source.ExploreURL = "https://source/explore"
	source.EnabledExplore = true
	source.Header = `{"Referer":"https://source"}`
	source.RuleContent = `{"content":"@js:java.getString('content')"}`

	got := CapabilityTags(*source)
	want := []string{"search", "explore", "headers", "javascript", "webview"}
	if len(got) != len(want) {
		t.Fatalf("capabilities=%v want=%v", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("capabilities=%v want=%v", got, want)
		}
	}
}
