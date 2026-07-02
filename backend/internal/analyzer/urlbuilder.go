package analyzer

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
)

// URLMeta holds the result of URL construction plus all parsed options
// from the legado URL JSON option suffix: URL,{"method":"POST","body":"...","charset":"gbk",...}
type URLMeta struct {
	URL     string
	Method  string            // GET or POST (default GET)
	Body    string            // POST body (with {{key}} already interpolated)
	Headers map[string]string // per-URL extra headers
	Charset string            // character encoding override
	Retry   int               // retry count on non-2xx
	WebView bool              // if true, source needs JS rendering (server-side can't do this)
}

// urlOption mirrors legado's UrlOption. Uses RawMessage for fields that may be
// either a string or structured type (headers as JSON string or map, webView as "true" or true).
type urlOption struct {
	Method  string          `json:"method"`
	Body    string          `json:"body"`
	Charset string          `json:"charset"`
	WebView json.RawMessage `json:"webView,omitempty"`
	Retry   json.RawMessage `json:"retry,omitempty"`
	WebJs   string          `json:"webJs,omitempty"`
	BodyJs  string          `json:"bodyJs,omitempty"`
	Js      string          `json:"js,omitempty"`
	DnsIp   string          `json:"dnsIp,omitempty"`
	Headers json.RawMessage `json:"headers,omitempty"`
}

// BuildURL constructs a request URL from a book source's URL template.
// Returns a URLMeta with the resolved URL and all parsed options.
// Handles:
//   - {{key}}, {{page}} template variables
//   - @js: inline JS evaluation
//   - ,{...} JSON option suffix (method, body, charset, headers, webView, retry)
//   - Relative URL resolution against baseURL
func BuildURL(template, key string, page int, baseURL string, jsVM *JSVM) (*URLMeta, error) {
	if template == "" {
		return nil, fmt.Errorf("analyzer: empty URL template")
	}

	urlStr := template
	meta := &URLMeta{
		Method:  "GET",
		Headers: make(map[string]string),
	}

	// Extract JSON option suffix: URL,{"method":"POST","body":"...",...}
	if idx := findJSONOption(urlStr); idx != -1 {
		optionRaw := urlStr[idx+1:]
		urlStr = urlStr[:idx]

		var opt urlOption
		if err := json.Unmarshal([]byte(optionRaw), &opt); err != nil {
			slog.Warn("urlbuilder: failed to parse URL JSON option",
				"option", optionRaw[:min(len(optionRaw), 100)], "err", err)
		} else {
			if opt.Method != "" {
				meta.Method = strings.ToUpper(opt.Method)
			}
			meta.Body = opt.Body
			meta.Charset = opt.Charset

			// Tolerant webView: accepts "true" (string), true (bool), "1", 1
			if len(opt.WebView) > 0 {
				var s string
				if err := json.Unmarshal(opt.WebView, &s); err == nil {
					meta.WebView = s == "true" || s == "1"
				} else {
					var b bool
					if err := json.Unmarshal(opt.WebView, &b); err == nil {
						meta.WebView = b
					}
				}
			}

			// Tolerant headers: accepts JSON string ("{\"UA\":\"...\"}") or map
			if len(opt.Headers) > 0 {
				if err := json.Unmarshal(opt.Headers, &meta.Headers); err != nil {
					var s string
					if err2 := json.Unmarshal(opt.Headers, &s); err2 == nil && s != "" {
						json.Unmarshal([]byte(s), &meta.Headers)
					}
				}
			}

			// Tolerant retry: accepts 2 (number) or "2" (string)
			if len(opt.Retry) > 0 {
				json.Unmarshal(opt.Retry, &meta.Retry)
				if meta.Retry == 0 {
					var s string
					if json.Unmarshal(opt.Retry, &s) == nil {
						fmt.Sscanf(s, "%d", &meta.Retry)
					}
				}
			}

			// ponytail: webJs, bodyJs, js, dnsIp parsed but not acted on yet
			_ = opt.WebJs
			_ = opt.BodyJs
			_ = opt.Js
			_ = opt.DnsIp
		}
	}

	// @js: URL construction
	if strings.HasPrefix(urlStr, "@js:") {
		jsCode := urlStr[4:]
		if jsVM != nil {
			v, err := jsVM.Eval(jsCode, "", baseURL)
			if err == nil {
				urlStr = ToString(v)
			}
		}
	}

	// Evaluate {{...}} JS expressions in the URL (handles {{cookie.removeCookie(source.key)}}, etc.)
	// First pass: simple variable replacement for {{key}} and {{page}} (no JS overhead)
	urlStr = strings.ReplaceAll(urlStr, "{{key}}", key)
	urlStr = strings.ReplaceAll(urlStr, "{{page}}", fmt.Sprintf("%d", page))

	// Second pass: evaluate remaining {{...}} as JS expressions
	if strings.Contains(urlStr, "{{") && jsVM != nil {
		extra := map[string]interface{}{"key": key, "page": page, "baseUrl": baseURL}
		urlStr = evalTemplateExpressions(urlStr, jsVM, baseURL, extra)
	}

	// Replace {{key}} in POST body too
	if meta.Body != "" {
		meta.Body = strings.ReplaceAll(meta.Body, "{{key}}", key)
		meta.Body = strings.ReplaceAll(meta.Body, "{{page}}", fmt.Sprintf("%d", page))
		if strings.Contains(meta.Body, "{{") && jsVM != nil {
			meta.Body = evalTemplateExpressions(meta.Body, jsVM, baseURL)
		}
	}

	// Resolve relative URLs
	if !strings.HasPrefix(urlStr, "http://") && !strings.HasPrefix(urlStr, "https://") && !strings.HasPrefix(urlStr, "@js:") {
		base := strings.TrimRight(baseURL, "/")
		path := strings.TrimLeft(urlStr, "/")
		urlStr = base + "/" + path
	}

	meta.URL = urlStr
	return meta, nil
}

// evalTemplateExpressions finds all {{...}} patterns and evaluates the content as JS.
// Handles cases like {{cookie.removeCookie(source.key)}}, {{source.key}}, etc.
// ponytail: simple regex-based extraction, no nesting support for {{...}} inside {{...}}.
var tmplRe = regexp.MustCompile(`\{\{([^}]+)\}\}`)

func evalTemplateExpressions(s string, jsVM *JSVM, baseURL string, extra ...map[string]interface{}) string {
	var bindings map[string]interface{}
	if len(extra) > 0 {
		bindings = extra[0]
	}
	return tmplRe.ReplaceAllStringFunc(s, func(match string) string {
		inner := strings.TrimSpace(match[2 : len(match)-2])
		if inner == "" {
			return match
		}
		var v interface{}
		var err error
		if bindings != nil {
			v, err = jsVM.Eval(inner, "", baseURL, bindings)
		} else {
			v, err = jsVM.Eval(inner, "", baseURL)
		}
		if err != nil {
			slog.Warn("urlbuilder: template eval failed", "expr", inner[:min(len(inner), 60)], "err", err)
			return match
		}
		return ToString(v)
	})
}

// findJSONOption finds the start of a ,{...} JSON option suffix in a URL.
// It scans from the end, counting brace depth to handle nested JSON.
// Returns -1 if no valid JSON option is found.
func findJSONOption(url string) int {
	// Look for ",{" starting from the end (it's a suffix)
	for i := len(url) - 2; i >= 0; i-- {
		if url[i] == ',' && i+1 < len(url) && url[i+1] == '{' {
			// Verify it parses as valid JSON
			if json.Valid([]byte(url[i+1:])) {
				return i
			}
			// Not valid JSON at this comma — try earlier ones
		}
	}
	return -1
}
