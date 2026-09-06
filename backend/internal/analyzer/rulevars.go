// Legado @put/@get variables are attached to parsed rule segments and evaluated in order.
package analyzer

import (
	"fmt"
	"strings"
)

func extractPutRules(rule string) (string, map[string]string) {
	var output strings.Builder
	values := make(map[string]string)
	lower := strings.ToLower(rule)
	start := 0
	for {
		index := strings.Index(lower[start:], "@put:")
		if index < 0 {
			output.WriteString(rule[start:])
			break
		}
		index += start
		output.WriteString(rule[start:index])
		open := index + len("@put:")
		for open < len(rule) && (rule[open] == ' ' || rule[open] == '\t' || rule[open] == '\n' || rule[open] == '\r') {
			open++
		}
		if open >= len(rule) || rule[open] != '{' {
			output.WriteString(rule[index : index+len("@put:")])
			start = index + len("@put:")
			continue
		}
		close := matchingBrace(rule, open)
		if close < 0 {
			output.WriteString(rule[index:])
			break
		}
		if parsed, ok := parseLenientRuleMap(rule[open : close+1]); ok {
			for key, value := range parsed {
				values[key] = value
			}
		}
		start = close + 1
	}
	if len(values) == 0 {
		values = nil
	}
	return strings.TrimSpace(output.String()), values
}

func matchingBrace(value string, open int) int {
	quote := byte(0)
	escaped := false
	for index := open + 1; index < len(value); index++ {
		char := value[index]
		if escaped {
			escaped = false
			continue
		}
		if char == '\\' && quote != 0 {
			escaped = true
			continue
		}
		if quote != 0 {
			if char == quote {
				quote = 0
			}
			continue
		}
		if char == '\'' || char == '"' {
			quote = char
			continue
		}
		if char == '}' {
			return index
		}
	}
	return -1
}

func parseLenientRuleMap(raw string) (map[string]string, bool) {
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
		key, next, ok := readRuleMapPart(body, index, ':')
		if !ok {
			return nil, false
		}
		index = skipRuleSpace(body, next)
		value, next, ok := readRuleMapPart(body, index, ',')
		if !ok {
			return nil, false
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" {
			return nil, false
		}
		values[key] = value
		index = next
		if index < len(body) && body[index] == ',' {
			index++
		}
	}
}

func skipRuleSpace(value string, index int) int {
	for index < len(value) && strings.ContainsRune(" \t\r\n", rune(value[index])) {
		index++
	}
	return index
}

func readRuleMapPart(value string, index int, delimiter byte) (string, int, bool) {
	index = skipRuleSpace(value, index)
	if index >= len(value) {
		return "", index, delimiter == ','
	}
	if value[index] == '\'' || value[index] == '"' {
		quote := value[index]
		var output strings.Builder
		for index++; index < len(value); index++ {
			char := value[index]
			if char == quote {
				index++
				index = skipRuleSpace(value, index)
				if delimiter == ':' {
					if index >= len(value) || value[index] != ':' {
						return "", index, false
					}
					index++
				}
				return output.String(), index, true
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

	start := index
	for index < len(value) && value[index] != delimiter {
		index++
	}
	if delimiter == ':' {
		if index >= len(value) {
			return "", index, false
		}
		return strings.TrimSpace(value[start:index]), index + 1, true
	}
	return strings.TrimSpace(value[start:index]), index, true
}

func containsRuleGet(rule string) bool {
	return strings.Contains(strings.ToLower(rule), "@get:{")
}

func (a *Analyzer) prepareRuleForEvaluation(rule Rule, content interface{}) (Rule, error) {
	for key, valueRule := range rule.PutRules {
		value, err := a.GetString(valueRule)
		if err != nil {
			return rule, fmt.Errorf("analyzer: @put %s: %w", key, err)
		}
		a.putRuleVariable(key, value)
	}
	if rule.HasGet {
		substituted := replaceRuleGets(rule.Template, a.getRuleVariable)
		rule.Expression, rule.ReplaceRegex, rule.Replacement, rule.ReplaceFirst = extractReplaceSuffix(substituted)
	}
	if strings.Contains(rule.Expression, "{{") {
		expanded, err := a.expandRuleTemplates(content, rule.Expression)
		if err != nil {
			return rule, err
		}
		rule.Expression = expanded
		if rule.Mode != ModeJS {
			rule.Literal = !strings.Contains(expanded, "{$")
		}
	}
	return rule, nil
}

func replaceRuleGets(rule string, get func(string) string) string {
	lower := strings.ToLower(rule)
	var output strings.Builder
	for {
		start := strings.Index(lower, "@get:{")
		if start < 0 {
			output.WriteString(rule)
			return output.String()
		}
		end := strings.Index(rule[start+len("@get:{"):], "}")
		if end < 0 {
			output.WriteString(rule)
			return output.String()
		}
		end += start + len("@get:{")
		output.WriteString(rule[:start])
		key := rule[start+len("@get:{") : end]
		output.WriteString(get(key))
		rule = rule[end+1:]
		lower = strings.ToLower(rule)
	}
}

func (a *Analyzer) putRuleVariable(key, value string) {
	variables := evaluationVariables{book: a.book, chapter: a.chapter, ruleData: a.ruleVars}
	variables.Put(key, value)
}

func (a *Analyzer) getRuleVariable(key string) string {
	variables := evaluationVariables{book: a.book, chapter: a.chapter, ruleData: a.ruleVars}
	return variables.Get(key)
}
