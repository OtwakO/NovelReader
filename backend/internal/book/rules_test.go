package book

import (
	"testing"
)

func TestParseRuleJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected map[string]string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: nil,
		},
		{
			name:  "simple string",
			input: "css selector",
			expected: map[string]string{
				"content": "css selector",
			},
		},
		{
			name:  "flat JSON object",
			input: `{"content": "div.content", "title": "h1.title"}`,
			expected: map[string]string{
				"content": "div.content",
				"title":   "h1.title",
			},
		},
		{
			name:  "nested object with rule field",
			input: `{"content": {"rule": "div.content"}, "title": {"selector": "h1.title"}}`,
			expected: map[string]string{
				"content": "div.content",
				"title":   "h1.title",
			},
		},
		{
			name:  "nested object with value field",
			input: `{"content": {"value": "div.content"}}`,
			expected: map[string]string{
				"content": "div.content",
			},
		},
		{
			name:  "nested object with css field",
			input: `{"content": {"css": "div.content"}}`,
			expected: map[string]string{
				"content": "div.content",
			},
		},
		{
			name:  "deeply nested object",
			input: `{"content": {"selector": {"rule": "div.content"}}}`,
			expected: map[string]string{
				"content": "div.content",
			},
		},
		{
			name:  "mixed flat and nested",
			input: `{"content": "div.content", "title": {"rule": "h1.title"}, "author": {"value": "span.author"}}`,
			expected: map[string]string{
				"content": "div.content",
				"title":   "h1.title",
				"author":  "span.author",
			},
		},
		{
			name:  "JSON with string with spaces",
			input: `{"content": "  div.content  "}`,
			expected: map[string]string{
				"content": "div.content",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseRuleJSON(tt.input)
			if tt.expected == nil {
				if result != nil {
					t.Errorf("expected nil, got %v", result)
				}
				return
			}
			if len(result) != len(tt.expected) {
				t.Errorf("expected %d fields, got %d: %v", len(tt.expected), len(result), result)
				return
			}
			for k, v := range tt.expected {
				if result[k] != v {
					t.Errorf("field %q: expected %q, got %q", k, v, result[k])
				}
			}
		})
	}
}

func TestExtractRuleValue(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{
			name:     "string",
			input:    "test",
			expected: "test",
		},
		{
			name:     "string with spaces",
			input:    "  test  ",
			expected: "test",
		},
		{
			name:     "nil",
			input:    nil,
			expected: "",
		},
		{
			name:     "map with rule",
			input:    map[string]interface{}{"rule": "test"},
			expected: "test",
		},
		{
			name:     "map with selector",
			input:    map[string]interface{}{"selector": "test"},
			expected: "test",
		},
		{
			name:     "map with value",
			input:    map[string]interface{}{"value": "test"},
			expected: "test",
		},
		{
			name:     "map with content",
			input:    map[string]interface{}{"content": "test"},
			expected: "test",
		},
		{
			name:     "map with text",
			input:    map[string]interface{}{"text": "test"},
			expected: "test",
		},
		{
			name:     "map with css",
			input:    map[string]interface{}{"css": "test"},
			expected: "test",
		},
		{
			name:     "nested map",
			input:    map[string]interface{}{"selector": map[string]interface{}{"rule": "test"}},
			expected: "test",
		},
		{
			name:     "map with any string value",
			input:    map[string]interface{}{"custom": "test"},
			expected: "test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractRuleValue(tt.input)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}
