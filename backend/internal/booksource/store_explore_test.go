// Explore source listing tests respect global and Explore-specific enablement.
package booksource

import (
	"path/filepath"
	"testing"

	"github.com/otwako/novelreader/internal/database"
)

func TestListExploreEnabledRequiresGlobalAndExploreEnablement(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "sources.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := NewStore(db)
	initializeBookSourceTestSchema(t, db)
	for _, source := range []*BookSource{
		{BookSourceURL: "https://explore.test", BookSourceName: "Explore", Enabled: false, EnabledExplore: true, ExploreURL: "分类::/books"},
		{BookSourceURL: "https://enabled.test", BookSourceName: "Enabled", Enabled: true, EnabledExplore: true, ExploreURL: "分类::/books"},
		{BookSourceURL: "https://search.test", BookSourceName: "Search", Enabled: true, EnabledExplore: false, ExploreURL: "分类::/books"},
		{BookSourceURL: "https://blank.test", BookSourceName: "Blank", Enabled: true, EnabledExplore: true},
	} {
		if err := store.Upsert(source); err != nil {
			t.Fatal(err)
		}
	}
	sources, err := store.ListExploreEnabled()
	if err != nil {
		t.Fatal(err)
	}
	if len(sources) != 1 || sources[0].BookSourceURL != "https://enabled.test" {
		t.Fatalf("sources=%+v", sources)
	}
}
