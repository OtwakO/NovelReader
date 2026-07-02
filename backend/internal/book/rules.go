package book

import (
	"encoding/json"
	"strings"
)

// parseRuleJSON parses a JSON rule object into a flat map of field → selector.
// Legado rules can be JSON objects or plain strings. This handles both.
// ponytail: flat map extraction. If the rule is just a string, treat it as "bookList".
func parseRuleJSON(ruleJSON string) map[string]string {
	if ruleJSON == "" {
		return nil
	}

	// Try JSON object
	var obj map[string]interface{}
	if err := json.Unmarshal([]byte(ruleJSON), &obj); err == nil {
		result := make(map[string]string, len(obj))
		for k, v := range obj {
			switch val := v.(type) {
			case string:
				result[k] = val
			case map[string]interface{}:
				// ponytail: nested rule objects skipped for now
				continue
			default:
				result[k] = toString(val)
			}
		}
		return result
	}

	// Not JSON — treat the whole string as a content rule
	return map[string]string{"content": strings.TrimSpace(ruleJSON)}
}

// parseHeaderJSON parses a header JSON string into a map.
// Format: {"User-Agent": "...", "Referer": "..."}
func parseHeaderJSON(headerJSON string) map[string]string {
	if headerJSON == "" {
		return nil
	}
	var obj map[string]string
	if err := json.Unmarshal([]byte(headerJSON), &obj); err != nil {
		return nil
	}
	return obj
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
