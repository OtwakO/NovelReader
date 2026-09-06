package candidate

import (
	"encoding/json"
	"errors"
)

// Variables cross the API as an opaque serialized string map. Validate without
// rewriting the snapshot: clients compare it verbatim when considering reuse.
// The HTTP boundary bounds the complete candidate request, including alternates.
func validateInputVariables(input Input) error {
	values := []string{input.VariableMap}
	for _, alternate := range input.AlternateSources {
		values = append(values, alternate.VariableMap)
	}
	for _, raw := range values {
		if raw == "" {
			continue
		}
		var variables map[string]interface{}
		if err := json.Unmarshal([]byte(raw), &variables); err != nil || variables == nil {
			return errors.New("variableMap must encode a JSON object of string values")
		}
		for _, value := range variables {
			if _, ok := value.(string); !ok {
				return errors.New("variableMap must encode a JSON object of string values")
			}
		}
	}
	return nil
}
