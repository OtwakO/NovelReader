package book

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/otwako/novelreader/internal/analyzer"
	"github.com/otwako/novelreader/internal/booksource"
)

// parseRuleJSON parses a JSON rule object into a flat map of field → selector.
// Legado rules can be JSON objects or plain strings. This handles both.
// Handles nested objects by extracting string values recursively.
func parseRuleJSON(ruleJSON string) map[string]string {
	if ruleJSON == "" {
		return nil
	}

	// Try JSON object
	var obj map[string]interface{}
	if err := json.Unmarshal([]byte(ruleJSON), &obj); err == nil {
		result := make(map[string]string, len(obj))
		for k, v := range obj {
			if val := extractRuleValue(v); val != "" {
				result[k] = val
			}
		}
		return result
	}

	// Not JSON — treat the whole string as a content rule
	return map[string]string{"content": strings.TrimSpace(ruleJSON)}
}

// extractRuleValue recursively extracts string values from rule objects.
// Handles both simple strings and nested objects with string values.
func extractRuleValue(v interface{}) string {
	switch val := v.(type) {
	case string:
		return strings.TrimSpace(val)
	case map[string]interface{}:
		// Recursively extract from nested objects
		// Look for common rule fields: rule, selector, value, content, text
		for _, key := range []string{"rule", "selector", "value", "content", "text", "css"} {
			if nested, ok := val[key]; ok {
				if extracted := extractRuleValue(nested); extracted != "" {
					return extracted
				}
			}
		}
		// If no common field found, try to extract any string value
		for _, nested := range val {
			if extracted := extractRuleValue(nested); extracted != "" {
				return extracted
			}
		}
		return ""
	case nil:
		return ""
	default:
		// For other types (numbers, booleans), convert to string
		return toString(v)
	}
}

// parseHeaderJSON parses a header JSON string into a map.
// Format: {"User-Agent": "...", "Referer": "..."}
func parseHeaderJSON(headerJSON string) map[string]string {
	obj, _ := parseLiteralHeaders(headerJSON)
	return obj
}

func evaluateSourceHeaders(ctx context.Context, vm *analyzer.JSVM, source booksource.BookSource, state analyzer.SourceState) (map[string]string, error) {
	header := strings.TrimSpace(source.Header)
	if !strings.HasPrefix(strings.ToLower(header), "@js:") {
		return parseLiteralHeaders(header)
	}
	if vm == nil {
		return nil, fmt.Errorf("source header JavaScript engine unavailable")
	}
	bindings := analyzer.URLBindings(nil, source.BookSourceURL, state)
	bindings["source"] = sourceContext(source)
	value, err := analyzer.EvalURLScript(ctx, vm, header[4:], "", source.BookSourceURL, nil, bindings)
	if err != nil {
		return nil, err
	}
	return parseLiteralHeaders(analyzer.ToString(value))
}

func parseLiteralHeaders(header string) (map[string]string, error) {
	if header == "" {
		return nil, nil
	}
	var obj map[string]string
	if json.Unmarshal([]byte(header), &obj) == nil {
		return obj, nil
	}
	if obj, ok := analyzer.ParseLenientStringMap(header); ok {
		return obj, nil
	}
	return nil, fmt.Errorf("invalid source header map")
}

func toString(v interface{}) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	b, _ := json.Marshal(v)
	return string(b)
}
