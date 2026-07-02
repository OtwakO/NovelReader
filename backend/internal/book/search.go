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
		searchFetcher: fetcher.NewStateless(perSourceTimeout), // no cookie jar = safe for multi-user
		jsVM:          jsVM,
		cache:         cache,
		sourceStore:   sourceStore,
		bookStore:     bookStore,
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

	for _, src := range candidates {
		wg.Add(1)
		sem <- struct{}{}
		go func(src booksource.BookSource) {
			defer wg.Done()
			defer func() { <-sem }()
			r, err := s.searchSource(ctx, src, query)
			ch <- jobResult{src, r, err}
		}(src)
	}

	go func() { wg.Wait(); close(ch) }()

	for r := range ch {
		if ctx.Err() != nil {
			break
		}
		onResult(r.src, r.results, r.err)
	}
	return ctx.Err()
}

// searchCandidates returns enabled sources with search capability.
func (s *Searcher) searchCandidates() ([]booksource.BookSource, error) {
	sources, err := s.sourceStore.ListEnabled()
	if err != nil {
		return nil, fmt.Errorf("search: list sources: %w", err)
	}
	var candidates []booksource.BookSource
	for _, src := range sources {
		if src.SearchURL != "" && src.RuleSearch != "" {
			candidates = append(candidates, src)
		}
	}
	return candidates, nil
}

// searchSource performs a single source search.
func (s *Searcher) searchSource(ctx context.Context, src booksource.BookSource, query string) ([]SearchResult, error) {
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
	results, err := s.parseSearchResultWithRule(src, resp.Body, src.RuleSearch)
	if err != nil {
		return nil, err
	}
	// Score each result against the query for relevance sorting
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

	var results []SearchResult
	for _, el := range elements {
		elHTML := analyzer.ToString(el)
		elAn := analyzer.New(elHTML, src.BookSourceURL, s.jsVM, s.cache)

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
			results = append(results, r)
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

func (s *Searcher) fetchAndAnalyze(urlStr, baseURL, headerJSON string) *analyzer.Analyzer {
	headers := parseHeaderJSON(headerJSON)
	fullURL := urlStr
	if !strings.HasPrefix(urlStr, "http://") && !strings.HasPrefix(urlStr, "https://") {
		fullURL = strings.TrimRight(baseURL, "/") + "/" + strings.TrimLeft(urlStr, "/")
	}
	resp, err := s.fetcher.Get(fullURL, headers)
	if err != nil {
		return nil
	}
	return analyzer.New(resp.Body, fullURL, s.jsVM, s.cache)
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
