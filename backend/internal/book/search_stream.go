// Shared bounded fan-out for legacy and batched source searches.
package book

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/otwako/novelreader/internal/booksource"
)

type searchJob struct {
	src    booksource.BookSource
	expand bool
}

type searchJobResult struct {
	src     booksource.BookSource
	results []SearchResult
	err     error
}

func (s *Searcher) searchSources(ctx context.Context, query string, candidates []booksource.BookSource, concurrency int, expandSourceID string, onResult SearchCallback) error {
	if len(candidates) == 0 {
		return nil
	}
	if concurrency < 1 || concurrency > s.concurrentPerSearch {
		concurrency = s.concurrentPerSearch
	}
	if concurrency < 1 {
		concurrency = defaultMaxConcurrentSearch
	}

	jobs := make(chan searchJob)
	results := make(chan searchJobResult, len(candidates))
	globalSlots := s.searchSlots
	if globalSlots == nil {
		globalSlots = make(chan struct{}, defaultMaxConcurrentGlobalSearch)
	}

	slog.Info("search: starting fan-out", "query", query, "sources", len(candidates), "concurrent", concurrency)

	var workers sync.WaitGroup
	for range min(concurrency, len(candidates)) {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for job := range jobs {
				select {
				case globalSlots <- struct{}{}:
				case <-ctx.Done():
					return
				}
				s.searchSourceJob(ctx, query, job, globalSlots, results)
			}
		}()
	}
	go func() {
		defer close(jobs)
		for _, src := range candidates {
			select {
			case jobs <- searchJob{src: src, expand: src.ID != "" && src.ID == expandSourceID}:
			case <-ctx.Done():
				return
			}
		}
	}()
	go func() {
		workers.Wait()
		close(results)
	}()

	successCount := 0
	errorCount := 0
	errCats := make(map[string]int)
	for result := range results {
		if ctx.Err() != nil {
			break
		}
		if result.err != nil {
			errorCount++
			category := searchErrorCategory(result.err.Error())
			errCats[category]++
			slog.Info("search: source failed", "source", result.src.BookSourceName, "cat", category, "err", result.err.Error()[:min(len(result.err.Error()), 120)])
		} else {
			successCount++
			slog.Debug("search: source completed", "source", result.src.BookSourceName, "results", len(result.results))
		}
		onResult(result.src, result.results, result.err)
	}

	parts := make([]string, 0, len(errCats))
	for category, count := range errCats {
		parts = append(parts, fmt.Sprintf("%s=%d", category, count))
	}
	slog.Info("search: finished",
		"query", query,
		"success", successCount,
		"errors", errorCount,
		"breakdown", strings.Join(parts, ", "),
		"total_sources", len(candidates),
		"cancelled", ctx.Err() != nil,
		"active_searches", s.capacity.activeSearches.Load(),
		"active_source_fetches", s.capacity.activeSourceFetches.Load())
	return ctx.Err()
}

func (s *Searcher) searchSourceJob(ctx context.Context, query string, job searchJob, globalSlots chan struct{}, results chan<- searchJobResult) {
	src := job.src
	s.capacity.activeSourceFetches.Add(1)
	s.capacity.totalSourceFetches.Add(1)
	defer s.capacity.activeSourceFetches.Add(-1)
	defer func() { <-globalSlots }()
	defer func() {
		if rec := recover(); rec != nil {
			s.capacity.failedSources.Add(1)
			slog.Error("search: panic in source goroutine", "source", src.BookSourceName, "panic", fmt.Sprintf("%v", rec))
			results <- searchJobResult{src, nil, fmt.Errorf("panic: %v", rec)}
		}
	}()

	found, err := s.searchSource(ctx, src, query)
	if job.expand {
		defaults, defaultErr := s.searchSourceWithHydrator(ctx, src, query, s.authenticationHydrator)
		switch {
		case err == nil && defaultErr == nil:
			found = mergeSourceBindings(found, defaults)
		case err != nil && defaultErr == nil:
			found, err = defaults, nil
		case err == nil && defaultErr != nil:
			slog.Info("search: current source default-state expansion failed", "source", src.BookSourceName, "error", defaultErr)
		}
	}
	if err != nil {
		s.capacity.failedSources.Add(1)
	} else {
		s.capacity.completedSources.Add(1)
	}
	results <- searchJobResult{src, found, err}
}

func mergeSourceBindings(groups ...[]SearchResult) []SearchResult {
	merged := make([]SearchResult, 0)
	seen := make(map[string]struct{})
	for _, group := range groups {
		for _, result := range group {
			key := result.SourceID + "\x00" + result.BookURL
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			merged = append(merged, result)
		}
	}
	return merged
}

func searchErrorCategory(message string) string {
	switch {
	case strings.Contains(message, "status 503"), strings.Contains(message, "status 403"):
		return "blocked (503/403)"
	case strings.Contains(message, "status "):
		return "non-200 status"
	case strings.Contains(message, "timeout"), strings.Contains(message, "Timeout"):
		return "timeout"
	case strings.Contains(message, "no such host"), strings.Contains(message, "DNS"):
		return "dns"
	case strings.Contains(message, "TLS"), strings.Contains(message, "certificate"):
		return "tls"
	case strings.Contains(message, "connection refused"), strings.Contains(message, "no route to host"):
		return "connection refused"
	case strings.Contains(message, "WebView"), strings.Contains(message, "webView"):
		return "needs JS (webView)"
	case strings.Contains(message, "no elements matched"):
		return "empty results (0 books)"
	default:
		return "other"
	}
}
