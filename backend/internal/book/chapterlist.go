package book

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/otwako/novelreader/internal/analyzer"
	"github.com/otwako/novelreader/internal/booksource"
)

// TOCPaginationError reports a failed paginated TOC while retaining enough
// context to diagnose the discarded partial result.
type TOCPaginationError struct {
	PageURL         string
	FailedURL       string
	PagesFetched    int
	ChaptersFetched int
	Operation       string
	Err             error
}

func (e *TOCPaginationError) Error() string {
	if e == nil {
		return "toc: pagination failed"
	}
	location := e.PageURL
	if e.FailedURL != "" {
		location = e.FailedURL
	}
	return fmt.Sprintf("toc: %s %s after %d pages (%d chapters): %v", e.Operation, location, e.PagesFetched, e.ChaptersFetched, e.Err)
}

func (e *TOCPaginationError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// ChapterListParser handles the full legado-compatible TOC parsing pipeline.
// Mirrors legado's BookChapterList.analyzeChapterList.
type ChapterListParser struct {
	src      booksource.BookSource
	jsVM     *analyzer.JSVM
	cache    *analyzer.CacheManager
	state    analyzer.SourceState
	book     *Book
	bookData map[string]interface{}
	ctx      context.Context
	fetch    func(urlStr string) (string, string, error) // returns (body, resolvedURL, error)
}

// ParseChapterList fetches and parses the complete chapter list, handling pagination.
func (p *ChapterListParser) ParseChapterList(tocURL, baseURL string) ([]Chapter, error) {
	rules := parseRuleJSON(p.src.RuleToc)
	if p.bookData == nil {
		p.bookData = bookContext(p.book, p.src)
	}
	if rules == nil {
		return nil, fmt.Errorf("toc: no rules for %s", p.src.BookSourceName)
	}

	listRule := rules["chapterList"]
	if listRule == "" {
		return nil, fmt.Errorf("toc: no chapterList rule")
	}

	// Handle - prefix (reverse) and + prefix (no-reverse)
	reverse := strings.HasPrefix(listRule, "-")
	if reverse || strings.HasPrefix(listRule, "+") {
		listRule = listRule[1:]
	}

	body, resolvedURL, err := p.fetch(tocURL)
	if err != nil {
		return nil, fmt.Errorf("toc: fetch: %w", err)
	}

	var allChapters []Chapter
	scheduledRequests := make(map[string]bool)
	processedPages := make(map[string]bool)
	initialPageKey := tocVisitKey(resolvedURL)
	scheduledRequests[resolvedURL] = true
	processedPages[initialPageKey] = true
	pendingURLs := make([]string, 0, 1)
	pageCount := 0

	// Handle pagination via nextTocUrl.
	nextRule := rules["nextTocUrl"]
	for {
		pageCount++
		chapters, _, err := p.parsePage(body, resolvedURL, listRule, rules)
		if err != nil {
			return nil, newTOCPaginationError("parse page", resolvedURL, resolvedURL, pageCount, allChapters, err)
		}
		allChapters = append(allChapters, chapters...)

		if nextRule != "" {
			// Extract next page URL(s) from the current page. A missing URL is the
			// normal terminal condition; rule evaluation failures are not.
			an := analyzer.New(body, resolvedURL, p.jsVM, p.cache)
			an.SetContext(p.ctx)
			setAnalyzerContextMaps(an, p.src, p.state, p.bookData, nil, nil)
			nextURLs, err := an.GetStringList(nextRule)
			if err != nil && !errors.Is(err, analyzer.ErrNoListValues) {
				return nil, newTOCPaginationError("nextTocUrl", resolvedURL, resolvedURL, pageCount, allChapters, err)
			}
			if err == nil {
				for _, rawURL := range nextURLs {
					rawURL = strings.TrimSpace(rawURL)
					if rawURL == "" {
						continue
					}
					nextURL := resolveURL(rawURL, resolvedURL)
					if nextURL == "" {
						continue
					}
					if scheduledRequests[nextURL] {
						slog.Warn("toc: pagination skipped a scheduled next request",
							"source", p.src.BookSourceName,
							"page", resolvedURL,
							"next", nextURL,
							"pages", pageCount,
							"chapters", uniqueChapterCount(allChapters),
						)
						continue
					}
					scheduledRequests[nextURL] = true
					pendingURLs = append(pendingURLs, nextURL)
				}
			}
		}

		if len(pendingURLs) == 0 {
			break
		}
		nextURL := pendingURLs[0]
		pendingURLs = pendingURLs[1:]
		pageURL := resolvedURL
		nextBody, nextResolvedURL, err := p.fetch(nextURL)
		if err != nil {
			return nil, newTOCPaginationError("next page", pageURL, nextURL, pageCount, allChapters, err)
		}
		resolvedKey := tocVisitKey(nextResolvedURL)
		requestedKey := tocVisitKey(nextURL)
		if processedPages[resolvedKey] && requestedKey != resolvedKey {
			slog.Warn("toc: pagination skipped a redirect to a processed page",
				"source", p.src.BookSourceName,
				"page", pageURL,
				"next", nextURL,
				"resolved", nextResolvedURL,
				"pages", pageCount,
				"chapters", uniqueChapterCount(allChapters),
			)
			continue
		}
		processedPages[resolvedKey] = true
		body, resolvedURL = nextBody, nextResolvedURL
	}

	// Legado reverses the accumulated pages before LinkedHashSet deduplication
	// unless the rule starts with '-', then applies the user's default
	// non-reversed TOC preference after deduplication. NovelReader has no
	// persisted per-book reverse preference yet, so that final reversal is
	// always applied.
	if !reverse {
		reverseChapters(allChapters)
	}
	seen := make(map[string]bool, len(allChapters))
	unique := make([]Chapter, 0, len(allChapters))
	for _, chapter := range allChapters {
		if seen[chapter.URL] {
			continue
		}
		seen[chapter.URL] = true
		unique = append(unique, chapter)
	}
	reverseChapters(unique)
	allChapters = unique

	// Assign indices after reversal and deduplication.
	for i := range allChapters {
		allChapters[i].Index = i
	}

	return allChapters, nil
}

func (p *ChapterListParser) parsePage(body, pageURL, listRule string, rules map[string]string) ([]Chapter, string, error) {
	an := analyzer.New(body, pageURL, p.jsVM, p.cache)
	an.SetContext(p.ctx)
	setAnalyzerContextMaps(an, p.src, p.state, p.bookData, nil, nil)
	elements, err := an.GetElements(listRule)
	if err != nil {
		return nil, "", fmt.Errorf("toc: get elements: %w", err)
	}

	nameRule := rules["chapterName"]
	urlRule := rules["chapterUrl"]
	vipRule := rules["isVip"]
	payRule := rules["isPay"]
	volumeRule := rules["isVolume"]
	timeRule := rules["updateTime"]

	var chapters []Chapter
	for elementIndex, el := range elements {
		current := &Chapter{}
		elHTML := analyzer.ToString(el)
		elAn := analyzer.New(elHTML, pageURL, p.jsVM, p.cache)
		elAn.SetContext(p.ctx)
		chapterData := chapterContext(p.book, current, pageURL)
		setAnalyzerContextMaps(elAn, p.src, p.state, p.bookData, chapterData, nil)

		title, titleIsField := jsElementField(el, nameRule)
		if !titleIsField {
			title = mustString(elAn, nameRule)
		}
		if title == "" {
			continue
		}
		// BookChapter is mutable in Legado: chapterUrl rules can use the title
		// extracted immediately before them.
		current.Title = title
		chapterData["title"] = title
		setAnalyzerContextMaps(elAn, p.src, p.state, p.bookData, chapterData, nil)

		chURL, urlIsField := jsElementField(el, urlRule)
		if !urlIsField {
			chURL = mustString(elAn, urlRule)
		}
		// Resolve chapter URL against the page it was found on.
		chURL = resolveURL(chURL, pageURL)
		current.URL = chURL
		chapterData["url"] = chURL
		setAnalyzerContextMaps(elAn, p.src, p.state, p.bookData, chapterData, nil)

		// Match Legado's mutable chapter lifecycle: update info, volume, URL
		// fallback, then VIP/pay flags.
		current.BaseURL = pageURL
		chapterData["baseUrl"] = pageURL
		if t := elementRuleString(el, timeRule, pageURL, p, current, p.bookData, chapterData); t != "" {
			current.Tag = t
			chapterData["tag"] = t
		}
		current.IsVolume = legadoTrue(elementRuleString(el, volumeRule, pageURL, p, current, p.bookData, chapterData))
		if chURL == "" {
			if current.IsVolume {
				chURL = title + strconv.Itoa(elementIndex)
			} else {
				chURL = pageURL
			}
		}
		current.URL = chURL
		chapterData["url"] = chURL
		chapterData["isVolume"] = current.IsVolume
		current.IsVip = legadoTrue(elementRuleString(el, vipRule, pageURL, p, current, p.bookData, chapterData))
		current.IsPay = legadoTrue(elementRuleString(el, payRule, pageURL, p, current, p.bookData, chapterData))
		chapterData["isVolume"] = current.IsVolume
		chapterData["isVip"] = current.IsVip
		chapterData["isPay"] = current.IsPay
		setAnalyzerContextMaps(elAn, p.src, p.state, p.bookData, chapterData, nil)
		chapters = append(chapters, *current)
	}

	// Log warning when elements matched but no chapters had extractable titles.
	// This distinguishes "legitimately empty TOC" from "extraction failed"
	// — the silent 200-OK path that currently produces "Chapters (0)".
	if len(elements) > 0 && len(chapters) == 0 {
		slog.Warn("toc: elements matched but no chapter names extracted — check chapterName rule",
			"source", p.src.BookSourceName,
			"elements", len(elements),
			"nameRule", nameRule,
			"urlRule", urlRule,
		)
		return nil, "", fmt.Errorf(
			"toc: %d elements matched but all chapter names empty — chapterName=%q chapterUrl=%q",
			len(elements), nameRule, urlRule)
	}

	return chapters, "", nil
}

func newTOCPaginationError(operation, pageURL, failedURL string, pages int, chapters []Chapter, err error) *TOCPaginationError {
	return &TOCPaginationError{
		PageURL:         pageURL,
		FailedURL:       failedURL,
		PagesFetched:    pages,
		ChaptersFetched: uniqueChapterCount(chapters),
		Operation:       operation,
		Err:             err,
	}
}

func uniqueChapterCount(chapters []Chapter) int {
	seen := make(map[string]bool, len(chapters))
	for _, chapter := range chapters {
		seen[chapter.URL] = true
	}
	return len(seen)
}

func tocVisitKey(rawURL string) string {
	urlPart, _ := splitURLOptionSuffix(rawURL)
	return urlPart
}

func reverseChapters(chapters []Chapter) {
	for i, j := 0, len(chapters)-1; i < j; i, j = i+1, j-1 {
		chapters[i], chapters[j] = chapters[j], chapters[i]
	}
}

func legadoTrue(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "", "false", "no", "not", "null", "0", "0.0":
		return false
	default:
		return true
	}
}

func jsElementField(value interface{}, rule string) (string, bool) {
	key := strings.TrimSpace(strings.TrimPrefix(rule, "@"))
	if key == "" {
		return "", false
	}
	switch object := value.(type) {
	case map[string]interface{}:
		field, ok := object[key]
		if !ok {
			return "", false
		}
		return analyzer.ToString(field), true
	case map[string]string:
		field, ok := object[key]
		return field, ok
	default:
		return "", false
	}
}

func elementRuleString(value interface{}, rule, pageURL string, parser *ChapterListParser, current *Chapter, bookData, chapterData map[string]interface{}) string {
	if value, ok := jsElementField(value, rule); ok {
		return value
	}
	an := analyzer.New(analyzer.ToString(value), pageURL, parser.jsVM, parser.cache)
	an.SetContext(parser.ctx)
	setAnalyzerContextMaps(an, parser.src, parser.state, bookData, chapterData, nil)
	return mustString(an, rule)
}

// GetNextContentURL fetches the next content URL if the chapter has a pagination rule.
// Not yet implemented — most chapters are single-page.
func (s *Searcher) GetNextContentURL(src booksource.BookSource, currentURL string) (string, error) {
	rules := parseRuleJSON(src.RuleContent)
	if rules == nil {
		return "", nil
	}
	nextRule := rules["nextContentUrl"]
	if nextRule == "" {
		return "", nil
	}
	an := s.fetchAndAnalyze(currentURL, src.BookSourceURL, src.Header, src.JSLib)
	if an == nil {
		return "", nil
	}
	return mustString(an, nextRule), nil
}
