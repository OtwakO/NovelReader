// Conformance test for URL templates with a normal URL followed by @js.
package analyzer

import "testing"

func TestBuildURLExpandsPageSelectorsInPostBodies(t *testing.T) {
	meta, err := BuildURL(`/search,{"method":"POST","body":"kw={{key}}<,&page={{page}}>&_token=token"}`, "剑来", 1, "https://example.test", NewJSVM())
	if err != nil {
		t.Fatal(err)
	}
	if meta.Body != "kw=剑来&page=1&_token=token" {
		t.Fatalf("body=%q", meta.Body)
	}
}

func TestBuildURLEvaluatesJavaScriptURLSegment(t *testing.T) {
	meta, err := BuildURL("/search\n@js:result + ',' + JSON.stringify({method:'POST',body:'kw='+key})", "剑来", 1, "https://example.test", NewJSVM())
	if err != nil {
		t.Fatal(err)
	}
	if meta.URL != "https://example.test/search" || meta.Method != "POST" || meta.Body != "kw=剑来" {
		t.Fatalf("meta=%+v", meta)
	}
}
