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
