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

// nextSegment extracts the next rule segment, splitting on || at top level.
func nextSegment(s string) (string, string, error) {
	depth := 0
	inJS := false
	for i := 0; i < len(s); i++ {
		switch {
		case i+3 < len(s) && s[i:i+4] == "<js>":
			inJS = true
			i += 3
		case i+4 < len(s) && s[i:i+5] == "</js>":
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
	// Extract ## replacement suffix before mode detection
	expression, replaceRegex, replacement, replaceFirst := extractReplaceSuffix(seg)

	// Determine mode
	mode := detectMode(expression, isJSON)
	return Rule{
		Mode:         mode,
		Expression:   expression,
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

	// Default: CSS
	return ModeCSS
}

// ponytail: {{...}} template interpolation (get-variable) not implemented here.
// It's handled during URL building in urlbuilder.go.
