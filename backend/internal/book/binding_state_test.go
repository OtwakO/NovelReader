package book

import (
	"strings"
	"testing"
)

func TestBindingStateDecodesLegacyArrayAndPromotesWithoutMetadataLoss(t *testing.T) {
	book := &Book{SourceID: "aggregate", SourceURL: "aggregate", BookURL: "/a", Origin: "Aggregate"}
	raw := `[
		{"sourceId":"aggregate","sourceUrl":"aggregate","bookUrl":"/a","sourceName":"Aggregate","discoveryQuery":"Book@A"},
		{"sourceId":"aggregate","sourceUrl":"aggregate","bookUrl":"/b","sourceName":"Aggregate","discoveryQuery":"Book@B"},
		{"sourceId":"aggregate","sourceUrl":"aggregate","bookUrl":"/c","sourceName":"Aggregate","discoveryQuery":"Book@C"}
	]`

	state, err := decodeBindingState(raw, book)
	if err != nil {
		t.Fatal(err)
	}
	if state.Active.DiscoveryQuery != "Book@A" || len(state.Alternates) != 2 {
		t.Fatalf("decoded state=%+v", state)
	}
	for _, target := range []struct {
		url, query string
	}{{"/c", "Book@C"}, {"/b", "Book@B"}, {"/a", "Book@A"}} {
		state, err = state.promote("aggregate", target.url)
		if err != nil {
			t.Fatal(err)
		}
		if state.Active.BookURL != target.url || state.Active.DiscoveryQuery != target.query {
			t.Fatalf("promoted state=%+v", state)
		}
		encoded, err := encodeBindingState(state)
		if err != nil {
			t.Fatal(err)
		}
		book.SourceID, book.BookURL = state.Active.SourceID, state.Active.BookURL
		state, err = decodeBindingState(encoded, book)
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestBindingStateUpsertPreservesMultipleResultsFromOneOpaqueQuery(t *testing.T) {
	state := bindingState{Version: bindingStateVersion, Active: AltSource{SourceID: "aggregate", SourceURL: "aggregate", BookURL: "/a", SourceName: "Aggregate"}}
	for _, binding := range []AltSource{
		{SourceID: "aggregate", SourceURL: "aggregate", BookURL: "/69", SourceName: "Aggregate", DiscoveryQuery: "Book@69"},
		{SourceID: "aggregate", SourceURL: "aggregate", BookURL: "/fake-69", SourceName: "Aggregate", DiscoveryQuery: "Book@69"},
	} {
		state = state.upsert(binding)
	}
	if len(state.Alternates) != 2 || state.Alternates[0].BookURL == state.Alternates[1].BookURL {
		t.Fatalf("multi-result query was collapsed: %+v", state.Alternates)
	}
}

func TestBindingStateClearKeepsActiveMetadata(t *testing.T) {
	state := bindingState{
		Version:    bindingStateVersion,
		Active:     AltSource{SourceID: "aggregate", SourceURL: "aggregate", BookURL: "/a", SourceName: "Aggregate", DiscoveryQuery: "Book@A"},
		Alternates: []AltSource{{SourceID: "aggregate", SourceURL: "aggregate", BookURL: "/b", SourceName: "Aggregate", DiscoveryQuery: "Book@B"}},
	}.clearAlternates()
	if state.Active.DiscoveryQuery != "Book@A" || len(state.Alternates) != 0 {
		t.Fatalf("cleared state=%+v", state)
	}
}

func TestBindingStateRejectsMalformedAndMismatchedData(t *testing.T) {
	book := &Book{SourceID: "aggregate", SourceURL: "aggregate", BookURL: "/a", Origin: "Aggregate"}
	for _, raw := range []string{
		`{"version":1,`,
		`{"version":1,"active":{"sourceId":"aggregate","sourceUrl":"aggregate","bookUrl":"/other","sourceName":"Aggregate"}}`,
	} {
		if _, err := decodeBindingState(raw, book); err == nil {
			t.Fatalf("accepted invalid binding state %q", raw)
		}
	}
}

func TestBindingStateKeepsLastChapterWithItsBinding(t *testing.T) {
	book := &Book{SourceID: "aggregate", SourceURL: "aggregate", BookURL: "/a", Origin: "Aggregate", LastChapter: "Initial provider hint"}
	state := bindingStateFromBook(book).upsert(AltSource{SourceID: "aggregate", SourceURL: "aggregate", BookURL: "/b", SourceName: "Aggregate", LastChapter: "Alternate provider hint"})
	state, err := state.promote("aggregate", "/b")
	if err != nil {
		t.Fatal(err)
	}
	if state.Active.LastChapter != "Alternate provider hint" || len(state.Alternates) != 1 || state.Alternates[0].LastChapter != "Initial provider hint" {
		t.Fatalf("state=%+v", state)
	}
}

func TestBindingStateEncodingUsesExplicitVersionedObject(t *testing.T) {
	encoded, err := encodeBindingState(bindingState{Active: AltSource{SourceID: "source", SourceURL: "source", BookURL: "/book", SourceName: "Source"}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(encoded, `{"version":1,"active":`) {
		t.Fatalf("encoded state=%s", encoded)
	}
}
