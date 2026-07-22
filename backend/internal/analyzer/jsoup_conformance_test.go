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

func TestJsoupBridgeEqMatchesJsoupBounds(t *testing.T) {
	vm := NewJSVM()
	value, err := vm.Eval(`var rows=org.jsoup.Jsoup.parse(result).select('.row'); rows.eq(9).size()`, `<p class="row">One</p>`, "https://example.test/")
	if err != nil || ToString(value) != "0" {
		t.Fatalf("positive out-of-range=%q err=%v", ToString(value), err)
	}
	if _, err := vm.Eval(`org.jsoup.Jsoup.parse(result).select('.row').eq(-1)`, `<p class="row">One</p>`, "https://example.test/"); err == nil {
		t.Fatal("negative eq index did not fail")
	}
}

func TestJsoupBridgeSupportsOwnTextMethod(t *testing.T) {
	vm := NewJSVM()
	value, err := vm.Eval(`org.jsoup.Jsoup.parse(result).select('.item').first().ownText()`, `<a class="item">  Direct
 text <span>Child</span><br> tail  </a>`, "https://example.test/")
	if err != nil || ToString(value) != "Direct text tail" {
		t.Fatalf("ownText()=%q err=%v", ToString(value), err)
	}
}

func TestJsoupBridgeCollectionMethodsStayOutOfForIn(t *testing.T) {
	vm := NewJSVM()
	value, err := vm.Eval(`var rows=org.jsoup.Jsoup.parse(result).select('.row'); var keys=[]; for (var key in rows) keys.push(key); keys.join('|')`, `<p class="row">One</p><p class="row">Two</p>`, "https://example.test/")
	if err != nil || ToString(value) != "0|1" {
		t.Fatalf("collection keys=%q err=%v", ToString(value), err)
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
