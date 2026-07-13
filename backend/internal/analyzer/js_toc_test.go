// Regression tests for Legado <js> TOC rules returning object arrays.
package analyzer

import "testing"

func TestGetElementsStripsJSBlockWrapper(t *testing.T) {
	a := New(`<b>1</b>/<b>2</b>`, "https://example.test/book.html", NewJSVM(), nil)
	values, err := a.GetElements(`<js>
var list=[{text:'第一章',href:'/chapter/1'},{text:'第二章',href:'/chapter/2'}];
list
</js>`)
	if err != nil {
		t.Fatal(err)
	}
	if len(values) != 2 {
		t.Fatalf("values=%#v", values)
	}
	first, ok := values[0].(map[string]interface{})
	if !ok || first["text"] != "第一章" || first["href"] != "/chapter/1" {
		t.Fatalf("first=%#v", values[0])
	}
}
