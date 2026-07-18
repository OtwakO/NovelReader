// Explore sessions bind source state, categories, and paging to opaque IDs.
package book

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/otwako/novelreader/internal/booksource"
	"github.com/otwako/novelreader/internal/sourceexec"
)

type exploreSession struct {
	mu            sync.Mutex
	source        booksource.BookSource
	state         *sourceexec.SourceSession
	categories    map[string]exploreKind
	entryIDs      []string
	generation    uint64
	retainedBooks int
	pages         map[string]*explorePageState
}

type explorePageState struct {
	next int
	seen map[string]bool
	last *ExplorePage
}

type exploreRegistry struct {
	mu       sync.Mutex
	sessions map[string]*exploreSession
	lastUsed map[string]time.Time
	leases   map[string]int
	max      int
	ttl      time.Duration
}

func newExploreRegistry(max int, ttl time.Duration) *exploreRegistry {
	if max < 1 {
		max = defaultMaxSessions
	}
	if ttl <= 0 {
		ttl = defaultSessionTTL
	}
	return &exploreRegistry{
		sessions: make(map[string]*exploreSession),
		lastUsed: make(map[string]time.Time),
		leases:   make(map[string]int),
		max:      max,
		ttl:      ttl,
	}
}

func (r *exploreRegistry) create(source booksource.BookSource, kinds []exploreKind, state *sourceexec.SourceSession) (string, *exploreSession, error) {
	if r == nil {
		return "", nil, fmt.Errorf("explore sessions unavailable")
	}
	categories, entryIDs := exploreCategories(kinds, 0)
	if state == nil {
		state = sourceexec.NewSourceSession()
	}
	session := &exploreSession{
		source: source, state: state, categories: categories, entryIDs: entryIDs,
		pages: make(map[string]*explorePageState),
	}
	now := time.Now()
	r.mu.Lock()
	defer r.mu.Unlock()
	r.evictExpiredLocked(now)
	if !r.makeRoomLocked() {
		return "", nil, fmt.Errorf("explore session capacity reached")
	}
	for {
		var token [16]byte
		if _, err := rand.Read(token[:]); err != nil {
			return "", nil, fmt.Errorf("explore session id: %w", err)
		}
		id := hex.EncodeToString(token[:])
		if r.sessions[id] == nil {
			r.sessions[id] = session
			r.lastUsed[id] = now
			return id, session, nil
		}
	}
}

func exploreCategories(kinds []exploreKind, generation uint64) (map[string]exploreKind, []string) {
	categories := make(map[string]exploreKind, len(kinds))
	entryIDs := make([]string, len(kinds))
	for index, kind := range kinds {
		id := fmt.Sprintf("entry-%d", index)
		if generation > 0 {
			id = fmt.Sprintf("entry-%d-%d", generation, index)
		}
		categories[id] = kind
		entryIDs[index] = id
	}
	return categories, entryIDs
}

func (r *exploreRegistry) acquire(id string) (*exploreSession, func()) {
	if r == nil || id == "" {
		return nil, func() {}
	}
	now := time.Now()
	r.mu.Lock()
	r.evictExpiredLocked(now)
	session := r.sessions[id]
	if session == nil {
		r.mu.Unlock()
		return nil, func() {}
	}
	r.leases[id]++
	r.lastUsed[id] = now
	r.mu.Unlock()
	return session, func() {
		r.mu.Lock()
		if r.leases[id] > 1 {
			r.leases[id]--
		} else {
			delete(r.leases, id)
		}
		if r.sessions[id] != nil {
			r.lastUsed[id] = time.Now()
		}
		r.mu.Unlock()
	}
}

func (r *exploreRegistry) evictExpiredLocked(now time.Time) {
	for id, lastUsed := range r.lastUsed {
		if r.leases[id] == 0 && now.Sub(lastUsed) > r.ttl {
			delete(r.sessions, id)
			delete(r.lastUsed, id)
		}
	}
}

func (r *exploreRegistry) makeRoomLocked() bool {
	for len(r.sessions) >= r.max {
		oldestID := ""
		var oldest time.Time
		for id, lastUsed := range r.lastUsed {
			if r.leases[id] == 0 && (oldestID == "" || lastUsed.Before(oldest)) {
				oldestID, oldest = id, lastUsed
			}
		}
		if oldestID == "" {
			return false
		}
		delete(r.sessions, oldestID)
		delete(r.lastUsed, oldestID)
	}
	return true
}
