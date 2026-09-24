package api

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/otwako/novelreader/internal/book"
	"github.com/otwako/novelreader/internal/readerstore"
)

func TestRuntimeInitializationHasSingleOwnership(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		manager := newReaderRuntimeManager(nil, nil, nil, nil, book.SearcherLimits{}, 1, time.Minute, nil)
		defer manager.Close()
		finish := make(chan struct{})
		var calls atomic.Int32
		failed := errors.New("initialization failed")
		manager.initialize = func(context.Context, readerstore.UserID) (*readerRuntime, error) {
			calls.Add(1)
			<-finish
			return nil, failed
		}
		results := make(chan error, 2)
		for range 2 {
			go func() { _, _, err := manager.acquire(context.Background(), "reader"); results <- err }()
		}
		synctest.Wait()
		if count := calls.Load(); count != 1 {
			t.Errorf("concurrent initialization count=%d, want 1", count)
		}
		otherCtx, cancel := context.WithCancel(context.Background())
		other := make(chan error, 1)
		go func() { _, _, err := manager.acquire(otherCtx, "other"); other <- err }()
		synctest.Wait()
		if count := calls.Load(); count != 1 {
			t.Errorf("opening did not reserve capacity: calls=%d", count)
		}
		cancel()
		synctest.Wait()
		close(finish)
		if err := <-other; !errors.Is(err, context.Canceled) {
			t.Errorf("capacity waiter cancellation=%v", err)
		}
		for range 2 {
			if err := <-results; !errors.Is(err, failed) {
				t.Errorf("initialization result=%v", err)
			}
		}
		if calls.Load() != 2 {
			t.Errorf("failed initialization did not release reservation: calls=%d", calls.Load())
		}
	})
}

func TestRuntimeLifecycleWaitsForInitialization(t *testing.T) {
	for _, operation := range []string{"quiesce", "shutdown"} {
		t.Run(operation, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				readers, err := readerstore.NewManager(t.TempDir(), 1)
				if err != nil {
					t.Fatal(err)
				}
				defer readers.Close()
				if err := readers.Create(t.Context(), runtimeTestUser); err != nil {
					t.Fatal(err)
				}
				home, err := readers.Open(t.Context(), runtimeTestUser)
				if err != nil {
					t.Fatal(err)
				}
				defer home.Close()
				manager := newReaderRuntimeManager(nil, nil, nil, nil, book.SearcherLimits{}, 1, time.Minute, nil)
				finish := make(chan struct{})
				manager.initialize = func(context.Context, readerstore.UserID) (*readerRuntime, error) {
					<-finish
					return &readerRuntime{home: home}, nil
				}
				acquired := make(chan error, 1)
				go func() { _, _, err := manager.acquire(context.Background(), "reader"); acquired <- err }()
				synctest.Wait()
				stopped := make(chan error, 1)
				go func() {
					if operation == "quiesce" {
						stopped <- manager.quiesce(context.Background(), "reader")
					} else {
						stopped <- manager.Close()
					}
				}()
				synctest.Wait()
				early := false
				select {
				case err := <-stopped:
					early = true
					t.Errorf("%s returned before initialization ended: %v", operation, err)
				default:
				}
				close(finish)
				acquireErr := <-acquired
				expected := ErrReaderRuntimeDeleting
				if operation == "shutdown" {
					expected = ErrReaderRuntimeClosed
				}
				if !errors.Is(acquireErr, expected) {
					t.Errorf("initialized during %s: %v", operation, acquireErr)
				}
				synctest.Wait()
				if !early {
					if err := <-stopped; err != nil {
						t.Errorf("lifecycle result=%v", err)
					}
				}
				manager.Close()
				ctx, cancel := context.WithTimeout(t.Context(), time.Second)
				defer cancel()
				if err := readers.Remove(ctx, runtimeTestUser); err != nil {
					t.Fatalf("initialization leaked a home lease: %v", err)
				}
			})
		})
	}
}
