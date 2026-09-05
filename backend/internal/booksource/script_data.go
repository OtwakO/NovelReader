package booksource

import "encoding/json"

// ScriptData exposes the source definition to rule evaluation using its native
// BookSource field names. Each caller receives its own metadata map.
func (s BookSource) ScriptData() map[string]interface{} {
	body, _ := json.Marshal(s)
	var values map[string]interface{}
	_ = json.Unmarshal(body, &values)
	return values
}
