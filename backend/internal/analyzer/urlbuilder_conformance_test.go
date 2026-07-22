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
