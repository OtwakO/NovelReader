package reading

import (
	"context"
	"encoding/json"
	"errors"
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
