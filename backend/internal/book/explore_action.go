package book

import (
	"context"
	"strings"

	"github.com/otwako/novelreader/internal/analyzer"
)

type ExploreActionRequest struct {
	SessionID string `json:"sessionId"`
	EntryID   string `json:"entryId"`
}

type ExploreActionEffect struct {
	Type    string `json:"type"`
	Message string `json:"message,omitempty"`
	URL     string `json:"url,omitempty"`
	Title   string `json:"title,omitempty"`
	Await   bool   `json:"await,omitempty"`
}

type ExploreActionResult struct {
	SourceID string                `json:"sourceId"`
	Effects  []ExploreActionEffect `json:"effects"`
}

// ExecuteExploreAction runs an action-only Explore URL in its retained source session.
func (s *Searcher) ExecuteExploreAction(ctx context.Context, request ExploreActionRequest) (ExploreActionResult, error) {
	session, release := s.explore.acquire(request.SessionID)
	defer release()
	if session == nil {
		return ExploreActionResult{}, newExploreError("session_not_found", "session", "Explore session is unavailable", false, nil)
	}
	session.mu.Lock()
	defer session.mu.Unlock()
	kind, ok := session.categories[request.EntryID]
	if !ok || !isExploreScriptAction(kind.URL) {
		return ExploreActionResult{}, newExploreError("action_not_found", "action", "Explore action is unavailable", false, nil)
	}
	effects := []ExploreActionEffect{}
	bridge := &analyzer.JSBridge{
		Toast: func(value interface{}) {
			effects = append(effects, ExploreActionEffect{Type: "notice", Message: analyzer.ToString(value)})
		},
		LongToast: func(value interface{}) {
			effects = append(effects, ExploreActionEffect{Type: "notice", Message: analyzer.ToString(value)})
		},
		StartBrowser: func(rawURL, title string) {
			effects = append(effects, ExploreActionEffect{Type: "browser_required", URL: rawURL, Title: title})
		},
		StartBrowserAwait: func(rawURL, title string) (string, bool) {
			effects = append(effects, ExploreActionEffect{Type: "browser_required", URL: rawURL, Title: title, Await: true})
			return "", true
		},
		RefreshExplore: func() { effects = append(effects, ExploreActionEffect{Type: "refresh_explore"}) },
	}
	if _, err := s.evaluateExploreJavaScriptWithBridge(ctx, session.source, session.state, unwrapExploreAction(kind.URL), bridge); err != nil {
		return ExploreActionResult{}, newExploreError("action_failed", "action", "Could not execute Explore action", true, err)
	}
	return ExploreActionResult{SourceID: session.source.ID, Effects: effects}, nil
}

func isExploreScriptAction(raw string) bool {
	trimmed := strings.TrimSpace(raw)
	return strings.HasPrefix(trimmed, `{{`) && strings.HasSuffix(trimmed, `}}`)
}

func unwrapExploreAction(raw string) string {
	trimmed := strings.TrimSpace(raw)
	return strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(trimmed, `{{`), `}}`))
}
