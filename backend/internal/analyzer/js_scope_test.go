// Conformance test for isolated JavaScript source evaluation scope.
package analyzer

import "testing"

func TestJSEvalAllowsRepeatedLetDeclarations(t *testing.T) {
	vm := NewJSVM()
	for i := 0; i < 2; i++ {
		value, err := vm.Eval(`let url = "https://example.test"; url`, "", "https://example.test")
		if err != nil {
			t.Fatal(err)
		}
		if got := ToString(value); got != "https://example.test" {
			t.Fatalf("value=%q", got)
		}
	}
}
