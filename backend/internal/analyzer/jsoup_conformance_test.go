// Conformance test for the Jsoup bridge used by raw JavaScript sources.
package analyzer

import "testing"

func TestJsoupBridgeSupportsJavaCollectionMethods(t *testing.T) {
	vm := NewJSVM()
	value, err := vm.Eval(`var rows=org.jsoup.Jsoup.parse(result).select('.row'); rows.size() + '|' + rows.eq(1).text() + '|' + rows[0].text()`, `<p class="row">One</p><p class="row">Two</p>`, "https://example.test/")
	if err != nil || ToString(value) != "2|Two|One" {
		t.Fatalf("collection methods=%q err=%v", ToString(value), err)
	}
}

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
