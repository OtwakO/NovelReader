// Conformance tests for Legado list connectors.
package analyzer

import "testing"

func TestGetStringListCombinesAndInterleavesLegadoRules(t *testing.T) {
	analyzer := New(`<div class="left">A1</div><div class="left">A2</div><div class="right">B1</div><div class="right">B2</div>`, "https://example.test/", NewJSVM(), nil)

	combined, err := analyzer.GetStringList(`@css:.left@text&&@css:.right@text`)
	if err != nil {
		t.Fatal(err)
	}
	if got := join(combined); got != "A1|A2|B1|B2" {
		t.Fatalf("&& result = %q", got)
	}

	interleaved, err := analyzer.GetStringList(`@css:.left@text%%@css:.right@text`)
	if err != nil {
		t.Fatal(err)
	}
	if got := join(interleaved); got != "A1|B1|A2|B2" {
		t.Fatalf("%% result = %q", got)
	}
}

func join(values []string) string {
	result := ""
	for i, value := range values {
		if i > 0 {
			result += "|"
		}
		result += value
	}
	return result
}
