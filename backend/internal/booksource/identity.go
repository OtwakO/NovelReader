package booksource

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

// DefinitionIdentity conservatively binds cached execution to the complete
// definition, including unknown source fields. Application timestamps are not
// source semantics. This is not a script dependency analyzer.
func (s BookSource) DefinitionIdentity() (string, error) {
	data, err := json.Marshal(s)
	if err != nil {
		return "", err
	}
	var definition map[string]any
	if err := json.Unmarshal(data, &definition); err != nil {
		return "", err
	}
	// Normalize known fields through their typed values: lossless export may
	// omit defaults until a timestamp edit materializes the typed representation.
	normalized, err := json.Marshal(bookSourceWire(s))
	if err != nil {
		return "", err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(normalized, &fields); err != nil {
		return "", err
	}
	for key := range ruleJSONFields {
		normalizeRuleField(fields, key)
	}
	for key := range bookSourceJSONFields {
		delete(definition, key)
	}
	for key, raw := range fields {
		var value any
		if err := json.Unmarshal(raw, &value); err != nil {
			return "", err
		}
		definition[key] = value
	}
	delete(definition, "createdAt")
	delete(definition, "updatedAt")
	canonical, err := json.Marshal(definition)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(canonical)
	return hex.EncodeToString(digest[:]), nil
}
