// Conformance tests for the transport-neutral source request contract.
package sourceexec

import (
	"testing"

	"github.com/otwako/novelreader/internal/analyzer"
)

func TestRequestSpecFromURLMetaPreservesExecutionMetadata(t *testing.T) {
	meta := &analyzer.URLMeta{
		URL:     "https://example.test/search",
		Method:  "POST",
		Body:    "q=encoded",
		Headers: map[string]string{"Referer": "https://example.test/"},
		Charset: "gb2312",
		Retry:   2,
		WebView: true,
		WebJS:   "click();",
		BodyJS:  "result.trim();",
		DNSIP:   "1.2.3.4",
		Origin:  "https://origin.test",
		Type:    "text",
	}

	spec := RequestSpecFromURLMeta(meta)
	if spec.URL != meta.URL || spec.Method != meta.Method || spec.Body != meta.Body {
		t.Fatalf("request core fields were not preserved: %+v", spec)
	}
	if spec.Charset != meta.Charset || spec.Retry != meta.Retry || !spec.WebView {
		t.Fatalf("request execution fields were not preserved: %+v", spec)
	}
	if spec.WebJS != meta.WebJS || spec.BodyJS != meta.BodyJS || spec.DNSIP != meta.DNSIP || spec.Origin != meta.Origin || spec.Type != meta.Type {
		t.Fatalf("request metadata was not preserved: %+v", spec)
	}
	if spec.Headers["Referer"] != meta.Headers["Referer"] {
		t.Fatalf("request headers were not preserved: %+v", spec.Headers)
	}
}

func TestMergeHeadersUsesCaseInsensitiveURLOverlay(t *testing.T) {
	merged := MergeHeaders(
		map[string]string{"user-agent": "source", "X-Source": "yes"},
		map[string]string{"User-Agent": "url", "x-option": "yes"},
	)
	if len(merged) != 3 || merged["User-Agent"] != "url" || merged["user-agent"] != "" {
		t.Fatalf("merged headers=%v", merged)
	}
	if merged["X-Source"] != "yes" || merged["x-option"] != "yes" {
		t.Fatalf("unrelated headers were lost: %v", merged)
	}
}

func TestResponseRetainsNonSuccessBodyForClassification(t *testing.T) {
	resp := Response{StatusCode: 403, Body: "challenge page", FinalURL: "https://example.test/"}
	if resp.StatusCode != 403 || resp.Body != "challenge page" || resp.FinalURL == "" {
		t.Fatalf("response lost diagnostic data: %+v", resp)
	}
}
