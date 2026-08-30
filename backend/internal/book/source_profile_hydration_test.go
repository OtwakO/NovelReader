package book

import (
	"context"
	"errors"
	"testing"

	"github.com/otwako/novelreader/internal/analyzer"
	"github.com/otwako/novelreader/internal/booksource"
	"github.com/otwako/novelreader/internal/sourceexec"
)

func TestOpenExploreReportsSetupRequiredWhenProfileHydrationFails(t *testing.T) {
	source := booksource.BookSource{ID: "source-a", BookSourceURL: "https://source.test", EnabledExplore: true, ExploreURL: "Books::https://source.test/books"}
	searcher := NewSearcher(nil, analyzer.NewJSVM(), analyzer.NewCacheManager(), exploreSourceFixtureStore{source: source}, nil)
	searcher.SetSourceSessionHydrator(func(context.Context, booksource.BookSource, *sourceexec.SourceSession) error {
		return errors.New("profile unavailable")
	})
	_, err := searcher.OpenExplore(t.Context(), source.ID)
	var exploreError *ExploreError
	if !errors.As(err, &exploreError) || exploreError.Code != "source_setup_required" || exploreError.Retryable {
		t.Fatalf("error=%v", err)
	}
}
