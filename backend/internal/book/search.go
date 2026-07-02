package book

import (
	"fmt"
	"sync"

	"github.com/otwako/novelreader/internal/analyzer"
	"github.com/otwako/novelreader/internal/booksource"
	"github.com/otwako/novelreader/internal/fetcher"
)

// Searcher orchestrates search, book info fetching, TOC fetching, and content fetching
// across book sources using the analyzer engine.
type Searcher struct {
	fetcher       *fetcher.Client
	jsVM          *analyzer.JSVM
	cache         *analyzer.CacheManager
	sourceStore   *booksource.Store
	bookStore     *Store
}

func NewSearcher(
	fetcher *fetcher.Client,
	jsVM *analyzer.JSVM,
	cache *analyzer.CacheManager,
	sourceStore *booksource.Store,
	bookStore *Store,
) *Searcher {
	return &Searcher{
		fetcher:     fetcher,
		jsVM:        jsVM,
		cache:       cache,
		sourceStore: sourceStore,
		bookStore:   bookStore,
	}
}

// Search searches across all enabled sources and returns deduplicated results.
func (s *Searcher) Search(query string) ([]SearchResult, error) {
	sources, err := s.sourceStore.ListEnabled()
	if err != nil {
		return nil, fmt.Errorf("search: list sources: %w", err)
	}

	type result struct {
		results []SearchResult
		err     error
		source  string
	}

	ch := make(chan result, len(sources))
	var wg sync.WaitGroup

	for _, src := range sources {
		wg.Add(1)
		go func(src booksource.BookSource) {
			defer wg.Done()
			r, err := s.searchSource(src, query)
			ch <- result{results: r, err: err, source: src.BookSourceName}
		}(src)
	}

	wg.Wait()
	close(ch)

	var all []SearchResult
	seen := make(map[string]bool)
	for r := range ch {
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
	}
	if all == nil {
		all = []SearchResult{}
	}
	return all, nil
}

func (s *Searcher) searchSource(src booksource.BookSource, query string) ([]SearchResult, error) {
	searchURL, headers, err := analyzer.BuildURL(src.SearchURL, query, 1, src.BookSourceURL, s.jsVM)
	if err != nil || searchURL == "" {
		return nil, fmt.Errorf("build url: %w", err)
	}

	resp, err := s.fetcher.Get(searchURL, headers)
	if err != nil {
		return nil, fmt.Errorf("fetch: %w", err)
	}

	if src.RuleSearch == "" {
		return nil, fmt.Errorf("source %q has no search rules", src.BookSourceName)
	}
	return s.parseSearchResultWithRule(src, resp.Body, src.RuleSearch)
}

// ponytail: flat-string ruleSearch fallback removed. Require structured rules.

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
		b.CoverURL = mustString(an, rules["coverUrl"])
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

// GetChapterList fetches and parses the TOC for a book.
func (s *Searcher) GetChapterList(src booksource.BookSource, bookURL, tocURL string) ([]Chapter, error) {
	fetchURL := tocURL
	if fetchURL == "" {
		fetchURL = bookURL
	}

	an := s.fetchAndAnalyze(fetchURL, src.BookSourceURL, src.Header)
	if an == nil {
		return nil, fmt.Errorf("chapter list: fetch failed")
	}

	rules := parseRuleJSON(src.RuleToc)
	if rules == nil {
		return nil, fmt.Errorf("chapter list: no rules for %s", src.BookSourceName)
	}

	chapterRule := rules["chapterList"]
	nameRule := rules["chapterName"]
	urlRule := rules["chapterUrl"]

	if chapterRule == "" {
		return nil, fmt.Errorf("chapter list: no chapterList rule")
	}

	elements, err := an.GetElements(chapterRule)
	if err != nil {
		return nil, fmt.Errorf("chapter list: %w", err)
	}

	var chapters []Chapter
	for i, el := range elements {
		elHTML := analyzer.ToString(el)
		elAn := analyzer.New(elHTML, src.BookSourceURL, s.jsVM, s.cache)

		title := mustString(elAn, nameRule)
		chURL := mustString(elAn, urlRule)
		if title == "" {
			title = mustString(elAn, "text")
		}
		if chURL == "" {
			chURL = mustString(elAn, "a@href")
		}
		if title == "" {
			continue
		}

		chapters = append(chapters, Chapter{
			ID:    fmt.Sprintf("%s_%d", bookURL, i),
			Index: i,
			Title: title,
			URL:   chURL,
		})
	}
	return chapters, nil
}

// GetChapterContent fetches and parses the content of a single chapter.
func (s *Searcher) GetChapterContent(src booksource.BookSource, chapterURL string) (string, error) {
	an := s.fetchAndAnalyze(chapterURL, src.BookSourceURL, src.Header)
	if an == nil {
		return "", fmt.Errorf("chapter content: fetch failed")
	}

	rules := parseRuleJSON(src.RuleContent)
	if rules == nil {
		return "", fmt.Errorf("chapter content: no rules")
	}

	contentRule := rules["content"]
	if contentRule == "" {
		return mustString(an, "text"), nil
	}
	return mustString(an, contentRule), nil
}

// fetchAndAnalyze fetches a URL and creates an Analyzer for the response.
func (s *Searcher) fetchAndAnalyze(urlStr, baseURL, headerJSON string) *analyzer.Analyzer {
	headers := parseHeaderJSON(headerJSON)
	resp, err := s.fetcher.Get(urlStr, headers)
	if err != nil {
		return nil
	}
	return analyzer.New(resp.Body, baseURL, s.jsVM, s.cache)
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
