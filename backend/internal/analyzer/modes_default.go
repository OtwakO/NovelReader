package analyzer

import (
	"fmt"
	"regexp"
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

	html, rootSelector := contextualizeHTMLSelection(html)
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
		sel = doc.Find(rootSelector).First()
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

	html, _ = contextualizeHTMLSelection(html)
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

	html, _ = contextualizeHTMLSelection(html)
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
	selType    string // "tag", "class", "id", "text", "children", "css"
	selVal     string // the tag name, class name, id, or text content
	noIndex    bool   // true if no index was specified (return all elements)
	index      int    // specific index to pick (negative = from end)
	exclude    []int  // indices to exclude
	rangeStart *int   // inclusive positional range start
	rangeEnd   *int   // exclusive positional range end
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
		// A single explicit Default selector such as
		// `class.foo bar` is still a selector, not CSS.
		if seg, parseErr := parseDefaultSegment(parts[0]); parseErr == nil && (seg.selType != "css" || len(seg.exclude) > 0 || seg.rangeStart != nil || seg.rangeEnd != nil) {
			return []defaultSegment{seg}, "", nil
		}
		return []defaultSegment{{selType: "css", selVal: expr, noIndex: true}}, "", nil
	}

	// Last part is the getter ONLY if it's a known getter keyword
	last := strings.TrimSpace(parts[len(parts)-1])
	segParts := parts
	getter := ""
	if isDefaultGetter(last) || (len(parts) == 2 && (strings.TrimSpace(parts[0]) == "" ||
		(hasNumericSelectorSuffix(strings.TrimSpace(parts[0])) && !isDefaultElementSelector(last)))) {
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
	if base, start, end, ok := cutDefaultRange(seg); ok {
		parsed, err := parseDefaultSegment(base)
		if err != nil {
			return defaultSegment{}, err
		}
		parsed.noIndex = true
		parsed.rangeStart = start
		parsed.rangeEnd = end
		return parsed, nil
	}
	if base, index, ok := cutDefaultExclusion(seg); ok {
		parsed, err := parseDefaultSegment(base)
		if err != nil {
			return defaultSegment{}, err
		}
		parsed.noIndex = true
		parsed.exclude = []int{index}
		return parsed, nil
	}

	// Check for <js> or @js: inline — passthrough as CSS (handled by caller)
	if strings.HasPrefix(seg, "<js>") || strings.HasPrefix(seg, "@js:") {
		return defaultSegment{selType: "css", selVal: seg, noIndex: true}, nil
	}

	// Handle pure CSS selectors embedded in Default mode (e.g. tbody>tr)
	// If it contains >, +, ~, :, # — it's CSS
	if strings.ContainsAny(seg, ">+~:#,") {
		return defaultSegment{selType: "css", selVal: seg, noIndex: true}, nil
	}

	// Legacy shorthand `.class.N` selects the Nth matching class.
	if strings.HasPrefix(seg, ".") {
		dotParts := strings.Split(seg, ".")
		if len(dotParts) == 3 && dotParts[1] != "" && isAllDigits(dotParts[2]) {
			index, _ := strconv.Atoi(dotParts[2])
			return defaultSegment{selType: "class", selVal: dotParts[1], index: index, noIndex: false}, nil
		}
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
				return defaultSegment{selType: "css", selVal: seg, noIndex: true}, nil
			}
		}
		if !isKnownType {
			return defaultSegment{selType: "css", selVal: seg, noIndex: true}, nil
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
		return defaultSegment{selType: "css", selVal: seg, noIndex: true}, nil
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

var defaultRange = regexp.MustCompile(`^(.*)\[(-?[0-9]*):(-?[0-9]*)\]$`)

func cutDefaultRange(segment string) (string, *int, *int, bool) {
	match := defaultRange.FindStringSubmatch(segment)
	if len(match) != 4 || strings.TrimSpace(match[1]) == "" || (match[2] == "" && match[3] == "") {
		return segment, nil, nil, false
	}
	parseBound := func(value string) (*int, bool) {
		if value == "" {
			return nil, true
		}
		bound, err := strconv.Atoi(value)
		return &bound, err == nil
	}
	start, startOK := parseBound(match[2])
	end, endOK := parseBound(match[3])
	return strings.TrimSpace(match[1]), start, end, startOK && endOK
}

var defaultExclusion = regexp.MustCompile(`^(.*)!(-?[0-9]+)$`)

func cutDefaultExclusion(segment string) (string, int, bool) {
	match := defaultExclusion.FindStringSubmatch(segment)
	if len(match) != 3 || strings.TrimSpace(match[1]) == "" {
		return segment, 0, false
	}
	index, err := strconv.Atoi(match[2])
	return strings.TrimSpace(match[1]), index, err == nil
}

// applyDefaultSegment traverses each current parent independently so positional
// selectors retain one match per book/card container, as Legado does.
func applyDefaultSegment(parents *goquery.Selection, seg defaultSegment) *goquery.Selection {
	result := parents.Slice(0, 0).Clone()
	parents.Each(func(_ int, parent *goquery.Selection) {
		selected := selectDefaultDescendants(parent, seg)
		selected = applyDefaultPosition(selected, seg)
		result = result.AddSelection(selected)
	})
	return result
}

func selectDefaultDescendants(sel *goquery.Selection, seg defaultSegment) *goquery.Selection {
	switch seg.selType {
	case "children":
		return sel.Children()
	case "class":
		classes := strings.Fields(seg.selVal)
		if len(classes) > 0 {
			return sel.Find("." + strings.Join(classes, "."))
		}
	case "id":
		if seg.selVal != "" {
			return sel.Find("#" + seg.selVal)
		}
	case "tag":
		if seg.selVal != "" {
			return sel.Find(seg.selVal)
		}
	case "text":
		if seg.selVal != "" {
			return sel.Find(":contains('" + seg.selVal + "')")
		}
	case "css":
		return applyDefaultCSS(sel, seg.selVal)
	}
	return sel.Slice(0, 0)
}

func applyDefaultPosition(sel *goquery.Selection, seg defaultSegment) *goquery.Selection {
	if sel.Length() == 0 {
		return sel
	}
	if seg.rangeStart != nil || seg.rangeEnd != nil {
		start, end := 0, sel.Length()
		if seg.rangeStart != nil {
			start = resolveDefaultIndex(*seg.rangeStart, sel.Length())
		}
		if seg.rangeEnd != nil {
			end = resolveDefaultIndex(*seg.rangeEnd, sel.Length())
		}
		start = max(0, min(start, sel.Length()))
		end = max(start, min(end, sel.Length()))
		return sel.Slice(start, end)
	}
	if !seg.noIndex && len(seg.exclude) == 0 {
		index := resolveDefaultIndex(seg.index, sel.Length())
		if index >= 0 && index < sel.Length() {
			return sel.Eq(index)
		}
		return sel.Slice(0, 0)
	}
	if len(seg.exclude) > 0 {
		excluded := make(map[int]bool, len(seg.exclude))
		for _, index := range seg.exclude {
			resolved := resolveDefaultIndex(index, sel.Length())
			if resolved >= 0 && resolved < sel.Length() {
				excluded[resolved] = true
			}
		}
		return sel.FilterFunction(func(index int, _ *goquery.Selection) bool { return !excluded[index] })
	}
	return sel
}

func resolveDefaultIndex(index, length int) int {
	if index < 0 {
		return length + index
	}
	return index
}

var defaultEqSelector = regexp.MustCompile(`^(.*):eq\((-?[0-9]+)\)(.*)$`)

func applyDefaultCSS(sel *goquery.Selection, selector string) *goquery.Selection {
	match := defaultEqSelector.FindStringSubmatch(selector)
	if len(match) != 4 {
		return sel.Find(selector)
	}
	selected := sel.Find(strings.TrimSpace(match[1]))
	index, _ := strconv.Atoi(match[2])
	if index < 0 {
		index = selected.Length() + index
	}
	if index < 0 || index >= selected.Length() {
		return selected.Filter("__never__")
	}
	selected = selected.Eq(index)
	if suffix := strings.TrimSpace(match[3]); suffix != "" {
		selected = selected.Find(suffix)
	}
	return selected
}

// extractDefaultGetter extracts a value from a selection based on the getter type.
func extractDefaultGetter(sel *goquery.Selection, getter string) string {
	if getter == "text" {
		var values []string
		sel.Each(func(_ int, item *goquery.Selection) {
			if value := strings.TrimSpace(item.Text()); value != "" {
				values = append(values, value)
			}
		})
		return strings.Join(values, "\n")
	}
	if getter != "" && getter != "textNodes" && getter != "ownText" && getter != "html" && getter != "all" {
		var values []string
		seen := make(map[string]struct{}, sel.Length())
		sel.Each(func(_ int, item *goquery.Selection) {
			value, _ := item.Attr(getter)
			if value == "" {
				return
			}
			if _, duplicate := seen[value]; duplicate {
				return
			}
			seen[value] = struct{}{}
			values = append(values, value)
		})
		return strings.Join(values, "\n")
	}
	if getter == "" {
		return strings.TrimSpace(sel.Text())
	}
	first := sel.First()
	switch getter {
	case "textNodes":
		var buf strings.Builder
		sel.Contents().Each(func(_ int, content *goquery.Selection) {
			if goquery.NodeName(content) == "#text" {
				buf.WriteString(content.Text())
			}
		})
		return strings.TrimSpace(buf.String())
	case "ownText":
		return strings.TrimSpace(first.Text())
	case "html", "all":
		value, _ := first.Html()
		return strings.TrimSpace(value)
	}
	return ""
}
