package analyzer

import (
	"fmt"
	"strings"

	"github.com/PaesslerAG/jsonpath"
)

// jsonQuery evaluates a JSONPath expression and returns the result as a string.
func jsonQuery(content, expr string) (string, error) {
	v, err := jsonpath.Get(expr, []byte(content))
	if err != nil {
		// Try without $ prefix (legado sometimes uses $. but goquery wants $.)
		if strings.HasPrefix(expr, "$.") || strings.HasPrefix(expr, "$[") {
			return "", fmt.Errorf("json: %w", err)
		}
		// Fallback: try with $. prefix
		v, err = jsonpath.Get("$."+expr, []byte(content))
		if err != nil {
			return "", fmt.Errorf("json: %w", err)
		}
	}
	return fmt.Sprintf("%v", v), nil
}

// jsonQueryList evaluates a JSONPath expression and returns a list of strings.
func jsonQueryList(content, expr string) ([]string, error) {
	v, err := jsonpath.Get(expr, []byte(content))
	if err != nil {
		if strings.HasPrefix(expr, "$.") || strings.HasPrefix(expr, "$[") {
			return nil, fmt.Errorf("json: %w", err)
		}
		v, err = jsonpath.Get("$."+expr, []byte(content))
		if err != nil {
			return nil, fmt.Errorf("json: %w", err)
		}
	}

	// Convert result to string list
	switch typed := v.(type) {
	case []interface{}:
		var results []string
		for _, item := range typed {
			results = append(results, fmt.Sprintf("%v", item))
		}
		return results, nil
	case string:
		// Could be JSON array string
		return []string{typed}, nil
	default:
		return []string{fmt.Sprintf("%v", v)}, nil
	}
}

// jsonQueryElements returns elements as interface{} list from JSONPath.
func jsonQueryElements(content, expr string) ([]interface{}, error) {
	v, err := jsonpath.Get(expr, []byte(content))
	if err != nil {
		if strings.HasPrefix(expr, "$.") || strings.HasPrefix(expr, "$[") {
			return nil, fmt.Errorf("json: %w", err)
		}
		v, err = jsonpath.Get("$."+expr, []byte(content))
		if err != nil {
			return nil, fmt.Errorf("json: %w", err)
		}
	}

	switch typed := v.(type) {
	case []interface{}:
		return typed, nil
	case interface{}:
		return []interface{}{typed}, nil
	default:
		return nil, fmt.Errorf("json: unexpected result type %T", v)
	}
}
