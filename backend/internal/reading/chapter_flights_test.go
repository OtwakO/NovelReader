package reading

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"
	"time"
)

func TestChapterFlightsShareWithoutTransferringCancellation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var flights chapterFlights
		key := chapterKey{bookID: "book", revision: 1}
		started, release := make(chan struct{}), make(chan struct{})
		leaderCtx, cancel := context.WithCancel(t.Context())
		defer cancel()
		leaderDone := make(chan error, 1)
		go func() {
			_, err := flights.do(leaderCtx, key, func(ctx context.Context) (chapterResult, error) {
				remaining := int64(1000)
				expires := time.Now().Add(time.Second)
				close(started)
				<-release
				return chapterResult{content: Content{ContentRevision: 1, FreshForMS: &remaining}, freshUntil: expires}, ctx.Err()
			})
			leaderDone <- err
		}()
		<-started
		result := make(chan Content, 1)
		go func() {
			value, err := flights.do(t.Context(), key, func(context.Context) (chapterResult, error) {
				t.Error("duplicate retrieval")
				return chapterResult{}, nil
			})
			if err != nil {
				t.Error(err)
			}
			result <- value
		}()
		canceledCtx, stop := context.WithCancel(t.Context())
		defer stop()
		detached := make(chan error, 1)
		go func() { _, err := flights.do(canceledCtx, key, nil); detached <- err }()
		synctest.Wait()
		stop()
		if err := <-detached; !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
		cancel()
		synctest.Wait()
		select {
		case <-leaderDone:
			t.Error("leader released before shared work drained")
		default:
		}
		time.Sleep(2 * time.Second) // Virtual time: publication/waiting cannot extend freshness.
		close(release)
		if err := <-leaderDone; !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
		if value := <-result; value.ContentRevision != 1 || value.FreshForMS == nil || *value.FreshForMS != 0 {
			t.Fatalf("lost shared result: %+v", value)
		}
	})
}

func TestChapterFlightsRefreshWaitsThenSharesNewRetrieval(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		key := chapterKey{bookID: "book", revision: 1}
		previous := &chapterFlight{done: make(chan struct{})}
		flights := chapterFlights{active: map[chapterKey]*chapterFlight{key: previous}}
		refreshKey := key
		refreshKey.refresh = true
		started, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
		go func() {
			defer close(done)
			_, err := flights.do(t.Context(), refreshKey, func(context.Context) (chapterResult, error) {
				close(started)
				<-release
				return chapterResult{content: Content{ContentRevision: 2}}, nil
			})
			if err != nil {
				t.Error(err)
			}
		}()
		synctest.Wait()
		shared := make(chan Content, 1)
		go func() {
			value, err := flights.do(t.Context(), refreshKey, nil)
			if err != nil {
				t.Error(err)
			}
			shared <- value
		}()
		synctest.Wait()
		select {
		case <-started:
			t.Fatal("Refresh started before previous work finished")
		default:
		}
		close(previous.done)
		<-started
		close(release)
		<-done
		if value := <-shared; value.ContentRevision != 2 {
			t.Fatalf("Refresh reused ordinary result: %+v", value)
		}
	})
}
