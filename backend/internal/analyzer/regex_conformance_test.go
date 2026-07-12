// Conformance tests for Legado standalone regex rules.
package analyzer

import "testing"

func TestStandaloneRegexRuleUsesReplacementAndFirstMatchMarker(t *testing.T) {
	analyzer := New(`<meta author="忘语">`, "https://example.test/", NewJSVM(), nil)
	value, err := analyzer.GetString(`##author="([^"]+)"##$1###`)
	if err != nil {
		t.Fatal(err)
	}
	if value != "<meta 忘语>" {
		t.Fatalf("regex value = %q, want replacement without ### marker", value)
	}
}
