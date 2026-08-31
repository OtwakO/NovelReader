package sourceinteraction

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/otwako/novelreader/internal/sourceexec"
	"github.com/otwako/novelreader/internal/webview"
)

var ErrBrowserSessionNotFound = errors.New("sourceinteraction: browser session not found")

// Browser opens and controls worker-owned interactive contexts.
type Browser interface {
	StartInteractive(context.Context, string, string, webview.InteractiveViewport, *sourceexec.SourceSession) (webview.InteractiveFrame, error)
	InteractiveFrame(context.Context, string) (webview.InteractiveFrame, error)
	SendInteractiveInput(context.Context, string, webview.InteractiveInput) (webview.InteractiveFrame, error)
	CloseInteractive(context.Context, string, string, bool, *sourceexec.SourceSession) error
}

type BrowserRequest struct {
	URL   string
	Title string
}

type browserSession struct {
	workerID string
	sourceID string
	startURL string
	session  *sourceexec.SourceSession
}

// BrowserSessions owns all interactive contexts for one Reader runtime.
type BrowserSessions struct {
	browser Browser
	mu      sync.Mutex
	pending map[string]BrowserRequest
	session *browserSession
}

func NewBrowserSessions(browser Browser) *BrowserSessions {
	if browser == nil {
		return nil
	}
	return &BrowserSessions{browser: browser, pending: make(map[string]BrowserRequest)}
}

// Register stores a one-use source-emitted browser request and returns an opaque reference.
func (s *BrowserSessions) Register(request BrowserRequest) string {
	if s == nil {
		return ""
	}
	requestID := newBrowserRequestID()
	s.mu.Lock()
	clear(s.pending)
	s.pending[requestID] = request
	s.mu.Unlock()
	return requestID
}

// Start consumes a registered browser request and replaces any prior Reader browser session.
func (s *BrowserSessions) Start(ctx context.Context, sourceID, requestID string, viewport webview.InteractiveViewport, session *sourceexec.SourceSession) (webview.InteractiveFrame, error) {
	if s == nil || s.browser == nil {
		return webview.InteractiveFrame{}, fmt.Errorf("sourceinteraction: interactive browser is unavailable")
	}
	s.mu.Lock()
	request, ok := s.pending[requestID]
	delete(s.pending, requestID)
	s.mu.Unlock()
	if !ok {
		return webview.InteractiveFrame{}, fmt.Errorf("sourceinteraction: browser request not found")
	}
	if err := validateBrowserURL(request.URL); err != nil {
		return webview.InteractiveFrame{}, err
	}
	s.CloseSource(ctx, "")
	frame, err := s.browser.StartInteractive(ctx, request.URL, request.Title, viewport, session)
	if err != nil {
		return webview.InteractiveFrame{}, err
	}
	owned := &browserSession{workerID: frame.SessionID, sourceID: sourceID, startURL: request.URL, session: session}
	s.mu.Lock()
	if s.session != nil {
		s.mu.Unlock()
		_ = s.browser.CloseInteractive(context.WithoutCancel(ctx), frame.SessionID, request.URL, false, session)
		return webview.InteractiveFrame{}, fmt.Errorf("sourceinteraction: browser session already active")
	}
	s.session = owned
	s.mu.Unlock()
	return frame, nil
}

func (s *BrowserSessions) Frame(ctx context.Context, sourceID, sessionID string) (webview.InteractiveFrame, error) {
	owned, err := s.owned(sourceID, sessionID)
	if err != nil {
		return webview.InteractiveFrame{}, err
	}
	frame, err := s.browser.InteractiveFrame(ctx, owned.workerID)
	if err != nil {
		s.forget(owned)
	}
	return frame, err
}

func (s *BrowserSessions) Input(ctx context.Context, sourceID, sessionID string, input webview.InteractiveInput) (webview.InteractiveFrame, error) {
	owned, err := s.owned(sourceID, sessionID)
	if err != nil {
		return webview.InteractiveFrame{}, err
	}
	frame, err := s.browser.SendInteractiveInput(ctx, owned.workerID, input)
	if err != nil {
		s.forget(owned)
	}
	return frame, err
}

func (s *BrowserSessions) Close(ctx context.Context, sourceID, sessionID string, save bool) (*sourceexec.SourceSession, error) {
	owned, err := s.owned(sourceID, sessionID)
	if err != nil {
		return nil, err
	}
	s.forget(owned)
	return owned.session, s.browser.CloseInteractive(ctx, owned.workerID, owned.startURL, save, owned.session)
}

// CloseSource closes the active session when sourceID matches; an empty ID closes any session.
func (s *BrowserSessions) CloseSource(ctx context.Context, sourceID string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	owned := s.session
	if owned == nil || sourceID != "" && owned.sourceID != sourceID {
		s.mu.Unlock()
		return
	}
	s.session = nil
	s.pending = make(map[string]BrowserRequest)
	s.mu.Unlock()
	_ = s.browser.CloseInteractive(ctx, owned.workerID, owned.startURL, false, owned.session)
}

func (s *BrowserSessions) owned(sourceID, sessionID string) (*browserSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	owned := s.session
	if owned == nil || owned.sourceID != sourceID || owned.workerID != sessionID {
		return nil, ErrBrowserSessionNotFound
	}
	return owned, nil
}

func (s *BrowserSessions) forget(owned *browserSession) {
	s.mu.Lock()
	if s.session == owned {
		s.session = nil
	}
	s.mu.Unlock()
}

func newBrowserRequestID() string { return strings.ReplaceAll(uuid.NewString(), "-", "") }

func validateBrowserURL(rawURL string) error {
	trimmed := strings.TrimSpace(rawURL)
	if strings.HasPrefix(trimmed, "data:") {
		const prefix = "data:text/html;base64,"
		if !strings.HasPrefix(trimmed, prefix) {
			return fmt.Errorf("sourceinteraction: invalid browser URL")
		}
		body, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(trimmed, prefix))
		if err != nil || len(body) == 0 || len(body) > 512*1024 {
			return fmt.Errorf("sourceinteraction: invalid browser URL")
		}
		return nil
	}
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Host == "" || parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("sourceinteraction: invalid browser URL")
	}
	return nil
}
