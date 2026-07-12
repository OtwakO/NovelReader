// Conformance tests for Legado URL options and request construction.
package analyzer

import "testing"

func TestBuildURLPreservesAllLegadoRequestOptions(t *testing.T) {
	meta, err := BuildURL(`https://example.test/search,{"method":"POST","body":"q={{java.base64Encode(key)}}","charset":"gb2312","webView":true,"webJs":"click();","bodyJs":"result.trim();","dnsIp":"1.2.3.4","origin":"https://origin.test","type":"text"}`, "凡人修仙传", 1, "https://example.test/", NewJSVM())
	if err != nil {
		t.Fatal(err)
	}
	if meta.Method != "POST" || meta.Charset != "gb2312" || !meta.WebView {
		t.Fatalf("request options lost: method=%q charset=%q webView=%v", meta.Method, meta.Charset, meta.WebView)
	}
	if meta.Body != "q=5Yeh5Lq65L+u5LuZ5Lyg" {
		t.Fatalf("body template was not evaluated with key binding: %q", meta.Body)
	}
	if meta.WebJS != "click();" || meta.BodyJS != "result.trim();" || meta.DNSIP != "1.2.3.4" || meta.Origin != "https://origin.test" || meta.Type != "text" {
		t.Fatalf("request metadata was not preserved: %+v", meta)
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
