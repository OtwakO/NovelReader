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

func TestStrictConnectorsSkipIncompatibleAndBlankBranches(t *testing.T) {
	an := New(`{"name":"fallback"}`, "https://example.test/", NewJSVM(), nil)

	value, err := an.GetStringStrict(`a.0@text||name`)
	if err != nil || value != "fallback" {
		t.Fatalf("incompatible fallback value=%q err=%v", value, err)
	}
	value, err = an.GetStringStrict(`name&&`)
	if err != nil || value != "fallback" {
		t.Fatalf("trailing && value=%q err=%v", value, err)
	}
	value, err = an.GetStringStrict(`.book-extra@text&&name`)
	if err != nil || value != "fallback" {
		t.Fatalf("mixed-mode AND value=%q err=%v", value, err)
	}
	if _, err := an.GetStringStrict(`@js:throw new Error('broken')||name`); err == nil {
		t.Fatal("JavaScript failure was incorrectly treated as a fallback miss")
	}
}

func TestElementFallbacksReuseJavaScriptPrefixResult(t *testing.T) {
	an := New(`{"featured":true}`, "https://example.test/", NewJSVM(), nil)
	rule := `<js>JSON.stringify({data:{items:[{name:'A'},{name:'B'}]}})</js>
class.item||$.data.items[*]`

	elements, err := an.GetElements(rule)
	if err != nil {
		t.Fatal(err)
	}
	if len(elements) != 2 {
		t.Fatalf("elements=%#v, want transformed JSON fallback", elements)
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
