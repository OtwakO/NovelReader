package api

import (
	"context"
	"time"

	"github.com/otwako/novelreader/internal/booksource"
	"github.com/otwako/novelreader/internal/sourceprofile"
)

func deleteSourceDefinition(ctx context.Context, store *booksource.Store, profiles *sourceprofile.Store, invalidate func(string), sourceID string) error {
	if err := store.Delete(sourceID); err != nil {
		return err
	}
	if profiles != nil {
		if err := profiles.Delete(ctx, sourceID); err != nil {
			return err
		}
	}
	if invalidate != nil {
		invalidate(sourceID)
	}
	return nil
}

func replaceSourceCollection(ctx context.Context, store *booksource.Store, profiles *sourceprofile.Store, invalidate func(string), collectionID string, sources []*booksource.BookSource, originFilename, etag, lastModified string, now time.Time) (booksource.ReplaceResult, error) {
	existing, err := store.ListByCollection(collectionID)
	if err != nil {
		return booksource.ReplaceResult{}, err
	}
	result, err := store.ReplaceCollection(ctx, collectionID, sources, originFilename, etag, lastModified, now)
	if err != nil {
		return booksource.ReplaceResult{}, err
	}
	if profiles != nil {
		if err := profiles.Reconcile(ctx); err != nil {
			return booksource.ReplaceResult{}, err
		}
	}
	for _, source := range existing {
		if invalidate != nil {
			invalidate(source.ID)
		}
	}
	return result, nil
}

func deleteSourceCollection(ctx context.Context, store *booksource.Store, profiles *sourceprofile.Store, invalidate func(string), collectionID string) error {
	existing, err := store.ListByCollection(collectionID)
	if err != nil {
		return err
	}
	if err := store.DeleteCollection(ctx, collectionID); err != nil {
		return err
	}
	if profiles != nil {
		if err := profiles.Reconcile(ctx); err != nil {
			return err
		}
	}
	for _, source := range existing {
		if invalidate != nil {
			invalidate(source.ID)
		}
	}
	return nil
}
