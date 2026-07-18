package analyzer

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/PaesslerAG/jsonpath"
)

// jsonQueryElement evaluates JSONPath without flattening objects or arrays.
func jsonQueryElement(content, expr string) (interface{}, error) {
	root, err := parseJSONValue(content)
	if err != nil {
		return nil, err
	}
	return selectJSONValue(root, expr)
}

// jsonQuery evaluates a JSONPath expression and returns the result as a string.
func jsonQuery(content, expr string) (string, error) {
	root, err := parseJSONValue(content)
	if err != nil {
		return "", err
	}
	value, err := selectJSONValue(root, expr)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%v", value), nil
}

// jsonQueryList evaluates a JSONPath expression and returns a list of strings.
func jsonQueryList(content, expr string) ([]string, error) {
	root, err := parseJSONValue(content)
	if err != nil {
		return nil, err
	}
	value, err := selectJSONValue(root, expr)
	if err != nil {
		return nil, err
	}

	switch typed := value.(type) {
	case []interface{}:
		results := make([]string, 0, len(typed))
		for _, item := range typed {
			results = append(results, fmt.Sprintf("%v", item))
		}
		return results, nil
	case string:
		return []string{typed}, nil
	default:
		return []string{fmt.Sprintf("%v", value)}, nil
	}
}

// jsonQueryElements returns all JSONPath elements without truncating long TOCs.
func jsonQueryElements(content, expr string) ([]interface{}, error) {
	root, err := parseJSONValue(content)
	if err != nil {
		return nil, err
	}
	value, err := selectJSONValue(root, expr)
	if err != nil {
		return nil, err
	}

	switch typed := value.(type) {
	case []interface{}:
		return typed, nil
	case nil:
		return nil, nil
	default:
		return []interface{}{typed}, nil
	}
}

func parseJSONValue(content string) (interface{}, error) {
	var root interface{}
	if err := json.Unmarshal([]byte(content), &root); err != nil {
		return nil, fmt.Errorf("json: decode: %w", err)
	}
	return root, nil
}

func selectJSONValue(root interface{}, expr string) (interface{}, error) {
	value, err := jsonpath.Get(expr, root)
	if err == nil {
		return value, nil
	}
	if strings.HasPrefix(expr, "$.") || strings.HasPrefix(expr, "$[") {
		return nil, classifyJSONPathError(err)
	}
	value, fallbackErr := jsonpath.Get("$."+expr, root)
	if fallbackErr != nil {
		return nil, classifyJSONPathError(fallbackErr)
	}
	return value, nil
}

func classifyJSONPathError(err error) error {
	message := err.Error()
	if strings.HasPrefix(message, "unknown key ") || strings.HasPrefix(message, "index ") && strings.HasSuffix(message, " out of bounds") {
		return fmt.Errorf("%w: %s", ErrNoElements, message)
	}
	return fmt.Errorf("json: %w", err)
}
