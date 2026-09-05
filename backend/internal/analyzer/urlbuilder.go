package analyzer

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"regexp"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/traditionalchinese"
	"golang.org/x/text/transform"
)

// URLMeta holds the result of URL construction plus all parsed options
// from the legado URL JSON option suffix: URL,{"method":"POST","body":"...","charset":"gbk",...}
type URLMeta struct {
	URL            string
	Method         string            // GET or POST (default GET)
	Body           string            // POST body (with {{key}} already interpolated)
	Headers        map[string]string // per-URL extra headers
	Charset        string            // character encoding override (e.g. "gbk", "gb2312")
	Retry          int               // retry count on non-2xx
	WebView        bool              // if true, source needs JS rendering
	WebViewDelayMS int               // optional post-load delay for WebView scripts
	WebJS          string            // JavaScript executed by a WebView before extraction
	BodyJS         string            // JavaScript transformation for the response body
	DNSIP          string            // optional source DNS/IP override
	Origin         string            // request origin override
	Type           string            // file/content type for downloads
}

// urlOption mirrors legado's UrlOption data class.
// Uses RawMessage for fields that may be either a string or structured type.
type urlOption struct {
	Method           string          `json:"method"`
	Body             json.RawMessage `json:"body"`
	Charset          string          `json:"charset"`
	WebView          json.RawMessage `json:"webView,omitempty"`
	Retry            json.RawMessage `json:"retry,omitempty"`
	WebJs            string          `json:"webJs,omitempty"`
	BodyJs           string          `json:"bodyJs,omitempty"`
	Js               string          `json:"js,omitempty"`
	DnsIp            string          `json:"dnsIp,omitempty"`
	Headers          json.RawMessage `json:"headers,omitempty"`
	Type             string          `json:"type,omitempty"`
	Origin           string          `json:"origin,omitempty"`
	WebViewDelayTime json.RawMessage `json:"webViewDelayTime,omitempty"`
}

// serverID remains unsupported because this deployment has one browser worker; it is
// preserved in imported source JSON but does not select a remote Legado server.

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

// URLContext carries the same typed crawl objects used by page analyzers.
type URLContext struct {
	Book        map[string]interface{}
	Chapter     map[string]interface{}
	NextChapter map[string]interface{}
	JSLib       string
}

// BuildURLWithContext expands a URL while preserving request cancellation for URL JavaScript.
func BuildURLWithContext(ctx context.Context, template, key string, page int, baseURL string, jsVM *JSVM, sourceState SourceState) (*URLMeta, error) {
	return BuildURLWithContextData(ctx, template, key, page, baseURL, jsVM, sourceState, nil)
}

// BuildURLWithContextData expands a URL with optional book/chapter JavaScript context.
func BuildURLWithContextData(ctx context.Context, template, key string, page int, baseURL string, jsVM *JSVM, sourceState SourceState, data *URLContext) (*URLMeta, error) {
	if template == "" {
		return nil, fmt.Errorf("analyzer: empty URL template")
	}

	urlStr := strings.TrimSpace(template)

	if strings.Contains(strings.ToLower(urlStr), "<js>") {
		var err error
		urlStr, err = evalURLJSTags(ctx, urlStr, key, page, baseURL, jsVM, sourceState, data)
		if err != nil {
			return nil, err
		}
	}

	// URL scripts consume the raw rule before interpolation, page selection,
	// and option parsing, matching Legado's analyzeJs phase.
	if jsIndex := findURLJSSegment(urlStr); jsIndex > 0 {
		baseTemplate := strings.TrimSpace(urlStr[:jsIndex])
		if jsVM == nil {
			return nil, fmt.Errorf("urlbuilder: @js: no JS engine available")
		}
		bindings := urlBindings(data, key, page, baseURL, sourceState)
		bindings["result"] = baseTemplate
		value, err := evalURLScript(ctx, jsVM, urlStr[jsIndex+4:], "", baseURL, data, bindings)
		if err != nil {
			return nil, fmt.Errorf("urlbuilder: @js: eval failed: %w", err)
		}
		resultURL := ToString(value)
		resultBase := baseURL
		if state, ok := sourceState.(interface{ LastURL() string }); ok && !strings.HasPrefix(resultURL, "http://") && !strings.HasPrefix(resultURL, "https://") {
			if lastURL := state.LastURL(); lastURL != "" {
				resultBase = lastURL
			}
		}
		return BuildURLWithContextData(ctx, resultURL, key, page, resultBase, jsVM, sourceState, data)
	}

	meta := &URLMeta{
		Method:  "GET",
		Headers: make(map[string]string),
	}

	// Strip newlines and carriage returns for non-@js: URLs.
	// @js: URLs need newlines for JS code. We remove newlines entirely so
	// that ,\n{ stays adjacent for trailing JSON-option extraction.
	if !strings.HasPrefix(urlStr, "@js:") {
		urlStr = strings.NewReplacer("\n", "", "\r", "").Replace(urlStr)
		urlStr = strings.TrimSpace(urlStr)
	}

	// @js: URL construction
	if strings.HasPrefix(urlStr, "@js:") {
		jsCode := urlStr[4:]
		if jsVM != nil {
			v, err := evalURLScript(ctx, jsVM, jsCode, "", baseURL, data, urlBindings(data, key, page, baseURL, sourceState))
			if err == nil {
				// Legado allows @js to return `url,{...options}`. Re-run
				// the normal URL parser so the returned POST/body/charset
				// metadata is not treated as part of the URL path.
				resultURL := ToString(v)
				resultBase := baseURL
				if state, ok := sourceState.(interface{ LastURL() string }); ok && !strings.HasPrefix(resultURL, "http://") && !strings.HasPrefix(resultURL, "https://") {
					if lastURL := state.LastURL(); lastURL != "" {
						resultBase = lastURL
					}
				}
				return BuildURLWithContextData(ctx, resultURL, key, page, resultBase, jsVM, sourceState, data)
			}
			return nil, fmt.Errorf("urlbuilder: @js: eval failed: %w", err)
		}
		return nil, fmt.Errorf("urlbuilder: @js: no JS engine available")
	}

	// Expand the entire rule before page selection and option parsing. Otherwise
	// JS comparison operators can be mistaken for selectors and options miss
	// interpolation (or cannot be parsed until their expressions are expanded).
	var err error
	urlStr, err = evalTemplateExpressionsContext(ctx, urlStr, jsVM, baseURL, urlBindings(data, key, page, baseURL, sourceState))
	if err != nil {
		return nil, fmt.Errorf("urlbuilder: template eval failed: %w", err)
	}
	urlStr = expandPageSelector(urlStr, page)

	// Store the js option for eval after URL is fully constructed.
	var optJs string
	if before, option, ok := extractJSONOption(urlStr); ok {
		urlStr = before
		optJs, err = applyURLJSONOption(meta, option)
		if err != nil {
			slog.Warn("urlbuilder: failed to parse URL JSON option",
				"option", option[:min(len(option), 100)], "err", err)
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
		bindings := urlBindings(data, key, page, baseURL, sourceState)
		bindings["java"] = map[string]interface{}{
			"url":       urlStr, // mutable — JS can change it
			"headerMap": meta.Headers,
		}
		if v, err := evalURLScript(ctx, jsVM, optJs, "", baseURL, data, bindings); err == nil {
			if s := ToString(v); s != "" {
				urlStr = s
			}
		} else {
			slog.Warn("urlbuilder: js option eval failed",
				"js", optJs[:min(len(optJs), 80)], "err", err)
		}
	}

	if meta.Method != "POST" && meta.Charset != "" {
		urlStr = encodeURLQuery(urlStr, meta.Charset)
	}

	meta.URL = urlStr
	return meta, nil
}

func encodeURLQuery(urlStr, charset string) string {
	if strings.EqualFold(charset, "utf-8") || strings.EqualFold(charset, "utf8") {
		return urlStr
	}
	question := strings.IndexByte(urlStr, '?')
	if question < 0 {
		return urlStr
	}
	fragment := strings.IndexByte(urlStr[question+1:], '#')
	queryEnd := len(urlStr)
	if fragment >= 0 {
		queryEnd = question + 1 + fragment
	}
	query := encodeQueryWithCharset(urlStr[question+1:queryEnd], charset)
	return urlStr[:question+1] + query + urlStr[queryEnd:]
}

func encodeQueryWithCharset(query, charset string) string {
	var encoded strings.Builder
	for offset := 0; offset < len(query); {
		if query[offset] == '%' && offset+2 < len(query) && isHex(query[offset+1]) && isHex(query[offset+2]) {
			encoded.WriteString(query[offset : offset+3])
			offset += 3
			continue
		}
		char, size := utf8.DecodeRuneInString(query[offset:])
		if char < utf8.RuneSelf && isAllowedQueryByte(byte(char)) {
			encoded.WriteByte(byte(char))
		} else {
			encoded.WriteString(EncodeParamValue(query[offset:offset+size], charset))
		}
		offset += size
	}
	return encoded.String()
}

func isAllowedQueryByte(value byte) bool {
	if isUnreserved(value) {
		return true
	}
	switch value {
	case '!', '$', '%', '&', '(', ')', '*', '+', ',', '/', ':', ';', '=', '?', '@', '[', '\\', ']', '^', '`', '{', '|', '}':
		return true
	default:
		return false
	}
}

func isHex(value byte) bool {
	return value >= '0' && value <= '9' || value >= 'a' && value <= 'f' || value >= 'A' && value <= 'F'
}

// evalTemplateExpressions finds all {{...}} patterns and evaluates the content as JS.
// Handles cases like {{cookie.removeCookie(source.key)}}, {{source.key}}, etc.
// ponytail: simple regex-based extraction, no nesting support for {{...}} inside {{...}}.
// URLBindings returns standard and crawl bindings for response body JavaScript.
func URLBindings(data *URLContext, baseURL string, state SourceState) map[string]interface{} {
	return urlBindings(data, "", 0, baseURL, state)
}

func urlBindings(data *URLContext, key string, page int, baseURL string, state SourceState) map[string]interface{} {
	bindings := map[string]interface{}{
		"key":            key,
		"page":           page,
		"baseUrl":        baseURL,
		"sourceState":    state,
		"nextChapter":    nil,
		"nextChapterUrl": nil,
	}
	if data == nil {
		return bindings
	}
	if data.Book != nil {
		bindings["book"] = data.Book
	}
	if data.Chapter != nil {
		bindings["chapter"] = data.Chapter
	}
	if data.NextChapter != nil {
		bindings["nextChapter"] = data.NextChapter
		bindings["nextChapterUrl"] = data.NextChapter["url"]
	}
	if data.JSLib != "" {
		bindings["__novelreader_jslib"] = data.JSLib
	}
	return bindings
}

// EvalURLScript evaluates JavaScript using URL and crawl context bindings.
func EvalURLScript(ctx context.Context, jsVM *JSVM, script, content, baseURL string, data *URLContext, bindings map[string]interface{}) (interface{}, error) {
	return evalURLScript(ctx, jsVM, script, content, baseURL, data, bindings)
}

func evalURLScript(ctx context.Context, jsVM *JSVM, script, content, baseURL string, data *URLContext, bindings map[string]interface{}) (interface{}, error) {
	if data != nil && data.JSLib != "" {
		script = data.JSLib + "\n" + script
	}
	return jsVM.EvalContext(ctx, script, content, baseURL, bindings)
}

// evalTemplateExpressions finds all {{...}} patterns and evaluates the inner content as JS.
// Uses brace-counting instead of regex to handle nested braces like {{var x={a:1}; x}}.
func parseOptionInt(raw json.RawMessage) int {
	if len(raw) == 0 {
		return 0
	}
	var value int
	if json.Unmarshal(raw, &value) == nil {
		return value
	}
	var text string
	if json.Unmarshal(raw, &text) == nil {
		fmt.Sscanf(text, "%d", &value)
	}
	return value
}

func applyURLJSONOption(meta *URLMeta, option string) (string, error) {
	var opt urlOption
	if err := json.Unmarshal([]byte(option), &opt); err != nil {
		return "", err
	}
	if opt.Method != "" {
		meta.Method = strings.ToUpper(opt.Method)
	}
	meta.Body = optionBodyString(opt.Body)
	meta.Charset = opt.Charset
	if len(opt.WebView) > 0 {
		var text string
		if json.Unmarshal(opt.WebView, &text) == nil {
			meta.WebView = text == "true" || text == "1"
		} else {
			_ = json.Unmarshal(opt.WebView, &meta.WebView)
		}
	}
	if len(opt.Headers) > 0 {
		if err := json.Unmarshal(opt.Headers, &meta.Headers); err != nil {
			var text string
			if json.Unmarshal(opt.Headers, &text) == nil && text != "" {
				_ = json.Unmarshal([]byte(text), &meta.Headers)
			}
		}
	}
	if len(opt.Retry) > 0 {
		_ = json.Unmarshal(opt.Retry, &meta.Retry)
		if meta.Retry == 0 {
			var text string
			if json.Unmarshal(opt.Retry, &text) == nil {
				_, _ = fmt.Sscanf(text, "%d", &meta.Retry)
			}
		}
	}
	meta.WebJS = opt.WebJs
	meta.WebViewDelayMS = parseOptionInt(opt.WebViewDelayTime)
	meta.BodyJS = opt.BodyJs
	meta.DNSIP = opt.DnsIp
	meta.Origin = opt.Origin
	meta.Type = opt.Type
	return opt.Js, nil
}

func optionBodyString(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return text
	}
	var value interface{}
	if err := json.Unmarshal(raw, &value); err != nil {
		return ""
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return string(encoded)
}

func evalURLJSTags(ctx context.Context, input, key string, page int, baseURL string, jsVM *JSVM, sourceState SourceState, data *URLContext) (string, error) {
	if jsVM == nil {
		return "", fmt.Errorf("urlbuilder: <js>: no JS engine available")
	}
	lower := strings.ToLower(input)
	result := input
	start := 0
	for {
		openRel := strings.Index(lower[start:], "<js>")
		if openRel < 0 {
			break
		}
		open := start + openRel
		closeRel := strings.Index(lower[open+4:], "</js>")
		if closeRel < 0 {
			return "", fmt.Errorf("urlbuilder: <js>: missing closing tag")
		}
		close := open + 4 + closeRel
		if prefix := strings.TrimSpace(input[start:open]); prefix != "" {
			result = strings.ReplaceAll(prefix, "@result", result)
		}
		bindings := urlBindings(data, key, page, baseURL, sourceState)
		bindings["result"] = result
		value, err := evalURLScript(ctx, jsVM, input[open+4:close], result, baseURL, data, bindings)
		if err != nil {
			return "", fmt.Errorf("urlbuilder: <js>: eval failed: %w", err)
		}
		result = ToString(value)
		start = close + len("</js>")
	}
	if suffix := strings.TrimSpace(input[start:]); suffix != "" {
		result = strings.ReplaceAll(suffix, "@result", result)
	}
	return result, nil
}

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
	selectPart := func(match string) string {
		parts := strings.Split(match[1:len(match)-1], ",")
		index := page - 1
		if index < 0 {
			index = 0
		}
		if index >= len(parts) {
			index = len(parts) - 1
		}
		return strings.TrimSpace(parts[index])
	}
	plainSelector := regexp.MustCompile(`<[^>]*,[^>]*>`)
	input = plainSelector.ReplaceAllStringFunc(input, selectPart)
	encodedSelector := regexp.MustCompile(`(?i)%3c(?:%[0-9a-f]{2}|[^%&])*%3e`)
	return encodedSelector.ReplaceAllStringFunc(input, func(match string) string {
		inner := match[len("%3C") : len(match)-len("%3E")]
		parts := regexp.MustCompile(`(?i)%2c`).Split(inner, -1)
		if len(parts) < 2 {
			return match
		}
		index := page - 1
		if index < 0 {
			index = 0
		}
		if index >= len(parts) {
			index = len(parts) - 1
		}
		return parts[index]
	})
}

func evalTemplateExpressionsContext(ctx context.Context, s string, jsVM *JSVM, baseURL string, bindings map[string]interface{}) (string, error) {
	return replaceTemplateExpressions(s, func(inner string) (string, error) {
		// Keep simple variables available without a JS engine, in the same pass
		// as other expressions so substituted text is not evaluated a second time.
		if inner == "key" || inner == "page" {
			return ToString(bindings[inner]), nil
		}
		if jsVM == nil {
			return "", fmt.Errorf("no JS engine available")
		}
		script := inner
		if jsLib, ok := bindings["__novelreader_jslib"].(string); ok && jsLib != "" {
			script = jsLib + "\n" + script
		}
		var value interface{}
		var err error
		if bindings != nil {
			value, err = jsVM.EvalContext(ctx, script, "", baseURL, bindings)
		} else {
			value, err = jsVM.EvalContext(ctx, script, "", baseURL)
		}
		return ToString(value), err
	})
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
// It returns brace-counted bounds so trailing JS remnants after the JSON object
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
					if normalized, ok := normalizeJSONOption(optionStr); ok {
						// Return URL up to comma (not including it) and normalized option.
						return url[:i], normalized, true
					}
				}
			}
		}
	}
	return url, "", false
}

func normalizeJSONOption(raw string) (string, bool) {
	if json.Valid([]byte(raw)) {
		return raw, true
	}
	var out strings.Builder
	out.Grow(len(raw))
	inDouble, inSingle, escaped := false, false, false
	for _, char := range raw {
		switch {
		case inDouble:
			out.WriteRune(char)
			if escaped {
				escaped = false
			} else if char == '\\' {
				escaped = true
			} else if char == '"' {
				inDouble = false
			}
		case inSingle:
			if escaped {
				if char == '"' {
					out.WriteString(`\"`)
				} else {
					out.WriteRune(char)
				}
				escaped = false
			} else if char == '\\' {
				escaped = true
			} else if char == '\'' {
				out.WriteByte('"')
				inSingle = false
			} else if char == '"' {
				out.WriteString(`\"`)
			} else {
				out.WriteRune(char)
			}
		default:
			switch char {
			case '"':
				inDouble = true
				out.WriteRune(char)
			case '\'':
				inSingle = true
				out.WriteByte('"')
			default:
				out.WriteRune(char)
			}
		}
	}
	if inDouble || inSingle || escaped {
		return "", false
	}
	normalized := quoteBareJSONKeys(out.String())
	return normalized, json.Valid([]byte(normalized))
}

func quoteBareJSONKeys(value string) string {
	var out strings.Builder
	out.Grow(len(value))
	inString, escaped := false, false
	for index := 0; index < len(value); index++ {
		char := value[index]
		out.WriteByte(char)
		if inString {
			if escaped {
				escaped = false
			} else if char == '\\' {
				escaped = true
			} else if char == '"' {
				inString = false
			}
			continue
		}
		if char == '"' {
			inString = true
			continue
		}
		if char != '{' && char != ',' {
			continue
		}
		start := index + 1
		for start < len(value) && isJSONWhitespace(value[start]) {
			out.WriteByte(value[start])
			start++
		}
		if start >= len(value) || value[start] == '"' || value[start] == '}' || value[start] == ']' {
			index = start - 1
			continue
		}
		end := start
		for end < len(value) && !isJSONWhitespace(value[end]) && !strings.ContainsRune(`:{}[],"`, rune(value[end])) {
			end++
		}
		colon := end
		for colon < len(value) && isJSONWhitespace(value[colon]) {
			colon++
		}
		if end > start && colon < len(value) && value[colon] == ':' {
			out.WriteByte('"')
			out.WriteString(value[start:end])
			out.WriteByte('"')
			out.WriteString(value[end : colon+1])
			index = colon
		} else {
			index = start - 1
		}
	}
	return out.String()
}

func isJSONWhitespace(char byte) bool {
	return char == ' ' || char == '\t' || char == '\n' || char == '\r'
}
