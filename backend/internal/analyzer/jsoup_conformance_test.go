// Conformance test for the Jsoup bridge used by raw JavaScript sources.
package analyzer

import "testing"

func TestJsoupBridgeSupportsChainedSelectionAttribute(t *testing.T) {
	a := New(`<input name="_token" value="abc">`, "https://example.test", NewJSVM(), NewCacheManager())
	value, err := a.jsEval(`org.jsoup.Jsoup.parse('<form><input name="_token" value="abc"></form>').select('form').select('input[name=_token]').attr('value')`, "")
	if err != nil {
		t.Fatal(err)
	}
	if got := ToString(value); got != "abc" {
		t.Fatalf("value=%q, want abc", got)
	}
}
