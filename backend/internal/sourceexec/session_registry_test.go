// Conformance tests for workflow session continuity.
package sourceexec

import (
	"strconv"
	"sync"
	"testing"
	"time"
)

func TestSessionRegistrySharesBookSessionWithAssociatedChapters(t *testing.T) {
	registry := NewSessionRegistry()
	bookSession := registry.GetOrCreateBook("source", "book")
	bookSession.PutVariable("token", "fixture")

	if got := registry.GetBook("source", "book"); got != bookSession {
		t.Fatal("book session was not retained")
	}
	registry.AssociateChapter("source", "book", "chapter-1")
	if got := registry.GetChapter("source", "chapter-1"); got != bookSession {
		t.Fatal("chapter did not inherit book session")
	}
	if got := registry.GetChapter("other", "chapter-1"); got != nil {
		t.Fatal("chapter session leaked across sources")
	}
}

func TestSessionRegistryEvictsOldestSessionWhenBounded(t *testing.T) {
	registry := NewSessionRegistryWithLimits(1, time.Hour)
	first := registry.GetOrCreateBook("source", "book-1")
	registry.GetOrCreateBook("source", "book-2")
	if registry.GetBook("source", "book-1") != nil {
		t.Fatal("oldest session was not evicted")
	}
	if registry.GetBook("source", "book-2") == nil || first == registry.GetBook("source", "book-2") {
		t.Fatal("newest session was not retained")
	}
}

func TestSessionRegistryEvictsIdleBookAndChapterAliases(t *testing.T) {
	registry := NewSessionRegistryWithLimits(10, time.Nanosecond)
	registry.GetOrCreateBook("source", "book").PutVariable("token", "fixture")
	registry.AssociateChapter("source", "book", "chapter")
	time.Sleep(time.Millisecond)
	registry.GetOrCreateBook("source", "new-book")
	if registry.GetBook("source", "book") != nil || registry.GetChapter("source", "chapter") != nil {
		t.Fatal("idle session aliases were not evicted")
	}
}

func TestSessionRegistryBoundsConcurrentSessionCreation(t *testing.T) {
	registry := NewSessionRegistryWithLimits(64, time.Hour)
	var wg sync.WaitGroup
	for i := 0; i < 256; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			bookURL := "book-" + strconv.Itoa(index)
			registry.GetOrCreateBook("source", bookURL)
			registry.AssociateChapter("source", bookURL, "chapter-"+strconv.Itoa(index))
			_ = registry.GetChapter("source", "chapter-"+strconv.Itoa(index))
		}(i)
	}
	wg.Wait()
	registry.mu.Lock()
	defer registry.mu.Unlock()
	if len(registry.lastUsed) > 64 {
		t.Fatalf("sessions=%d, want <= 64", len(registry.lastUsed))
	}
}

func TestSessionRegistryAssociatesReplacementBookIdentity(t *testing.T) {
	registry := NewSessionRegistry()
	session := registry.GetOrCreateBook("source", "old-book")
	session.PutVariable("token", "fixture")

	registry.AssociateBook("source", "new-book", session)

	if got := registry.GetBook("source", "new-book"); got != session || got.GetVariable("token") != "fixture" {
		t.Fatal("replacement book identity did not retain the workflow session")
	}
}

func TestSessionRegistryCreatesIsolatedBookSessions(t *testing.T) {
	registry := NewSessionRegistry()
	first := registry.GetOrCreateBook("source", "book-1")
	second := registry.GetOrCreateBook("source", "book-2")
	if first == second {
		t.Fatal("different books share a session")
	}
}
