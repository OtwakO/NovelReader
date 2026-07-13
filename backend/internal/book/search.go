package book

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/otwako/novelreader/internal/analyzer"
	"github.com/otwako/novelreader/internal/booksource"
	"github.com/otwako/novelreader/internal/fetcher"
	"github.com/otwako/novelreader/internal/sourceexec"
)

const (
	maxConcurrentSearch  = 50               // max concurrent HTTP fetches for search
	searchOverallTimeout = 30 * time.Second // max time for the entire search
	perSourceTimeout     = 10 * time.Second // max time per single source
	maxResultsPerSource  = 20               // max results returned per source
)

// Searcher orchestrates search, book info, TOC, and content fetching.
type TransportFactory func(client *fetcher.Client, session *sourceexec.SourceSession) sourceexec.Transport

type Searcher struct {
	fetcher          *fetcher.Client
	transportFactory TransportFactory
	jsVM             *analyzer.JSVM
	cache            *analyzer.CacheManager
	sourceStore      *booksource.Store
	bookStore        *Store
	sessions         *sourceexec.SessionRegistry
	// per-source rate limiting (concurrentRate)
	rateMu     sync.Mutex
	lastAccess map[string]time.Time // keyed by BookSourceURL
}

func NewSearcher(
	hc *fetcher.Client,
	jsVM *analyzer.JSVM,
	cache *analyzer.CacheManager,
	sourceStore *booksource.Store,
	bookStore *Store,
) *Searcher {
	return &Searcher{
		fetcher:     hc,
		jsVM:        jsVM,
		cache:       cache,
		sourceStore: sourceStore,
		bookStore:   bookStore,
		sessions:    sourceexec.NewSessionRegistry(),
		rateMu:      sync.Mutex{},
		lastAccess:  make(map[string]time.Time),
	}
}

// SetTransportFactory injects transport policy without coupling book workflows to clients.
func (s *Searcher) SetTransportFactory(factory TransportFactory) { s.transportFactory = factory }

func (s *Searcher) newTransport(client *fetcher.Client, session *sourceexec.SourceSession) sourceexec.Transport {
	if s.transportFactory != nil {
		return s.transportFactory(client, session)
	}
	return sourceexec.NewHTTPTransportForSession(client, session)
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
	candidates, err := s.searchCandidates()
	if err != nil {
		return err
	}
	if len(candidates) == 0 {
		return nil
	}

	type jobResult struct {
		src     booksource.BookSource
		results []SearchResult
		err     error
	}

	ch := make(chan jobResult, len(candidates))
	var wg sync.WaitGroup
	sem := make(chan struct{}, maxConcurrentSearch)

	slog.Info("search: starting fan-out",
		"query", query, "sources", len(candidates), "concurrent", maxConcurrentSearch)

	for _, src := range candidates {
		wg.Add(1)
		sem <- struct{}{}
		go func(src booksource.BookSource) {
			defer wg.Done()
			defer func() { <-sem }()
			// Recover from panics in individual source search to avoid killing the process
			defer func() {
				if rec := recover(); rec != nil {
					slog.Error("search: panic in source goroutine",
						"source", src.BookSourceName,
						"panic", fmt.Sprintf("%v", rec))
					ch <- jobResult{src, nil, fmt.Errorf("panic: %v", rec)}
				}
			}()
			r, err := s.searchSource(ctx, src, query)
			ch <- jobResult{src, r, err}
		}(src)
	}

	go func() { wg.Wait(); close(ch) }()

	successCount := 0
	errorCount := 0
	// Error type categorization for debugging
	type errCat struct {
		count   int
		example string
	}
	errCats := make(map[string]*errCat)
	for r := range ch {
		if ctx.Err() != nil {
			break
		}
		if r.err != nil {
			errorCount++
			errStr := r.err.Error()
			// Categorize by error prefix
			cat := "other"
			switch {
			case strings.Contains(errStr, "status 503"), strings.Contains(errStr, "status 403"):
				cat = "blocked (503/403)"
			case strings.Contains(errStr, "status "):
				cat = "non-200 status"
			case strings.Contains(errStr, "timeout"), strings.Contains(errStr, "Timeout"):
				cat = "timeout"
			case strings.Contains(errStr, "no such host"), strings.Contains(errStr, "DNS"):
				cat = "dns"
			case strings.Contains(errStr, "TLS"), strings.Contains(errStr, "certificate"):
				cat = "tls"
			case strings.Contains(errStr, "connection refused"), strings.Contains(errStr, "no route to host"):
				cat = "connection refused"
			case strings.Contains(errStr, "WebView"), strings.Contains(errStr, "webView"):
				cat = "needs JS (webView)"
			case strings.Contains(errStr, "no elements matched"):
				cat = "empty results (0 books)"
			}
			if _, ok := errCats[cat]; !ok {
				errCats[cat] = &errCat{example: errStr}
			}
			errCats[cat].count++
			slog.Info("search: source failed",
				"source", r.src.BookSourceName, "cat", cat, "err", errStr[:min(len(errStr), 120)])
		} else {
			successCount++
			slog.Debug("search: source completed",
				"source", r.src.BookSourceName, "results", len(r.results))
		}
		onResult(r.src, r.results, r.err)
	}

	// Build error summary
	errSummary := ""
	for cat, info := range errCats {
		if errSummary != "" {
			errSummary += ", "
		}
		errSummary += fmt.Sprintf("%s=%d", cat, info.count)
	}

	slog.Info("search: finished",
		"query", query,
		"success", successCount,
		"errors", errorCount,
		"breakdown", errSummary,
		"total_sources", len(candidates),
		"cancelled", ctx.Err() != nil)
	return ctx.Err()
}

// searchCandidates returns enabled text-type sources with search capability.
// ponytail: src.ConcurrentRate (1% coverage) is stored but not enforced yet.
// Legado uses it as millis-between-requests throttle. Add per-source semaphore
// when sources with explicit rate limits produce measurable failures.
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

// rateLimitWait blocks until the per-source rate limit allows the next request.
// legado's concurrentRate is in milliseconds between requests.
func (s *Searcher) rateLimitWait(src booksource.BookSource) {
	if src.ConcurrentRate == "" {
		return // no rate limit configured, use system default
	}
	// Parse the rate in milliseconds (e.g. "2000" = 2s between requests)
	var rateMs int
	if _, err := fmt.Sscanf(src.ConcurrentRate, "%d", &rateMs); err != nil || rateMs <= 0 {
		return
	}
	s.rateMu.Lock()
	last, ok := s.lastAccess[src.BookSourceURL]
	now := time.Now()
	s.lastAccess[src.BookSourceURL] = now
	s.rateMu.Unlock()
	if ok {
		elapsed := now.Sub(last)
		wait := time.Duration(rateMs)*time.Millisecond - elapsed
		if wait > 0 {
			time.Sleep(wait)
		}
	}
}

// searchSource performs a single source search.
func (s *Searcher) searchSource(ctx context.Context, src booksource.BookSource, query string) ([]SearchResult, error) {
	srcCtx, cancel := context.WithTimeout(ctx, perSourceTimeout)
	defer cancel()

	// Search owns one session/client pair so cookies and source variables cannot leak
	// between concurrent sources while remaining available to multi-stage rules.
	session := sourceexec.NewSourceSession()
	session.SetRequestHeaders(parseHeaderJSON(src.Header))
	transport := s.newTransport(fetcher.NewInsecure(perSourceTimeout), session)
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

	if spec.WebView {
		slog.Warn("search: source needs WebView (JS rendering), skipping",
			"source", src.BookSourceName)
		return nil, fmt.Errorf("source requires JS rendering (webView:true)")
	}

	s.rateLimitWait(src)
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
		return nil, fmt.Errorf("search: bookList: %w", err)
	}

	// Cap early — don't waste parse work on discarded elements
	if len(elements) > maxResultsPerSource {
		elements = elements[:maxResultsPerSource]
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
			if v := mustString(elAn, f.rule); v != "" {
				switch f.key {
				case "name":
					r.Name = v
				case "author":
					r.Author = v
				case "bookUrl":
					r.BookURL = v
				case "coverUrl":
					r.CoverURL = v
				case "intro":
					r.Intro = v
				case "kind":
					r.Kind = v
				case "lastChapter":
					r.LastChapter = v
				}
			}
		}

		if r.Name != "" && r.BookURL != "" {
			if urlRe != nil && !urlRe.MatchString(r.BookURL) {
				continue // bookUrl doesn't match source's urlPattern
			}
			results = append(results, r)
		}
	}
	for i := range results {
		results[i].BookURL = resolveURL(results[i].BookURL, baseURL)
		results[i].CoverURL = resolveURL(results[i].CoverURL, baseURL)
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
	ctx, cancel := context.WithTimeout(context.Background(), perSourceTimeout)
	defer cancel()

	session := s.sessions.GetOrCreateBook(src.BookSourceURL, bookURL)
	session.SetRequestHeaders(parseHeaderJSON(src.Header))
	transport := s.newTransport(fetcher.NewInsecure(perSourceTimeout), session)
	executor := sourceexec.NewExecutorWithSession(s.jsVM, transport, session)
	spec, err := executor.BuildContext(ctx, bookURL, "", 1, src.BookSourceURL)
	if err != nil || spec.URL == "" {
		if err == nil {
			err = fmt.Errorf("empty URL")
		}
		return nil, fmt.Errorf("book info: build URL: %w", err)
	}
	spec.Headers = sourceexec.MergeHeaders(parseHeaderJSON(src.Header), spec.Headers)
	if spec.WebView {
		return nil, fmt.Errorf("book info: source requires JS rendering (webView:true)")
	}
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
	an := analyzer.New(response.Body, baseURL, s.jsVM, s.cache)
	an.SetJSLib(src.JSLib)
	an.SetSourceState(session)
	an.SetContext(ctx)

	// Set book context for JS rules that reference book.name, book.author, etc.
	an.SetBookData(map[string]string{
		"bookUrl":    bookURL,
		"origin":     src.BookSourceURL,
		"originName": src.BookSourceName,
	})

	rules := parseRuleJSON(src.RuleBookInfo)
	b := &Book{
		SourceURL: src.BookSourceURL,
		BookURL:   bookURL,
		Origin:    src.BookSourceName,
	}

	if rules != nil {
		b.Name = mustString(an, rules["name"])
		b.Author = mustString(an, rules["author"])
		b.CoverURL = resolveURL(mustString(an, rules["coverUrl"]), baseURL)
		b.Intro = mustString(an, rules["intro"])
		b.Kind = mustString(an, rules["kind"])
		b.LastChapter = mustString(an, rules["lastChapter"])
		b.UpdateTime = mustString(an, rules["updateTime"])
		b.WordCount = mustString(an, rules["wordCount"])

		// resolve tocUrl against bookUrl, not source root
		if tocURL := mustString(an, rules["tocUrl"]); tocURL != "" {
			b.TocURL = resolveURL(tocURL, baseURL)
		}
	}

	if b.TocURL == "" {
		slog.Debug("book info: tocUrl not extracted from ruleBookInfo"+
			" — chapter fetch will fallback to book detail page",
			"source", src.BookSourceName,
			"book", b.Name,
		)
	}
	return b, nil
}

// GetChapterList fetches and parses the TOC using the legado-compatible parser.
// If tocURL is empty, it attempts to auto-detect a TOC page link from the book detail page
// by scanning for common TOC URL patterns when the extracted chapters look invalid.
func (s *Searcher) GetChapterList(src booksource.BookSource, bookURL, tocURL string) ([]Chapter, error) {
	fetchURL := tocURL
	if fetchURL == "" {
		fetchURL = bookURL
	}
	ctx, cancel := context.WithTimeout(context.Background(), perSourceTimeout)
	defer cancel()

	session := s.sessions.GetOrCreateBook(src.BookSourceURL, bookURL)
	session.SetRequestHeaders(parseHeaderJSON(src.Header))
	transport := s.newTransport(fetcher.NewInsecure(perSourceTimeout), session)
	executor := sourceexec.NewExecutorWithSession(s.jsVM, transport, session)
	parser := &ChapterListParser{
		src:   src,
		jsVM:  s.jsVM,
		cache: s.cache,
		state: session,
		ctx:   ctx,
		fetch: func(urlStr string) (string, string, error) {
			spec, err := executor.BuildContext(ctx, urlStr, "", 1, src.BookSourceURL)
			if err != nil {
				return "", "", err
			}
			spec.Headers = sourceexec.MergeHeaders(parseHeaderJSON(src.Header), spec.Headers)
			if spec.WebView {
				return "", "", fmt.Errorf("source requires JS rendering (webView:true)")
			}
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

	// Auto-detect TOC page if chapters look invalid (e.g., URLs are book URLs instead of chapter URLs)
	if len(chapters) > 0 && tocURL == "" && !looksLikeChapters(chapters, bookURL) {
		slog.Debug("chapter list: extracted chapters look invalid, attempting TOC auto-detection",
			"source", src.BookSourceName,
			"bookURL", bookURL,
			"extractedCount", len(chapters),
		)

		// Try to find a TOC link on the book detail page
		if detectedTOC := detectTOCPage(fetchURL, src.BookSourceURL, s.fetcher, parseHeaderJSON(src.Header)); detectedTOC != "" {
			slog.Info("chapter list: auto-detected TOC page",
				"source", src.BookSourceName,
				"tocURL", detectedTOC,
			)

			// Re-parse with the detected TOC page
			chapters, err = parser.ParseChapterList(detectedTOC, src.BookSourceURL)
			if err != nil {
				slog.Warn("chapter list: TOC auto-detection failed, falling back to book page",
					"source", src.BookSourceName,
					"error", err.Error(),
				)
				// Fall back to original chapters
				chapters, _ = parser.ParseChapterList(fetchURL, src.BookSourceURL)
			}
		}
	}
	return chapters, nil
}

// looksLikeChapters validates whether extracted chapters look like actual chapter data.
// Returns false if most URLs look like book detail pages instead of chapter pages.
func looksLikeChapters(chapters []Chapter, bookURL string) bool {
	if len(chapters) == 0 {
		return false
	}

	// Count chapters with suspicious URLs (look like book detail pages)
	suspiciousCount := 0
	validChapterCount := 0

	for _, ch := range chapters {
		if ch.URL == "" {
			continue // volume headers are OK
		}

		// Check if URL contains book URL patterns (suggesting it's a book page, not a chapter)
		if strings.Contains(ch.URL, "/book/") || strings.Contains(ch.URL, "/shu/") || strings.Contains(ch.URL, "/info/") {
			suspiciousCount++
		}

		// Check if URL looks like a valid chapter URL
		if strings.Contains(ch.URL, "/chapter/") ||
			strings.Contains(ch.URL, "/read/") ||
			strings.Contains(ch.URL, "/content/") ||
			strings.Contains(ch.URL, "/c/") ||
			strings.Contains(ch.URL, "/ch/") {
			validChapterCount++
		}
	}

	// Count non-volume chapters
	nonVolumeCount := 0
	for _, ch := range chapters {
		if ch.URL != "" && !ch.IsVolume {
			nonVolumeCount++
		}
	}

	if nonVolumeCount == 0 {
		return false // all volumes, no actual chapters
	}

	// If more than 50% have book-like URLs, it's likely wrong
	if float64(suspiciousCount)/float64(nonVolumeCount) >= 0.5 {
		return false
	}

	// If we have at least some valid chapter URLs, it's probably correct
	if validChapterCount > 0 && float64(validChapterCount)/float64(nonVolumeCount) >= 0.5 {
		return true
	}

	// Default: assume correct if not obviously wrong
	return true
}

// detectTOCPage scans the book detail page for links to a separate TOC page.
// Returns the detected TOC URL or empty string if not found.
func detectTOCPage(bookPageURL, sourceURL string, fetcher *fetcher.Client, headers map[string]string) string {
	// Resolve book page URL to absolute URL
	fullURL := resolveURL(bookPageURL, sourceURL)

	resp, err := fetcher.Get(fullURL, headers)
	if err != nil {
		return ""
	}

	// Common TOC URL patterns (case-insensitive)
	tocPatterns := []string{
		`href="([^"]*(?:chapterlist|mulu|catalog|directory|chapter-list|目录)[^"]*)"`,
		`href="([^"]*(?:/chapter/|/mulu/|/catalog/)[^"]*)"`,
		`href="([^"]*(?:作品目录|章节目录|全部章节|目录)[^"]*)"`,
		`>([^<]*(?:作品目录|章节目录|全部章节|目录)[^<]*)<`,
	}

	baseURL := resp.Body
	for _, pattern := range tocPatterns {
		re := regexp.MustCompile(`(?i)` + pattern)
		matches := re.FindAllStringSubmatch(baseURL, -1)
		for _, match := range matches {
			if len(match) >= 2 {
				candidate := match[1]
				// Resolve relative URL
				resolved := resolveURL(candidate, fullURL)
				// Skip if it's the same as the book page
				if resolved != fullURL && strings.HasPrefix(resolved, "http") {
					return resolved
				}
			}
		}
	}

	return ""
}

// GetChapterContent fetches and parses chapter content using ruleContent.
// If the standard rule extraction returns empty content, it attempts to
// find content embedded as JSON in <script> tags (common for Vue.js/React SPAs).
func (s *Searcher) GetChapterContent(src booksource.BookSource, chapterURL string) (string, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), perSourceTimeout)
	defer cancel()

	session := s.sessions.GetChapter(src.BookSourceURL, chapterURL)
	if session == nil {
		session = sourceexec.NewSourceSession()
	}
	session.SetRequestHeaders(parseHeaderJSON(src.Header))
	transport := s.newTransport(fetcher.NewInsecure(perSourceTimeout), session)
	executor := sourceexec.NewExecutorWithSession(s.jsVM, transport, session)
	spec, err := executor.BuildContext(ctx, chapterURL, "", 1, src.BookSourceURL)
	if err != nil || spec.URL == "" {
		if err == nil {
			err = fmt.Errorf("empty URL")
		}
		return "", "", fmt.Errorf("content: build URL: %w", err)
	}
	spec.Headers = sourceexec.MergeHeaders(parseHeaderJSON(src.Header), spec.Headers)
	if spec.WebView {
		return "", "", fmt.Errorf("content: source requires JS rendering (webView:true)")
	}
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
	an.SetJSLib(src.JSLib)
	an.SetSourceState(session)
	an.SetContext(ctx)
	// Bind chapter context for JS rules that reference chapter.url, chapter.baseUrl
	an.SetChapterData(map[string]string{
		"url":     fullURL,
		"baseUrl": fullURL,
	})
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
		if pageContent == "" && strings.Contains(body, "<script") {
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
	visited := map[string]bool{fullURL: true}
	for nextRule := rules["nextContentUrl"]; nextRule != ""; {
		nextURLs, err := an.GetStringList(nextRule)
		if err != nil || len(nextURLs) == 0 {
			break
		}
		var nextURL string
		for _, candidate := range nextURLs {
			candidate = resolveURL(candidate, fullURL)
			if candidate == "" || visited[candidate] {
				continue
			}
			// Legado stops when a content-page link reaches the next TOC
			// chapter; otherwise a normal next-chapter link is mistaken for
			// another page of the current chapter.
			if s.sessions.IsChapter(src.BookSourceURL, candidate) {
				break
			}
			nextURL = candidate
			break
		}
		if nextURL == "" {
			break
		}
		visited[nextURL] = true

		nextSpec, err := executor.BuildContext(ctx, nextURL, "", 1, src.BookSourceURL)
		if err != nil {
			return "", "", fmt.Errorf("content: next page build: %w", err)
		}
		nextSpec.Headers = sourceexec.MergeHeaders(parseHeaderJSON(src.Header), nextSpec.Headers)
		if nextSpec.WebView {
			return "", "", fmt.Errorf("content: next page requires JS rendering (webView:true)")
		}
		slog.Debug("content: fetching next page", "source", src.BookSourceName, "url", nextSpec.URL, "method", nextSpec.Method)
		nextResponse, err := transport.Do(ctx, nextSpec)
		if err != nil {
			return "", "", fmt.Errorf("content: next page fetch: %w", err)
		}
		nextResponse, err = executor.TransformResponse(ctx, nextSpec, nextResponse)
		if err != nil {
			return "", "", fmt.Errorf("content: next page bodyJs: %w", err)
		}
		slog.Debug("content: next page response", "source", src.BookSourceName, "url", nextSpec.URL, "status", nextResponse.StatusCode, "finalURL", nextResponse.FinalURL)
		if nextResponse.StatusCode < http.StatusOK || nextResponse.StatusCode >= http.StatusMultipleChoices {
			return "", "", fmt.Errorf("content: next page status %d from %s", nextResponse.StatusCode, src.BookSourceName)
		}
		nextFullURL := nextResponse.FinalURL
		if nextFullURL == "" {
			nextFullURL = nextSpec.URL
		}
		nextAnalyzer := analyzer.New(nextResponse.Body, nextFullURL, s.jsVM, s.cache)
		nextAnalyzer.SetJSLib(src.JSLib)
		nextAnalyzer.SetSourceState(session)
		nextAnalyzer.SetContext(ctx)
		nextAnalyzer.SetChapterData(map[string]string{"url": nextFullURL, "baseUrl": nextFullURL})
		if pageContent := extractPage(nextAnalyzer, nextResponse.Body, nextFullURL); pageContent != "" {
			content += "\n" + pageContent
		}
		an = nextAnalyzer
		fullURL = nextFullURL
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

func (s *Searcher) fetchAndAnalyze(urlStr, baseURL, headerJSON, jsLib string) *analyzer.Analyzer {
	headers := parseHeaderJSON(headerJSON)
	fullURL := resolveURL(urlStr, baseURL)
	resp, err := s.fetcher.Get(fullURL, headers)
	if err != nil {
		return nil
	}
	an := analyzer.New(resp.Body, fullURL, s.jsVM, s.cache)
	an.SetJSLib(jsLib)
	return an
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
	// Use stdlib URL resolution: handles ../, //, and absolute-path (/foo) correctly.
	// e.g., base="https://site.com/novel/123" + url="/catalog/456" → "https://site.com/catalog/456"
	base, err := url.Parse(baseURL)
	if err != nil {
		// Fallback to naive join on parse error
		return strings.TrimRight(baseURL, "/") + "/" + strings.TrimLeft(urlStr, "/")
	}
	ref, err := url.Parse(urlStr)
	if err != nil {
		return strings.TrimRight(baseURL, "/") + "/" + strings.TrimLeft(urlStr, "/")
	}
	return base.ResolveReference(ref).String()
}
