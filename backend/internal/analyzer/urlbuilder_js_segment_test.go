// Conformance test for URL templates with a normal URL followed by @js.
package analyzer

import "testing"

func TestBuildURLExpandsPageSelectorsInPostBodies(t *testing.T) {
	for _, test := range []struct {
		page int
		want string
	}{
		{page: 1, want: "kw=剑来&_token=token"},
		{page: 2, want: "kw=剑来&page=2&_token=token"},
		{page: 3, want: "kw=剑来&page=3&_token=token"},
	} {
		meta, err := BuildURL(`/search,{"method":"POST","body":"kw={{key}}<,&page={{page}}>&_token=token"}`, "剑来", test.page, "https://example.test", NewJSVM())
		if err != nil {
			t.Fatal(err)
		}
		if meta.Body != test.want {
			t.Fatalf("page=%d body=%q want=%q", test.page, meta.Body, test.want)
		}
	}
}

func TestBuildURLDoesNotTreatLiteralAtJSAsSegment(t *testing.T) {
	meta, err := BuildURL(`/search?q=@js:literal`, "剑来", 1, "https://example.test", NewJSVM())
	if err != nil {
		t.Fatal(err)
	}
	if meta.URL != "https://example.test/search?q=@js:literal" {
		t.Fatalf("url=%q", meta.URL)
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
