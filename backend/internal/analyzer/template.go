// Rule and URL templates share one brace-aware interpolation scanner.
package analyzer

import (
	"errors"
	"strings"
)

func (a *Analyzer) expandRuleTemplates(content interface{}, input string) (string, error) {
	return replaceTemplateExpressions(input, func(expression string) (string, error) {
		if isEmbeddedRule(expression) {
			current := *a
			current.content = content
			current.isJSON = looksLikeJSON(ToString(content))
			value, err := current.GetString(expression)
			if isEmbeddedJSONRule(expression) && errors.Is(err, errInvalidJSONInput) {
				return "", nil
			}
			return value, err
		}
		value, err := a.jsEval(expression, content)
		return ToString(value), err
	})
}

func isEmbeddedRule(expression string) bool {
	expression = strings.TrimSpace(expression)
	return strings.HasPrefix(expression, "@") || isEmbeddedJSONRule(expression) || strings.HasPrefix(expression, "//")
}

func isEmbeddedJSONRule(expression string) bool {
	expression = strings.TrimSpace(expression)
	return strings.HasPrefix(expression, "$.") || strings.HasPrefix(expression, "$[")
}

func replaceTemplateExpressions(input string, evaluate func(string) (string, error)) (string, error) {
	var output strings.Builder
	var firstErr error
	for offset := 0; offset < len(input); {
		relativeStart := strings.Index(input[offset:], "{{")
		if relativeStart < 0 {
			output.WriteString(input[offset:])
			break
		}
		start := offset + relativeStart
		output.WriteString(input[offset:start])
		innerStart := start + 2
		depth, end := 0, -1
		var quote byte
		escaped := false
		for index := innerStart; index < len(input); index++ {
			char := input[index]
			if quote != 0 {
				if escaped {
					escaped = false
				} else if char == '\\' {
					escaped = true
				} else if char == quote {
					quote = 0
				}
				continue
			}
			if char == '\'' || char == '"' || char == '`' {
				quote = char
				continue
			}
			switch {
			case index+1 < len(input) && input[index:index+2] == "{{":
				depth++
				index++
			case index+1 < len(input) && input[index:index+2] == "}}":
				if depth == 0 {
					end = index
					index = len(input)
				} else {
					depth--
					index++
				}
			case char == '{':
				depth++
			case char == '}':
				depth--
			}
		}
		if end < 0 {
			output.WriteString(input[start:])
			break
		}
		whole := input[start : end+2]
		inner := strings.TrimSpace(input[innerStart:end])
		if inner == "" {
			output.WriteString(whole)
		} else if value, err := evaluate(inner); err != nil {
			output.WriteString(whole)
			if firstErr == nil {
				firstErr = err
			}
		} else {
			output.WriteString(value)
		}
		offset = end + 2
	}
	return output.String(), firstErr
}
