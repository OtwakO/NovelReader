package analyzer

import (
	"strings"
	"testing"
)

func TestBuildURLExpansionPhaseOrder(t *testing.T) {
	for _, test := range []struct {
		name     string
		template string
		wantURL  string
		header   string
	}{
		{
			name:     "options are interpolated",
			template: `/search,{"headers":{"X-Query":"{{key}}"}}`,
			wantURL:  "https://example.com/search",
			header:   "Example",
		},
		{
			name:     "comparisons are not page selectors",
			template: `/search?p={{page < 2 ? Math.max(2, page) : (page > 4 ? 4 : page)}}`,
			wantURL:  "https://example.com/search?p=2",
		},
		{
			name:     "interpolation can generate selectors and options",
			template: `/search?p={{'<first,second>'}},{{'{"headers":{"X-Query":"' + key + '"}}'}}`,
			wantURL:  "https://example.com/search?p=first",
			header:   "Example",
		},
		{
			name:     "nested object braces are not template delimiters",
			template: `/search,{{JSON.stringify({headers: {'X-Query':key}})}}`,
			wantURL:  "https://example.com/search",
			header:   "Example",
		},
		{
			name:     "URL script runs before interpolation",
			template: "/search?q={{key}}\n@js:result.replace('{{key}}', 'from-script')",
			wantURL:  "https://example.com/search?q=from-script",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			meta, err := BuildURL(test.template, "Example", 1, "https://example.com", NewJSVM())
			if err != nil {
				t.Fatal(err)
			}
			if meta.URL != test.wantURL || meta.Headers["X-Query"] != test.header {
				t.Fatalf("meta=%+v; want URL=%q X-Query=%q", meta, test.wantURL, test.header)
			}
		})
	}
}

func TestBuildURLInterpolatesOptionsWithJSLib(t *testing.T) {
	data := &URLContext{JSLib: `function query() { return key + page; }`}
	meta, err := BuildURLWithContextData(t.Context(), `/search,{"method":"POST","body":{"q":"{{query()}}"},"retry":{{page + 1}},"headers":{"X-Page":"<first,later>"}}`, "Example", 1, "https://example.com", NewJSVM(), nil, data)
	if err != nil {
		t.Fatal(err)
	}
	if meta.URL != "https://example.com/search" || meta.Method != "POST" || meta.Body != `{"q":"Example1"}` || meta.Retry != 2 || meta.Headers["X-Page"] != "first" {
		t.Fatalf("meta=%+v", meta)
	}
}

func TestBuildURLRejectsInterpolationErrors(t *testing.T) {
	for _, template := range []string{
		`/search?q={{missingFunction()}}`,
		`/search,{"headers":{"X-Query":"{{missingFunction()}}"}}`,
	} {
		meta, err := BuildURL(template, "Example", 1, "https://example.com", NewJSVM())
		if err == nil || !strings.Contains(err.Error(), "template eval failed") || meta != nil {
			t.Fatalf("template=%q: meta=%+v err=%v; want interpolation error and no request", template, meta, err)
		}
	}
	meta, err := BuildURL(`/search?q={{key.toUpperCase()}}`, "Example", 1, "https://example.com", nil)
	if err == nil || meta != nil {
		t.Fatalf("meta=%+v err=%v; want missing engine error and no request", meta, err)
	}
}

func TestURLPhasesShareOnlyRequestLocalVariables(t *testing.T) {
	vm := NewJSVM()
	data := &URLContext{}
	meta, err := BuildURLWithContextData(t.Context(), `@js:java.put('token', 'saved'); '/{{java.get("token")}}'`, "", 1, "https://example.test", vm, nil, data)
	if err != nil || meta.URL != "https://example.test/saved" {
		t.Fatalf("URL phases lost their variables: meta=%+v err=%v", meta, err)
	}
	meta, err = BuildURLWithContextData(t.Context(), `/{{java.get("token")}}`, "", 1, "https://example.test", vm, nil, data)
	if err != nil || meta.URL != "https://example.test/" {
		t.Fatalf("later request inherited implicit variables: meta=%+v err=%v", meta, err)
	}
}
