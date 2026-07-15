// Conformance tests for JavaScript-to-rule-engine helper methods.
package analyzer

import "testing"

func TestJavaHelpersReevaluateCurrentAnalyzer(t *testing.T) {
	analyzer := New(`<div class="book">First</div><div class="book">Second</div>`, "https://example.test/", NewJSVM(), nil)

	value, err := analyzer.jsEval(`java.getString('@css:.book@text')`, "")
	if err != nil || ToString(value) != "FirstSecond" {
		t.Fatalf("java.getString = %q, err=%v", ToString(value), err)
	}

	value, err = analyzer.jsEval(`java.getElements('@css:.book')`, "")
	if err != nil {
		t.Fatal(err)
	}
	if values, ok := value.([]interface{}); !ok || len(values) != 2 {
		t.Fatalf("java.getElements = %#v, want two elements", value)
	}

	if _, err := analyzer.jsEval(`java.setContent('<p>updated</p>'); java.getString('@css:p@text')`, ""); err != nil {
		t.Fatal(err)
	}
	value, err = analyzer.jsEval(`java.getString('@css:p@text')`, "")
	if err != nil || ToString(value) != "updated" {
		t.Fatalf("setContent result = %q, err=%v", ToString(value), err)
	}

	if _, err := analyzer.jsEval(`java.setContent({payload:{name:'structured'}})`, ""); err != nil {
		t.Fatal(err)
	}
	value, err = analyzer.GetString("$.payload.name")
	if err != nil || value != "structured" {
		t.Fatalf("structured setContent result = %q, err=%v", value, err)
	}
}
