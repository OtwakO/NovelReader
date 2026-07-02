package booksource

import (
	"encoding/json"
	"fmt"
	"strings"
)

// unmarshalStrict decodes JSON with case-insensitive field matching (legado uses camelCase).
func unmarshalStrict(data []byte, v interface{}) error {
	dec := json.NewDecoder(strings.NewReader(string(data)))
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("booksource: decode: %w", err)
	}
	if dec.More() {
		return fmt.Errorf("booksource: trailing data after JSON value")
	}
	return nil
}

// MarshalRule converts a rule struct to JSON string for DB storage.
func MarshalRule(v interface{}) (string, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("booksource: marshal rule: %w", err)
	}
	return string(data), nil
}

// ImportSources parses a JSON array (or single object) of book sources.
// Returns the list of valid sources and any decode errors encountered.
func ImportSources(data []byte) ([]*BookSource, error) {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return nil, fmt.Errorf("booksource: empty import data")
	}

	var sources []*BookSource
	// Try array first, then single object.
	if err := json.Unmarshal([]byte(trimmed), &sources); err == nil {
		for _, s := range sources {
			if s.RespondTime == 0 {
				s.RespondTime = 180000
			}
		}
		return sources, nil
	}

	// Single source.
	single, err := NewFromJSON([]byte(trimmed))
	if err != nil {
		return nil, fmt.Errorf("booksource: import: %w", err)
	}
	return []*BookSource{single}, nil
}


