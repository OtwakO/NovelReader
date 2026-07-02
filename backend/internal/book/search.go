package book

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/otwako/novelreader/internal/analyzer"
	"github.com/otwako/novelreader/internal/booksource"
	"github.com/otwako/novelreader/internal/fetcher"
)

// Searcher orchestrates search, book info, TOC, and content fetching.
// Concurrency design: search fans out across all sources with a shared context deadline.
// Slow sources are abandoned rather than waited on.
type Searcher struct {
	fetcher       *fetcher.Client // general purpose (15s timeout)
	searchFetcher *fetcher.Client // search-specific (8s timeout per source)
	jsVM          *analyzer.JSVM
	cache         *analyzer.CacheManager
	sourceStore   *booksource.Store
	bookStore     *Store
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
		searchFetcher: fetcher.NewWithTimeout(8 * time.Second),
		jsVM:          jsVM,
		cache:         cache,
		sourceStore:   sourceStore,
		bookStore:     bookStore,
	}
}

const (
	maxConcurrentSearch = 50    // max concurrent HTTP fetches for search
	searchOverallTimeout = 30 * time.Second // max time for the entire search
	perSourceTimeout     = 8 * time.Second  // max time per single source
)

// Search searches across all sources with search capability.
// Returns partial results after overallTimeout — slow sources are skipped.
func (s *Searcher) Search(query string) ([]SearchResult, error) {
	sources, err := s.sourceStore.ListEnabled()
	if err != nil {
		return nil, fmt.Errorf("search: list sources: %w", err)
	}

	// Filter to sources with search URL and rules
	var candidates []booksource.BookSource
	for _, src := range sources {
		if src.SearchURL != "" && src.RuleSearch != "" {
			candidates = append(candidates, src)
		}
	}
	if len(candidates) == 0 {
		return []SearchResult{}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), searchOverallTimeout)
	defer cancel()

	type jobResult struct {
		results []SearchResult
		err     error
		name    string
	}

	ch := make(chan jobResult, len(candidates))
	var wg sync.WaitGroup
	sem := make(chan struct{}, maxConcurrentSearch)

	for _, src := range candidates {
		wg.Add(1)
		sem <- struct{}{} // acquire sem — blocks if at capacity
		go func(src booksource.BookSource) {
			defer wg.Done()
			defer func() { <-sem }()
			r, err := s.searchSource(ctx, src, query)
			select {
			case ch <- jobResult{results: r, err: err, name: src.BookSourceName}:
			case <-ctx.Done():
				// context cancelled, don't send
			}
		}(src)
	}

	// Wait for all goroutines in a separate goroutine so we can select
	go func() {
		wg.Wait()
		close(ch)
	}()

	// Collect results until channel closes or context expires
	var all []SearchResult
	seen := make(map[string]bool)
	done := false
	for !done {
		select {
		case r, ok := <-ch:
			if !ok {
				done = true
				break
			}
			if r.err != nil {
				continue
			}
			for _, res := range r.results {
				key := res.SourceURL + ":" + res.BookURL
				if !seen[key] {
					seen[key] = true
					all = append(all, res)
				}
			}
		case <-ctx.Done():
			// Timeout — collect what we have so far
			done = true
		}
	}

	if all == nil {
		all = []SearchResult{}
	}
	return all, nil
}

// searchSource performs a single source search with per-source context.
func (s *Searcher) searchSource(ctx context.Context, src booksource.BookSource, query string) ([]SearchResult, error) {
	// Per-source deadline
	srcCtx, cancel := context.WithTimeout(ctx, perSourceTimeout)
	defer cancel()

	searchURL, headers, err := analyzer.BuildURL(src.SearchURL, query, 1, src.BookSourceURL, s.jsVM)
	if err != nil || searchURL == "" {
		return nil, fmt.Errorf("build url: %w", err)
	}

	resp, err := s.searchFetcher.GetContext(srcCtx, searchURL, headers)
	if err != nil {
		return nil, fmt.Errorf("fetch: %w", err)
	}

	if src.RuleSearch == "" {
		return nil, fmt.Errorf("source %q has no search rules", src.BookSourceName)
	}
	return s.parseSearchResultWithRule(src, resp.Body, src.RuleSearch)
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
	elements, err := an.GetElements(bookListRule)
	if err != nil {
		return nil, fmt.Errorf("search: bookList: %w", err)
	}

	var results []SearchResult
	for _, el := range elements {
		elHTML := analyzer.ToString(el)
		elAn := analyzer.New(elHTML, src.BookSourceURL, s.jsVM, s.cache)

		result := SearchResult{
			SourceURL:  src.BookSourceURL,
			SourceName: src.BookSourceName,
		}

		if v := mustString(elAn, rules["name"]); v != "" {
			result.Name = v
		}
		if v := mustString(elAn, rules["author"]); v != "" {
			result.Author = v
		}
		if v := mustString(elAn, rules["coverUrl"]); v != "" {
			result.CoverURL = v
		}
		if v := mustString(elAn, rules["intro"]); v != "" {
			result.Intro = v
		}
		if v := mustString(elAn, rules["kind"]); v != "" {
			result.Kind = v
		}
		if v := mustString(elAn, rules["lastChapter"]); v != "" {
			result.LastChapter = v
		}
		if v := mustString(elAn, rules["bookUrl"]); v != "" {
			result.BookURL = v
		}

		if result.Name != "" && result.BookURL != "" {
			results = append(results, result)
		}
	}
	return results, nil
}

// GetBookInfo fetches and parses book info using ruleBookInfo.
func (s *Searcher) GetBookInfo(src booksource.BookSource, bookURL string) (*Book, error) {
	an := s.fetchAndAnalyze(bookURL, src.BookSourceURL, src.Header)
	if an == nil {
		return nil, fmt.Errorf("book info: fetch failed for %s", bookURL)
	}

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

		tocURL := mustString(an, rules["tocUrl"])
		if tocURL != "" {
			b.TocURL = tocURL
		}
	}

	return b, nil
}

// resolveURL makes a relative URL absolute against a base.
func resolveURL(urlStr, baseURL string) string {
	if urlStr == "" {
		return ""
	}
	if strings.HasPrefix(urlStr, "http://") || strings.HasPrefix(urlStr, "https://") {
		return urlStr
	}
	return strings.TrimRight(baseURL, "/") + "/" + strings.TrimLeft(urlStr, "/")
}

// resolveURLPage resolves a relative URL against a specific page URL (for chapter/toc links).
func resolveURLPage(urlStr, pageURL string) string {
	if urlStr == "" {
		return ""
	}
	if strings.HasPrefix(urlStr, "http://") || strings.HasPrefix(urlStr, "https://") {
		return urlStr
	}
	// Resolve against the page URL's directory
	if idx := strings.LastIndex(pageURL, "/"); idx > 8 { // past "http://"
		return pageURL[:idx+1] + strings.TrimLeft(urlStr, "/")
	}
	return strings.TrimRight(pageURL, "/") + "/" + strings.TrimLeft(urlStr, "/")
}

// GetChapterList fetches and parses the TOC using the legado-compatible chapter list parser.
// Handles pagination (nextTocUrl), volume detection, reverse ordering, VIP/pay markers.
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
			// Resolve relative URL
			fullURL := urlStr
			if !strings.HasPrefix(urlStr, "http://") && !strings.HasPrefix(urlStr, "https://") {
				fullURL = strings.TrimRight(src.BookSourceURL, "/") + "/" + strings.TrimLeft(urlStr, "/")
			}
			resp, err := s.fetcher.Get(fullURL, nil)
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

	// Assign IDs for storage
	for i := range chapters {
		chapters[i].ID = fmt.Sprintf("%s_%d", bookURL, i)
	}
	return chapters, nil
}

// GetChapterContent fetches and parses the content of a single chapter.
// Supports legado ContentRule fields: content, subContent, title, webJs, sourceRegex, replaceRegex.
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
	rules := parseRuleJSON(src.RuleContent)
	if rules == nil {
		return mustString(an, "body@text"), "", nil
	}

	// Extract title from content page if rule exists
	chapterTitle := ""
	if titleRule := rules["title"]; titleRule != "" {
		chapterTitle = mustString(an, titleRule)
	}

	// Main content
	contentRule := rules["content"]
	content := ""
	if contentRule != "" {
		content = mustString(an, contentRule)
	} else {
		content = mustString(an, "body@text")
	}

	// Sub content (appended to main content)
	if subRule := rules["subContent"]; subRule != "" {
		sub := mustString(an, subRule)
		if sub != "" {
			content += "\n" + sub
		}
	}

	// ponytail: webJs (headless JS execution), sourceRegex, replaceRegex not yet implemented.
	// These are less common in book source rules.

	return content, chapterTitle, nil
}

// fetchAndAnalyze fetches a URL (resolving relative paths) and creates an Analyzer.
func (s *Searcher) fetchAndAnalyze(urlStr, baseURL, headerJSON string) *analyzer.Analyzer {
	headers := parseHeaderJSON(headerJSON)
	// Resolve relative URLs against source base URL
	fullURL := urlStr
	if !strings.HasPrefix(urlStr, "http://") && !strings.HasPrefix(urlStr, "https://") {
		base := strings.TrimRight(baseURL, "/")
		path := strings.TrimLeft(urlStr, "/")
		fullURL = base + "/" + path
	}
	resp, err := s.fetcher.Get(fullURL, headers)
	if err != nil {
		return nil
	}
	return analyzer.New(resp.Body, fullURL, s.jsVM, s.cache)
}

// mustString runs a rule on an analyzer and returns the result or empty string.
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
