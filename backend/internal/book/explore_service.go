// Explore service executes one source-native category at a time.
package book

import (
	"context"
	"strings"

	"github.com/otwako/novelreader/internal/booksource"
	"github.com/otwako/novelreader/internal/sourceexec"
)

type exploreSourceStore interface {
	ListExploreEnabled() ([]booksource.BookSource, error)
	GetExploreEnabledByID(string) (*booksource.BookSource, error)
}

type ExploreSource struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Group        string   `json:"group"`
	Capabilities []string `json:"capabilities,omitempty"`
}

type ExploreEntry struct {
	ID         string   `json:"id"`
	Title      string   `json:"title"`
	Type       string   `json:"type"`
	Selectable bool     `json:"selectable"`
	Actionable bool     `json:"actionable,omitempty"`
	Value      string   `json:"value,omitempty"`
	Options    []string `json:"options,omitempty"`
}

type ExploreDiagnostic struct {
	Code      string `json:"code"`
	Stage     string `json:"stage"`
	Severity  string `json:"severity"`
	Retryable bool   `json:"retryable"`
	Message   string `json:"message"`
}

type ExploreCatalog struct {
	Source      ExploreSource       `json:"source"`
	SessionID   string              `json:"sessionId"`
	Entries     []ExploreEntry      `json:"entries"`
	Diagnostics []ExploreDiagnostic `json:"diagnostics"`
}

type ExplorePageRequest struct {
	SessionID  string `json:"sessionId"`
	CategoryID string `json:"categoryId"`
	Page       int    `json:"page"`
}

type ExplorePage struct {
	SourceID    string              `json:"sourceId"`
	SessionID   string              `json:"sessionId"`
	CategoryID  string              `json:"categoryId"`
	Page        int                 `json:"page"`
	NextPage    int                 `json:"nextPage"`
	Books       []SearchResult      `json:"books"`
	Exhausted   bool                `json:"exhausted"`
	Diagnostics []ExploreDiagnostic `json:"diagnostics"`
}

type ExploreError struct {
	Code           string                  `json:"code"`
	Stage          string                  `json:"stage"`
	Classification sourceexec.FailureClass `json:"classification,omitempty"`
	Retryable      bool                    `json:"retryable"`
	ExpectedPage   int                     `json:"nextPage,omitempty"`
	Message        string                  `json:"message"`
	cause          error
}

func (e *ExploreError) Error() string {
	if e.cause != nil {
		return e.Message + ": " + e.cause.Error()
	}
	return e.Message
}

func (e *ExploreError) Unwrap() error { return e.cause }

func newExploreError(code, stage, message string, retryable bool, cause error, classification ...sourceexec.FailureClass) *ExploreError {
	err := &ExploreError{Code: code, Stage: stage, Retryable: retryable, Message: message, cause: cause}
	if len(classification) > 0 {
		err.Classification = sourceexec.ClassifyFailure(cause, classification[0])
	}
	return err
}

// ExploreSources returns sources eligible independently of normal search.
func (s *Searcher) ExploreSources() ([]ExploreSource, error) {
	store, ok := s.sourceStore.(exploreSourceStore)
	if !ok {
		return nil, newExploreError("source_store_unavailable", "source", "Explore source store unavailable", false, nil)
	}
	sources, err := store.ListExploreEnabled()
	if err != nil {
		return nil, newExploreError("source_list_failed", "source", "Could not list Explore sources", true, err)
	}
	result := make([]ExploreSource, 0, len(sources))
	for _, source := range sources {
		if source.EnabledExplore && strings.TrimSpace(source.ExploreURL) != "" {
			result = append(result, exploreSource(source))
		}
	}
	return result, nil
}

// OpenExplore parses one source's navigation and starts a bounded session.
func (s *Searcher) OpenExplore(ctx context.Context, sourceID string) (ExploreCatalog, error) {
	store, ok := s.sourceStore.(exploreSourceStore)
	if !ok {
		return ExploreCatalog{}, newExploreError("source_store_unavailable", "source", "Explore source store unavailable", false, nil)
	}
	source, err := store.GetExploreEnabledByID(sourceID)
	if err != nil {
		return ExploreCatalog{}, newExploreError("source_load_failed", "source", "Could not load Explore source", true, err)
	}
	if source == nil || !source.EnabledExplore || strings.TrimSpace(source.ExploreURL) == "" {
		return ExploreCatalog{}, newExploreError("source_unavailable", "source", "Explore source is unavailable", false, nil)
	}
	raw := strings.TrimSpace(source.ExploreURL)
	state := sourceexec.NewSourceSession()
	if err := s.prepareSourceSession(ctx, *source, state); err != nil {
		return ExploreCatalog{}, newExploreError("source_setup_required", "source", "Explore source setup is required", false, err)
	}
	if strings.HasPrefix(strings.ToLower(raw), "@js:") || strings.HasPrefix(strings.ToLower(raw), "<js>") {
		raw, err = s.evaluateExploreScript(ctx, *source, state, raw)
		if err != nil {
			return ExploreCatalog{}, newExploreError("category_script_failed", "category", "Could not generate Explore categories", true, err, sourceexec.FailureJavaScriptRuntime)
		}
	}
	kinds, err := parseExploreKinds(raw)
	if err != nil {
		return ExploreCatalog{}, newExploreError("category_parse_failed", "category", "Could not parse Explore categories", false, err, sourceexec.FailureInvalidResult)
	}
	select {
	case <-ctx.Done():
		return ExploreCatalog{}, newExploreError("category_cancelled", "category", "Explore category loading cancelled", true, ctx.Err())
	default:
	}
	initializeExploreControls(kinds, state)
	sessionID, session, err := s.explore.create(*source, kinds, state)
	if err != nil {
		return ExploreCatalog{}, newExploreError("session_create_failed", "session", "Could not start Explore session", true, err)
	}
	return exploreCatalog(sessionID, session), nil
}

func exploreSource(source booksource.BookSource) ExploreSource {
	return ExploreSource{ID: source.ID, Name: source.BookSourceName, Group: source.BookSourceGroup, Capabilities: booksource.CapabilityTags(source)}
}
