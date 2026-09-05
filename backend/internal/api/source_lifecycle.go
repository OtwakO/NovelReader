package api

import (
	"context"
	"fmt"
	"time"

	"github.com/otwako/novelreader/internal/book"
	"github.com/otwako/novelreader/internal/booksource"
	"github.com/otwako/novelreader/internal/sourceinteraction"
	"github.com/otwako/novelreader/internal/sourceprofile"
)

const sourceRuntimeCleanupTimeout = 5 * time.Second

// Manual and scheduled mutations invalidate the same source-owned runtime state.
func invalidateSourceRuntime(searcher *book.Searcher, browser *sourceinteraction.BrowserSessions, sourceID string) {
	if searcher != nil {
		searcher.DeleteSourceSession(sourceID)
	}
	if browser != nil {
		ctx, cancel := context.WithTimeout(context.Background(), sourceRuntimeCleanupTimeout)
		defer cancel()
		browser.CloseSource(ctx, sourceID)
	}
}

func deleteSourceDefinition(ctx context.Context, store *booksource.Store, profiles *sourceprofile.Store, invalidate func(string), sourceID string) error {
	if err := store.Delete(sourceID); err != nil {
		return err
	}
	if invalidate != nil {
		invalidate(sourceID)
	}
	if profiles != nil {
		if err := profiles.Delete(ctx, sourceID); err != nil {
			return fmt.Errorf("source deletion committed but owned state cleanup failed; reconciliation will retry: %w", err)
		}
	}
	return nil
}

func replaceSourceCollection(ctx context.Context, store *booksource.Store, profiles *sourceprofile.Store, invalidate func(string), collectionID string, sources []*booksource.BookSource, originFilename, etag, lastModified string, now time.Time) (booksource.ReplaceResult, error) {
	result, err := store.ReplaceCollection(ctx, collectionID, sources, originFilename, etag, lastModified, now)
	if err != nil {
		return booksource.ReplaceResult{}, err
	}
	return result, finishCollectionMutation(ctx, profiles, invalidate, result.PreviousSourceIDs)
}

func deleteSourceCollection(ctx context.Context, store *booksource.Store, profiles *sourceprofile.Store, invalidate func(string), collectionID string) error {
	sourceIDs, err := store.DeleteCollection(ctx, collectionID)
	if err != nil {
		return err
	}
	return finishCollectionMutation(ctx, profiles, invalidate, sourceIDs)
}

func finishCollectionMutation(ctx context.Context, profiles *sourceprofile.Store, invalidate func(string), sourceIDs []string) error {
	for _, sourceID := range sourceIDs {
		if invalidate != nil {
			invalidate(sourceID)
		}
	}
	// Credentials live in a separate DB. Invalidation must survive a post-commit
	// cleanup failure; reconciliation is retried by the scheduler and on runtime startup.
	if profiles != nil {
		if err := profiles.Reconcile(ctx); err != nil {
			return fmt.Errorf("collection change committed but owned state cleanup failed; reconciliation will retry: %w", err)
		}
	}
	return nil
}
