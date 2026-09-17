package reading

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/otwako/novelreader/internal/epub"
	"github.com/otwako/novelreader/internal/library"
)

func TestEPUBCatalogPreservesSectionOrdinalsAndMembership(t *testing.T) {
	prepared, _ := prepareReadingEPUB(t)
	prepared.Sections = append(prepared.Sections, epub.PreparedSectionInfo{Title: "Later main", Main: true})
	catalog := epubCatalog(9, prepared)
	for index, chapter := range catalog.Chapters {
		if chapter.Index != index || chapter.Auxiliary != !prepared.Sections[index].Main || chapter.IsVolume {
			t.Fatalf("incorrect section membership: %+v", chapter)
		}
	}
	if catalog.ContentRevision != 9 {
		t.Fatal("lost revision")
	}
	encoded, err := json.Marshal(catalog)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(encoded), `"auxiliary":true`) != 1 || strings.Contains(string(encoded), `"auxiliary":false`) {
		t.Fatalf("unexpected optional membership: %s", encoded)
	}
	legacy, err := json.Marshal(Catalog{Chapters: []Chapter{{Index: 0, Title: "Main"}}, ContentRevision: 2})
	if err != nil {
		t.Fatal(err)
	}
	if string(legacy) != `{"chapters":[{"index":0,"title":"Main","isVolume":false}],"contentRevision":2}` {
		t.Fatalf("legacy wire changed: %s", legacy)
	}
}

// Only chapter lookup is involved in location validation. The embedded interface
// makes accidental content/catalog calls fail instead of supplying unused stubs.
type locationProvider struct {
	provider
	result Chapter
	err    error
}

func (p locationProvider) chapter(context.Context, string, int64, int) (Chapter, error) {
	return p.result, p.err
}

func TestLocationSeparatesProgressFromAuxiliaryBookmarks(t *testing.T) {
	for _, test := range []struct {
		name                     string
		chapter                  Chapter
		err                      error
		progressErr, bookmarkErr error
	}{
		{name: "main", chapter: Chapter{Index: 0, Title: "Main"}},
		{name: "auxiliary", chapter: Chapter{Index: 1, Title: "Notes", Auxiliary: true}, progressErr: ErrInvalidLocation},
		{name: "volume", chapter: Chapter{IsVolume: true}, progressErr: ErrInvalidLocation, bookmarkErr: ErrInvalidLocation},
		{name: "missing", err: ErrChapterNotFound, progressErr: ErrInvalidLocation, bookmarkErr: ErrInvalidLocation},
		{name: "stale", err: library.ErrStateChanged, progressErr: library.ErrStateChanged, bookmarkErr: library.ErrStateChanged},
	} {
		t.Run(test.name, func(t *testing.T) {
			p := locationProvider{result: test.chapter, err: test.err}
			_, err := location(context.Background(), p, "book", 9, test.chapter.Index, true)
			if !errors.Is(err, test.progressErr) {
				t.Fatalf("progress: %v", err)
			}
			_, err = location(context.Background(), p, "book", 9, test.chapter.Index, false)
			if !errors.Is(err, test.bookmarkErr) {
				t.Fatalf("bookmark: %v", err)
			}
		})
	}
}

// This synthetic wire fixture is also consumed by the frontend parser tests.
func TestEPUBNavigationWireContract(t *testing.T) {
	prepared := epub.Preparation{
		Sections: []epub.PreparedSectionInfo{{Title: "First", Main: true}, {}, {Title: "Last", Main: true}},
		Navigation: epub.ResolvedNavigation{Source: "nav", Entries: []epub.ResolvedNavigationEntry{{Label: "Part", Children: []epub.ResolvedNavigationEntry{
			{Label: "Later first", Target: &epub.SectionTarget{Section: 2}},
			{Label: "Opening", Target: &epub.SectionTarget{Section: 0, Anchor: "a1"}},
			{Label: "Another heading", Target: &epub.SectionTarget{Section: 0, Anchor: "a2"}},
			{Label: "Unavailable parent", Unavailable: true, Children: []epub.ResolvedNavigationEntry{{Label: "Notes", Target: &epub.SectionTarget{Section: 1, Anchor: "a3"}}}},
		}}}},
	}
	expected, err := os.ReadFile("testdata/epub-navigation.json")
	if err != nil {
		t.Fatal(err)
	}
	var want any
	if err := json.Unmarshal(expected, &want); err != nil {
		t.Fatal(err)
	}
	for _, source := range []string{"nav", "ncx"} {
		prepared.Navigation.Source = source
		encoded, err := json.Marshal(epubCatalog(9, prepared))
		if err != nil {
			t.Fatal(err)
		}
		var got any
		if err := json.Unmarshal(encoded, &got); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("%s projection differs from shared wire fixture: %s", source, encoded)
		}
	}
	// Preparation, rather than this projection, owns spine fallback generation.
	prepared.Navigation = epub.ResolvedNavigation{Source: "spine", Entries: []epub.ResolvedNavigationEntry{{Label: "Section 1", Target: &epub.SectionTarget{Section: 0}}}}
	fallback := epubCatalog(9, prepared)
	if fallback.Navigation.Source != "sections" || fallback.Navigation.Entries[0].Label != "Section 1" || fallback.Navigation.Entries[0].Target.ChapterIndex != 0 {
		t.Fatalf("lost fallback provenance: %+v", fallback.Navigation)
	}
}
