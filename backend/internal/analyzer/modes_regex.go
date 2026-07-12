package analyzer

import (
	"fmt"
	"regexp"
	"strings"
)

// regexQuery applies a regex pattern and returns the first match.
// Supports $0, $1, etc. capture group references.
// Supports ##pattern##replacement syntax.
func regexQuery(content, expr string) (string, error) {
	// If this is a replace pattern (##pattern##replacement)
	if strings.HasPrefix(expr, "##") {
		result, err := applyReplaceFromExpr(content, expr)
		if err != nil {
			return "", err
		}
		return result, nil
	}

	// Split && for multiple regex patterns chained together
	patterns := strings.Split(expr, "&&")
	var lastResult string = content

	for _, pattern := range patterns {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}
		result, err := applyRegex(lastResult, pattern)
		if err != nil {
			return "", err
		}
		lastResult = result
	}
	return lastResult, nil
}

// regexQueryList returns all matches from a regex.
func regexQueryList(content, expr string) ([]string, error) {
	re, err := regexp.Compile(expr)
	if err != nil {
		return nil, fmt.Errorf("regex: compile: %w", err)
	}
	matches := re.FindAllStringSubmatch(content, -1)
	var results []string
	for _, match := range matches {
		if len(match) > 1 {
			results = append(results, match[1]) // first capture group
		} else {
			results = append(results, match[0])
		}
	}
	return results, nil
}

// regexQueryElements returns capture groups as interface{}.
func regexQueryElements(content, expr string) ([]interface{}, error) {
	re, err := regexp.Compile(expr)
	if err != nil {
		return nil, fmt.Errorf("regex: compile: %w", err)
	}
	matches := re.FindAllStringSubmatch(content, -1)
	var results []interface{}
	for _, match := range matches {
		if len(match) > 1 {
			results = append(results, match[1])
		} else {
			results = append(results, match[0])
		}
	}
	return results, nil
}

// applyRegex applies a single regex pattern and returns the match.
// Pattern format: pattern or pattern##replacement
func applyRegex(content, expr string) (string, error) {
	// Check for replacement syntax: pattern##replacement
	parts := strings.Split(expr, "##")
	pattern := parts[0]

	re, err := regexp.Compile(pattern)
	if err != nil {
		return "", fmt.Errorf("regex: compile: %w", err)
	}

	if len(parts) >= 2 {
		replacement := parts[1]
		if len(parts) >= 3 {
			// $$ in replacement = $ (literal)
			replacement = parts[1]
			_ = parts[2] // ignore, third part is replacement when using ###
		}
		result := re.ReplaceAllString(content, replacement)
		return result, nil
	}

	// Find first match
	match := re.FindStringSubmatch(content)
	if len(match) > 1 {
		return match[1], nil
	}
	if len(match) == 1 {
		return match[0], nil
	}
	return "", nil
}

// applyReplaceFromExpr handles ##pattern##replacement expressions.
func applyReplaceFromExpr(content, expr string) (string, error) {
	// Remove leading ##
	inner := expr[2:]
	// Split on ## to get pattern and replacement
	parts := strings.SplitN(inner, "##", 2)
	if len(parts) < 2 {
		return content, nil
	}
	pattern := parts[0]
	replacement := parts[1]
	first := strings.HasSuffix(replacement, "###")
	if first {
		replacement = strings.TrimSuffix(replacement, "###")
	}
	return applyReplace(content, pattern, replacement, first)
}

// applyReplace applies a regex replacement.
func applyReplace(content, pattern, replacement string, first bool) (string, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return content, fmt.Errorf("regex: replace compile: %w", err)
	}
	if first {
		idx := re.FindStringIndex(content)
		if idx == nil {
			return content, nil
		}
		return content[:idx[0]] + re.ReplaceAllString(content[idx[0]:idx[1]], replacement) + content[idx[1]:], nil
	}
	return re.ReplaceAllString(content, replacement), nil
}
