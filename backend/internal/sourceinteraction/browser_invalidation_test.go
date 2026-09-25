package sourceinteraction

import (
	"context"
	"github.com/otwako/novelreader/internal/sourceexec"
	"github.com/otwako/novelreader/internal/webview"
	"testing"
)

func TestPendingBrowserInvalidation(t *testing.T) {
	s := NewBrowserSessions(&browserFixture{})
	id := s.Register("source-a", BrowserRequest{URL: "https://example.test/login"})
	s.CloseSource(t.Context(), "source-a")
	frame, err := s.Start(t.Context(), "source-a", id, webview.InteractiveViewport{}, sourceexec.NewSourceSession())
	if err == nil {
		t.Fatalf("invalidated pending request still launched: %s", frame.SessionID)
	}
}

type blockingBrowser struct {
	browserFixture
	entered, release chan struct{}
}

func (b *blockingBrowser) StartInteractive(_ context.Context, rawURL, _ string, _ webview.InteractiveViewport, _ *sourceexec.SourceSession) (webview.InteractiveFrame, error) {
	if rawURL == "https://example.test/replacement" {
		return webview.InteractiveFrame{SessionID: "replacement"}, nil
	}
	close(b.entered)
	<-b.release
	return webview.InteractiveFrame{SessionID: "late-session"}, nil
}
func TestStartingBrowserInvalidation(t *testing.T) {
	b := &blockingBrowser{entered: make(chan struct{}), release: make(chan struct{})}
	s := NewBrowserSessions(b)
	id := s.Register("source-a", BrowserRequest{URL: "https://example.test/login"})
	result := make(chan error, 1)
	go func() {
		_, err := s.Start(t.Context(), "source-a", id, webview.InteractiveViewport{}, sourceexec.NewSourceSession())
		result <- err
	}()
	<-b.entered
	s.CloseSource(t.Context(), "source-a")
	close(b.release)
	if err := <-result; err == nil {
		t.Fatal("browser start published after source invalidation")
	}
	if b.closed != 1 {
		t.Fatalf("invalidated worker was not closed: %d", b.closed)
	}
}

func TestBrowserRequestsAreSourceOwned(t *testing.T) {
	browser := &browserFixture{}
	sessions := NewBrowserSessions(browser)
	id := sessions.Register("source-a", BrowserRequest{URL: "https://example.test/login"})
	if _, err := sessions.Start(t.Context(), "source-b", id, webview.InteractiveViewport{}, sourceexec.NewSourceSession()); err == nil {
		t.Fatal("another source consumed the request")
	}
	sessions.CloseSource(t.Context(), "source-b")
	if _, err := sessions.Start(t.Context(), "source-a", id, webview.InteractiveViewport{}, sourceexec.NewSourceSession()); err != nil {
		t.Fatalf("unrelated invalidation consumed the request: %v", err)
	}
	pending := sessions.Register("source-b", BrowserRequest{URL: "https://example.test/login"})
	sessions.CloseSource(t.Context(), "source-a")
	if _, err := sessions.Start(t.Context(), "source-b", pending, webview.InteractiveViewport{}, sourceexec.NewSourceSession()); err != nil {
		t.Fatalf("closing an active source erased another source's pending request: %v", err)
	}
	sessions.CloseSource(t.Context(), "")
}

func TestReplacementOwnsBrowserWhileOlderLaunchDrains(t *testing.T) {
	browser := &blockingBrowser{entered: make(chan struct{}), release: make(chan struct{})}
	sessions := NewBrowserSessions(browser)
	id := sessions.Register("source-a", BrowserRequest{URL: "https://example.test/login"})
	result := make(chan error, 1)
	go func() {
		_, err := sessions.Start(t.Context(), "source-a", id, webview.InteractiveViewport{}, sourceexec.NewSourceSession())
		result <- err
	}()
	<-browser.entered
	replacement := sessions.Register("source-b", BrowserRequest{URL: "https://example.test/replacement"})
	frame, err := sessions.Start(t.Context(), "source-b", replacement, webview.InteractiveViewport{}, sourceexec.NewSourceSession())
	close(browser.release)
	oldErr := <-result
	if err != nil || frame.SessionID != "replacement" || oldErr == nil {
		t.Fatalf("replacement=%+v error=%v old error=%v", frame, err, oldErr)
	}
	if _, err := sessions.owned("source-b", frame.SessionID); err != nil {
		t.Fatal("late launch displaced replacement")
	}
	if browser.closed != 1 {
		t.Fatalf("late worker cleanup count=%d", browser.closed)
	}
	sessions.CloseSource(t.Context(), "")
}
