package book

import "strings"

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
