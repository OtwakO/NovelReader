package analyzer

import "testing"

func TestJSVMForkStateUsesConfiguredDeviceIdentity(t *testing.T) {
	vm := NewJSVMWithPoolSize(1).ForkStateWithDeviceID("0123456789abcdef")
	value, err := vm.Eval(`[java.androidId(), java.deviceID()]`, "", "")
	if err != nil {
		t.Fatal(err)
	}
	values, ok := value.([]interface{})
	if !ok || len(values) != 2 || values[0] != "0123456789abcdef" || values[1] != "0123456789abcdef" {
		t.Fatalf("device identities=%#v", value)
	}
}

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
