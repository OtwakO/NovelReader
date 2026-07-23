package analyzer

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/PaesslerAG/jsonpath"
)

var (
	errInvalidJSONInput = errors.New("json: invalid input")
	errInvalidJSONPath  = errors.New("json: invalid path")
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
	if value, matched, interpolationErr := interpolateJSON(root, expr); matched || interpolationErr != nil {
		return value, interpolationErr
	}
	value, err := selectJSONValue(root, expr)
	if err != nil {
		return "", err
	}
	if values, ok := value.([]interface{}); ok {
		result := make([]string, len(values))
		for index, item := range values {
			result[index] = ToString(item)
		}
		return strings.Join(result, "\n"), nil
	}
	return fmt.Sprintf("%v", value), nil
}

// jsonQueryList evaluates a JSONPath expression and returns a list of strings.
func jsonQueryList(content, expr string) ([]string, error) {
	root, err := parseJSONValue(content)
	if err != nil {
		return nil, err
	}
	if value, matched, interpolationErr := interpolateJSON(root, expr); matched || interpolationErr != nil {
		if interpolationErr != nil {
			return nil, interpolationErr
		}
		return []string{value}, nil
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
	if values, matched, err := filterObjectWildcard(content, expr); matched || err != nil {
		return values, err
	}
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

func filterObjectWildcard(content, expr string) ([]interface{}, bool, error) {
	const prefix = "$.*[?(@."
	if !strings.HasPrefix(expr, prefix) || !strings.HasSuffix(expr, ")]") {
		return nil, false, nil
	}
	key := strings.TrimSuffix(strings.TrimPrefix(expr, prefix), ")]")
	if key == "" || strings.ContainsAny(key, " []()?!@$") {
		return nil, false, nil
	}
	if !json.Valid([]byte(content)) {
		return nil, true, fmt.Errorf("%w: malformed document", errInvalidJSONInput)
	}
	decoder := json.NewDecoder(strings.NewReader(content))
	token, err := decoder.Token()
	if err != nil {
		return nil, true, fmt.Errorf("%w: %v", errInvalidJSONInput, err)
	}
	delimiter, ok := token.(json.Delim)
	if !ok || delimiter != '{' && delimiter != '[' {
		return nil, false, nil
	}
	var values []interface{}
	for decoder.More() {
		if delimiter == '{' {
			if _, err := decoder.Token(); err != nil {
				return nil, true, fmt.Errorf("%w: %v", errInvalidJSONInput, err)
			}
		}
		var item interface{}
		if err := decoder.Decode(&item); err != nil {
			return nil, true, fmt.Errorf("%w: %v", errInvalidJSONInput, err)
		}
		switch child := item.(type) {
		case map[string]interface{}:
			if hasJSONFilterValue(child, key) {
				values = append(values, item)
			}
		case []interface{}:
			for _, value := range child {
				if record, ok := value.(map[string]interface{}); ok && hasJSONFilterValue(record, key) {
					values = append(values, value)
				}
			}
		}
	}
	return values, true, nil
}

func hasJSONFilterValue(object map[string]interface{}, key string) bool {
	value, exists := object[key]
	return exists && value != nil && value != false && value != ""
}

func parseJSONValue(content string) (interface{}, error) {
	var root interface{}
	if err := json.Unmarshal([]byte(content), &root); err != nil {
		return nil, fmt.Errorf("%w: %v", errInvalidJSONInput, err)
	}
	return root, nil
}

func interpolateJSON(root interface{}, expression string) (string, bool, error) {
	var output strings.Builder
	matched := false
	for offset := 0; offset < len(expression); {
		start := strings.Index(expression[offset:], "{$")
		if start < 0 {
			output.WriteString(expression[offset:])
			break
		}
		start += offset
		if start > 0 && expression[start-1] == '{' {
			output.WriteString(expression[offset : start+2])
			offset = start + 2
			continue
		}
		end := strings.IndexByte(expression[start+2:], '}')
		if end < 0 {
			output.WriteString(expression[offset:])
			break
		}
		end += start + 2
		output.WriteString(expression[offset:start])
		value, err := selectJSONValue(root, expression[start+1:end])
		if err != nil {
			return "", true, err
		}
		output.WriteString(ToString(value))
		matched = true
		offset = end + 1
	}
	return output.String(), matched, nil
}

func selectJSONValue(root interface{}, expr string) (interface{}, error) {
	expr = normalizeDottedJSONWildcards(expr)
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

func normalizeDottedJSONWildcards(expr string) string {
	var normalized strings.Builder
	var quote byte
	for offset := 0; offset < len(expr); {
		if quote != 0 {
			if expr[offset] == '\\' && offset+1 < len(expr) {
				normalized.WriteString(expr[offset : offset+2])
				offset += 2
				continue
			}
			normalized.WriteByte(expr[offset])
			if expr[offset] == quote {
				quote = 0
			}
			offset++
			continue
		}
		if expr[offset] == '\'' || expr[offset] == '"' {
			quote = expr[offset]
			normalized.WriteByte(expr[offset])
			offset++
			continue
		}
		if strings.HasPrefix(expr[offset:], ".[*]") {
			normalized.WriteString("[*]")
			offset += len(".[*]")
			continue
		}
		normalized.WriteByte(expr[offset])
		offset++
	}
	return normalized.String()
}

func classifyJSONPathError(err error) error {
	message := err.Error()
	if strings.HasPrefix(message, "unknown key ") || strings.HasPrefix(message, "index ") && strings.HasSuffix(message, " out of bounds") ||
		strings.Contains(message, "unsupported value type <nil> for select") {
		return fmt.Errorf("%w: %s", ErrNoElements, message)
	}
	return fmt.Errorf("%w: %v", errInvalidJSONPath, err)
}
