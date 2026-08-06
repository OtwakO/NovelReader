// Transport-neutral request and response contracts shared by all booksource workflows.
package sourceexec

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/otwako/novelreader/internal/analyzer"
)

// RequestSpec is the fully expanded request described by a Legado URL rule.
type RequestSpec struct {
	URL          string
	Method       string
	Body         string
	Headers      map[string]string
	Charset      string
	Retry        int
	WebView      bool
	WebViewDelay int
	WebJS        string
	SourceRegex  string
	BodyJS       string
	DNSIP        string
	Origin       string
	Type         string
}

// Response retains transport data needed for parsing and failure classification.
type Response struct {
	StatusCode    int
	Headers       map[string][]string
	Body          string
	FinalURL      string
	Transport     string
	RedirectChain []string
}

// Transport executes a fully expanded source request.
type Transport interface {
	Do(context.Context, RequestSpec) (Response, error)
}

// RequestSpecFromURLMeta converts the analyzer's URL result without sharing mutable maps.
func RequestSpecFromURLMeta(meta *analyzer.URLMeta) RequestSpec {
	if meta == nil {
		return RequestSpec{}
	}
	return RequestSpec{
		URL:          meta.URL,
		Method:       meta.Method,
		Body:         meta.Body,
		Headers:      cloneHeaders(meta.Headers),
		Charset:      meta.Charset,
		Retry:        meta.Retry,
		WebView:      meta.WebView,
		WebViewDelay: meta.WebViewDelayMS,
		WebJS:        meta.WebJS,
		BodyJS:       meta.BodyJS,
		DNSIP:        meta.DNSIP,
		Origin:       meta.Origin,
		Type:         meta.Type,
	}
}

// MergeHeaders overlays URL-level headers on source-level headers by HTTP name.
func MergeHeaders(base, overlay map[string]string) map[string]string {
	merged := cloneHeaders(base)
	for key, value := range overlay {
		for existing := range merged {
			if strings.EqualFold(existing, key) {
				delete(merged, existing)
			}
		}
		merged[key] = value
	}
	return merged
}

// PreparePOST applies Legado's form-vs-raw body rules and returns body/content type.
func PreparePOST(body, charset string, headers map[string]string) (string, string) {
	contentType := ""
	for key, value := range headers {
		if strings.EqualFold(key, "Content-Type") {
			contentType = value
			break
		}
	}
	if contentType == "" {
		if isJSONBody(body) {
			contentType = "application/json"
		} else {
			contentType = "application/x-www-form-urlencoded"
		}
	}
	if isFormContentType(contentType) {
		body = EncodeRequestBody(body, charset)
	}
	return body, contentType
}

func isJSONBody(body string) bool {
	trimmed := strings.TrimSpace(body)
	return trimmed != "" && (trimmed[0] == '{' || trimmed[0] == '[') && json.Valid([]byte(trimmed))
}

func isFormContentType(contentType string) bool {
	return strings.EqualFold(strings.TrimSpace(strings.SplitN(contentType, ";", 2)[0]), "application/x-www-form-urlencoded")
}

// EncodeRequestBody encodes form values using the source's request charset.
func EncodeRequestBody(body, charset string) string {
	if body == "" || charset == "" {
		return body
	}
	pairs := strings.Split(body, "&")
	for i, pair := range pairs {
		eq := strings.IndexByte(pair, '=')
		if eq < 0 {
			continue
		}
		pairs[i] = pair[:eq] + "=" + analyzer.EncodeParamValue(pair[eq+1:], charset)
	}
	return strings.Join(pairs, "&")
}

func cloneHeaders(headers map[string]string) map[string]string {
	if len(headers) == 0 {
		return map[string]string{}
	}
	copy := make(map[string]string, len(headers))
	for key, value := range headers {
		copy[key] = value
	}
	return copy
}
