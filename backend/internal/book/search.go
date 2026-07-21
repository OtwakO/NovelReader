package book

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/otwako/novelreader/internal/analyzer"
	"github.com/otwako/novelreader/internal/booksource"
	"github.com/otwako/novelreader/internal/fetcher"
	"github.com/otwako/novelreader/internal/sourceexec"
)

const (
	defaultMaxConcurrentSearch       = 16               // small-container source fetches per search
	defaultMaxConcurrentGlobalSearch = 32               // small-container source fetches per process
	defaultMaxSessions               = 1024             // retained workflow sessions
	defaultSessionTTL                = 30 * time.Minute // idle workflow session expiry
	searchOverallTimeout             = 30 * time.Second // max time for the entire search
	perSourceTimeout                 = 10 * time.Second // max time per single source
	maxResultsPerSource              = 20               // max results returned per source
)

// SearcherLimits bounds process-local work and retained workflow state.
type SearcherLimits struct {
	ConcurrentPerSearch int
	ConcurrentGlobal    int
	MaxSessions         int
	SessionTTL          time.Duration
	WorkflowTimeout     time.Duration
}

// DefaultSearcherLimits returns conservative limits for a 2-vCPU/4-GB container.
func DefaultSearcherLimits() SearcherLimits {
	return SearcherLimits{
		ConcurrentPerSearch: defaultMaxConcurrentSearch,
		ConcurrentGlobal:    defaultMaxConcurrentGlobalSearch,
		MaxSessions:         defaultMaxSessions,
		SessionTTL:          defaultSessionTTL,
		WorkflowTimeout:     perSourceTimeout,
	}
}

// CapacityStats reports process-local Searcher work without exposing internal limiters.
type CapacityStats struct {
	ActiveSearches      int64
	ActiveSourceFetches int64
	TotalSearches       int64
	TotalSourceFetches  int64
	CompletedSources    int64
	FailedSources       int64
}

type capacityCounters struct {
	activeSearches      atomic.Int64
	activeSourceFetches atomic.Int64
	totalSearches       atomic.Int64
	totalSourceFetches  atomic.Int64
	completedSources    atomic.Int64
	failedSources       atomic.Int64
}

type sourceLister interface {
	ListEnabled() ([]booksource.BookSource, error)
}

// Searcher orchestrates search, book info, TOC, and content fetching.
type TransportFactory func(client *fetcher.Client, session *sourceexec.SourceSession) sourceexec.Transport
type WebViewTransportFactory func(session *sourceexec.SourceSession) sourceexec.Transport

type Searcher struct {
	fetcher                 *fetcher.Client
	transportFactory        TransportFactory
	webViewTransportFactory WebViewTransportFactory
	jsVM                    *analyzer.JSVM
	cache                   *analyzer.CacheManager
	sourceStore             sourceLister
	bookStore               *Store
	sessions                *sourceexec.SessionRegistry
	explore                 *exploreRegistry
	workflowTimeout         time.Duration
	concurrentPerSearch     int
	searchSlots             chan struct{}
	capacity                capacityCounters
	// per-source rate limiting (concurrentRate)
	rateMu     sync.Mutex
	lastAccess map[string]time.Time // keyed by BookSourceURL
}

func NewSearcher(
	hc *fetcher.Client,
	jsVM *analyzer.JSVM,
	cache *analyzer.CacheManager,
	sourceStore sourceLister,
	bookStore *Store,
) *Searcher {
	return NewSearcherWithLimits(hc, jsVM, cache, sourceStore, bookStore, DefaultSearcherLimits())
}

// NewSearcherWithLimits creates a Searcher with explicit process capacity bounds.
func NewSearcherWithLimits(
	hc *fetcher.Client,
	jsVM *analyzer.JSVM,
	cache *analyzer.CacheManager,
	sourceStore sourceLister,
	bookStore *Store,
	limits SearcherLimits,
) *Searcher {
	defaults := DefaultSearcherLimits()
	if limits.ConcurrentPerSearch < 1 {
		limits.ConcurrentPerSearch = defaults.ConcurrentPerSearch
	}
	if limits.ConcurrentGlobal < 1 {
		limits.ConcurrentGlobal = defaults.ConcurrentGlobal
	}
	if limits.MaxSessions < 1 {
		limits.MaxSessions = defaults.MaxSessions
	}
	if limits.SessionTTL <= 0 {
		limits.SessionTTL = defaults.SessionTTL
	}
	if limits.WorkflowTimeout <= 0 {
		limits.WorkflowTimeout = defaults.WorkflowTimeout
	}
	sharedFetcher := hc
	if hc != nil {
		sharedFetcher = hc.StatelessClone()
	}
	return &Searcher{
		fetcher:             sharedFetcher,
		jsVM:                jsVM,
		cache:               cache,
		sourceStore:         sourceStore,
		bookStore:           bookStore,
		sessions:            sourceexec.NewSessionRegistryWithLimits(limits.MaxSessions, limits.SessionTTL),
		explore:             newExploreRegistry(limits.MaxSessions, limits.SessionTTL),
		workflowTimeout:     limits.WorkflowTimeout,
		concurrentPerSearch: limits.ConcurrentPerSearch,
		searchSlots:         make(chan struct{}, limits.ConcurrentGlobal),
		rateMu:              sync.Mutex{},
		lastAccess:          make(map[string]time.Time),
	}
}

// SetTransportFactory injects normal transport policy without coupling book workflows to clients.
func (s *Searcher) SetTransportFactory(factory TransportFactory) { s.transportFactory = factory }

// SetWebViewTransportFactory injects the optional browser transport policy.
func (s *Searcher) SetWebViewTransportFactory(factory WebViewTransportFactory) {
	s.webViewTransportFactory = factory
}

// SetWorkflowTimeout changes the per-stage timeout; the default is ten seconds.
func (s *Searcher) SetWorkflowTimeout(timeout time.Duration) {
	if timeout > 0 {
		s.workflowTimeout = timeout
	}
}

func (s *Searcher) sourceTimeout() time.Duration {
	if s.workflowTimeout > 0 {
		return s.workflowTimeout
	}
	return perSourceTimeout
}

func (s *Searcher) workflowClient() *fetcher.Client {
	if s.fetcher != nil {
		return s.fetcher
	}
	return fetcher.NewInsecure(s.sourceTimeout())
}

func (s *Searcher) newTransport(client *fetcher.Client, session *sourceexec.SourceSession) *sourceexec.RoutingTransport {
	normal := sourceexec.Transport(sourceexec.NewHTTPTransportForSession(client, session))
	if s.transportFactory != nil {
		normal = s.transportFactory(client, session)
	}
	var browser sourceexec.Transport
	if s.webViewTransportFactory != nil {
		browser = s.webViewTransportFactory(session)
	}
	return sourceexec.NewRoutingTransport(normal, browser)
}

// CapacityStats returns a point-in-time view of search pressure.
func (s *Searcher) CapacityStats() CapacityStats {
	return CapacityStats{
		ActiveSearches: s.capacity.activeSearches.Load(), ActiveSourceFetches: s.capacity.activeSourceFetches.Load(),
		TotalSearches: s.capacity.totalSearches.Load(), TotalSourceFetches: s.capacity.totalSourceFetches.Load(),
		CompletedSources: s.capacity.completedSources.Load(), FailedSources: s.capacity.failedSources.Load(),
	}
}

// Search is the old synchronous aggregator. Kept for backward compat.
func (s *Searcher) Search(query string) ([]SearchResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), searchOverallTimeout)
	defer cancel()

	var mu sync.Mutex
	var all []SearchResult
	seen := make(map[string]bool)

	err := s.SearchStream(ctx, query, func(src booksource.BookSource, results []SearchResult, _ error) {
		mu.Lock()
		for _, r := range results {
			key := r.SourceURL + ":" + r.BookURL
			if !seen[key] {
				seen[key] = true
				all = append(all, r)
			}
		}
		mu.Unlock()
	})
	if all == nil {
		all = []SearchResult{}
	}
	return all, err
}

// SearchCallback is called for each source that completes, with its results.
type SearchCallback func(src booksource.BookSource, results []SearchResult, err error)

// SearchStream fans out across all sources, calling onResult for each as it completes.
// Context cancellation (client disconnect) aborts all in-flight requests.
func (s *Searcher) SearchStream(ctx context.Context, query string, onResult SearchCallback) error {
	s.capacity.activeSearches.Add(1)
	s.capacity.totalSearches.Add(1)
	defer s.capacity.activeSearches.Add(-1)

	candidates, err := s.searchCandidates()
	if err != nil {
		return err
	}
	return s.searchSources(ctx, query, candidates, s.concurrentPerSearch, onResult)
}

// searchCandidates returns enabled text-type sources with search capability.
func (s *Searcher) searchCandidates() ([]booksource.BookSource, error) {
	sources, err := s.sourceStore.ListEnabled()
	if err != nil {
		return nil, fmt.Errorf("search: list sources: %w", err)
	}
	var candidates []booksource.BookSource
	for _, src := range sources {
		if src.BookSourceType == 0 && src.SearchURL != "" && src.RuleSearch != "" {
			candidates = append(candidates, src)
		}
	}
	return candidates, nil
}

// rateLimitWait blocks cancellation-safely until the source permits a request.
func (s *Searcher) rateLimitWait(ctx context.Context, src booksource.BookSource) error {
	if src.ConcurrentRate == "" {
		return nil
	}
	var rateMs int
	if _, err := fmt.Sscanf(src.ConcurrentRate, "%d", &rateMs); err != nil || rateMs <= 0 {
		return nil
	}
	rate := time.Duration(rateMs) * time.Millisecond
	now := time.Now()
	s.rateMu.Lock()
	reserved := now
	if previous, ok := s.lastAccess[src.BookSourceURL]; ok {
		reserved = previous.Add(rate)
		if reserved.Before(now) {
			reserved = now
		}
	}
	s.lastAccess[src.BookSourceURL] = reserved
	s.rateMu.Unlock()
	wait := time.Until(reserved)
	if wait <= 0 {
		return nil
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// searchSource performs a single source search.
func (s *Searcher) searchSource(ctx context.Context, src booksource.BookSource, query string) ([]SearchResult, error) {
	srcCtx, cancel := context.WithTimeout(ctx, s.sourceTimeout())
	defer cancel()

	// Search owns one session/client pair so cookies and source variables cannot leak
	// between concurrent sources while remaining available to multi-stage rules.
	session := sourceexec.NewSourceSession()
	session.SetRequestHeaders(parseHeaderJSON(src.Header))
	transport := s.newTransport(s.workflowClient(), session)
	defer transport.CloseIdleConnections()
	executor := sourceexec.NewExecutorWithSession(s.jsVM, transport, session)
	spec, err := executor.BuildContext(srcCtx, src.SearchURL, query, 1, src.BookSourceURL)
	if err != nil || spec.URL == "" {
		if err == nil {
			err = fmt.Errorf("empty URL")
		}
		return nil, fmt.Errorf("build url: %w", err)
	}

	// Merge source-level headers first, then URL-option headers overlay.
	spec.Headers = sourceexec.MergeHeaders(parseHeaderJSON(src.Header), spec.Headers)

	slog.Debug("search: fetching source",
		"source", src.BookSourceName,
		"method", spec.Method,
		"url", spec.URL,
		"charset", spec.Charset)

	if err := s.rateLimitWait(srcCtx, src); err != nil {
		return nil, fmt.Errorf("rate limit: %w", err)
	}
	resp, err := transport.Do(srcCtx, spec)
	if err != nil {
		return nil, fmt.Errorf("fetch: %w", err)
	}
	resp, err = executor.TransformResponse(srcCtx, spec, resp)
	if err != nil {
		return nil, fmt.Errorf("fetch bodyJs: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch: status %d from %s", resp.StatusCode, src.BookSourceName)
	}

	if src.RuleSearch == "" {
		return nil, fmt.Errorf("source %q has no search rules", src.BookSourceName)
	}
	resultBaseURL := resp.FinalURL
	if resultBaseURL == "" {
		resultBaseURL = spec.URL
	}
	results, err := s.parseSearchResultWithRuleStateContextAtURL(srcCtx, src, resp.Body, src.RuleSearch, resultBaseURL, session)
	if err != nil {
		return nil, err
	}
	for i := range results {
		results[i].Score = scoreResult(query, results[i].Name)
	}
	return results, nil
}

// parseSearchResultWithRule parses search results using structured SearchRule JSON.
func (s *Searcher) parseSearchResultWithRule(src booksource.BookSource, html, ruleJSON string) ([]SearchResult, error) {
	return s.parseSearchResultWithRuleState(src, html, ruleJSON, nil)
}

func (s *Searcher) parseSearchResultWithRuleState(src booksource.BookSource, html, ruleJSON string, state analyzer.SourceState) ([]SearchResult, error) {
	return s.parseSearchResultWithRuleStateContext(context.Background(), src, html, ruleJSON, state)
}

func (s *Searcher) parseSearchResultWithRuleStateContext(ctx context.Context, src booksource.BookSource, html, ruleJSON string, state analyzer.SourceState) ([]SearchResult, error) {
	return s.parseSearchResultWithRuleStateContextAtURL(ctx, src, html, ruleJSON, src.BookSourceURL, state)
}

func (s *Searcher) parseSearchResultWithRuleStateContextAtURL(ctx context.Context, src booksource.BookSource, html, ruleJSON, baseURL string, state analyzer.SourceState) ([]SearchResult, error) {
	return s.parseSearchResultWithRuleStateContextAtURLLimit(ctx, src, html, ruleJSON, baseURL, state, maxResultsPerSource, false, false)
}

func (s *Searcher) parseSearchResultWithRuleStateContextAtURLLimit(ctx context.Context, src booksource.BookSource, html, ruleJSON, baseURL string, state analyzer.SourceState, limit int, allowEmpty, strictFields bool) ([]SearchResult, error) {
	if baseURL == "" {
		baseURL = src.BookSourceURL
	}
	rules := parseRuleJSON(ruleJSON)
	if rules == nil {
		return nil, fmt.Errorf("search: invalid rule JSON for %s", src.BookSourceName)
	}
	bookListRule := rules["bookList"]
	if bookListRule == "" {
		return nil, fmt.Errorf("search: no bookList rule for %s", src.BookSourceName)
	}

	an := analyzer.New(html, baseURL, s.jsVM, s.cache)
	an.SetJSLib(src.JSLib)
	an.SetSourceState(state)
	an.SetContext(ctx)
	elements, err := an.GetElements(bookListRule)
	if err != nil {
		if allowEmpty && errors.Is(err, analyzer.ErrNoElements) {
			return nil, nil
		}
		return nil, fmt.Errorf("search: bookList: %w", err)
	}

	// Search caps early; source-native Explore pages deliberately pass no cap.
	if limit > 0 && len(elements) > limit {
		elements = elements[:limit]
	}

	// Pre-extract non-empty field rules to skip rule-string parsing per element
	var fieldRules []struct {
		key  string
		rule string
	}
	for _, key := range []string{"name", "author", "bookUrl", "coverUrl", "intro", "kind", "lastChapter"} {
		if r := rules[key]; r != "" {
			fieldRules = append(fieldRules, struct {
				key  string
				rule string
			}{key, r})
		}
	}

	// pre-compile bookUrlPattern regex if the source provides one
	var urlRe *regexp.Regexp
	if src.BookURLPattern != "" {
		if re, err := regexp.Compile(src.BookURLPattern); err == nil {
			urlRe = re
		}
	}

	var results []SearchResult
	for _, el := range elements {
		elHTML := analyzer.ToString(el)
		elAn := analyzer.New(elHTML, baseURL, s.jsVM, s.cache)
		elAn.SetJSLib(src.JSLib)
		elAn.SetSourceState(state)
		elAn.SetContext(ctx)

		r := SearchResult{
			SourceURL:  src.BookSourceURL,
			SourceName: src.BookSourceName,
		}

		for _, f := range fieldRules {
			var value string
			var fieldErr error
			required := f.key == "name" || f.key == "bookUrl"
			if strictFields && required {
				value, fieldErr = elAn.GetStringStrict(f.rule)
			} else {
				value, fieldErr = elAn.GetString(f.rule)
			}
			if fieldErr != nil {
				if errors.Is(fieldErr, analyzer.ErrNoElements) {
					continue
				}
				if strictFields && required {
					return nil, fmt.Errorf("search: %s: %w", f.key, fieldErr)
				}
				continue
			}
			if value == "" {
				continue
			}
			switch f.key {
			case "name":
				r.Name = value
			case "author":
				r.Author = value
			case "bookUrl":
				r.BookURL = value
			case "coverUrl":
				r.CoverURL = value
			case "intro":
				r.Intro = value
			case "kind":
				r.Kind = value
			case "lastChapter":
				r.LastChapter = value
			}
		}

		if r.Name != "" && r.BookURL != "" {
			r.BookURL = resolveURL(r.BookURL, baseURL)
			r.CoverURL = resolveURL(r.CoverURL, baseURL)
			if urlRe != nil && !urlRe.MatchString(r.BookURL) {
				continue // bookUrl doesn't match source's urlPattern
			}
			results = append(results, r)
		}
	}
	return results, nil
}

// ParseSearchResult parses one raw search response using a source's ruleSearch.
func (s *Searcher) ParseSearchResult(src booksource.BookSource, html string) ([]SearchResult, error) {
	return s.parseSearchResultWithRuleStateContext(context.Background(), src, html, src.RuleSearch, nil)
}

// ParseSearchResultWithState parses one raw search response with source session state.
func (s *Searcher) ParseSearchResultWithState(src booksource.BookSource, html string, state analyzer.SourceState) ([]SearchResult, error) {
	return s.parseSearchResultWithRuleStateContext(context.Background(), src, html, src.RuleSearch, state)
}

// ParseSearchResultWithStateAtURL parses results against the response page URL.
func (s *Searcher) ParseSearchResultWithStateAtURL(src booksource.BookSource, html, baseURL string, state analyzer.SourceState) ([]SearchResult, error) {
	return s.parseSearchResultWithRuleStateContextAtURL(context.Background(), src, html, src.RuleSearch, baseURL, state)
}

// GetBookInfo fetches and parses book info using ruleBookInfo.
func (s *Searcher) GetBookInfo(src booksource.BookSource, bookURL string) (*Book, error) {
	ctx, cancel := context.WithTimeout(context.Background(), s.sourceTimeout())
	defer cancel()

	session := s.sessions.GetOrCreateBook(src.BookSourceURL, bookURL)
	session.SetRequestHeaders(parseHeaderJSON(src.Header))
	transport := s.newTransport(s.workflowClient(), session)
	executor := sourceexec.NewExecutorWithSession(s.jsVM, transport, session)
	b := &Book{
		SourceURL: src.BookSourceURL,
		BookURL:   bookURL,
		Origin:    src.BookSourceName,
	}
	bookData := bookContext(b, src)
	setExecutorContextWithBookData(executor, src, bookData, b, nil, nil, bookURL)
	spec, err := executor.BuildContext(ctx, bookURL, "", 1, src.BookSourceURL)
	if err != nil || spec.URL == "" {
		if err == nil {
			err = fmt.Errorf("empty URL")
		}
		return nil, fmt.Errorf("book info: build URL: %w", err)
	}
	spec.Headers = sourceexec.MergeHeaders(parseHeaderJSON(src.Header), spec.Headers)
	response, err := transport.Do(ctx, spec)
	if err != nil {
		return nil, fmt.Errorf("book info: fetch: %w", err)
	}
	response, err = executor.TransformResponse(ctx, spec, response)
	if err != nil {
		return nil, fmt.Errorf("book info: bodyJs: %w", err)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("book info: status %d from %s", response.StatusCode, src.BookSourceName)
	}
	baseURL := response.FinalURL
	if baseURL == "" {
		baseURL = spec.URL
	}
	parsed, err := s.parseBookInfoResponse(ctx, src, response.Body, baseURL, b, bookData, session)
	if err != nil {
		return nil, err
	}
	if parsed.TocURL == "" {
		slog.Debug("book info: tocUrl not extracted from ruleBookInfo"+
			" — chapter fetch will fallback to book detail page",
			"source", src.BookSourceName,
			"book", parsed.Name,
		)
	}
	return parsed, nil
}

// GetChapterList fetches and parses the declared TOC, using the book page when tocURL is empty.
func (s *Searcher) GetChapterList(src booksource.BookSource, bookURL, tocURL string) ([]Chapter, error) {
	return s.GetChapterListForBook(src, &Book{
		SourceURL: src.BookSourceURL,
		BookURL:   bookURL,
		Origin:    src.BookSourceName,
	}, tocURL)
}

// GetChapterListForBook parses a TOC with the complete stored book context.
func (s *Searcher) GetChapterListForBook(src booksource.BookSource, b *Book, tocURL string) ([]Chapter, error) {
	if b == nil {
		return nil, fmt.Errorf("chapter list: book is required")
	}
	bookURL := b.BookURL
	fetchURL := tocURL
	if fetchURL == "" {
		fetchURL = bookURL
	}
	ctx, cancel := context.WithTimeout(context.Background(), s.sourceTimeout())
	defer cancel()

	session := s.sessions.GetOrCreateBook(src.BookSourceURL, bookURL)
	session.SetRequestHeaders(parseHeaderJSON(src.Header))
	transport := s.newTransport(s.workflowClient(), session)
	executor := sourceexec.NewExecutorWithSession(s.jsVM, transport, session)
	bookData := bookContext(b, src)
	setExecutorContextWithBookData(executor, src, bookData, b, nil, nil, fetchURL)
	parser := &ChapterListParser{
		src:      src,
		jsVM:     s.jsVM,
		cache:    s.cache,
		state:    session,
		book:     b,
		bookData: bookData,
		ctx:      ctx,
		fetch: func(urlStr string) (string, string, error) {
			spec, err := executor.BuildContext(ctx, urlStr, "", 1, src.BookSourceURL)
			if err != nil {
				return "", "", err
			}
			spec.Headers = sourceexec.MergeHeaders(parseHeaderJSON(src.Header), spec.Headers)
			response, err := transport.Do(ctx, spec)
			if err != nil {
				return "", "", err
			}
			response, err = executor.TransformResponse(ctx, spec, response)
			if err != nil {
				return "", "", fmt.Errorf("bodyJs: %w", err)
			}
			if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
				return "", "", fmt.Errorf("status %d from %s", response.StatusCode, src.BookSourceName)
			}
			finalURL := response.FinalURL
			if finalURL == "" {
				finalURL = spec.URL
			}
			return response.Body, finalURL, nil
		},
	}

	chapters, err := parser.ParseChapterList(fetchURL, src.BookSourceURL)
	if err != nil {
		return nil, fmt.Errorf("chapter list: %w", err)
	}
	for _, chapter := range chapters {
		if chapter.URL != "" {
			s.sessions.AssociateChapter(src.BookSourceURL, bookURL, chapter.URL)
		}
	}

	return chapters, nil
}

// GetChapterContent keeps the legacy URL-only API for callers without stored context.
func (s *Searcher) GetChapterContent(src booksource.BookSource, chapterURL string) (string, string, error) {
	return s.GetChapterContentForBook(src, nil, &Chapter{URL: chapterURL}, nil)
}

// GetChapterContentForBook fetches content with the complete book/current/next context.
// Script JSON is considered only when the source declares no content rule.
func (s *Searcher) GetChapterContentForBook(src booksource.BookSource, b *Book, current, next *Chapter) (string, string, error) {
	if current == nil || current.URL == "" {
		return "", "", fmt.Errorf("content: current chapter is required")
	}
	chapterURL := current.URL
	ctx, cancel := context.WithTimeout(context.Background(), s.sourceTimeout())
	defer cancel()

	var session *sourceexec.SourceSession
	if b != nil && b.BookURL != "" {
		session = s.sessions.GetOrCreateBook(src.BookSourceURL, b.BookURL)
	} else {
		session = s.sessions.GetChapter(src.BookSourceURL, chapterURL)
	}
	if session == nil {
		session = sourceexec.NewSourceSession()
	}
	session.SetRequestHeaders(parseHeaderJSON(src.Header))
	transport := s.newTransport(s.workflowClient(), session)
	executor := sourceexec.NewExecutorWithSession(s.jsVM, transport, session)
	bookData := bookContext(b, src)
	setExecutorContextWithBookData(executor, src, bookData, b, current, next, chapterURL)
	spec, err := executor.BuildContext(ctx, chapterURL, "", 1, src.BookSourceURL)
	if err != nil || spec.URL == "" {
		if err == nil {
			err = fmt.Errorf("empty URL")
		}
		return "", "", fmt.Errorf("content: build URL: %w", err)
	}
	spec.Headers = sourceexec.MergeHeaders(parseHeaderJSON(src.Header), spec.Headers)
	response, err := transport.Do(ctx, spec)
	if err != nil {
		return "", "", fmt.Errorf("content: fetch: %w", err)
	}
	response, err = executor.TransformResponse(ctx, spec, response)
	if err != nil {
		return "", "", fmt.Errorf("content: bodyJs: %w", err)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return "", "", fmt.Errorf("content: status %d from %s", response.StatusCode, src.BookSourceName)
	}
	fullURL := response.FinalURL
	if fullURL == "" {
		fullURL = spec.URL
	}

	an := analyzer.New(response.Body, fullURL, s.jsVM, s.cache)
	an.SetContext(ctx)
	setAnalyzerContextWithBookData(an, src, session, bookData, b, current, next, fullURL)
	rules := parseRuleJSON(src.RuleContent)
	if rules == nil {
		return mustString(an, "body@text"), "", nil
	}

	chapterTitle := ""
	if titleRule := rules["title"]; titleRule != "" {
		chapterTitle = mustString(an, titleRule)
	}

	contentRule := rules["content"]
	subRule := rules["subContent"]
	extractPage := func(pageAnalyzer *analyzer.Analyzer, body, pageURL string) string {
		pageContent := ""
		if contentRule != "" {
			pageContent = mustString(pageAnalyzer, contentRule)
		} else {
			pageContent = mustString(pageAnalyzer, "body@text")
		}
		if pageContent == "" && contentRule == "" && strings.Contains(body, "<script") {
			if extracted := extractContentFromScriptJSON(body); extracted != "" {
				pageContent = extracted
				slog.Debug("content: extracted from script tag JSON fallback",
					"source", src.BookSourceName, "url", pageURL)
			}
		}
		if subRule != "" {
			if sub := mustString(pageAnalyzer, subRule); sub != "" {
				pageContent += "\n" + sub
			}
		}
		return pageContent
	}

	content := extractPage(an, response.Body, fullURL)
	nextRule := rules["nextContentUrl"]
	scheduledRequests := map[string]bool{fullURL: true}
	processedPages := map[string]bool{contentURLKey(fullURL): true}
	pendingURLs := make([]string, 0, 1)
	pagesFetched := 1

	// Legado follows one next URL as a sequential chain. Multiple URLs are
	// fetched once as a fixed page set; their pages do not recursively expand.
	enqueueCandidates := func(candidates []string) bool {
		for _, candidate := range candidates {
			candidate = resolveURL(strings.TrimSpace(candidate), fullURL)
			if candidate == "" || scheduledRequests[candidate] {
				continue
			}
			// Legado stops when a content-page link reaches the next TOC
			// chapter; otherwise a normal next-chapter link is mistaken for
			// another page of the current chapter.
			if next != nil && sameChapterURL(candidate, next.URL, fullURL) {
				return true
			}
			if next == nil && s.sessions.IsChapter(src.BookSourceURL, contentURLKey(candidate)) {
				return true
			}
			scheduledRequests[candidate] = true
			pendingURLs = append(pendingURLs, candidate)
		}
		return false
	}

	singleChain := false
	if nextRule != "" {
		nextURLs, err := an.GetStringList(nextRule)
		if err != nil && !errors.Is(err, analyzer.ErrNoListValues) {
			return "", "", newContentPaginationError("nextContentUrl", fullURL, fullURL, pagesFetched, err)
		}
		if err == nil {
			singleChain = len(nextURLs) == 1
			enqueueCandidates(nextURLs)
		}
	}

	for len(pendingURLs) > 0 {
		nextURL := pendingURLs[0]
		pendingURLs = pendingURLs[1:]
		nextSpec, err := executor.BuildContext(ctx, nextURL, "", 1, src.BookSourceURL)
		if err != nil {
			return "", "", newContentPaginationError("next page build", fullURL, nextURL, pagesFetched, err)
		}
		nextSpec.Headers = sourceexec.MergeHeaders(parseHeaderJSON(src.Header), nextSpec.Headers)
		slog.Debug("content: fetching next page", "source", src.BookSourceName, "url", nextSpec.URL, "method", nextSpec.Method)
		nextResponse, err := transport.Do(ctx, nextSpec)
		if err != nil {
			return "", "", newContentPaginationError("next page fetch", fullURL, nextURL, pagesFetched, err)
		}
		nextResponse, err = executor.TransformResponse(ctx, nextSpec, nextResponse)
		if err != nil {
			return "", "", newContentPaginationError("next page bodyJs", fullURL, nextURL, pagesFetched, err)
		}
		slog.Debug("content: next page response", "source", src.BookSourceName, "url", nextSpec.URL, "status", nextResponse.StatusCode, "finalURL", nextResponse.FinalURL)
		if nextResponse.StatusCode < http.StatusOK || nextResponse.StatusCode >= http.StatusMultipleChoices {
			return "", "", newContentPaginationError("next page status", fullURL, nextURL, pagesFetched, fmt.Errorf("status %d from %s", nextResponse.StatusCode, src.BookSourceName))
		}
		nextFullURL := nextResponse.FinalURL
		if nextFullURL == "" {
			nextFullURL = nextSpec.URL
		}
		if next != nil && sameChapterURL(nextFullURL, next.URL, fullURL) {
			// This queued request crossed the TOC boundary. Do not parse it,
			// but continue draining other fixed-set content pages.
			continue
		}
		resolvedKey := contentURLKey(nextFullURL)
		requestedKey := contentURLKey(nextURL)
		if processedPages[resolvedKey] && requestedKey != resolvedKey {
			continue
		}
		processedPages[resolvedKey] = true
		pagesFetched++
		nextAnalyzer := analyzer.New(nextResponse.Body, nextFullURL, s.jsVM, s.cache)
		nextAnalyzer.SetContext(ctx)
		setAnalyzerContextWithBookData(nextAnalyzer, src, session, bookData, b, current, next, nextFullURL)
		if pageContent := extractPage(nextAnalyzer, nextResponse.Body, nextFullURL); pageContent != "" {
			content += "\n" + pageContent
		}
		an = nextAnalyzer
		fullURL = nextFullURL

		if !singleChain || nextRule == "" {
			continue
		}
		nextURLs, err := an.GetStringList(nextRule)
		if err != nil {
			if errors.Is(err, analyzer.ErrNoListValues) {
				break
			}
			return "", "", newContentPaginationError("nextContentUrl", fullURL, fullURL, pagesFetched, err)
		}
		if len(nextURLs) == 0 {
			break
		}
		enqueueCandidates(nextURLs[:1])
	}
	return content, chapterTitle, nil
}

// extractContentFromScriptJSON attempts to extract chapter content from JSON
// embedded in <script> tags (common in Vue.js, React, and other SPA frameworks).
// It looks for common patterns like chapterContent, content, text, articleBody, etc.
func extractContentFromScriptJSON(html string) string {
	// Common JSON keys that typically contain chapter/article content
	contentKeys := []string{
		"chapterContent",
		"articleBody",
		"text",
		"content",
		"body",
	}

	// Find all <script> tags with (?s) for DOTALL mode (multi-line match)
	scriptRe := regexp.MustCompile(`(?s)<script[^>]*>(.*?)</script>`)
	matches := scriptRe.FindAllStringSubmatch(html, -1)
	slog.Debug("content: script tag fallback found", "scripts", len(matches))

	for i, match := range matches {
		if len(match) < 2 {
			continue
		}
		scriptContent := strings.TrimSpace(match[1])
		if len(scriptContent) < 100 {
			continue // skip small scripts (not JSON data)
		}

		// Try to parse as JSON
		var data interface{}
		if err := json.Unmarshal([]byte(scriptContent), &data); err != nil {
			slog.Debug("content: script tag not JSON", "idx", i, "len", len(scriptContent))
			continue
		}

		slog.Debug("content: valid JSON in script tag", "idx", i, "len", len(scriptContent))

		// Search recursively for content
		if result := findStringInJSON(data, contentKeys, 0); result != "" {
			slog.Debug("content: found in JSON", "idx", i, "len", len(result))
			return result
		}
	}

	return ""
}

// findStringInJSON recursively searches a JSON structure for a string value
// whose key matches one of the target keys, limiting search depth to avoid
// excessive recursion on large JSON blobs.
func findStringInJSON(data interface{}, targetKeys []string, depth int) string {
	if depth > 10 {
		return "" // limit recursion depth
	}

	switch v := data.(type) {
	case map[string]interface{}:
		for key, val := range v {
			for _, target := range targetKeys {
				if strings.EqualFold(key, target) {
					if str, ok := val.(string); ok && len(str) > 50 {
						return str // found a substantial content string
					}
				}
			}
			if result := findStringInJSON(val, targetKeys, depth+1); result != "" {
				return result
			}
		}
	case []interface{}:
		for _, item := range v {
			if result := findStringInJSON(item, targetKeys, depth+1); result != "" {
				return result
			}
		}
	}

	return ""
}

func mustString(a *analyzer.Analyzer, rule string) string {
	if rule == "" {
		return ""
	}
	s, err := a.GetString(rule)
	if err != nil {
		return ""
	}
	return s
}

func resolveURL(urlStr, baseURL string) string {
	if urlStr == "" {
		return ""
	}
	if strings.HasPrefix(urlStr, "http://") || strings.HasPrefix(urlStr, "https://") {
		return urlStr
	}
	// Resolve the URL portion separately so `{...}` request options remain
	// parseable by sourceexec after relative-link resolution.
	urlPart, optionSuffix := splitURLOptionSuffix(urlStr)
	// Use stdlib URL resolution: handles ../, //, and absolute-path (/foo) correctly.
	// e.g., base="https://site.com/novel/123" + url="/catalog/456" → "https://site.com/catalog/456"
	base, err := url.Parse(baseURL)
	if err != nil {
		return strings.TrimRight(baseURL, "/") + "/" + strings.TrimLeft(urlStr, "/")
	}
	ref, err := url.Parse(urlPart)
	if err != nil {
		return strings.TrimRight(baseURL, "/") + "/" + strings.TrimLeft(urlStr, "/")
	}
	return base.ResolveReference(ref).String() + optionSuffix
}

func splitURLOptionSuffix(value string) (string, string) {
	for i := len(value) - 2; i >= 0; i-- {
		if value[i] != ',' {
			continue
		}
		j := i + 1
		for j < len(value) && (value[j] == ' ' || value[j] == '\t' || value[j] == '\n' || value[j] == '\r') {
			j++
		}
		if j >= len(value) || value[j] != '{' {
			continue
		}
		depth := 1
		k := j + 1
		for k < len(value) && depth > 0 {
			switch value[k] {
			case '{':
				depth++
			case '}':
				depth--
			}
			k++
		}
		if depth == 0 && strings.TrimSpace(value[k:]) == "" &&
			(json.Valid([]byte(value[j:k])) || strings.Contains(value[j:k], "'")) {
			return value[:i], value[i:]
		}
	}
	return value, ""
}
