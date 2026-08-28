package api

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/otwako/novelreader/internal/booksource"
	"github.com/otwako/novelreader/internal/readerstore"
)

type sourceCollectionScheduler struct {
	runtimes *readerRuntimeManager
	loader   *booksource.RemoteLoader
	users    func(context.Context) ([]readerstore.UserID, error)
	interval time.Duration
	stop     chan struct{}
	done     chan struct{}
	once     sync.Once
}

func newSourceCollectionScheduler(runtimes *readerRuntimeManager, loader *booksource.RemoteLoader, users func(context.Context) ([]readerstore.UserID, error)) *sourceCollectionScheduler {
	return &sourceCollectionScheduler{runtimes: runtimes, loader: loader, users: users, interval: time.Hour, stop: make(chan struct{}), done: make(chan struct{})}
}

func (s *sourceCollectionScheduler) Start() {
	go s.run()
}

func (s *sourceCollectionScheduler) Close() {
	if s == nil {
		return
	}
	s.once.Do(func() { close(s.stop) })
	<-s.done
}

func (s *sourceCollectionScheduler) run() {
	defer close(s.done)
	timer := time.NewTimer(time.Minute)
	defer timer.Stop()
	for {
		select {
		case <-timer.C:
			s.syncDue(context.Background())
			timer.Reset(s.interval)
		case <-s.stop:
			return
		}
	}
}

func (s *sourceCollectionScheduler) syncDue(ctx context.Context) {
	users, err := s.users(ctx)
	if err != nil {
		slog.Warn("source collection scheduler could not list readers", "error", err)
		return
	}
	for _, userID := range users {
		runtime, release, err := s.runtimes.acquire(ctx, userID)
		if err != nil {
			continue
		}
		collections, err := runtime.sourceStore.ListDueCollections(time.Now())
		if err == nil {
			for _, collection := range collections {
				s.syncCollection(ctx, runtime.sourceStore, collection)
			}
		}
		release()
	}
}

func (s *sourceCollectionScheduler) syncCollection(ctx context.Context, store *booksource.Store, collection booksource.Collection) {
	now := time.Now()
	document, err := s.loader.Load(ctx, collection.OriginURL, collection.ETag, collection.LastModified)
	if err != nil {
		_ = store.RecordCollectionFailure(ctx, collection.ID, err.Error(), now)
		return
	}
	if document.NotModified {
		_ = store.RecordCollectionSuccess(ctx, collection.ID, now)
		return
	}
	sources, err := booksource.ImportSources(document.Body)
	if err != nil {
		_ = store.RecordCollectionFailure(ctx, collection.ID, err.Error(), now)
		return
	}
	if collection.SourceCount > 0 && len(sources) == 0 {
		_ = store.RecordCollectionFailure(ctx, collection.ID, "scheduled sync refused an unexpectedly empty collection", now)
		return
	}
	if _, err := store.ReplaceCollection(ctx, collection.ID, sources, "", document.ETag, document.LastModified, now); err != nil {
		_ = store.RecordCollectionFailure(ctx, collection.ID, err.Error(), now)
	}
}
