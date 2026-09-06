package book

import (
	"sort"
	"strings"
)

// scoreResult ranks title matches ahead of author matches at the same specificity.
// ponytail: byte-prefix on UTF-8 is fine — rune boundaries are self-synchronizing.
func scoreResult(query, name, author string) int {
	q := strings.TrimSpace(query)
	if q == "" {
		return 0
	}

	n := normName(name)
	a := normAuthor(author)
	switch {
	case n == q:
		return 100
	case a == q:
		return 90
	case strings.HasPrefix(n, q):
		return 80
	case strings.HasPrefix(a, q):
		return 70
	case strings.Contains(n, q):
		return 60
	case strings.Contains(a, q):
		return 50
	default:
		return 20
	}
}

// normName normalizes a book name for comparison.
func normName(name string) string { return strings.TrimSpace(name) }

// authorPrefixes stripped from author before comparison. Different sources format
// author as "忘语", "作者：忘语", or "作者:忘语" — same book, different prefix.
var authorPrefixes = []string{"作者：", "作者:", "作\u0020者\u0020："}

// normAuthor normalizes author string for comparison.
func normAuthor(author string) string {
	a := strings.TrimSpace(author)
	for _, p := range authorPrefixes {
		if strings.HasPrefix(a, p) {
			a = strings.TrimSpace(strings.TrimPrefix(a, p))
			break
		}
	}
	return a
}

// sameBook checks if two search results refer to the same book.
// Single source of truth for merge — used both for grouping and guard.
// If both have non-empty authors and they differ, it's not the same book.
func sameBook(a, b SearchResult) bool {
	if normName(a.Name) != normName(b.Name) {
		return false
	}
	aa, ba := normAuthor(a.Author), normAuthor(b.Author)
	if aa != "" && ba != "" && aa != ba {
		return false
	}
	return true
}

// bindingFromSearchResult preserves source-returned display metadata without
// interpreting it as provider identity.
func bindingFromSearchResult(result SearchResult) AltSource {
	return AltSource{
		SourceID:     result.SourceID,
		SourceURL:    result.SourceURL,
		BookURL:      result.BookURL,
		SourceName:   result.SourceName,
		SourceGroup:  result.SourceGroup,
		Capabilities: append([]string(nil), result.Capabilities...),
		LastChapter:  result.LastChapter,
		VariableMap:  result.VariableMap,
	}
}

// MergeAndSort merges same-book results from different sources, sorts by relevance.
// Groups are keyed by normalized name. Within each name bucket, sameBook() prevents
// false merges (same name, different author).
func MergeAndSort(query string, results []SearchResult) []SearchResult {
	if len(results) == 0 {
		return results
	}

	// Score each result
	for i := range results {
		if results[i].Score == 0 {
			results[i].Score = scoreResult(query, results[i].Name, results[i].Author)
		}
	}

	type group struct {
		primary SearchResult
		alts    []AltSource
	}

	// Buckets keyed by normalized name; within a bucket, sameBook() disambiguates
	buckets := make(map[string][]*group)

	for _, r := range results {
		nameKey := normName(r.Name)
		matches := buckets[nameKey]

		// Scan existing groups in this bucket for a same-book match
		var matched *group
		for _, g := range matches {
			if sameBook(r, g.primary) {
				matched = g
				break
			}
		}

		if matched == nil {
			// New group
			buckets[nameKey] = append(matches, &group{primary: r})
			continue
		}

		// Merge into existing group: promote if better
		if r.Score > matched.primary.Score ||
			(r.Score == matched.primary.Score && len(r.CoverURL) > len(matched.primary.CoverURL)) {
			matched.alts = append(matched.alts, bindingFromSearchResult(matched.primary))
			matched.primary = r
		} else {
			matched.alts = append(matched.alts, bindingFromSearchResult(r))
		}
	}

	// Flatten all buckets
	var merged []SearchResult
	for _, groups := range buckets {
		for _, g := range groups {
			if len(g.alts) > 0 {
				g.primary.AlternateSources = g.alts
			}
			merged = append(merged, g.primary)
		}
	}

	// Sort by score desc, tiebreak by name length asc (shorter = tighter)
	sort.SliceStable(merged, func(i, j int) bool {
		if merged[i].Score != merged[j].Score {
			return merged[i].Score > merged[j].Score
		}
		return len(merged[i].Name) < len(merged[j].Name)
	})

	return merged
}
