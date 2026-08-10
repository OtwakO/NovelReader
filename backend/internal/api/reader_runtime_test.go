package api

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/otwako/novelreader/internal/analyzer"
	"github.com/otwako/novelreader/internal/book"
	"github.com/otwako/novelreader/internal/fetcher"
	"github.com/otwako/novelreader/internal/readerstore"
)

const runtimeTestUser readerstore.UserID = "11111111-1111-4111-8111-111111111111"

func TestReaderRuntimeQuiesceDrainsWorkPurgesStateAndRejectsNewAcquisitions(t *testing.T) {
	readers, err := readerstore.NewManager(t.TempDir(), 2)
	if err != nil {
		t.Fatal(err)
	}
	defer readers.Close()
	if err := readers.Create(context.Background(), runtimeTestUser); err != nil {
		t.Fatal(err)
	}
	limits := book.DefaultSearcherLimits()
	root := book.NewSearcherWithLimits(fetcher.New(), analyzer.NewJSVM(), analyzer.NewCacheManager(), nil, nil, limits)
	manager := newReaderRuntimeManager(readers, root, analyzer.NewJSVM(), limits, 2, time.Hour)
	defer manager.Close()
	runtime, release, err := manager.acquire(context.Background(), runtimeTestUser)
	if err != nil {
		t.Fatal(err)
	}
	runtime.searcher.CapacityStats()
	quiesced := make(chan error, 1)
	go func() { quiesced <- manager.quiesce(context.Background(), runtimeTestUser) }()
	deadline := time.Now().Add(time.Second)
	for {
		_, releaseProbe, err := manager.acquire(context.Background(), runtimeTestUser)
		if errors.Is(err, ErrReaderRuntimeDeleting) {
			break
		}
		if err != nil {
			t.Fatalf("probe acquire: %v", err)
		}
		releaseProbe()
		if time.Now().After(deadline) {
			t.Fatal("quiesce did not reject new acquisitions")
		}
		time.Sleep(time.Millisecond)
	}
	select {
	case err := <-quiesced:
		t.Fatalf("quiesce completed before request released: %v", err)
	default:
	}
	release()
	if err := <-quiesced; err != nil {
		t.Fatal(err)
	}
	if manager.runtimes[runtimeTestUser] != nil {
		t.Fatal("runtime state remains after quiesce")
	}
	if _, _, err := manager.acquire(context.Background(), runtimeTestUser); !errors.Is(err, ErrReaderRuntimeDeleting) {
		t.Fatalf("post-quiesce acquire error=%v", err)
	}
}
