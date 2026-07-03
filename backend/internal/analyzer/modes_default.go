package analyzer

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// defaultQuery evaluates a legado Default-format rule against HTML.
// Format: type.name.position@type.name.position@getter
// Examples:
//
//	class.odd.0@tag.a.0@text           — 1st .odd, then 1st a, get text
//	tag.dd.0@tag.h1@text##全文阅读      — with ## replace
//	id.jieqi_page_contents@class.c_row  — by ID, then .c_row elements
//	td.2@text                           — 3rd td, get text
//	td.-1@text                          — last td, get text
//	tbody>tr@ a.0@href                  — CSS-like tbody>tr, then 1st a href
//	text                                — standalone getter on current element
//	@href                               — attribute getter on current element
//
// ponytail: this implements the most common Default patterns. Position ranges
// like [start:end:step] and multi-index [0,2,4] are deferred — most sources
// use simple index or negative index.
func defaultQuery(html, expr string) (string, error) {
	parts, getter, err := parseDefault(expr)
	if err != nil {
		return "", err
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return "", fmt.Errorf("default: parse: %w", err)
	}

	sel := doc.Selection
	for _, part := range parts {
		sel = applyDefaultSegment(sel, part)
		if sel.Length() == 0 {
			return "", nil
		}
	}

	// When no selector segments, apply getter to first real element (body's first child),
	// not the document root — so @href on <a href="/ch">text</a> returns "/ch".
	if len(parts) == 0 && getter != "" {
		sel = doc.Find("body").Children().First()
		if sel.Length() == 0 {
			// Fallback: use the first non-html element
			sel = doc.Selection.Children().First()
		}
	}

	return extractDefaultGetter(sel, getter), nil
}

// defaultQueryList evaluates a Default rule returning multiple string results.
func defaultQueryList(html, expr string) ([]string, error) {
	parts, getter, err := parseDefault(expr)
	if err != nil {
		return nil, err
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, fmt.Errorf("default: parse: %w", err)
	}

	sel := doc.Selection
	for _, part := range parts {
		sel = applyDefaultSegment(sel, part)
		if sel.Length() == 0 {
			return nil, nil
		}
	}

	var results []string
	sel.Each(func(i int, s *goquery.Selection) {
		v := extractDefaultGetter(s, getter)
		if v != "" {
			results = append(results, v)
		}
	})
	return results, nil
}

// defaultQueryElements returns elements for further chaining.
// Returns outer HTML so field rules (chapterName, chapterUrl) can match the
// selected element itself (e.g., <a href="...">text</a>), not just children.
func defaultQueryElements(html, expr string) ([]interface{}, error) {
	parts, _, err := parseDefault(expr)
	if err != nil {
		return nil, err
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, fmt.Errorf("default: parse: %w", err)
	}

	sel := doc.Selection
	for _, part := range parts {
		sel = applyDefaultSegment(sel, part)
		if sel.Length() == 0 {
			return nil, nil
		}
	}

	var results []interface{}
	for i := 0; i < sel.Length(); i++ {
		// ponytail: OuterHtml preserves the element tag and attributes,
		// so field rules like @href and text work against the selected node.
		h, _ := goquery.OuterHtml(sel.Eq(i))
		results = append(results, h)
	}
	return results, nil
}

// defaultSegment describes one segment of a Default rule.
type defaultSegment struct {
	selType string // "tag", "class", "id", "text", "children", "css"
	selVal  string // the tag name, class name, id, or text content
	noIndex bool   // true if no index was specified (return all elements)
	index   int    // specific index to pick (negative = from end)
	exclude []int  // indices to exclude
}

// parseDefault splits a Default expression into segments and getter.
// Format: part@part@...@getter
// The last @-separated part is the getter only if it's a known getter keyword.
// Otherwise, all parts are selector segments and text is the implicit getter.
//
// Special cases:
//   - "text" (bare getter): no segments, getter="text" — apply to current element
//   - "@href" (getter on current element): no segments, getter="href"
func parseDefault(expr string) ([]defaultSegment, string, error) {
	parts := strings.Split(expr, "@")
	if len(parts) < 2 {
		// Single part: could be a standalone getter keyword
		// (text, ownText, href, src, html, all, children, textNodes)
		if isDefaultGetter(parts[0]) {
			return nil, parts[0], nil // no selector segments, apply getter to root
		}
		return []defaultSegment{{selType: "css", selVal: expr}}, "", nil
	}

	// Last part is the getter ONLY if it's a known getter keyword
	last := strings.TrimSpace(parts[len(parts)-1])
	segParts := parts
	getter := ""
	if isDefaultGetter(last) {
		getter = last
		segParts = parts[:len(parts)-1]
	}

	var segments []defaultSegment
	for _, part := range segParts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue // empty segment like @href (before-@ is empty) — apply getter to root
		}
		seg, err := parseDefaultSegment(part)
		if err != nil {
			return nil, "", fmt.Errorf("default: parse segment %q: %w", part, err)
		}
		segments = append(segments, seg)
	}

	return segments, getter, nil
}

// parseDefaultSegment parses one segment like "class.odd.0" or "tag.a" or "td.2"
func parseDefaultSegment(seg string) (defaultSegment, error) {
	seg = strings.TrimSpace(seg)
	if seg == "" {
		return defaultSegment{}, fmt.Errorf("empty segment")
	}

	// Check for <js> or @js: inline — passthrough as CSS (handled by caller)
	if strings.HasPrefix(seg, "<js>") || strings.HasPrefix(seg, "@js:") {
		return defaultSegment{selType: "css", selVal: seg}, nil
	}

	// Handle pure CSS selectors embedded in Default mode (e.g. tbody>tr)
	// If it contains >, +, ~, :, # — it's CSS
	if strings.ContainsAny(seg, ">+~:#,") {
		return defaultSegment{selType: "css", selVal: seg}, nil
	}

	// Split on dots
	dotParts := strings.Split(seg, ".")

	// First part is the type
	typePart := dotParts[0]
	isKnownType := false
	switch typePart {
	case "class", "id", "tag", "text", "children":
		isKnownType = true
	}

	if !isKnownType {
		// Implicit tag: "td.2" → tag=td, index=2
		// Only treat as tag if followed by a numeric index
		if len(dotParts) >= 2 {
			last := dotParts[len(dotParts)-1]
			if isAllDigits(last) || last == "-1" {
				// Implicit tag: treat first part as tag name
				typePart = "tag"
				isKnownType = true
				// Re-arrange: dotParts[0] is the tag name, rest after is index
				// e.g., "td.2" → type=tag, name=td, index=2
				// Keep dotParts as-is and handle below
			} else if strings.ContainsAny(seg, ">+~:#,") {
				// CSS selector with combinators
				return defaultSegment{selType: "css", selVal: seg}, nil
			}
		}
		if !isKnownType {
			return defaultSegment{selType: "css", selVal: seg}, nil
		}
	}

	if typePart == "children" {
		return defaultSegment{selType: "children"}, nil
	}

	s := defaultSegment{selType: typePart, noIndex: true, index: 0}

	switch len(dotParts) {
	case 1:
		// Just the type: "tag" or "children" with no further spec
		if typePart == "children" {
			// children needs no name
		} else if typePart == "tag" {
			// "tag" with no name means all tags? Unusual, treat as CSS
			s.selType = "css"
			s.selVal = "*"
		}

	case 2:
		// type.name or type.position
		second := dotParts[1]
		if isAllDigits(second) || strings.HasPrefix(second, "!") {
			// type.position: e.g., "td.2" or "tr.-1"
			s.selVal = dotParts[0]
			s.index, _ = strconv.Atoi(second)
			s.noIndex = false
			if strings.HasPrefix(second, "!") {
				s.noIndex = true
				s.exclude = append(s.exclude, parseIntOrZero(second[1:]))
			}
		} else {
			// type.name: e.g., "class.odd"
			s.selVal = second
		}

	case 3:
		// type.name.position: e.g., "class.odd.0"
		s.selVal = dotParts[1]
		posStr := dotParts[2]
		if isAllDigits(posStr) || strings.HasPrefix(posStr, "!") {
			s.index, _ = strconv.Atoi(posStr)
			s.noIndex = false
			if strings.HasPrefix(posStr, "!") {
				s.noIndex = true
				s.exclude = append(s.exclude, parseIntOrZero(posStr[1:]))
			}
		}

	default:
		// 4+ parts — unexpected, treat as CSS fallback
		return defaultSegment{selType: "css", selVal: seg}, nil
	}

	return s, nil
}

// parseIntOrZero parses an int string, returning 0 on error.
func parseIntOrZero(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return n
}

// applyDefaultSegment applies one segment to a goquery selection.
func applyDefaultSegment(sel *goquery.Selection, seg defaultSegment) *goquery.Selection {
	switch seg.selType {
	case "children":
		return sel.Children()
	case "class":
		if seg.selVal != "" {
			sel = sel.Find("." + seg.selVal)
		}
	case "id":
		if seg.selVal != "" {
			sel = sel.Find("#" + seg.selVal)
		}
	case "tag":
		if seg.selVal != "" {
			sel = sel.Find(seg.selVal)
		}
	case "text":
		if seg.selVal != "" {
			sel = sel.Find(":contains('" + seg.selVal + "')")
		}
	case "css":
		sel = sel.Find(seg.selVal)
	}

	if sel.Length() == 0 {
		return sel
	}

	// Apply index/exclude
	if !seg.noIndex && len(seg.exclude) == 0 {
		// Resolve negative indices (e.g. -1 = last, -2 = second-to-last)
		idx := seg.index
		if idx < 0 {
			idx = sel.Length() + idx
		}
		if idx >= 0 && idx < sel.Length() {
			return sel.Eq(idx)
		}
		return sel
	}

	// Apply exclusions
	if len(seg.exclude) > 0 {
		excludeSet := make(map[int]bool)
		for _, e := range seg.exclude {
			excludeSet[e] = true
		}
		var filtered []int
		for i := 0; i < sel.Length(); i++ {
			if !excludeSet[i] {
				filtered = append(filtered, i)
			}
		}
		if len(filtered) == 0 {
			return sel
		}
		return sel.Eq(filtered[0])
	}

	return sel
}

// extractDefaultGetter extracts a value from a selection based on the getter type.
func extractDefaultGetter(sel *goquery.Selection, getter string) string {
	if getter == "" {
		return strings.TrimSpace(sel.Text())
	}

	switch getter {
	case "text":
		return strings.TrimSpace(sel.First().Text())
	case "textNodes":
		// textNodes gets only direct text, not descendant text
		var buf strings.Builder
		sel.Contents().Each(func(i int, s *goquery.Selection) {
			if goquery.NodeName(s) == "#text" {
				buf.WriteString(s.Text())
			}
		})
		return strings.TrimSpace(buf.String())
	case "ownText":
		return strings.TrimSpace(sel.First().Text())
	case "html":
		v, _ := sel.Html()
		return strings.TrimSpace(v)
	case "all":
		v, _ := sel.Html()
		return strings.TrimSpace(v)
	case "href":
		v, _ := sel.First().Attr("href")
		return v
	case "src":
		v, _ := sel.First().Attr("src")
		return v
	default:
		// Try as attribute name
		v, _ := sel.First().Attr(getter)
		if v != "" {
			return v
		}
		return strings.TrimSpace(sel.First().Text())
	}
}
