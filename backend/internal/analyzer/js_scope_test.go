// Conformance test for isolated JavaScript source evaluation scope.
package analyzer

import "testing"

func TestJSLibFunctionsShareTheEvaluationGlobalObject(t *testing.T) {
	vm := NewJSVM()
	script := `
function baseURL() { return 'https://example.test'; }
function request(path) { return this.baseURL() + path; }
let mode = 'search';
request('/' + mode)`
	value, err := vm.Eval(script, "", "https://example.test")
	if err != nil {
		t.Fatal(err)
	}
	if got := ToString(value); got != "https://example.test/search" {
		t.Fatalf("value=%q", got)
	}
}

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
