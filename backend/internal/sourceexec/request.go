// Transport-neutral request and response contracts shared by all booksource workflows.
package sourceexec

import (
	"context"

	"github.com/otwako/novelreader/internal/analyzer"
)

// RequestSpec is the fully expanded request described by a Legado URL rule.
type RequestSpec struct {
	URL     string
	Method  string
	Body    string
	Headers map[string]string
	Charset string
	Retry   int
	WebView bool
	WebJS   string
	BodyJS  string
	DNSIP   string
	Origin  string
	Type    string
}

// Response retains transport data needed for parsing and failure classification.
type Response struct {
	StatusCode int
	Headers    map[string][]string
	Body       string
	FinalURL   string
	Transport  string
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
		URL:     meta.URL,
		Method:  meta.Method,
		Body:    meta.Body,
		Headers: cloneHeaders(meta.Headers),
		Charset: meta.Charset,
		Retry:   meta.Retry,
		WebView: meta.WebView,
		WebJS:   meta.WebJS,
		BodyJS:  meta.BodyJS,
		DNSIP:   meta.DNSIP,
		Origin:  meta.Origin,
		Type:    meta.Type,
	}
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
