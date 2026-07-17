// Explore page execution reuses bounded source transport and result parsing.
package book

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/otwako/novelreader/internal/sourceexec"
)

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

	pageCtx, cancel := context.WithTimeout(ctx, s.sourceTimeout())
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

	session.state.SetRequestHeaders(parseHeaderJSON(session.source.Header))
	transport := s.newTransport(s.workflowClient(), session.state)
	defer transport.CloseIdleConnections()
	executor := sourceexec.NewExecutorWithSession(s.jsVM, transport, session.state)
	spec, err := executor.BuildContext(pageCtx, kind.URL, "", request.Page, session.source.BookSourceURL)
	if err != nil || spec.URL == "" {
		if err == nil {
			err = fmt.Errorf("empty URL")
		}
		return ExplorePage{}, newExploreError("request_build_failed", "request", "Could not build Explore request", false, err)
	}
	spec.Headers = sourceexec.MergeHeaders(parseHeaderJSON(session.source.Header), spec.Headers)
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
	if response.StatusCode != http.StatusOK {
		return ExplorePage{}, newExploreError("http_status", "transport", "Explore source returned an error status", true, fmt.Errorf("status %d", response.StatusCode))
	}
	baseURL := response.FinalURL
	if baseURL == "" {
		baseURL = spec.URL
	}
	ruleJSON, reverse, err := exploreResultRules(session.source)
	if err != nil {
		return ExplorePage{}, newExploreError("result_rule_failed", "result", "Explore result rules are invalid", false, err)
	}
	books, err := s.parseSearchResultWithRuleStateContextAtURLLimit(pageCtx, session.source, response.Body, ruleJSON, baseURL, session.state, 0, true)
	if err != nil {
		return ExplorePage{}, newExploreError("result_rule_failed", "result", "Could not parse Explore results", false, err)
	}
	if len(books) == 0 && session.source.BookURLPattern == "" {
		book := &Book{SourceURL: session.source.BookSourceURL, BookURL: baseURL, Origin: session.source.BookSourceName}
		book, detailErr := s.parseBookInfoResponse(pageCtx, session.source, response.Body, baseURL, book, nil, session.state)
		if detailErr != nil {
			return ExplorePage{}, newExploreError("result_rule_failed", "result", "Could not parse Explore detail fallback", false, detailErr)
		}
		if book.Name != "" {
			books = append(books, searchResultFromBook(book))
		}
	}
	if reverse {
		for left, right := 0, len(books)-1; left < right; left, right = left+1, right-1 {
			books[left], books[right] = books[right], books[left]
		}
	}
	unique := books[:0]
	for _, result := range books {
		if !state.seen[result.BookURL] {
			state.seen[result.BookURL] = true
			unique = append(unique, result)
		}
	}
	if unique == nil {
		unique = []SearchResult{}
	}
	pageResult = ExplorePage{
		SourceID: session.source.BookSourceURL, SessionID: request.SessionID, CategoryID: request.CategoryID,
		Page: request.Page, NextPage: request.Page + 1, Books: unique, Exhausted: len(unique) == 0,
		Diagnostics: []ExploreDiagnostic{},
	}
	state.next = pageResult.NextPage
	state.last = &pageResult
	return pageResult, nil
}

func searchResultFromBook(book *Book) SearchResult {
	return SearchResult{
		Name: book.Name, Author: book.Author, CoverURL: book.CoverURL, Intro: book.Intro, Kind: book.Kind,
		LastChapter: book.LastChapter, UpdateTime: book.UpdateTime, WordCount: book.WordCount,
		BookURL: book.BookURL, SourceURL: book.SourceURL, SourceName: book.Origin,
	}
}
