package analyzer

import (
	"fmt"
	"strings"
)

// ParseRules splits a rule string into ordered Rule segments.
// Handles:
//   - Chain separator: || between independent rule segments
//   - JS blocks: <js>...</js> and @js:...
//   - Mode prefixes: @XPath:, @Json:, @CSS:
//   - Auto-detection: JSON ($., $[), XPath (/)
//   - ## suffix: ##pattern##replacement### (replace-first) on any segment
func ParseRules(ruleStr string, isJSON bool) ([]Rule, error) {
	if ruleStr == "" {
		return nil, fmt.Errorf("analyzer: empty rule")
	}

	var rules []Rule
	remaining := strings.TrimSpace(ruleStr)

	for remaining != "" {
		remaining = strings.TrimSpace(remaining)
		if remaining == "" {
			break
		}

		var seg string
		var err error
		seg, remaining, err = nextSegment(remaining)
		if err != nil {
			return nil, err
		}
		if seg == "" {
			continue
		}

		rule := buildRule(seg, isJSON)
		rules = append(rules, rule)
	}

	if len(rules) == 0 {
		return nil, fmt.Errorf("analyzer: no rules parsed from %q", ruleStr)
	}
	return rules, nil
}

// nextSegment extracts the next rule segment, splitting on:
//
//	|| — OR connector (top level only)
//	<js> — start of JS block in mid-chain (e.g. tag.li<js>result+='x'</js>)
//	</js> — end of JS block
func nextSegment(s string) (string, string, error) {
	depth := 0
	inJS := false
	for i := 0; i < len(s); i++ {
		switch {
		case i+3 < len(s) && strings.EqualFold(s[i:i+4], "@js:") && !inJS && depth == 0:
			pre := strings.TrimSpace(s[:i])
			if pre != "" {
				return pre, s[i:], nil
			}
			return strings.TrimSpace(s), "", nil
		case i+3 < len(s) && s[i:i+4] == "<js>":
			if !inJS && depth == 0 {
				// <js> acts as a segment boundary — return everything before it
				pre := strings.TrimSpace(s[:i])
				if pre != "" {
					return pre, s[i:], nil
				}
			}
			inJS = true
			i += 3
		case i+4 < len(s) && s[i:i+5] == "</js>":
			if inJS && depth == 0 {
				// </js> ends the JS block — return the whole <js>...</js> segment
				end := i + 5
				return s[:end], strings.TrimSpace(s[end:]), nil
			}
			inJS = false
			i += 4
		case s[i] == '{' && !inJS:
			depth++
		case s[i] == '}' && !inJS:
			depth--
		case s[i] == '|' && i+1 < len(s) && s[i+1] == '|' && depth == 0 && !inJS:
			return strings.TrimSpace(s[:i]), strings.TrimSpace(s[i+2:]), nil
		}
	}
	return strings.TrimSpace(s), "", nil
}

// buildRule creates a Rule from a single segment, determining mode and extracting ## suffix.
func buildRule(seg string, isJSON bool) Rule {
	template, putRules := extractPutRules(seg)
	expression, replaceRegex, replacement, replaceFirst := extractReplaceSuffix(template)
	mode := detectMode(expression, isJSON)
	hasGet := containsRuleGet(template)
	getIndex := strings.Index(strings.ToLower(template), "@get:{")
	replaceIndex := strings.Index(template, "##")
	literal := hasGet && mode != ModeJS && (replaceIndex < 0 || getIndex < replaceIndex)
	return Rule{
		Mode:         mode,
		Expression:   expression,
		Template:     template,
		PutRules:     putRules,
		HasGet:       hasGet,
		Literal:      literal,
		ReplaceRegex: replaceRegex,
		Replacement:  replacement,
		ReplaceFirst: replaceFirst,
	}
}

// extractReplaceSuffix extracts ##pattern##replacement### from the end of a rule.
// Legado format:
//
//	selector##regex##replacement     — replace all
//	selector##regex##replacement###  — replace first
//	selector##regex##               — replace with empty
func extractReplaceSuffix(seg string) (expression, regex, replacement string, replaceFirst bool) {
	// Must contain at least one ##
	idx := strings.Index(seg, "##")
	if idx == -1 {
		return seg, "", "", false
	}

	// Check if it's a ##-prefixed regex (pure regex rule)
	if strings.HasPrefix(seg, "##") {
		return seg, "", "", false
	}

	// Split on ## — the expression is everything before the first ##
	// The rest is ##regex##replacement###...
	expression = seg[:idx]
	suffix := seg[idx+2:]

	// If nothing after ##, return expression as-is
	if suffix == "" {
		return seg, "", "", false
	}

	// Split suffix on ## to get regex and optional replacement
	parts := strings.SplitN(suffix, "##", 3)
	regex = parts[0]
	if len(parts) >= 2 {
		replacement = parts[1]
	}
	if len(parts) >= 3 {
		replaceFirst = true
		// ### means third segment is also part of replacement
		if parts[2] == "" {
			replaceFirst = true
		}
	}

	// If regex is empty, no replacement
	if regex == "" {
		return expression, "", "", false
	}

	return expression, regex, replacement, replaceFirst
}

// detectMode determines the parsing mode from a rule expression prefix/content.
func detectMode(expr string, isJSON bool) Mode {
	// JS blocks
	if strings.HasPrefix(expr, "<js>") || strings.HasPrefix(expr, "@js:") {
		return ModeJS
	}
	// Standalone Legado regex/replacement rules use a leading ##.
	if strings.HasPrefix(expr, "##") {
		return ModeRegex
	}

	// Explicit prefixes
	upper := strings.ToUpper(expr)
	if strings.HasPrefix(upper, "@CSS:") {
		return ModeCSS
	}
	if strings.HasPrefix(upper, "@XPATH:") {
		return ModeXPath
	}
	if strings.HasPrefix(upper, "@JSON:") {
		return ModeJSON
	}

	// JSONPath detection
	if strings.HasPrefix(expr, "$.") || strings.HasPrefix(expr, "$[") {
		return ModeJSON
	}

	// XPath: starts with /
	if strings.HasPrefix(expr, "/") {
		return ModeXPath
	}

	// If content is JSON, default to JSON mode
	if isJSON {
		return ModeJSON
	}

	// Detect legado Default mode: class.odd.0@tag.a@text or tag.li or similar
	// Patterns that distinguish Default from CSS:
	// 1. Multiple @ separators (segment@segment@getter)
	// 2. A segment ending with .N where N is a number (position index)
	// 3. A @getter suffix where getter is a Default keyword
	// 4. Starts with a numeric index like [0], [-1:0], 0
	if looksLikeDefault(expr) {
		return ModeDefault
	}

	// Default: CSS
	return ModeCSS
}

// looksLikeDefault heuristically checks if expr is a legado Default rule.
func looksLikeDefault(expr string) bool {
	// A bare Default getter keyword (text, ownText, textNodes, href, src, html, all, children)
	// applied to the current element — legado treats these as Default mode.
	if isDefaultGetter(expr) {
		return true
	}

	// @getter or @attr on current element: @href, @text, @src, @data-id etc.
	// The @ prefix with no selector before it means "current element's attribute".
	if strings.HasPrefix(expr, "@") && !strings.ContainsAny(expr, " \t\n>+~:#,") {
		return true
	}

	// Multiple @ separators — definitely Default
	if strings.Count(expr, "@") > 1 {
		return true
	}

	n := strings.Count(expr, ".")

	// Single @: could be CSS (tag@attr) or Default (type.name@next)
	if atIdx := strings.LastIndex(expr, "@"); atIdx >= 0 {
		beforeAt := expr[:atIdx]
		afterAt := expr[atIdx+1:]

		// If after @ is a Default getter keyword AND before @ is structured → Default
		if isDefaultGetter(afterAt) {
			// But simple things like "a@text" are CSS unless they have Default structure
			if n > 0 || strings.ContainsAny(beforeAt, ">+~:#") {
				return true
			}
		}

		// CSS shorthand followed by an element selector is Legado Default
		// traversal (`#j@li`, `.book-list@a.0`), not a CSS attribute getter.
		if (strings.HasPrefix(beforeAt, ".") || strings.HasPrefix(beforeAt, "#")) && isDefaultElementSelector(afterAt) {
			return true
		}

		// If before @ has a Default type prefix (class., id., tag.)
		firstDot := strings.Index(beforeAt, ".")
		if firstDot > 0 {
			typePrefix := beforeAt[:firstDot]
			switch typePrefix {
			case "class", "id", "tag", "text", "children":
				return true
			}
		}

		// If before @ has multiple dots with a numeric end, it's Default
		if n >= 2 {
			segments := strings.Split(beforeAt, ".")
			last := segments[len(segments)-1]
			if isAllDigits(last) || last == "-1" || (strings.HasPrefix(last, "!") && len(last) > 1 && isAllDigits(last[1:])) {
				return true
			}
		}

		// Legacy class/index selectors such as .directoryArea:eq(1)@p@a
		// and .directoryArea.1@p@a are Default traversal rules.
		lastDot := strings.LastIndex(beforeAt, ".")
		if strings.Contains(beforeAt, ":eq(") || (strings.HasPrefix(beforeAt, ".") && lastDot > 0 && isAllDigits(beforeAt[lastDot+1:])) {
			return true
		}

		// before @ has #id selector — probably CSS
		// before @ is just a word + attr — probably CSS
		return false
	}

	// No @: check for Default patterns (multi-segment with type prefix, or numeric index)
	dotSegments := strings.Split(expr, ".")

	// Check type prefix
	typePrefix := dotSegments[0]
	switch typePrefix {
	case "class", "id":
		return len(dotSegments) >= 2
	case "tag", "text":
		// `tag.div` remains CSS-compatible because it is ambiguous with a
		// literal <tag class="div"> selector; indexed forms are Default.
		if len(dotSegments) >= 3 {
			return true
		}
	case "children":
		return true
	}

	// Array index syntax at start
	if strings.HasPrefix(expr, "[") && strings.Contains(expr, ":") {
		return true
	}

	return false
}

// isDefaultGetter returns true if s is a known Default-mode getter keyword.
func isDefaultElementSelector(s string) bool {
	name := strings.ToLower(strings.SplitN(s, ".", 2)[0])
	switch name {
	case "a", "article", "body", "dd", "div", "dl", "dt", "h1", "h2", "h3", "h4", "h5", "h6", "img", "li", "main", "ol", "p", "section", "span", "table", "tbody", "td", "th", "tr", "ul":
		return true
	}
	return false
}

func isDefaultGetter(s string) bool {
	switch s {
	case "text", "textNodes", "ownText", "href", "src", "html", "all", "children":
		return true
	}
	return false
}

// isAllDigits returns true if s is non-empty and consists only of digits.
// Accepts an optional leading minus sign for negative indices.
func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	start := 0
	if s[0] == '-' {
		if len(s) == 1 {
			return false
		}
		start = 1
	}
	for _, c := range s[start:] {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// ponytail: {{...}} template interpolation (get-variable) not implemented here.
// It's handled during URL building in urlbuilder.go.
