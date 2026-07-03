package book

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/otwako/novelreader/internal/analyzer"
	"github.com/otwako/novelreader/internal/booksource"
	"github.com/otwako/novelreader/internal/fetcher"
)

const (
	maxConcurrentSearch  = 50               // max concurrent HTTP fetches for search
	searchOverallTimeout = 30 * time.Second // max time for the entire search
	perSourceTimeout     = 10 * time.Second // max time per single source
	maxResultsPerSource  = 20               // max results returned per source
)

// Searcher orchestrates search, book info, TOC, and content fetching.
type Searcher struct {
	fetcher       *fetcher.Client
	searchFetcher *fetcher.Client // 8s timeout for search
	jsVM          *analyzer.JSVM
	cache         *analyzer.CacheManager
	sourceStore   *booksource.Store
	bookStore     *Store
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
		fetcher:       hc,
		searchFetcher: fetcher.NewInsecureStateless(perSourceTimeout),
		jsVM:          jsVM,
		cache:         cache,
		sourceStore:   sourceStore,
		bookStore:     bookStore,
		rateMu:        sync.Mutex{},
		lastAccess:    make(map[string]time.Time),
	}
}

// SetSearchFetcher replaces the default search fetcher with a custom one.
func (s *Searcher) SetSearchFetcher(f *fetcher.Client) { s.searchFetcher = f }

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
		count int
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

// encodeBody encodes POST body values in the specified charset.
// ponytail: simple key=value splitting; doesn't handle nested args like key[]=a&key[]=b.
func encodeBody(body, charset string) string {
	if body == "" {
		return body
	}
	if charset == "" {
		return body
	}
	pairs := strings.Split(body, "&")
	out := make([]string, 0, len(pairs))
	for _, pair := range pairs {
		eqIdx := strings.IndexByte(pair, '=')
		if eqIdx == -1 {
			out = append(out, pair)
			continue
		}
		key := pair[:eqIdx]
		value := pair[eqIdx+1:]
		out = append(out, key+"="+analyzer.EncodeParamValue(value, charset))
	}
	return strings.Join(out, "&")
}

// searchSource performs a single source search.
func (s *Searcher) searchSource(ctx context.Context, src booksource.BookSource, query string) ([]SearchResult, error) {
	srcCtx, cancel := context.WithTimeout(ctx, perSourceTimeout)
	defer cancel()

	meta, err := analyzer.BuildURL(src.SearchURL, query, 1, src.BookSourceURL, s.jsVM)
	if err != nil || meta == nil || meta.URL == "" {
		return nil, fmt.Errorf("build url: %w", err)
	}

	// Merge headers: source-level header first, then URL-option headers overlay
	headers := make(map[string]string)
	for k, v := range parseHeaderJSON(src.Header) {
		headers[k] = v
	}
	for k, v := range meta.Headers {
		headers[k] = v
	}

	// Log the resolved search URL at Debug level (enable with DEBUG=1)
	slog.Debug("search: fetching source",
		"source", src.BookSourceName,
		"method", meta.Method,
		"url", meta.URL,
		"charset", meta.Charset)

	if meta.WebView {
		slog.Warn("search: source needs WebView (JS rendering), skipping",
			"source", src.BookSourceName)
		return nil, fmt.Errorf("source requires JS rendering (webView:true)")
	}

	// Respect per-source rate limit (concurrentRate in milliseconds)
	s.rateLimitWait(src)

	// Encode POST body in the specified charset if non-UTF-8
	if meta.Method == "POST" && meta.Body != "" && meta.Charset != "" {
		meta.Body = encodeBody(meta.Body, meta.Charset)
	}

	var resp *fetcher.Response
	if meta.Method == "POST" {
		resp, err = s.searchFetcher.PostContext(srcCtx, meta.URL, meta.Body, headers, meta.Retry)
	} else {
		resp, err = s.searchFetcher.GetContext(srcCtx, meta.URL, headers, meta.Retry)
	}
	if err != nil {
		return nil, fmt.Errorf("fetch: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch: status %d from %s", resp.StatusCode, src.BookSourceName)
	}

	if src.RuleSearch == "" {
		return nil, fmt.Errorf("source %q has no search rules", src.BookSourceName)
	}
	results, err := s.parseSearchResultWithRule(src, resp.Body, src.RuleSearch)
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
	rules := parseRuleJSON(ruleJSON)
	if rules == nil {
		return nil, fmt.Errorf("search: invalid rule JSON for %s", src.BookSourceName)
	}
	bookListRule := rules["bookList"]
	if bookListRule == "" {
		return nil, fmt.Errorf("search: no bookList rule for %s", src.BookSourceName)
	}

	an := analyzer.New(html, src.BookSourceURL, s.jsVM, s.cache)
	an.SetJSLib(src.JSLib)
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
		elAn := analyzer.New(elHTML, src.BookSourceURL, s.jsVM, s.cache)
		elAn.SetJSLib(src.JSLib)

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
	return results, nil
}

// GetBookInfo fetches and parses book info using ruleBookInfo.
func (s *Searcher) GetBookInfo(src booksource.BookSource, bookURL string) (*Book, error) {
	an := s.fetchAndAnalyze(bookURL, src.BookSourceURL, src.Header, src.JSLib)
	if an == nil {
		return nil, fmt.Errorf("book info: fetch failed for %s", bookURL)
	}

	// Set book context for JS rules that reference book.name, book.author, etc.
	an.SetBookData(map[string]string{
		"bookUrl": bookURL,
		"origin": src.BookSourceURL,
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
		b.CoverURL = resolveURL(mustString(an, rules["coverUrl"]), src.BookSourceURL)
		b.Intro = mustString(an, rules["intro"])
		b.Kind = mustString(an, rules["kind"])
		b.LastChapter = mustString(an, rules["lastChapter"])
		b.UpdateTime = mustString(an, rules["updateTime"])
		b.WordCount = mustString(an, rules["wordCount"])

		// resolve tocUrl against bookUrl, not source root
		if tocURL := mustString(an, rules["tocUrl"]); tocURL != "" {
			b.TocURL = resolveURL(tocURL, b.BookURL)
		}
	}
	return b, nil
}

// GetChapterList fetches and parses the TOC using the legado-compatible parser.
func (s *Searcher) GetChapterList(src booksource.BookSource, bookURL, tocURL string) ([]Chapter, error) {
	fetchURL := tocURL
	if fetchURL == "" {
		fetchURL = bookURL
	}

	parser := &ChapterListParser{
		src:   src,
		jsVM:  s.jsVM,
		cache: s.cache,
		fetch: func(urlStr string) (string, string, error) {
			fullURL := urlStr
			if !strings.HasPrefix(urlStr, "http://") && !strings.HasPrefix(urlStr, "https://") {
				fullURL = strings.TrimRight(src.BookSourceURL, "/") + "/" + strings.TrimLeft(urlStr, "/")
			}
			resp, err := s.fetcher.Get(fullURL, parseHeaderJSON(src.Header))
			if err != nil {
				return "", "", err
			}
			return resp.Body, fullURL, nil
		},
	}

	chapters, err := parser.ParseChapterList(fetchURL, src.BookSourceURL)
	if err != nil {
		return nil, fmt.Errorf("chapter list: %w", err)
	}
	for i := range chapters {
		chapters[i].ID = fmt.Sprintf("%s_%d", bookURL, i)
	}
	return chapters, nil
}

// GetChapterContent fetches and parses chapter content using ruleContent.
func (s *Searcher) GetChapterContent(src booksource.BookSource, chapterURL string) (string, string, error) {
	fullURL := chapterURL
	if !strings.HasPrefix(chapterURL, "http://") && !strings.HasPrefix(chapterURL, "https://") {
		fullURL = strings.TrimRight(src.BookSourceURL, "/") + "/" + strings.TrimLeft(chapterURL, "/")
	}

	headers := parseHeaderJSON(src.Header)
	resp, err := s.fetcher.Get(fullURL, headers)
	if err != nil {
		return "", "", fmt.Errorf("content: fetch: %w", err)
	}

	an := analyzer.New(resp.Body, fullURL, s.jsVM, s.cache)
	an.SetJSLib(src.JSLib)
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
	content := ""
	if contentRule != "" {
		content = mustString(an, contentRule)
	} else {
		content = mustString(an, "body@text")
	}

	if subRule := rules["subContent"]; subRule != "" {
		if sub := mustString(an, subRule); sub != "" {
			content += "\n" + sub
		}
	}
	return content, chapterTitle, nil
}

func (s *Searcher) fetchAndAnalyze(urlStr, baseURL, headerJSON, jsLib string) *analyzer.Analyzer {
	headers := parseHeaderJSON(headerJSON)
	fullURL := urlStr
	if !strings.HasPrefix(urlStr, "http://") && !strings.HasPrefix(urlStr, "https://") {
		fullURL = strings.TrimRight(baseURL, "/") + "/" + strings.TrimLeft(urlStr, "/")
	}
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
	return strings.TrimRight(baseURL, "/") + "/" + strings.TrimLeft(urlStr, "/")
}
