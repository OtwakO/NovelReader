package api

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/otwako/novelreader/internal/booksource"
	"github.com/otwako/novelreader/internal/readerstore"
)

const (
	sourceCollectionListTimeout   = 10 * time.Second
	sourceCollectionReaderTimeout = 10 * time.Second
	sourceCollectionSyncTimeout   = time.Minute
	sourceCollectionRecordTimeout = 5 * time.Second
)

type sourceCollectionScheduler struct {
	runtimes  *readerRuntimeManager
	loader    *booksource.RemoteLoader
	users     func(context.Context) ([]readerstore.UserID, error)
	interval  time.Duration
	ctx       context.Context
	cancel    context.CancelFunc
	done      chan struct{}
	closeOnce sync.Once
}

func newSourceCollectionScheduler(runtimes *readerRuntimeManager, loader *booksource.RemoteLoader, users func(context.Context) ([]readerstore.UserID, error)) *sourceCollectionScheduler {
	ctx, cancel := context.WithCancel(context.Background())
	return &sourceCollectionScheduler{runtimes: runtimes, loader: loader, users: users, interval: time.Hour, ctx: ctx, cancel: cancel, done: make(chan struct{})}
}

func (s *sourceCollectionScheduler) Start() {
	go s.run()
}

func (s *sourceCollectionScheduler) Close() {
	if s == nil {
		return
	}
	s.closeOnce.Do(s.cancel)
	<-s.done
}

func (s *sourceCollectionScheduler) run() {
	defer close(s.done)
	timer := time.NewTimer(time.Minute)
	defer timer.Stop()
	for {
		select {
		case <-timer.C:
			s.syncDue(s.ctx)
			timer.Reset(s.interval)
		case <-s.ctx.Done():
			return
		}
	}
}

func (s *sourceCollectionScheduler) syncDue(ctx context.Context) {
	listCtx, cancel := context.WithTimeout(ctx, sourceCollectionListTimeout)
	users, err := s.users(listCtx)
	cancel()
	if err != nil {
		if ctx.Err() == nil {
			slog.Warn("source collection scheduler could not list readers", "error", err)
		}
		return
	}
	for _, userID := range users {
		if ctx.Err() != nil {
			return
		}
		readerCtx, cancel := context.WithTimeout(ctx, sourceCollectionReaderTimeout)
		runtime, release, err := s.runtimes.acquire(readerCtx, userID)
		if err != nil {
			cancel()
			if ctx.Err() == nil {
				slog.Warn("source collection scheduler could not acquire reader runtime", "error", err)
			}
			continue
		}
		collections, err := runtime.sourceStore.ListDueCollections(readerCtx, time.Now())
		cancel()
		if err != nil {
			release()
			slog.Warn("source collection scheduler could not list due collections", "error", err)
			continue
		}
		for _, collection := range collections {
			syncCtx, cancel := context.WithTimeout(ctx, sourceCollectionSyncTimeout)
			s.syncCollection(syncCtx, runtime, collection)
			cancel()
			if ctx.Err() != nil {
				break
			}
		}
		release()
	}
}

func (s *sourceCollectionScheduler) syncCollection(ctx context.Context, runtime *readerRuntime, collection booksource.Collection) {
	store := runtime.sourceStore
	now := time.Now()
	document, err := s.loader.Load(ctx, collection.OriginURL, collection.ETag, collection.LastModified)
	if err != nil {
		s.recordCollectionFailure(store, collection.ID, err, now)
		return
	}
	if document.NotModified {
		s.recordCollectionSuccess(store, collection.ID, now)
		return
	}
	sources, err := booksource.ImportSources(document.Body)
	if err != nil {
		s.recordCollectionFailure(store, collection.ID, err, now)
		return
	}
	if collection.SourceCount > 0 && len(sources) == 0 {
		s.recordCollectionFailure(store, collection.ID, errors.New("scheduled sync refused an unexpectedly empty collection"), now)
		return
	}
	if _, err := replaceSourceCollection(ctx, store, runtime.sourceProfiles, runtime.searcher.DeleteSourceSession, collection.ID, sources, "", document.ETag, document.LastModified, now); err != nil {
		s.recordCollectionFailure(store, collection.ID, err, now)
	}
}

func (s *sourceCollectionScheduler) recordCollectionSuccess(store *booksource.Store, collectionID string, now time.Time) {
	ctx, cancel := context.WithTimeout(s.ctx, sourceCollectionRecordTimeout)
	defer cancel()
	if err := store.RecordCollectionSuccess(ctx, collectionID, now); err != nil && s.ctx.Err() == nil {
		slog.Warn("source collection scheduler could not record successful sync", "collection_id", collectionID, "error", err)
	}
}

func (s *sourceCollectionScheduler) recordCollectionFailure(store *booksource.Store, collectionID string, cause error, now time.Time) {
	ctx, cancel := context.WithTimeout(s.ctx, sourceCollectionRecordTimeout)
	defer cancel()
	if err := store.RecordCollectionFailure(ctx, collectionID, cause.Error(), now); err != nil && s.ctx.Err() == nil {
		slog.Warn("source collection scheduler could not record failed sync", "collection_id", collectionID, "error", err)
	}
}
