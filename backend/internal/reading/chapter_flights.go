package reading

import (
	"context"
	"sync"
	"time"
)

// Flights share results, not execution locks. SourceSession leases still own
// ordering across chapters, images and other workflows of the same book.
type chapterKey struct {
	bookID, sourceID, definition, bookURL, chapterURL string
	revision                                          int64
	index                                             int
	refresh                                           bool
}
// Retain the original deadline: a late joiner must not inherit the freshness
// measured before processing, resource admission or cache persistence finished.
type chapterResult struct {
	content    Content
	freshUntil time.Time
}

type chapterFlight struct {
	done        chan struct{}
	resultValue chapterResult
	err         error
}
type chapterFlights struct {
	mu     sync.Mutex
	active map[chapterKey]*chapterFlight
}

func (g *chapterFlights) do(ctx context.Context, key chapterKey, run func(context.Context) (chapterResult, error)) (Content, error) {
	if err := ctx.Err(); err != nil {
		return Content{}, err
	}
	g.mu.Lock()
	if g.active == nil {
		g.active = make(map[chapterKey]*chapterFlight)
	}
	// A miss can join Refresh, but Refresh must never join an ordinary cache read.
	refreshKey := key
	refreshKey.refresh = true
	existing := g.active[refreshKey]
	if existing == nil && !key.refresh {
		existing = g.active[key]
	}
	if existing != nil {
		g.mu.Unlock()
		select {
		case <-ctx.Done():
			return Content{}, ctx.Err()
		case <-existing.done:
			return existing.result(ctx)
		}
	}
	var previous *chapterFlight
	if key.refresh {
		ordinaryKey := key
		ordinaryKey.refresh = false
		previous = g.active[ordinaryKey]
	}
	flight := &chapterFlight{done: make(chan struct{})}
	g.active[key] = flight
	g.mu.Unlock()

	// Run synchronously: this initiating handler keeps its reader-runtime lease
	// until all work drains, even after disconnect. Execution has the Searcher's
	// existing source timeout; followers can detach without canceling its owner.
	if previous != nil {
		<-previous.done
	}
	flight.resultValue, flight.err = run(context.WithoutCancel(ctx))
	g.mu.Lock()
	delete(g.active, key)
	close(flight.done)
	g.mu.Unlock()
	return flight.result(ctx)
}

func (f *chapterFlight) result(ctx context.Context) (Content, error) {
	if err := ctx.Err(); err != nil {
		return Content{}, err
	}
	content := f.resultValue.content
	if content.FreshForMS != nil {
		remaining := max(int64(0), min(*content.FreshForMS, time.Until(f.resultValue.freshUntil).Milliseconds()))
		content.FreshForMS = &remaining
	}
	return content, f.err
}
