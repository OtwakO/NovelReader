// Conformance tests for session-backed Legado JavaScript bindings.
package analyzer

import "testing"

type testSourceState struct {
	cookies map[string]string
	vars    map[string]string
	memory  map[string]interface{}
}

func (s *testSourceState) GetCookie(_, key string) string { return s.cookies[key] }
func (s *testSourceState) CookieHeader(string) string     { return "sid=fixture" }
func (s *testSourceState) SetCookie(_, key, value string) error {
	s.cookies[key] = value
	return nil
}
func (s *testSourceState) RemoveCookies(string) error              { s.cookies = map[string]string{}; return nil }
func (s *testSourceState) GetVariable(key string) string           { return s.vars[key] }
func (s *testSourceState) PutVariable(key, value string)           { s.vars[key] = value }
func (s *testSourceState) GetMemory(key string) interface{}        { return s.memory[key] }
func (s *testSourceState) PutMemory(key string, value interface{}) { s.memory[key] = value }

func TestJSVMBindsLegadoObjectsToSourceState(t *testing.T) {
	state := &testSourceState{
		cookies: map[string]string{"sid": "fixture"},
		vars:    map[string]string{},
		memory:  map[string]interface{}{},
	}
	vm := NewJSVM()
	bindings := map[string]interface{}{"sourceState": state}

	value, err := vm.Eval("cookie.getKey('https://example.test', 'sid')", "", "https://example.test/", bindings)
	if err != nil || value != "fixture" {
		t.Fatalf("cookie binding = %v, err=%v", value, err)
	}
	value, err = vm.Eval("source.putVariable('value'); source.put('saved', 'yes'); cache.putMemory('x', 'memory'); cache.put('legacy', 'cached'); cache.get('legacy')", "", "https://example.test/", bindings)
	if err != nil {
		t.Fatal(err)
	}
	if value != "cached" || state.vars["https://example.test/"] != "value" || state.memory["saved"] != "yes" || state.memory["x"] != "memory" || state.memory["legacy"] != "cached" {
		t.Fatalf("session state was not updated: value=%v vars=%v memory=%v", value, state.vars, state.memory)
	}
}
