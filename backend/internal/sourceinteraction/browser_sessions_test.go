package sourceinteraction

import (
	"context"
	"sync"
	"testing"

	"github.com/otwako/novelreader/internal/sourceexec"
	"github.com/otwako/novelreader/internal/webview"
)

type browserFixture struct {
	mu         sync.Mutex
	closed     int
	returnHTML bool
}

func (b *browserFixture) StartInteractive(context.Context, string, string, webview.InteractiveViewport, *sourceexec.SourceSession) (webview.InteractiveFrame, error) {
	return webview.InteractiveFrame{SessionID: "worker-session"}, nil
}
func (b *browserFixture) InteractiveFrame(context.Context, string) (webview.InteractiveFrame, error) {
	return webview.InteractiveFrame{SessionID: "worker-session"}, nil
}
func (b *browserFixture) SendInteractiveInput(context.Context, string, webview.InteractiveInput) (webview.InteractiveFrame, error) {
	return webview.InteractiveFrame{SessionID: "worker-session"}, nil
}
func (b *browserFixture) CloseInteractive(_ context.Context, _, _ string, _ bool, returnHTML bool, _ *sourceexec.SourceSession) (webview.InteractiveCloseResult, error) {
	b.mu.Lock()
	b.closed++
	b.returnHTML = returnHTML
	b.mu.Unlock()
	return webview.InteractiveCloseResult{HTML: "<html></html>"}, nil
}

func TestBrowserSessionsConsumeOpaqueRequestAndCloseOnSourceInvalidation(t *testing.T) {
	browser := &browserFixture{}
	sessions := NewBrowserSessions(browser)
	requestID := sessions.Register(BrowserRequest{URL: "https://example.test/login", Title: "Login"})
	frame, err := sessions.Start(t.Context(), "source-a", requestID, webview.InteractiveViewport{}, sourceexec.NewSourceSession())
	if err != nil || frame.SessionID == "" {
		t.Fatalf("frame=%+v error=%v", frame, err)
	}
	if _, err := sessions.Start(t.Context(), "source-a", requestID, webview.InteractiveViewport{}, sourceexec.NewSourceSession()); err == nil {
		t.Fatal("browser request reference was reusable")
	}
	sessions.CloseSource(t.Context(), "source-a")
	if browser.closed != 1 {
		t.Fatalf("closed=%d", browser.closed)
	}
}

func TestBrowserSessionsRequestsHTMLOnlyForAwaitContinuation(t *testing.T) {
	browser := &browserFixture{}
	sessions := NewBrowserSessions(browser)
	continuation := &ActionContinuation{Request: ActionRequest{Revision: "revision", ActionID: "action-0"}}
	requestID := sessions.Register(BrowserRequest{URL: "https://example.test/settings", Continuation: continuation})
	frame, err := sessions.Start(t.Context(), "source-a", requestID, webview.InteractiveViewport{}, sourceexec.NewSourceSession())
	if err != nil {
		t.Fatal(err)
	}
	closed, err := sessions.Close(t.Context(), "source-a", frame.SessionID, true)
	if err != nil || !browser.returnHTML || closed.HTML == "" || closed.Continuation == nil {
		t.Fatalf("closed=%+v returnHTML=%v error=%v", closed, browser.returnHTML, err)
	}
}

func TestRegisterBrowserRequestsDoesNotExposeSourceURL(t *testing.T) {
	sessions := NewBrowserSessions(&browserFixture{})
	effects := RegisterBrowserRequests([]Effect{{Type: "browser_required", URL: "https://example.test/login", Title: "Login"}}, sessions)
	if effects[0].URL != "" || effects[0].BrowserRequestID == "" {
		t.Fatalf("effect=%+v", effects[0])
	}
}

func TestRegisterBrowserRequestsRegistersAwaitLaunches(t *testing.T) {
	sessions := NewBrowserSessions(&browserFixture{})
	effects := RegisterBrowserRequests([]Effect{{Type: "browser_required", URL: "https://example.test/register", Await: true}}, sessions)
	if effects[0].URL != "" || effects[0].BrowserRequestID == "" || !effects[0].Await {
		t.Fatalf("effect=%+v", effects[0])
	}
}

func TestBrowserSessionsAcceptsBoundedHTMLDataDocument(t *testing.T) {
	browser := &browserFixture{}
	sessions := NewBrowserSessions(browser)
	requestID := sessions.Register(BrowserRequest{URL: "data:text/html;base64,PGgxPlNldHRpbmdzPC9oMT4=", Title: "Settings"})
	if _, err := sessions.Start(t.Context(), "source-a", requestID, webview.InteractiveViewport{}, sourceexec.NewSourceSession()); err != nil {
		t.Fatal(err)
	}
}

func TestBrowserSessionsRejectsNonHTMLDataDocument(t *testing.T) {
	sessions := NewBrowserSessions(&browserFixture{})
	requestID := sessions.Register(BrowserRequest{URL: "data:text/javascript;base64,YWxlcnQoMSk="})
	if _, err := sessions.Start(t.Context(), "source-a", requestID, webview.InteractiveViewport{}, sourceexec.NewSourceSession()); err == nil {
		t.Fatal("expected non-HTML data URL to be rejected")
	}
}
