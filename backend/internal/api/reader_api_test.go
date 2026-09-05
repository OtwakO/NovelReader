package api

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"
)

// newReaderTestServer binds narrowly supplied reader dependencies while retaining
// the public Server wrapper used by HTTP fixtures.
func newReaderTestServer(runtime *readerRuntime) *Server {
	services := &readerServices{}
	a := newReaderAPI(runtime, services)
	runtime.api = a
	s := &Server{mux: a.mux, standalone: a, services: services}
	if runtime.db != nil {
		s.health = runtime.db
	}
	return s
}

func TestReaderHandlerReusePreservesCandidateLeaseAndReplacement(t *testing.T) {
	server, sessions, _, aliceID, cleanup := newOwnershipServer(t)
	defer cleanup()
	runtime, release, err := server.runtimes.acquire(t.Context(), aliceID)
	if err != nil {
		t.Fatal(err)
	}
	handler := runtime.api
	release()
	var candidateRelease func()
	// This route exists only on the cached handler. It also exercises the extra
	// lease that a candidate operation must retain beyond its starting request.
	handler.mux.HandleFunc("GET /api/lease-probe", func(w http.ResponseWriter, r *http.Request) {
		lease, err := handler.acquireCandidateRuntime(r, string(aliceID))
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, err.Error())
			return
		}
		candidateRelease = lease.Release
		w.WriteHeader(http.StatusNoContent)
	})
	response := authenticatedOwnershipRequest(t, server, sessions, aliceID, "/api/lease-probe")
	if response.Code != http.StatusNoContent || candidateRelease == nil {
		t.Fatalf("cached reader handler not served: %d", response.Code)
	}
	defer func() {
		if candidateRelease != nil {
			candidateRelease()
		}
	}()
	server.runtimes.mu.Lock()
	references := runtime.references
	server.runtimes.mu.Unlock()
	if references != 1 {
		t.Fatalf("request lease was not released independently: references=%d", references)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()
	if err := server.runtimes.quiesce(ctx, aliceID); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("candidate lease did not hold runtime: %v", err)
	}
	candidateRelease()
	candidateRelease = nil
	drainCtx, cancelDrain := context.WithTimeout(t.Context(), time.Second)
	defer cancelDrain()
	if err := server.runtimes.quiesce(drainCtx, aliceID); err != nil {
		t.Fatal(err)
	}
	server.runtimes.resume(aliceID)
	replacement, releaseReplacement, err := server.runtimes.acquire(t.Context(), aliceID)
	if err != nil {
		t.Fatal(err)
	}
	defer releaseReplacement()
	if replacement.api == handler || replacement.api.readerRuntime != replacement {
		t.Fatal("replacement reused the retired reader binding")
	}
	response = authenticatedOwnershipRequest(t, server, sessions, aliceID, "/api/lease-probe")
	if response.Code != http.StatusNotFound {
		t.Fatalf("retired handler remains routable: %d", response.Code)
	}
}
