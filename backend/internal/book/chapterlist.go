package book

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/otwako/novelreader/internal/analyzer"
	"github.com/otwako/novelreader/internal/booksource"
)

// ChapterListParser handles the full legado-compatible TOC parsing pipeline.
// Mirrors legado's BookChapterList.analyzeChapterList.
type ChapterListParser struct {
	src    booksource.BookSource
	jsVM   *analyzer.JSVM
	cache  *analyzer.CacheManager
	fetch  func(urlStr string) (string, string, error) // returns (body, resolvedURL, error)
}

// ParseChapterList fetches and parses the complete chapter list, handling pagination.
func (p *ChapterListParser) ParseChapterList(tocURL, baseURL string) ([]Chapter, error) {
	rules := parseRuleJSON(p.src.RuleToc)
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
	seen := make(map[string]bool) // dedup by URL+title
	visitedNext := make(map[string]bool)
	visitedNext[resolvedURL] = true

	// Handle pagination via nextTocUrl
	nextRule := rules["nextTocUrl"]
	for {
		chapters, nextURL, err := p.parsePage(body, resolvedURL, listRule, rules)
		if err != nil {
			return nil, err
		}
		for _, ch := range chapters {
			key := ch.URL + "|" + ch.Title
			if !seen[key] {
				seen[key] = true
				allChapters = append(allChapters, ch)
			}
		}

		if nextRule == "" {
			break
		}

		// Extract next page URL(s) from current page
		an := analyzer.New(body, resolvedURL, p.jsVM, p.cache)
		nextURLs, err := an.GetStringList(nextRule)
		if err != nil || len(nextURLs) == 0 {
			break
		}

		nextURL = nextURLs[0]
		if nextURL == "" || visitedNext[nextURL] {
			break
		}
		visitedNext[nextURL] = true

		// Resolve relative URL — use resolveURL for proper ../ and / handling
		nextURL = resolveURL(nextURL, resolvedURL)

		body, resolvedURL, err = p.fetch(nextURL)
		if err != nil {
			break
		}
	}

	// Reverse if needed (legado: if not reverse, reverse; if reverse, keep order)
	// legado logic: if !reverse { reverse() }; later: if !book.getReverseToc() { reverse() }
	// Default book behavior: reverse the list so chapter 0 is the first chapter
	if !reverse {
		for i, j := 0, len(allChapters)-1; i < j; i, j = i+1, j-1 {
			allChapters[i], allChapters[j] = allChapters[j], allChapters[i]
		}
	}

	// Assign indices
	for i := range allChapters {
		allChapters[i].Index = i
	}

	return allChapters, nil
}

func (p *ChapterListParser) parsePage(body, pageURL, listRule string, rules map[string]string) ([]Chapter, string, error) {
	an := analyzer.New(body, pageURL, p.jsVM, p.cache)
	elements, err := an.GetElements(listRule)
	if err != nil {
		return nil, "", fmt.Errorf("toc: get elements: %w", err)
	}

	nameRule := rules["chapterName"]
	urlRule := rules["chapterUrl"]
	vipRule := rules["isVip"]
	volumeRule := rules["isVolume"]
	timeRule := rules["updateTime"]

	var chapters []Chapter
	for _, el := range elements {
		elHTML := analyzer.ToString(el)
		elAn := analyzer.New(elHTML, pageURL, p.jsVM, p.cache)

		title := mustString(elAn, nameRule)
		chURL := mustString(elAn, urlRule)
		if title == "" {
			continue
		}

		// Resolve chapter URL against the page it was found on
		chURL = resolveURL(chURL, pageURL)

		// Volume detection: empty URL or explicit isVolume rule
		isVolume := mustString(elAn, volumeRule)
		if chURL == "" {
			isVolume = "true" // infer volume from missing URL
		}

		ch := Chapter{
			Title:    title,
			URL:      chURL,
			IsVip:    mustString(elAn, vipRule) == "true",
			IsVolume: isVolume == "true",
		}

		if t := mustString(elAn, timeRule); t != "" {
			_ = t
		}

		chapters = append(chapters, ch)
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
