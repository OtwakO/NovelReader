package analyzer

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"regexp"
	"strings"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/traditionalchinese"
	"golang.org/x/text/transform"
)

// URLMeta holds the result of URL construction plus all parsed options
// from the legado URL JSON option suffix: URL,{"method":"POST","body":"...","charset":"gbk",...}
type URLMeta struct {
	URL     string
	Method  string            // GET or POST (default GET)
	Body    string            // POST body (with {{key}} already interpolated)
	Headers map[string]string // per-URL extra headers
	Charset string            // character encoding override (e.g. "gbk", "gb2312")
	Retry   int               // retry count on non-2xx
	WebView bool              // if true, source needs JS rendering
	WebJS   string            // JavaScript executed by a WebView before extraction
	BodyJS  string            // JavaScript transformation for the response body
	DNSIP   string            // optional source DNS/IP override
	Origin  string            // request origin override
	Type    string            // file/content type for downloads
}

// urlOption mirrors legado's UrlOption data class.
// Uses RawMessage for fields that may be either a string or structured type.
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
	Type    string          `json:"type,omitempty"`
	Origin  string          `json:"origin,omitempty"`
}

// ponytail: serverID and webViewDelayTime exist in legado's UrlOption but are omitted
// since we don't implement multi-server or WebView rendering on the backend.

// BuildURL constructs a request URL from a book source's URL template.
// Returns a URLMeta with the resolved URL and all parsed options.
// Handles:
//   - {{key}}, {{page}} template variables
//   - @js: inline JS evaluation
//   - ,{...} JSON option suffix (method, body, charset, headers, webView, retry)
//   - Relative URL resolution against baseURL
func BuildURL(template, key string, page int, baseURL string, jsVM *JSVM) (*URLMeta, error) {
	return BuildURLWithContext(context.Background(), template, key, page, baseURL, jsVM, nil)
}

// BuildURLWithState expands a URL with an optional Legado source session.
func BuildURLWithState(template, key string, page int, baseURL string, jsVM *JSVM, sourceState SourceState) (*URLMeta, error) {
	return BuildURLWithContext(context.Background(), template, key, page, baseURL, jsVM, sourceState)
}

// BuildURLWithContext expands a URL while preserving request cancellation for URL JavaScript.
func BuildURLWithContext(ctx context.Context, template, key string, page int, baseURL string, jsVM *JSVM, sourceState SourceState) (*URLMeta, error) {
	if template == "" {
		return nil, fmt.Errorf("analyzer: empty URL template")
	}

	urlStr := strings.TrimSpace(template)

	// Legado also permits a normal URL followed by an @js: segment. Evaluate
	// the segment against the already-built URL so `result` has the expected value.
	if jsIndex := findURLJSSegment(urlStr); jsIndex > 0 {
		baseTemplate := strings.TrimSpace(urlStr[:jsIndex])
		baseMeta, err := BuildURLWithContext(ctx, baseTemplate, key, page, baseURL, jsVM, sourceState)
		if err != nil {
			return nil, err
		}
		if jsVM == nil {
			return nil, fmt.Errorf("urlbuilder: @js: no JS engine available")
		}
		value, err := jsVM.EvalContext(ctx, urlStr[jsIndex+4:], "", baseURL, map[string]interface{}{
			"key": key, "page": page, "result": baseMeta.URL, "baseUrl": baseURL, "sourceState": sourceState,
		})
		if err != nil {
			return nil, fmt.Errorf("urlbuilder: @js: eval failed: %w", err)
		}
		return BuildURLWithContext(ctx, ToString(value), key, page, baseURL, jsVM, sourceState)
	}

	meta := &URLMeta{
		Method:  "GET",
		Headers: make(map[string]string),
	}

	// Strip newlines and carriage returns for non-@js: URLs.
	// @js: URLs need newlines for JS code. We remove newlines entirely so
	// that ,\n{ stays adjacent (crucial for findJSONOption detection).
	if !strings.HasPrefix(urlStr, "@js:") {
		urlStr = strings.NewReplacer("\n", "", "\r", "").Replace(urlStr)
		urlStr = strings.TrimSpace(urlStr)
	}

	// Store the js option for eval after URL is fully constructed
	var optJs string

	// Extract JSON option suffix only from ordinary URL templates. JavaScript
	// templates can contain unrelated object literals and must be evaluated first.
	if !strings.HasPrefix(urlStr, "@js:") {
		if before, option, ok := extractJSONOption(urlStr); ok {
			urlStr = before

			var opt urlOption
			if err := json.Unmarshal([]byte(option), &opt); err != nil {
				slog.Warn("urlbuilder: failed to parse URL JSON option",
					"option", option[:min(len(option), 100)], "err", err)
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

				meta.WebJS = opt.WebJs
				meta.BodyJS = opt.BodyJs
				meta.DNSIP = opt.DnsIp
				meta.Origin = opt.Origin
				meta.Type = opt.Type

				optJs = opt.Js // stored for post-resolution eval
			}
		}
	}

	// @js: URL construction
	if strings.HasPrefix(urlStr, "@js:") {
		jsCode := urlStr[4:]
		if jsVM != nil {
			v, err := jsVM.EvalContext(ctx, jsCode, "", baseURL, map[string]interface{}{"key": key, "page": page, "sourceState": sourceState})
			if err == nil {
				// Legado allows @js to return `url,{...options}`. Re-run
				// the normal URL parser so the returned POST/body/charset
				// metadata is not treated as part of the URL path.
				return BuildURLWithContext(ctx, ToString(v), key, page, baseURL, jsVM, sourceState)
			}
			return nil, fmt.Errorf("urlbuilder: @js: eval failed: %w", err)
		}
		return nil, fmt.Errorf("urlbuilder: @js: no JS engine available")
	}

	// Handle <,{{page}}> page-selection syntax: <,a,b,c> picks page 1→a, 2→b, 3→c.
	// For page 1, <,...> should be removed (produces empty string).
	// We use a simple regex to remove <...> segments that match the page pattern.
	// ponytail: only handles the common case; complex JS inside <...> is deferred.
	urlStr = expandPageSelector(urlStr, page)

	// Evaluate {{...}} JS expressions in the URL (handles {{cookie.removeCookie(source.key)}}, etc.)
	// First pass: simple variable replacement for {{key}} and {{page}} (no JS overhead)
	urlStr = strings.ReplaceAll(urlStr, "{{key}}", key)
	urlStr = strings.ReplaceAll(urlStr, "{{page}}", fmt.Sprintf("%d", page))

	// Second pass: evaluate remaining {{...}} as JS expressions
	if strings.Contains(urlStr, "{{") && jsVM != nil {
		extra := map[string]interface{}{"key": key, "page": page, "baseUrl": baseURL, "sourceState": sourceState}
		urlStr = evalTemplateExpressionsContext(ctx, urlStr, jsVM, baseURL, extra)
	}

	// Replace {{key}} in POST body too
	if meta.Body != "" {
		meta.Body = expandPageSelector(meta.Body, page)
		meta.Body = strings.ReplaceAll(meta.Body, "{{key}}", key)
		meta.Body = strings.ReplaceAll(meta.Body, "{{page}}", fmt.Sprintf("%d", page))
		if strings.Contains(meta.Body, "{{") && jsVM != nil {
			meta.Body = evalTemplateExpressionsContext(ctx, meta.Body, jsVM, baseURL,
				map[string]interface{}{"key": key, "page": page, "baseUrl": baseURL})
		}
	}

	// Resolve relative URLs using RFC 3986 semantics, matching Legado's URL resolver.
	if !strings.HasPrefix(urlStr, "http://") && !strings.HasPrefix(urlStr, "https://") && !strings.HasPrefix(urlStr, "@js:") {
		base, err := url.Parse(baseURL)
		if err != nil {
			return nil, fmt.Errorf("urlbuilder: invalid base URL %q: %w", baseURL, err)
		}
		ref, err := url.Parse(urlStr)
		if err != nil {
			return nil, fmt.Errorf("urlbuilder: invalid relative URL %q: %w", urlStr, err)
		}
		urlStr = base.ResolveReference(ref).String()
	}

	// Execute URL option's js parameter: can modify url, headerMap, etc.
	// In legado this is eval'd after full URL construction, before the request.
	// Supported patterns: {"js":"java.url=java.url+'yyyy'"}, {"js":"java.headerMap.put('x','y')"}
	if optJs != "" && jsVM != nil {
		bindings := map[string]interface{}{
			"java": map[string]interface{}{
				"url":       urlStr, // mutable — JS can change it
				"headerMap": meta.Headers,
			},
			"key":         key,
			"page":        page,
			"baseUrl":     baseURL,
			"sourceState": sourceState,
		}
		if v, err := jsVM.EvalContext(ctx, optJs, "", baseURL, bindings); err == nil {
			if s := ToString(v); s != "" {
				urlStr = s
			}
		} else {
			slog.Warn("urlbuilder: js option eval failed",
				"js", optJs[:min(len(optJs), 80)], "err", err)
		}
	}

	meta.URL = urlStr
	return meta, nil
}

// evalTemplateExpressions finds all {{...}} patterns and evaluates the content as JS.
// Handles cases like {{cookie.removeCookie(source.key)}}, {{source.key}}, etc.
// ponytail: simple regex-based extraction, no nesting support for {{...}} inside {{...}}.
// evalTemplateExpressions finds all {{...}} patterns and evaluates the inner content as JS.
// Uses brace-counting instead of regex to handle nested braces like {{var x={a:1}; x}}.
func findURLJSSegment(input string) int {
	for offset := 0; ; {
		rel := strings.Index(input[offset:], "@js:")
		if rel < 0 {
			return -1
		}
		index := offset + rel
		if index > 0 {
			prefix := strings.TrimRight(input[:index], " \t")
			if strings.HasSuffix(prefix, "\n") || strings.HasSuffix(prefix, "\r") {
				return index
			}
		}
		offset = index + len("@js:")
	}
}

func expandPageSelector(input string, page int) string {
	if !strings.Contains(input, "<,") {
		return input
	}
	pageSelRe := regexp.MustCompile(`<,[^>]*>`)
	return pageSelRe.ReplaceAllStringFunc(input, func(match string) string {
		parts := strings.Split(match[1:len(match)-1], ",")
		if page > 0 && page < len(parts) {
			return strings.TrimSpace(parts[page-1])
		}
		return strings.TrimSpace(parts[len(parts)-1])
	})
}

func evalTemplateExpressionsContext(ctx context.Context, s string, jsVM *JSVM, baseURL string, extra ...map[string]interface{}) string {
	var bindings map[string]interface{}
	if len(extra) > 0 {
		bindings = extra[0]
	}

	var result strings.Builder
	i := 0
	for i < len(s) {
		// Find next {{
		start := strings.Index(s[i:], "{{")
		if start == -1 {
			result.WriteString(s[i:])
			break
		}
		start += i
		result.WriteString(s[i:start])
		i = start + 2

		// Find matching }} with brace-counting for nested {}
		depth := 0
		end := -1
		for j := i; j < len(s); j++ {
			if s[j] == '{' && j+1 < len(s) && s[j+1] == '{' {
				// Nested {{ — shouldn't happen but handle safely
				depth++
				j++
			} else if j+1 < len(s) && s[j] == '}' && s[j+1] == '}' {
				if depth == 0 {
					end = j
					break
				}
				depth--
				j++
			} else if s[j] == '{' {
				depth++
			} else if s[j] == '}' {
				depth--
				if depth < 0 {
					// Unbalanced — treat as literal
					end = -1
					break
				}
			}
		}

		if end == -1 {
			// No matching }}, keep as literal
			result.WriteString(s[start:i])
			continue
		}

		inner := strings.TrimSpace(s[i:end])
		if inner == "" {
			result.WriteString(s[start : end+2])
			i = end + 2
			continue
		}

		var v interface{}
		var err error
		if bindings != nil {
			v, err = jsVM.EvalContext(ctx, inner, "", baseURL, bindings)
		} else {
			v, err = jsVM.EvalContext(ctx, inner, "", baseURL)
		}
		if err != nil {
			slog.Warn("urlbuilder: template eval failed", "expr", inner[:min(len(inner), 60)], "err", err)
			result.WriteString(s[start : end+2])
		} else {
			result.WriteString(ToString(v))
		}
		i = end + 2
	}
	return result.String()
}

// EncodeParamValue URL-encodes a value in the specified charset.
// If charset is empty or "utf-8", standard Go URL encoding is used.
// For legacy charsets (gbk, gb2312), the value is transcoded before encoding.
// ponytail: simple charset handling — most sources work with UTF-8.
func EncodeParamValue(value, charset string) string {
	if charset == "" || strings.EqualFold(charset, "utf-8") || strings.EqualFold(charset, "utf8") {
		return url.QueryEscape(value)
	}
	// For non-UTF-8 charsets, we need to transcode then URL-encode.
	// This handles gbk, gb2312 which are common in Chinese novel sites.
	encoded, err := encodeWithCharset(value, charset)
	if err != nil {
		// Fall back to UTF-8 encoding
		return url.QueryEscape(value)
	}
	return encoded
}

// encodeWithCharset converts a UTF-8 string to the target charset and URL-encodes it.
func encodeWithCharset(value, charset string) (string, error) {
	enc, err := lookupEncoding(charset)
	if err != nil {
		return "", err
	}
	// Encode to target charset bytes then URL-percent-encode each byte
	// Use transform.String to convert from UTF-8 to target charset
	raw, _, err := transform.String(enc.NewEncoder(), value)
	if err != nil {
		return "", err
	}
	// Percent-encode each byte (can't use url.QueryEscape — it re-encodes as UTF-8)
	rawBytes := []byte(raw)
	escaped := make([]byte, 0, len(rawBytes)*3)
	for _, b := range rawBytes {
		if isUnreserved(b) {
			escaped = append(escaped, b)
		} else {
			escaped = append(escaped, '%', hexChars[b>>4], hexChars[b&0x0f])
		}
	}
	return string(escaped), nil
}

var hexChars = []byte("0123456789ABCDEF")

func isUnreserved(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') ||
		b == '-' || b == '_' || b == '.' || b == '~'
}

// lookupEncoding returns the golang.org/x/text encoding for the given charset name.
func lookupEncoding(charset string) (encoding.Encoding, error) {
	switch strings.ToLower(charset) {
	case "gbk", "gb2312", "gb18030", "chs":
		return simplifiedchinese.GBK, nil
	case "big5", "big5hkscs", "cht":
		return traditionalchinese.Big5, nil
	default:
		return nil, fmt.Errorf("unsupported charset: %s", charset)
	}
}

// extractJSONOption finds a trailing ,{...} JSON option suffix and returns
// the URL before it, the option string, and whether found.
// Unlike findJSONOption (which only returned the comma index), this returns
// the brace-counted bounds so trailing JS remnants after the JSON object
// (e.g. ,{"method":"POST"}';extra stuff) don't corrupt json.Unmarshal.
func extractJSONOption(url string) (before string, option string, found bool) {
	for i := len(url) - 2; i >= 0; i-- {
		if url[i] == ',' {
			// Skip whitespace between comma and expected brace
			j := i + 1
			for j < len(url) && (url[j] == ' ' || url[j] == '\t' || url[j] == '\n' || url[j] == '\r') {
				j++
			}
			if j < len(url) && url[j] == '{' {
				// Brace-count to extract just the JSON object, ignoring trailing junk
				depth := 1
				k := j + 1
				for k < len(url) && depth > 0 {
					if url[k] == '{' {
						depth++
					} else if url[k] == '}' {
						depth--
					}
					k++
				}
				if depth == 0 {
					optionStr := url[j:k]
					if json.Valid([]byte(optionStr)) {
						// Return URL up to comma (not including it) and option without comma
						return url[:i], optionStr, true
					}
				}
			}
		}
	}
	return url, "", false
}

// findJSONOption is deprecated. Use extractJSONOption instead.
func findJSONOption(url string) int {
	before, _, found := extractJSONOption(url)
	if found {
		return len(before)
	}
	return -1
}
