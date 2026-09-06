// Source-switch tests keep canonical progress on readable target chapters.
package book

import (
	"path/filepath"
	"testing"

	"github.com/otwako/novelreader/internal/database"
)

func TestSwitchSourcePreservesNWayBindingMetadataAcrossReloads(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "books.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	initializeBookTestSchema(t, db)
	store := NewStore(db)
	bindings := []AltSource{
		{SourceID: "aggregate", SourceURL: "aggregate", BookURL: "/a", SourceName: "Aggregate", SourceGroup: "A", DiscoveryQuery: "Book@A", Capabilities: []string{"search"}},
		{SourceID: "aggregate", SourceURL: "aggregate", BookURL: "/b", SourceName: "Aggregate", SourceGroup: "B", DiscoveryQuery: "Book@B", Capabilities: []string{"search", "toc"}},
		{SourceID: "aggregate", SourceURL: "aggregate", BookURL: "/c", SourceName: "Aggregate", SourceGroup: "C", DiscoveryQuery: "Book@C", Capabilities: []string{"toc"}},
	}
	wantVariables := make(map[string]string)
	for index := range bindings {
		bindings[index].VariableMap = `{"binding":"` + bindings[index].BookURL + `"}`
		wantVariables[bindings[index].BookURL] = bindings[index].VariableMap
	}
	active := bindings[0]
	if err := store.AddBook(&Book{ID: "book", Name: "Book", Author: "Author", SourceID: active.SourceID, SourceURL: active.SourceURL, BookURL: active.BookURL, Origin: active.SourceName, VariableMap: active.VariableMap, ActiveSource: &active, AlternateSources: bindings[1:]}); err != nil {
		t.Fatal(err)
	}
	chapters := []Chapter{{Index: 0, Title: "Chapter", URL: "/chapter"}}
	for _, wanted := range []AltSource{bindings[2], bindings[1], bindings[0]} {
		current, err := store.GetBook("book")
		if err != nil {
			t.Fatal(err)
		}
		for _, alternate := range current.AlternateSources {
			if alternate.VariableMap != wantVariables[alternate.BookURL] {
				t.Fatalf("lost binding variables before switch: %+v", alternate)
			}
		}
		wantVariables[wanted.BookURL] = `{"refreshed":"` + wanted.BookURL + `"}`
		target := Book{SourceID: wanted.SourceID, SourceURL: wanted.SourceURL, BookURL: wanted.BookURL, Origin: wanted.SourceName, VariableMap: wantVariables[wanted.BookURL]}
		if err := store.SwitchSource("book", current.StateVersion, target, chapters, 0, 0); err != nil {
			t.Fatal(err)
		}
		reloaded, err := store.GetBook("book")
		if err != nil {
			t.Fatal(err)
		}
		if reloaded.ActiveSource == nil || reloaded.ActiveSource.BookURL != wanted.BookURL || reloaded.ActiveSource.SourceGroup != wanted.SourceGroup || reloaded.ActiveSource.DiscoveryQuery != wanted.DiscoveryQuery {
			t.Fatalf("active binding after switch to %s: %+v", wanted.BookURL, reloaded.ActiveSource)
		}
		if reloaded.VariableMap != wantVariables[wanted.BookURL] || reloaded.ActiveSource.VariableMap != "" {
			t.Fatalf("active variables must have one owner: %+v", reloaded)
		}
		if len(reloaded.AlternateSources) != 2 {
			t.Fatalf("alternates after switch to %s: %+v", wanted.BookURL, reloaded.AlternateSources)
		}
	}
}

func TestMigrateChapterIndexMatchesNormalizedTitleThenClampsRawIndex(t *testing.T) {
	chapters := []Chapter{
		{Index: 0, Title: "Volume", IsVolume: true},
		{Index: 1, Title: "第 12 章：风雨！"},
		{Index: 2, Title: "After"},
	}
	if index, mapping := MigrateChapterIndex(chapters, "第12章 风雨", 2); index != 1 || mapping != "title" {
		t.Fatalf("title index=%d mapping=%q", index, mapping)
	}
	if index, mapping := MigrateChapterIndex(chapters, "Missing", 99); index != 2 || mapping != "index" {
		t.Fatalf("fallback index=%d mapping=%q", index, mapping)
	}
	if index, mapping := MigrateChapterIndex(chapters, "Missing", 0); index != 1 || mapping != "index" {
		t.Fatalf("volume fallback index=%d mapping=%q", index, mapping)
	}
	if index, mapping := MigrateChapterIndex([]Chapter{{Index: 0, IsVolume: true}}, "Missing", 0); index != -1 || mapping != "" {
		t.Fatalf("unreadable index=%d mapping=%q", index, mapping)
	}
}
