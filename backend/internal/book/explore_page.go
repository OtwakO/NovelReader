// Explore page execution reuses bounded source transport and result parsing.
package book

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/otwako/novelreader/internal/analyzer"
	"github.com/otwako/novelreader/internal/sourceexec"
)

const maxExploreRetainedBooks = 2000

// GetExplorePage executes the expected page or replays the last successful one.
func (s *Searcher) GetExplorePage(ctx context.Context, request ExplorePageRequest) (pageResult ExplorePage, err error) {
	session, release := s.explore.acquire(request.SessionID)
	if session == nil {
		return ExplorePage{}, newExploreError("invalid_session", "session", "Explore session is missing or expired", false, nil)
	}
	defer release()
	session.mu.Lock()
	defer session.mu.Unlock()
	kind, ok := session.categories[request.CategoryID]
	if !ok || kind.Type != exploreKindURL || kind.URL == "" {
		return ExplorePage{}, newExploreError("invalid_category", "category", "Explore category is not selectable", false, nil)
	}
	state := session.pages[request.CategoryID]
	if state == nil {
		state = &explorePageState{next: 1, seen: make(map[string]bool)}
		session.pages[request.CategoryID] = state
	}
	if state.last != nil && request.Page == state.last.Page {
		return *state.last, nil
	}
	if state.last != nil && state.last.Exhausted {
		return ExplorePage{}, newExploreError("page_exhausted", "page", "Explore category has no more pages", false, nil)
	}
	if request.Page != state.next {
		err := newExploreError("page_conflict", "page", "Explore page is out of sequence", false, nil)
		err.ExpectedPage = state.next
		return ExplorePage{}, err
	}

	pageCtx, cancel := context.WithTimeout(ctx, s.exploreTimeout())
	defer cancel()
	if err := s.rateLimitWait(pageCtx, session.source); err != nil {
		return ExplorePage{}, newExploreError("rate_limit_cancelled", "capacity", "Explore request cancelled while rate-limited", true, err)
	}
	select {
	case s.searchSlots <- struct{}{}:
	case <-pageCtx.Done():
		return ExplorePage{}, newExploreError("capacity_cancelled", "capacity", "Explore request cancelled while waiting for capacity", true, pageCtx.Err())
	}
	s.capacity.activeSourceFetches.Add(1)
	s.capacity.totalSourceFetches.Add(1)
	defer func() {
		<-s.searchSlots
		s.capacity.activeSourceFetches.Add(-1)
		if err != nil {
			s.capacity.failedSources.Add(1)
		} else {
			s.capacity.completedSources.Add(1)
		}
	}()

	sourceHeaders, err := evaluateSourceHeaders(pageCtx, s.jsVM, session.source, session.state)
	if err != nil {
		return ExplorePage{}, newExploreError("request_build_failed", "request", "Could not build Explore source headers", false, err)
	}
	session.state.SetRequestHeaders(sourceHeaders)
	transport := s.newTransport(s.workflowClientWithTimeout(s.exploreTimeout()), session.state)
	defer transport.CloseIdleConnections()
	executor := sourceexec.NewExecutorWithSession(s.jsVM, transport, session.state)
	variables := make(map[string]string)
	executor.SetURLContext(&analyzer.URLContext{Source: session.source.ScriptData(), RuleData: variables, JSLib: session.source.JSLib})
	spec, err := executor.BuildContext(pageCtx, kind.URL, "", request.Page, session.source.BookSourceURL)
	if err != nil || spec.URL == "" {
		if err == nil {
			err = fmt.Errorf("empty URL")
		}
		return ExplorePage{}, newExploreError("request_build_failed", "request", "Could not build Explore request", false, err)
	}
	spec.Headers = sourceexec.MergeHeaders(sourceHeaders, spec.Headers)
	response, err := transport.Do(pageCtx, spec)
	if errors.Is(err, sourceexec.ErrWebViewTransportUnavailable) {
		return ExplorePage{}, newExploreError("unsupported_capability", "transport", "Explore source requires WebView support", false, err)
	}
	if err != nil {
		return ExplorePage{}, newExploreError("transport_failed", "transport", "Explore request failed", true, err)
	}
	response, err = executor.TransformResponse(pageCtx, spec, response)
	if err != nil {
		return ExplorePage{}, newExploreError("response_transform_failed", "response", "Explore response transform failed", true, err)
	}
	statusDiagnostic := ExploreDiagnostic{}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		statusDiagnostic = ExploreDiagnostic{
			Code: "http_status", Stage: "transport", Severity: "warning", Retryable: true,
			Message: fmt.Sprintf("Explore source returned HTTP status %d; rules evaluated the received response body", response.StatusCode),
		}
	}
	baseURL := response.FinalURL
	if baseURL == "" {
		baseURL = spec.URL
	}
	ruleJSON, reverse, err := exploreResultRules(session.source)
	if err != nil {
		return ExplorePage{}, newExploreError("result_rule_failed", "result", "Explore result rules are invalid", false, err)
	}
	books, err := s.parseDiscoveryResults(pageCtx, session.source, response.Body, ruleJSON, baseURL, session.state, discoveryParseOptions{allowEmpty: true, strictFields: true, variables: variables})
	if err != nil {
		return ExplorePage{}, newExploreError("result_rule_failed", "result", "Could not parse Explore results", false, err)
	}
	if reverse {
		for left, right := 0, len(books)-1; left < right; left, right = left+1, right-1 {
			books[left], books[right] = books[right], books[left]
		}
	}
	unique, truncated := retainExploreBooks(session, state, books)
	diagnostics := []ExploreDiagnostic{}
	if statusDiagnostic.Code != "" {
		diagnostics = append(diagnostics, statusDiagnostic)
	}
	if truncated {
		diagnostics = append(diagnostics, ExploreDiagnostic{
			Code: "result_truncated", Stage: "capacity", Severity: "warning", Retryable: false,
			Message: "Explore results were truncated at the session retained-result limit",
		})
	}
	pageResult = ExplorePage{
		SourceID: session.source.ID, SessionID: request.SessionID, CategoryID: request.CategoryID,
		Page: request.Page, NextPage: request.Page + 1, Books: unique, Exhausted: truncated || len(unique) == 0,
		Diagnostics: diagnostics,
	}
	state.next = pageResult.NextPage
	state.last = &pageResult
	return pageResult, nil
}

func retainExploreBooks(session *exploreSession, state *explorePageState, books []SearchResult) ([]SearchResult, bool) {
	remaining := maxExploreRetainedBooks - session.retainedBooks
	if remaining < 0 {
		remaining = 0
	}
	unique := make([]SearchResult, 0, min(len(books), remaining))
	for _, result := range books {
		if result.BookURL == "" || state.seen[result.BookURL] {
			continue
		}
		if len(unique) == remaining {
			return unique, true
		}
		state.seen[result.BookURL] = true
		session.retainedBooks++
		unique = append(unique, result)
	}
	return unique, false
}
