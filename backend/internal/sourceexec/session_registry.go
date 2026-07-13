// Scoped workflow session registry for detail, TOC, and content continuity.
package sourceexec

import "sync"

// SessionRegistry keeps source state shared within one server/workflow scope.
// The caller must provide a user-scoped registry when multiple users are supported.
type SessionRegistry struct {
	mu       sync.RWMutex
	books    map[string]*SourceSession
	chapters map[string]*SourceSession
}

// NewSessionRegistry creates an empty workflow session registry.
func NewSessionRegistry() *SessionRegistry {
	return &SessionRegistry{
		books:    make(map[string]*SourceSession),
		chapters: make(map[string]*SourceSession),
	}
}

// GetOrCreateBook returns the stable session for one source/book pair.
func (r *SessionRegistry) GetOrCreateBook(sourceURL, bookURL string) *SourceSession {
	if r == nil {
		return NewSourceSession()
	}
	key := sessionKey(sourceURL, bookURL)
	r.mu.Lock()
	defer r.mu.Unlock()
	if session := r.books[key]; session != nil {
		return session
	}
	session := NewSourceSession()
	r.books[key] = session
	return session
}

// GetBook returns an existing book session without creating one.
func (r *SessionRegistry) GetBook(sourceURL, bookURL string) *SourceSession {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.books[sessionKey(sourceURL, bookURL)]
}

// AssociateChapter maps a chapter URL to its book session.
func (r *SessionRegistry) AssociateChapter(sourceURL, bookURL, chapterURL string) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if session := r.books[sessionKey(sourceURL, bookURL)]; session != nil {
		r.chapters[sessionKey(sourceURL, chapterURL)] = session
	}
}

// GetChapter returns the session associated with a chapter URL.
func (r *SessionRegistry) GetChapter(sourceURL, chapterURL string) *SourceSession {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.chapters[sessionKey(sourceURL, chapterURL)]
}

// IsChapter reports whether a URL belongs to a collected chapter list.
func (r *SessionRegistry) IsChapter(sourceURL, chapterURL string) bool {
	if r == nil {
		return false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.chapters[sessionKey(sourceURL, chapterURL)]
	return ok
}

func sessionKey(sourceURL, resourceURL string) string { return sourceURL + "\x00" + resourceURL }
