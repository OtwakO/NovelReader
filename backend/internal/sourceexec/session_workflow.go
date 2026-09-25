package sourceexec

import (
	"context"
	"errors"
	"time"
)

var ErrSessionRetired = errors.New("sourceexec: source session was retired")

type sessionWorkflow struct {
	gate       chan struct{}
	references int
	retired    bool
}

// AcquireWorkflow pins session identity while waiting and running. A book URL
// selects its shared session; legacy URL-only content uses the chapter alias,
// or an isolated session if no alias exists. Release exactly once after the
// whole workflow, not between its individual script or transport operations.
// Nested operations use the supplied session without reacquiring this lease.
func (r *SessionRegistry) AcquireWorkflow(ctx context.Context, sourceID, bookURL, chapterURL string) (*SourceSession, func(), error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	// A registry-less caller (for example standalone inline-image decoding)
	// has no shared session to pin or serialize.
	if r == nil {
		return NewSourceSession(), func() {}, nil
	}
	r.mu.Lock()
	r.evictLocked(time.Now())
	var session *SourceSession
	if bookURL != "" {
		session = r.books[sessionKey(sourceID, bookURL)]
	} else {
		session = r.chapters[sessionKey(sourceID, chapterURL)]
	}
	if session == nil {
		session = NewSourceSession()
		if bookURL != "" {
			r.books[sessionKey(sourceID, bookURL)] = session
		}
	}
	workflow := r.workflows[session]
	if workflow == nil {
		workflow = &sessionWorkflow{gate: make(chan struct{}, 1)}
		r.workflows[session] = workflow
	}
	workflow.references++
	r.touchLocked(session)
	r.evictLocked(time.Now())
	r.mu.Unlock()

	unpin := func() {
		r.mu.Lock()
		defer r.mu.Unlock()
		workflow.references--
		if workflow.references == 0 {
			delete(r.workflows, session)
		}
		if !workflow.retired {
			r.touchLocked(session)
		}
		r.evictLocked(time.Now())
	}
	select {
	case <-ctx.Done():
		unpin()
		return nil, nil, ctx.Err()
	case workflow.gate <- struct{}{}:
	}
	release := func() { <-workflow.gate; unpin() }
	r.mu.Lock()
	retired := workflow.retired
	r.mu.Unlock()
	if err := ctx.Err(); err != nil {
		release()
		return nil, nil, err
	}
	if retired {
		release()
		return nil, nil, ErrSessionRetired
	}
	return session, release, nil
}

func (r *SessionRegistry) pinnedLocked(session *SourceSession) bool {
	return r.workflows[session] != nil
}
