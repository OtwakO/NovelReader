package analyzer

import "testing"

func TestJSEvalCopiesPreparedSourceData(t *testing.T) {
	vm := NewJSVMWithPoolSize(1)
	sourceData := map[string]interface{}{
		"bookSourceName": "Fixture",
		"loginUi":        map[string]interface{}{"button": "Login"},
	}
	bindings := map[string]interface{}{"source": sourceData}

	if _, err := vm.EvalContext(t.Context(), `source.bookSourceName = "Changed"; source.loginUi.button = "Changed"`, "", "https://fixture.test", bindings); err != nil {
		t.Fatal(err)
	}
	value, err := vm.EvalContext(t.Context(), `source.bookSourceName + ':' + source.loginUi.button`, "", "https://fixture.test", bindings)
	if err != nil {
		t.Fatal(err)
	}
	if value != "Fixture:Login" {
		t.Fatalf("source metadata mutation leaked between evaluations: %v", value)
	}
}

func TestSourceIdentityAndVariablesSurvivePageChanges(t *testing.T) {
	vm := NewJSVMWithPoolSize(1)
	source := map[string]interface{}{"bookSourceUrl": "https://example.com"}
	first := &testSourceState{vars: map[string]string{}, memory: map[string]interface{}{}}
	other := &testSourceState{vars: map[string]string{}, memory: map[string]interface{}{}}
	bindings := map[string]interface{}{"source": source, "sourceState": first}
	if _, err := vm.EvalContext(t.Context(), `source.putVariable("saved")`, "", "https://example.com/search", bindings); err != nil {
		t.Fatal(err)
	}
	for _, page := range []string{"https://example.com/explore", "https://example.com/book/1/chapter/2"} {
		value, err := vm.EvalContext(t.Context(), `source.key + '|' + source.getKey() + '|' + source.getVariable()`, "", page, bindings)
		if err != nil || value != "https://example.com|https://example.com|saved" {
			t.Fatalf("page %s: value=%v err=%v", page, value, err)
		}
	}
	// The same source URL can belong to different installed sources/readers.
	bindings["sourceState"] = other
	value, err := vm.EvalContext(t.Context(), `source.getVariable()`, "", "https://example.com/book/1", bindings)
	if err != nil || value != "" {
		t.Fatalf("independent source state leaked: value=%v err=%v", value, err)
	}
}
