package book

import (
	"sort"
	"strings"
)

// scoreResult ranks a result name against the query. Higher is better.
// ponytail: byte-prefix on UTF-8 is fine — rune boundaries are self-synchronizing.
func scoreResult(query, name string) int {
	q := strings.TrimSpace(query)
	n := strings.TrimSpace(name)
	switch {
	case q == "":
		return 0
	case n == q:
		return 100 // exact match
	case strings.HasPrefix(n, q):
		return 80 // prefix: 凡人修仙传xxx
	case strings.Contains(n, q):
		return 60 // substring: x凡人修仙传x
	default:
		return 20 // source's own fuzzy/pinyin, no direct name match
	}
}

// normName normalizes a book name for comparison.
func normName(name string) string {
	return strings.TrimSpace(name)
}

// sameBook checks if two search results refer to the same book.
// Uses exact normalized name match. If both have non-empty author, requires author match too.
func sameBook(a, b SearchResult) bool {
	if normName(a.Name) != normName(b.Name) {
		return false
	}
	if a.Author != "" && b.Author != "" && a.Author != b.Author {
		return false
	}
	return true
}

// mergeKey returns the dedup key for a search result.
// ponytail: exact name match only. Add edit-distance if false negatives are reported.
func mergeKey(r SearchResult) string {
	key := normName(r.Name)
	if r.Author != "" {
		key += "|" + r.Author
	}
	return key
}

// mergeAndSort takes a flat list of search results and returns merged+scored groups.
// Within each group the best-scored item (richest metadata) serves as primary;
// remaining sources become AlternateSources.
// Groups are sorted by score descending (tiebreak: shorter name = likely more relevant).
// ponytail: single-pass merge — O(n) map + O(k log k) sort on merged groups.
func MergeAndSort(query string, results []SearchResult) []SearchResult {
	if len(results) == 0 {
		return results
	}

	// Score each result (if not already scored)
	for i := range results {
		if results[i].Score == 0 {
			results[i].Score = scoreResult(query, results[i].Name)
		}
	}

	type group struct {
		primary SearchResult
		alts    []AltSource
	}
	groups := make(map[string]*group)

	for _, r := range results {
		key := mergeKey(r)
		g, ok := groups[key]
		if !ok {
			groups[key] = &group{primary: r}
			continue
		}

		// Is this result a better primary? (higher score, or same score but richer metadata)
		if r.Score > g.primary.Score ||
			(r.Score == g.primary.Score && len(r.CoverURL) > len(g.primary.CoverURL)) {
			// Demote current primary to alternate
			g.alts = append(g.alts, AltSource{
				SourceURL:  g.primary.SourceURL,
				BookURL:    g.primary.BookURL,
				SourceName: g.primary.SourceName,
			})
			g.primary = r
		} else {
			g.alts = append(g.alts, AltSource{
				SourceURL:  r.SourceURL,
				BookURL:    r.BookURL,
				SourceName: r.SourceName,
			})
		}
	}

	// Flatten groups
	merged := make([]SearchResult, 0, len(groups))
	for _, g := range groups {
		if len(g.alts) > 0 {
			g.primary.AlternateSources = g.alts
		}
		merged = append(merged, g.primary)
	}

	// Sort by score desc, tiebreak by name length asc (shorter = more focused)
	sort.SliceStable(merged, func(i, j int) bool {
		if merged[i].Score != merged[j].Score {
			return merged[i].Score > merged[j].Score
		}
		return len(merged[i].Name) < len(merged[j].Name)
	})

	return merged
}
