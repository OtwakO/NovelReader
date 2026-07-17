// Explore controls update retained source state and refresh catalogs only when requested by source JavaScript.
package book

import (
	"context"
	"strings"

	"github.com/otwako/novelreader/internal/analyzer"
	"github.com/otwako/novelreader/internal/sourceexec"
)

const maxExploreControlValueBytes = 4096

const (
	exploreKindText   = "text"
	exploreKindButton = "button"
	exploreKindToggle = "toggle"
	exploreKindSelect = "select"
)

type ExploreControlRequest struct {
	SessionID string  `json:"sessionId"`
	ControlID string  `json:"controlId"`
	Value     *string `json:"value"`
}

// UpdateExploreControl applies one typed control update inside its Explore session.
func (s *Searcher) UpdateExploreControl(ctx context.Context, request ExploreControlRequest) (ExploreCatalog, error) {
	session, release := s.explore.acquire(request.SessionID)
	defer release()
	if session == nil {
		return ExploreCatalog{}, newExploreError("session_not_found", "session", "Explore session is unavailable", false, nil)
	}

	session.mu.Lock()
	defer session.mu.Unlock()
	kind, ok := session.categories[request.ControlID]
	if !ok {
		return ExploreCatalog{}, newExploreError("control_not_found", "control", "Explore control is unavailable", false, nil)
	}
	values := exploreInfoMap(session.state)
	typeName := strings.ToLower(strings.TrimSpace(kind.Type))
	switch typeName {
	case exploreKindText:
		if request.Value == nil || len(*request.Value) > maxExploreControlValueBytes {
			return ExploreCatalog{}, invalidExploreControlValue()
		}
		values[kind.Title] = *request.Value
	case exploreKindButton:
		if request.Value != nil {
			return ExploreCatalog{}, invalidExploreControlValue()
		}
	case exploreKindToggle, exploreKindSelect:
		if request.Value == nil || !containsExploreOption(kind, *request.Value) {
			return ExploreCatalog{}, invalidExploreControlValue()
		}
		values[kind.Title] = *request.Value
	case exploreKindURL:
		return ExploreCatalog{}, newExploreError("invalid_control_type", "control", "Explore entry is not a control", false, nil)
	default:
		return ExploreCatalog{}, newExploreError("unsupported_control_type", "control", "Explore control type is unsupported", false, nil)
	}

	session.state.PutMemory(analyzer.RefreshExploreMemoryKey, false)
	if strings.TrimSpace(kind.Action) != "" {
		if _, err := s.evaluateExploreJavaScript(ctx, session.source, session.state, kind.Action); err != nil {
			return ExploreCatalog{}, newExploreError("control_action_failed", "control", "Could not apply Explore control", true, err)
		}
	}
	refresh, _ := session.state.GetMemory(analyzer.RefreshExploreMemoryKey).(bool)
	session.state.PutMemory(analyzer.RefreshExploreMemoryKey, false)
	if refresh {
		kinds, err := s.refreshExploreKinds(ctx, session)
		if err != nil {
			return ExploreCatalog{}, err
		}
		initializeExploreControls(kinds, session.state)
		nextGeneration := session.generation + 1
		session.categories, session.entryIDs = exploreCategories(kinds, nextGeneration)
		session.generation = nextGeneration
		session.pages = make(map[string]*explorePageState)
	}
	return exploreCatalog(request.SessionID, session), nil
}

func (s *Searcher) refreshExploreKinds(ctx context.Context, session *exploreSession) ([]exploreKind, error) {
	raw := strings.TrimSpace(session.source.ExploreURL)
	var err error
	if strings.HasPrefix(strings.ToLower(raw), "@js:") || strings.HasPrefix(strings.ToLower(raw), "<js>") {
		raw, err = s.evaluateExploreScript(ctx, session.source, session.state, raw)
		if err != nil {
			return nil, newExploreError("category_script_failed", "category", "Could not refresh Explore categories", true, err)
		}
	}
	kinds, err := parseExploreKinds(raw)
	if err != nil {
		return nil, newExploreError("category_parse_failed", "category", "Could not refresh Explore categories", false, err)
	}
	return kinds, nil
}

func invalidExploreControlValue() error {
	return newExploreError("invalid_control_value", "control", "Explore control value is invalid", false, nil)
}

func containsExploreOption(kind exploreKind, value string) bool {
	for _, option := range kind.Chars {
		if option != nil && *option == value {
			return true
		}
	}
	return false
}

func initializeExploreControls(kinds []exploreKind, state *sourceexec.SourceSession) {
	values := exploreInfoMap(state)
	for _, kind := range kinds {
		typeName := strings.ToLower(strings.TrimSpace(kind.Type))
		if typeName != exploreKindToggle && typeName != exploreKindSelect {
			continue
		}
		if analyzer.ToString(values[kind.Title]) != "" {
			continue
		}
		if kind.Default != nil {
			values[kind.Title] = *kind.Default
			continue
		}
		for _, option := range kind.Chars {
			if option != nil {
				values[kind.Title] = *option
				break
			}
		}
	}
}

func exploreCatalog(sessionID string, session *exploreSession) ExploreCatalog {
	entries := make([]ExploreEntry, 0, len(session.entryIDs))
	values := exploreInfoMap(session.state)
	for _, id := range session.entryIDs {
		kind, ok := session.categories[id]
		if !ok {
			continue
		}
		options := make([]string, 0, len(kind.Chars))
		for _, option := range kind.Chars {
			if option != nil {
				options = append(options, *option)
			}
		}
		entry := ExploreEntry{
			ID: id, Title: kind.Title, Type: kind.Type,
			Selectable: kind.Type == exploreKindURL && kind.URL != "", Options: options,
		}
		switch strings.ToLower(strings.TrimSpace(kind.Type)) {
		case exploreKindText, exploreKindToggle, exploreKindSelect:
			entry.Value = analyzer.ToString(values[kind.Title])
		}
		entries = append(entries, entry)
	}
	return ExploreCatalog{Source: exploreSource(session.source), SessionID: sessionID, Entries: entries, Diagnostics: []ExploreDiagnostic{}}
}
