package analyzer

import "testing"

func TestJSVMForkStateSharesExecutionButIsolatesMutableState(t *testing.T) {
	root := NewJSVMWithPoolSize(1)
	first := root.ForkState()
	second := root.ForkState()
	if first.executor != second.executor {
		t.Fatal("forks do not share the bounded executor")
	}
	if _, err := first.Eval(`java.put("secret", "alice")`, "", ""); err != nil {
		t.Fatal(err)
	}
	value, err := second.Eval(`java.get("secret")`, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if value != "" {
		t.Fatalf("second fork observed first state: %#v", value)
	}
}
