// Conformance tests for Legado URL options and request construction.
package analyzer

import "testing"

func TestBuildURLPreservesAllLegadoRequestOptions(t *testing.T) {
	meta, err := BuildURL(`https://example.test/search,{"method":"POST","body":"q={{java.base64Encode(key)}}","charset":"gb2312","webView":true,"webViewDelayTime":"500","webJs":"click();","bodyJs":"result.trim();","dnsIp":"1.2.3.4","origin":"https://origin.test","type":"text"}`, "凡人修仙传", 1, "https://example.test/", NewJSVM())
	if err != nil {
		t.Fatal(err)
	}
	if meta.Method != "POST" || meta.Charset != "gb2312" || !meta.WebView {
		t.Fatalf("request options lost: method=%q charset=%q webView=%v", meta.Method, meta.Charset, meta.WebView)
	}
	if meta.Body != "q=5Yeh5Lq65L+u5LuZ5Lyg" {
		t.Fatalf("body template was not evaluated with key binding: %q", meta.Body)
	}
	if meta.WebJS != "click();" || meta.WebViewDelayMS != 500 || meta.BodyJS != "result.trim();" || meta.DNSIP != "1.2.3.4" || meta.Origin != "https://origin.test" || meta.Type != "text" {
		t.Fatalf("request metadata was not preserved: %+v", meta)
	}
}

func TestBuildURLAcceptsLenientLegadoOptionKeys(t *testing.T) {
	tests := []struct {
		name     string
		template string
		wantURL  string
		method   string
		body     string
	}{
		{
			name:     "unknown legacy option is removed from detail URL",
			template: `https://example.test/book,{Cookie:"xmanhua_lang=2"}`,
			wantURL:  "https://example.test/book",
			method:   "GET",
		},
		{
			name:     "unquoted request keys configure search POST",
			template: `https://example.test/search,{method:"post",body:"searchkey={{key}}"}`,
			wantURL:  "https://example.test/search",
			method:   "POST",
			body:     "searchkey=凡人修仙传",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			meta, err := BuildURL(test.template, "凡人修仙传", 1, "https://example.test/", nil)
			if err != nil {
				t.Fatal(err)
			}
			if meta.URL != test.wantURL || meta.Method != test.method || meta.Body != test.body {
				t.Fatalf("meta=%+v", meta)
			}
		})
	}
}

func TestBuildURLEncodesGetQueryUsingDeclaredCharset(t *testing.T) {
	meta, err := BuildURL(`https://example.test/search?keyword={{key}}, {"charset":"gb2312"}`, "凡人修仙传", 1, "https://example.test/", nil)
	if err != nil {
		t.Fatal(err)
	}
	want := "https://example.test/search?keyword=" + EncodeParamValue("凡人修仙传", "gb2312")
	if meta.URL != want {
		t.Fatalf("url=%q want=%q", meta.URL, want)
	}
}

func TestBuildURLLeavesAlreadyEncodedGetQueryIntact(t *testing.T) {
	meta, err := BuildURL(`https://example.test/search?keyword=%B7%B2%C8%CB, {"charset":"gb2312"}`, "", 1, "https://example.test/", nil)
	if err != nil {
		t.Fatal(err)
	}
	if meta.URL != "https://example.test/search?keyword=%B7%B2%C8%CB" {
		t.Fatalf("url=%q", meta.URL)
	}
}

func TestBuildURLEncodesMixedQueryWithoutReencodingExistingBytes(t *testing.T) {
	meta, err := BuildURL(`https://example.test/search?fixed=%B7%B2&keyword={{key}}, {"charset":"gb2312"}`, "凡人", 1, "https://example.test/", nil)
	if err != nil {
		t.Fatal(err)
	}
	want := "https://example.test/search?fixed=%B7%B2&keyword=" + EncodeParamValue("凡人", "gb2312")
	if meta.URL != want {
		t.Fatalf("url=%q want=%q", meta.URL, want)
	}
}

func TestBuildURLPreservesReservedQueryCharactersWithDeclaredCharset(t *testing.T) {
	meta, err := BuildURL(`https://example.test/search?callback=https://reader.test/a+b:c/d&literal=100%, {"charset":"gb2312"}`, "", 1, "https://example.test/", nil)
	if err != nil {
		t.Fatal(err)
	}
	if meta.URL != "https://example.test/search?callback=https://reader.test/a+b:c/d&literal=100%" {
		t.Fatalf("url=%q", meta.URL)
	}
}

func TestBuildURLExecutesWholeJSProgramBeforeURLParsing(t *testing.T) {
	meta, err := BuildURL("<js>'https://example.test/search,{\"method\":\"POST\",\"body\":\"q=' + key + '\"}'</js>", "凡人修仙传", 1, "https://example.test/", NewJSVM())
	if err != nil {
		t.Fatal(err)
	}
	if meta.URL != "https://example.test/search" || meta.Method != "POST" || meta.Body != "q=凡人修仙传" {
		t.Fatalf("meta=%+v", meta)
	}
}

func TestBuildURLExecutesInlineJSBeforeOrdinarySuffix(t *testing.T) {
	state := &testSourceState{cookies: map[string]string{}, vars: map[string]string{}, memory: map[string]interface{}{}}
	meta, err := BuildURLWithState(`<js>source.put('token', key)</js>https://example.test/search?q={{source.get('token')}}`, "凡人修仙传", 1, "https://example.test/", NewJSVM(), state)
	if err != nil {
		t.Fatal(err)
	}
	if meta.URL != "https://example.test/search?q=凡人修仙传" {
		t.Fatalf("url=%q", meta.URL)
	}
}

func TestBuildURLChainsCaseInsensitiveJSTagsWithResultText(t *testing.T) {
	meta, err := BuildURL(`<JS>'https://example.test/' + key</JS><js>result + '/page/' + page</js>@result?from=legado`, "book", 2, "https://example.test/", NewJSVM())
	if err != nil {
		t.Fatal(err)
	}
	if meta.URL != "https://example.test/book/page/2?from=legado" {
		t.Fatalf("url=%q", meta.URL)
	}
}

func TestBuildURLPreservesStructuredJSONBody(t *testing.T) {
	meta, err := BuildURL(`https://example.test/search,{"method":"POST","body":{"q":"{{key}}"}}`, "搜索", 1, "https://example.test/", nil)
	if err != nil {
		t.Fatal(err)
	}
	if meta.Body != `{"q":"搜索"}` {
		t.Fatalf("structured body=%q", meta.Body)
	}
}

func TestBuildURLExpandsEncodedPageSelector(t *testing.T) {
	meta, err := BuildURL(`https://example.test/list?body=%7B%22offset%22%3A%22%3C0%2C150%2C300%3E%22%7D`, "", 2, "https://example.test/", nil)
	if err != nil {
		t.Fatal(err)
	}
	if meta.URL != `https://example.test/list?body=%7B%22offset%22%3A%22150%22%7D` {
		t.Fatalf("url=%q", meta.URL)
	}
}

func TestBuildURLPreservesEncodedPageSelectorValues(t *testing.T) {
	meta, err := BuildURL(`https://example.test/list?body=%3Cfirst%2Bvalue%2Csecond%26value%2Cthird%3Dvalue%3E`, "", 2, "https://example.test/", nil)
	if err != nil {
		t.Fatal(err)
	}
	if meta.URL != `https://example.test/list?body=second%26value` {
		t.Fatalf("url=%q", meta.URL)
	}
}

func TestBuildURLResolvesRootRelativePathAgainstHost(t *testing.T) {
	meta, err := BuildURL(`/search?q={{key}}`, "x", 1, "https://example.test/novels/book-1", nil)
	if err != nil {
		t.Fatal(err)
	}
	if meta.URL != "https://example.test/search?q=x" {
		t.Fatalf("root-relative URL resolved incorrectly: %q", meta.URL)
	}
}
