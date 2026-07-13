// Conformance tests for Legado Default selector index syntax.
package analyzer

import "testing"

func TestDefaultRuleSupportsEqAndDotIndexSelectors(t *testing.T) {
	html := `<div class="directoryArea"><p><a href="/wrong">wrong</a></p></div><div class="directoryArea"><p><a href="/one">one</a></p><p><a href="/two">two</a></p></div>`
	analyzer := New(html, "https://example.test/", NewJSVM(), nil)

	for _, rule := range []string{
		`.directoryArea:eq(1)@p@a`,
		`.directoryArea.1@p@a`,
	} {
		values, err := analyzer.GetStringList(rule)
		if err != nil {
			t.Fatalf("rule %q: %v", rule, err)
		}
		if len(values) != 2 || values[0] != "one" || values[1] != "two" {
			t.Fatalf("rule %q returned %#v", rule, values)
		}
	}
}
