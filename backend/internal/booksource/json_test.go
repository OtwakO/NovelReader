// Round-trip and persistence tests for lossless BookSource imports.
package booksource

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/otwako/novelreader/internal/database"
)

func TestBookSourceJSONRoundTripPreservesUnknownFieldsAndRuleShape(t *testing.T) {
	input := []byte(`{
		"bookSourceUrl":"https://source.test",
		"bookSourceName":"Fixture",
		"ruleSearch":{"bookList":".result","name":"a@text"},
		"ruleContent":"{\"content\":\".chapter\"}",
		"respondTime":"123",
		"futureField":{"enabled":true},
		"customButton":["search"]
	}`)

	source, err := NewFromJSON(input)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(source)
	if err != nil {
		t.Fatal(err)
	}

	var got map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &got); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"ruleSearch", "respondTime", "futureField", "customButton"} {
		if _, ok := got[field]; !ok {
			t.Fatalf("round-trip dropped %s: %s", field, encoded)
		}
	}
	if string(got["ruleSearch"]) != string(json.RawMessage(`{"bookList":".result","name":"a@text"}`)) {
		t.Fatalf("ruleSearch=%s", got["ruleSearch"])
	}
	if string(got["respondTime"]) != `"123"` {
		t.Fatalf("respondTime=%s", got["respondTime"])
	}
	if string(got["ruleContent"]) != `"{\"content\":\".chapter\"}"` {
		t.Fatalf("ruleContent=%s", got["ruleContent"])
	}

	source.BookSourceURL = "https://changed.test"
	encoded, err = json.Marshal(source)
	if err != nil {
		t.Fatal(err)
	}
	if !containsJSONField(encoded, "futureField") || !containsJSONField(encoded, "customButton") {
		t.Fatalf("changed source dropped unknown fields: %s", encoded)
	}
}

func TestBookSourceJSONDefaultsExploreEnabledLikeLegado(t *testing.T) {
	missing, err := NewFromJSON([]byte(`{"bookSourceUrl":"https://default.test","exploreUrl":"分类::/books"}`))
	if err != nil {
		t.Fatal(err)
	}
	disabled, err := NewFromJSON([]byte(`{"bookSourceUrl":"https://disabled.test","enabledExplore":false,"exploreUrl":"分类::/books"}`))
	if err != nil {
		t.Fatal(err)
	}
	if !missing.EnabledExplore || disabled.EnabledExplore {
		t.Fatalf("missing=%v disabled=%v, want omitted=true and explicit false", missing.EnabledExplore, disabled.EnabledExplore)
	}
}

func TestBookSourceStoreRoundTripPreservesRawJSON(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "sources.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	store := NewStore(db)
	initializeBookSourceTestSchema(t, db)
	source, err := NewFromJSON([]byte(`{
		"bookSourceUrl":"https://persist.test",
		"bookSourceName":"Persisted",
		"ruleContent":"{\"content\":\".chapter\"}",
		"futureField":{"version":2}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.ImportBatch([]*BookSource{source}); err != nil {
		t.Fatal(err)
	}
	got, err := store.GetByID(source.ID)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if !containsJSONField(encoded, "futureField") {
		t.Fatalf("persisted source dropped unknown field: %s", encoded)
	}
	var values map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &values); err != nil {
		t.Fatal(err)
	}
	if string(values["ruleContent"]) != `"{\"content\":\".chapter\"}"` {
		t.Fatalf("persisted ruleContent=%s", values["ruleContent"])
	}
}

func containsJSONField(data []byte, field string) bool {
	var values map[string]json.RawMessage
	if json.Unmarshal(data, &values) != nil {
		return false
	}
	_, ok := values[field]
	return ok
}
