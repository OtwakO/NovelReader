// Lenient map parsing supports Legado's JavaScript-style source configuration objects.
package analyzer

import "strings"

// ParseLenientStringMap accepts quoted JSON-like string maps, including single quotes.
func ParseLenientStringMap(raw string) (map[string]string, bool) {
	raw = strings.TrimSpace(raw)
	if len(raw) < 2 || raw[0] != '{' || raw[len(raw)-1] != '}' {
		return nil, false
	}
	body := raw[1 : len(raw)-1]
	values := make(map[string]string)
	for index := 0; ; {
		index = skipRuleSpace(body, index)
		if index >= len(body) {
			return values, true
		}
		key, next, ok := readStringMapKey(body, index)
		if !ok {
			return nil, false
		}
		index = skipRuleSpace(body, next)
		if index >= len(body) || body[index] != ':' {
			return nil, false
		}
		value, next, ok := readQuotedStringMapPart(body, skipRuleSpace(body, index+1))
		if !ok {
			return nil, false
		}
		values[key] = value
		index = skipRuleSpace(body, next)
		if index >= len(body) {
			return values, true
		}
		if body[index] != ',' {
			return nil, false
		}
		index++
	}
}

func readStringMapKey(value string, index int) (string, int, bool) {
	if index < len(value) && (value[index] == '\'' || value[index] == '"') {
		return readQuotedStringMapPart(value, index)
	}
	start := index
	for index < len(value) && (value[index] == '_' || value[index] == '-' || value[index] >= '0' && value[index] <= '9' || value[index] >= 'A' && value[index] <= 'Z' || value[index] >= 'a' && value[index] <= 'z') {
		index++
	}
	if index == start {
		return "", index, false
	}
	return value[start:index], index, true
}

func readQuotedStringMapPart(value string, index int) (string, int, bool) {
	if index >= len(value) || value[index] != '\'' && value[index] != '"' {
		return "", index, false
	}
	quote := value[index]
	var output strings.Builder
	for index++; index < len(value); index++ {
		char := value[index]
		if char == quote {
			return output.String(), index + 1, true
		}
		if char == '\\' && index+1 < len(value) {
			index++
			switch value[index] {
			case 'n':
				output.WriteByte('\n')
			case 'r':
				output.WriteByte('\r')
			case 't':
				output.WriteByte('\t')
			case '\\', '\'', '"':
				output.WriteByte(value[index])
			default:
				output.WriteByte('\\')
				output.WriteByte(value[index])
			}
			continue
		}
		output.WriteByte(char)
	}
	return "", index, false
}
