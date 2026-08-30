// Bounded workflow session registry for detail, TOC, and content continuity.
package sourceexec

import (
	"strings"
	"sync"
	"time"
)

const (
	defaultMaxSessions = 4096
	defaultSessionTTL  = time.Hour
)

// SessionRegistry keeps source state shared within one server/workflow scope.
// The caller must provide a user-scoped registry when multiple users are supported.
type SessionRegistry struct {
	mu       sync.Mutex
	books    map[string]*SourceSession
	chapters map[string]*SourceSession
	lastUsed map[*SourceSession]time.Time
	max      int
	ttl      time.Duration
}

// NewSessionRegistry creates a registry with bounded memory and one-hour idle expiry.
func NewSessionRegistry() *SessionRegistry {
	return NewSessionRegistryWithLimits(defaultMaxSessions, defaultSessionTTL)
}

// NewSessionRegistryWithLimits creates a registry with deterministic eviction policy.
func NewSessionRegistryWithLimits(maxSessions int, idleTTL time.Duration) *SessionRegistry {
	if maxSessions < 1 {
		maxSessions = defaultMaxSessions
	}
	if idleTTL <= 0 {
		idleTTL = defaultSessionTTL
	}
	return &SessionRegistry{
		books:    make(map[string]*SourceSession),
		chapters: make(map[string]*SourceSession),
		lastUsed: make(map[*SourceSession]time.Time),
		max:      maxSessions,
		ttl:      idleTTL,
	}
}

// GetOrCreateBook returns the stable session for one source/book pair.
func (r *SessionRegistry) GetOrCreateBook(sourceURL, bookURL string) *SourceSession {
	if r == nil {
		return NewSourceSession()
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.evictLocked(time.Now())
	key := sessionKey(sourceURL, bookURL)
	if session := r.books[key]; session != nil {
		r.touchLocked(session)
		return session
	}
	session := NewSourceSession()
	r.books[key] = session
	r.touchLocked(session)
	r.evictLocked(time.Now())
	return session
}

// AssociateBook maps another source/book identity to an existing workflow session.
func (r *SessionRegistry) AssociateBook(sourceURL, bookURL string, session *SourceSession) {
	if r == nil || session == nil || bookURL == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.evictLocked(time.Now())
	r.books[sessionKey(sourceURL, bookURL)] = session
	r.touchLocked(session)
	r.evictLocked(time.Now())
}

// GetBook returns an existing book session without creating one.
func (r *SessionRegistry) GetBook(sourceURL, bookURL string) *SourceSession {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.evictLocked(time.Now())
	session := r.books[sessionKey(sourceURL, bookURL)]
	if session != nil {
		r.touchLocked(session)
	}
	return session
}

// AssociateChapter maps a chapter URL to its book session.
func (r *SessionRegistry) AssociateChapter(sourceURL, bookURL, chapterURL string) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.evictLocked(time.Now())
	if session := r.books[sessionKey(sourceURL, bookURL)]; session != nil {
		r.chapters[sessionKey(sourceURL, chapterURL)] = session
		r.touchLocked(session)
	}
}

// GetChapter returns the session associated with a chapter URL.
func (r *SessionRegistry) GetChapter(sourceURL, chapterURL string) *SourceSession {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.evictLocked(time.Now())
	session := r.chapters[sessionKey(sourceURL, chapterURL)]
	if session != nil {
		r.touchLocked(session)
	}
	return session
}

// IsChapter reports whether a URL belongs to a collected chapter list.
// DeleteSource removes every session and alias owned by one immutable Source ID.
func (r *SessionRegistry) DeleteSource(sourceID string) {
	prefix := sourceID + "\x00"
	r.mu.Lock()
	defer r.mu.Unlock()
	removed := make(map[*SourceSession]struct{})
	for key, session := range r.books {
		if strings.HasPrefix(key, prefix) {
			delete(r.books, key)
			removed[session] = struct{}{}
		}
	}
	for key, session := range r.chapters {
		if strings.HasPrefix(key, prefix) {
			delete(r.chapters, key)
			removed[session] = struct{}{}
		}
	}
	for session := range removed {
		delete(r.lastUsed, session)
	}
}

func (r *SessionRegistry) IsChapter(sourceURL, chapterURL string) bool {
	if r == nil {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.evictLocked(time.Now())
	session := r.chapters[sessionKey(sourceURL, chapterURL)]
	if session != nil {
		r.touchLocked(session)
	}
	return session != nil
}

func (r *SessionRegistry) touchLocked(session *SourceSession) {
	r.lastUsed[session] = time.Now()
}

func (r *SessionRegistry) evictLocked(now time.Time) {
	for session, lastUsed := range r.lastUsed {
		if now.Sub(lastUsed) > r.ttl {
			r.removeLocked(session)
		}
	}
	for len(r.lastUsed) > r.max {
		var oldest *SourceSession
		var oldestAt time.Time
		for session, lastUsed := range r.lastUsed {
			if oldest == nil || lastUsed.Before(oldestAt) {
				oldest, oldestAt = session, lastUsed
			}
		}
		r.removeLocked(oldest)
	}
}

func (r *SessionRegistry) removeLocked(target *SourceSession) {
	for key, session := range r.books {
		if session == target {
			delete(r.books, key)
		}
	}
	for key, session := range r.chapters {
		if session == target {
			delete(r.chapters, key)
		}
	}
	delete(r.lastUsed, target)
}

func sessionKey(sourceURL, resourceURL string) string { return sourceURL + "\x00" + resourceURL }
