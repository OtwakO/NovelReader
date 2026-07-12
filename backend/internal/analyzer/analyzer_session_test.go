// Conformance test for session propagation through Analyzer rule evaluation.
package analyzer

import "testing"

func TestAnalyzerPropagatesSourceStateToJavaScriptRules(t *testing.T) {
	state := &testSourceState{
		cookies: map[string]string{"sid": "fixture"},
		vars:    map[string]string{},
		memory:  map[string]interface{}{},
	}
	analyzer := New("", "https://example.test/", NewJSVM(), nil)
	analyzer.SetSourceState(state)

	value, err := analyzer.jsEval("cookie.getKey(baseUrl, 'sid')", "")
	if err != nil || ToString(value) != "fixture" {
		t.Fatalf("analyzer JS value = %q, err=%v", ToString(value), err)
	}
}
