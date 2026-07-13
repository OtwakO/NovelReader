// Conformance test for URL options returned by Legado @js search rules.
package analyzer

import "testing"

func TestBuildURLIgnoresObjectLiteralsInsideJavaScript(t *testing.T) {
	meta, err := BuildURL(`@js:let headers={"x":"y"}; "https://example.test/search,"+JSON.stringify({method:"POST",body:"q="+key})`, "凡人修仙传", 1, "https://example.test", NewJSVM())
	if err != nil {
		t.Fatal(err)
	}
	if meta.Method != "POST" || meta.Body != "q=凡人修仙传" {
		t.Fatalf("meta=%+v", meta)
	}
}

func TestBuildURLParsesOptionsReturnedByJavaScript(t *testing.T) {
	meta, err := BuildURL(`@js:"https://example.test/search,"+JSON.stringify({method:"POST",body:"q="+key})`, "凡人修仙传", 1, "https://example.test", NewJSVM())
	if err != nil {
		t.Fatal(err)
	}
	if meta.Method != "POST" || meta.Body != "q=凡人修仙传" || meta.URL != "https://example.test/search" {
		t.Fatalf("meta=%+v", meta)
	}
}
