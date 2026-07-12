// Conformance tests for workflow session continuity.
package sourceexec

import "testing"

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

func TestSessionRegistryCreatesIsolatedBookSessions(t *testing.T) {
	registry := NewSessionRegistry()
	first := registry.GetOrCreateBook("source", "book-1")
	second := registry.GetOrCreateBook("source", "book-2")
	if first == second {
		t.Fatal("different books share a session")
	}
}
