package book

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"time"

	"github.com/dlclark/regexp2/v2"
	"github.com/otwako/novelreader/internal/analyzer"
	"github.com/otwako/novelreader/internal/booksource"
)

// parseDiscoveryResults classifies the received page, without fetching it again.
// A URL pattern forces detail parsing only for Search. Both discovery modes can
// fall back on an empty element list when the source has no URL pattern.
func (s *Searcher) parseDiscoveryResults(ctx context.Context, src booksource.BookSource, body, rules, baseURL string, state analyzer.SourceState, options discoveryParseOptions) ([]SearchResult, error) {
	if baseURL == "" {
		baseURL = src.BookSourceURL
	}
	if options.variables == nil {
		options.variables = make(map[string]string)
	}
	parseDetail := func() ([]SearchResult, error) {
		b := &Book{SourceID: src.ID, SourceURL: src.BookSourceURL, BookURL: baseURL, Origin: src.BookSourceName}
		data := bookContext(b, src)
		data["variableMap"] = maps.Clone(options.variables)
		b, err := s.parseBookInfoResponse(ctx, src, body, baseURL, b, data, state)
		if err != nil {
			return nil, err
		}
		if b.Name == "" {
			return nil, nil
		}
		result := searchResultFromBook(b)
		result.SourceGroup = src.BookSourceGroup
		result.Capabilities = booksource.CapabilityTags(src)
		return []SearchResult{result}, nil
	}
	if options.search && src.BookURLPattern != "" {
		// Reuse the installed backtracking engine for Java-style lookarounds and
		// backreferences; bound matching on untrusted source patterns.
		pattern, err := regexp2.Compile(`\A(?:` + src.BookURLPattern + `)\z`)
		if err != nil {
			return nil, fmt.Errorf("search: bookUrlPattern: %w", err)
		}
		pattern.MatchTimeout = 100 * time.Millisecond
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		matched, err := pattern.MatchString(baseURL)
		if err != nil {
			return nil, fmt.Errorf("search: bookUrlPattern: %w", err)
		}
		if matched {
			return parseDetail()
		}
	}
	results, err := s.parseDiscoveryList(ctx, src, body, rules, baseURL, state, options)
	if errors.Is(err, analyzer.ErrNoElements) {
		if src.BookURLPattern == "" {
			return parseDetail()
		}
		if options.allowEmpty {
			return nil, nil
		}
	}
	return results, err
}

func searchResultFromBook(book *Book) SearchResult {
	return SearchResult{
		Name: book.Name, Author: book.Author, CoverURL: book.CoverURL, Intro: book.Intro, Kind: book.Kind,
		LastChapter: book.LastChapter, UpdateTime: book.UpdateTime, WordCount: book.WordCount,
		BookURL: book.BookURL, SourceID: book.SourceID, SourceURL: book.SourceURL, SourceName: book.Origin,
		VariableMap: book.VariableMap,
	}
}
