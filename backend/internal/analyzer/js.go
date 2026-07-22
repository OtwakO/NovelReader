package analyzer

import (
	"context"
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/dop251/goja"
	"github.com/otwako/novelreader/internal/fetcher"
)

// JSVM provides a pool of goja runtimes for JavaScript evaluation in book source rules.
// Each eval borrows a runtime, executes, and returns it.
type JSVM struct {
	pool        chan *goja.Runtime
	initCode    string
	mu          sync.Mutex
	hc          fetcher.HTTPClient     // for java.get/java.post from JS
	cacheData   map[string]string      // java.put/java.get storage
	memoryCache map[string]interface{} // cache.putMemory/cache.getFromMemory
}

// NewJSVM creates a JSVM with the compatibility pool size of 16 runtimes.
func NewJSVM() *JSVM { return NewJSVMWithPoolSize(16) }

// NewJSVMWithPoolSize creates a bounded JavaScript runtime pool.
func NewJSVMWithPoolSize(poolSize int) *JSVM {
	if poolSize < 1 {
		poolSize = 1
	}
	pool := make(chan *goja.Runtime, poolSize)
	for range poolSize {
		pool <- goja.New()
	}
	return &JSVM{pool: pool}
}

// SourceState is the session surface exposed to Legado JavaScript bindings.
// It is intentionally defined here so analyzer does not depend on sourceexec.
const RefreshExploreMemoryKey = "__novelreader_refresh_explore"

type SourceState interface {
	GetCookie(rawURL, key string) string
	CookieHeader(rawURL string) string
	SetCookie(rawURL, key, value string) error
	RemoveCookies(rawURL string) error
	GetVariable(key string) string
	PutVariable(key, value string)
	GetMemory(key string) interface{}
	PutMemory(key string, value interface{})
}

// SetFetcher provides an HTTP client for java.get/java.post calls from JS.
func (vm *JSVM) SetFetcher(hc fetcher.HTTPClient) {
	vm.hc = hc
}

// LoadLib validates and sets shared JavaScript library code for every fresh runtime.
func (vm *JSVM) LoadLib(code string) error {
	if code == "" {
		return nil
	}
	if _, err := goja.New().RunString(code); err != nil {
		return fmt.Errorf("js: load lib: %w", err)
	}
	vm.mu.Lock()
	vm.initCode = code
	vm.mu.Unlock()

	available := len(vm.pool)
	for range available {
		<-vm.pool
	}
	for range available {
		vm.pool <- vm.newRuntime()
	}
	return nil
}

func (vm *JSVM) newRuntime() *goja.Runtime {
	rt := goja.New()
	vm.mu.Lock()
	code := vm.initCode
	vm.mu.Unlock()
	if code != "" {
		_, _ = rt.RunString(code)
	}
	return rt
}

// Eval evaluates JS on a borrowed runtime with standard bindings.
// extra can contain: key, page, book (map), chapter (map), src (alias for content).
func (vm *JSVM) Eval(script string, content interface{}, baseURL string, extra ...map[string]interface{}) (interface{}, error) {
	return vm.EvalContext(context.Background(), script, content, baseURL, extra...)
}

// EvalContext evaluates JavaScript while preserving the caller's cancellation context.
func (vm *JSVM) EvalContext(ctx context.Context, script string, content interface{}, baseURL string, extra ...map[string]interface{}) (interface{}, error) {
	var rt *goja.Runtime
	select {
	case rt = <-vm.pool:
	case <-ctx.Done():
		return "", fmt.Errorf("js eval: %w", ctx.Err())
	}
	interruptDone := make(chan struct{})
	stopInterrupt := context.AfterFunc(ctx, func() {
		rt.Interrupt(ctx.Err())
		close(interruptDone)
	})
	defer func() {
		if !stopInterrupt() {
			<-interruptDone
		}
		rt.ClearInterrupt()
		vm.pool <- vm.newRuntime()
	}()

	// Bootstrap polyfills that legado's Rhino supports but goja doesn't.
	// Map() — In Legado's Android Rhino, Map() is called as a function (not constructor)
	// for value mapping / parsing. Sources use: Map("search").split(",")[0]||1
	// In goja, Map is the ES6 Map constructor which requires `new`. We shadow it.
	_, _ = rt.RunString(`
Map = function(a) {
  if (a === undefined || a === null) return "";
  if (typeof a === 'string') return a;
  return String(a || "");
};
`)

	// Set up org.jsoup bridge — legado sources use org.jsoup.Jsoup.parse(html).select(css)
	// to parse HTML strings in JS. We delegate to goquery.
	org := newJSoupBridge(rt, baseURL)
	_ = rt.Set("org", org)

	var sourceState SourceState
	var activeAnalyzer *Analyzer
	if len(extra) > 0 {
		if state, ok := extra[0]["sourceState"].(SourceState); ok {
			sourceState = state
		}
		if current, ok := extra[0]["analyzer"].(*Analyzer); ok {
			activeAnalyzer = current
		}
	}

	// Bind standard objects
	// java MUST be a map with lowercase keys — goja exposes Go struct methods capitalized
	// (EncodeURI, not encodeURI). Following legado's JsExtensions naming convention.
	hc := vm.hc
	if session, ok := sourceState.(fetcher.CookieSession); ok && hc != nil {
		const clientMemoryKey = "__novelreader_js_http_client"
		if cached, ok := sourceState.GetMemory(clientMemoryKey).(fetcher.HTTPClient); ok {
			hc = cached
		} else {
			if factory, ok := vm.hc.(interface {
				ForSource(fetcher.CookieSession) fetcher.HTTPClient
			}); ok {
				hc = factory.ForSource(session)
			} else {
				hc = fetcher.NewSessionHTTPClient(hc, session)
			}
			sourceState.PutMemory(clientMemoryKey, hc)
		}
	}
	h := &jsHelpers{vm: vm, rt: rt, hc: hc, ctx: ctx, analyzer: activeAnalyzer, state: sourceState, baseURL: baseURL}
	_ = rt.Set("result", content)
	_ = rt.Set("src", content) // alias matching legado's `src` variable
	_ = rt.Set("baseUrl", baseURL)
	_ = rt.Set("java", map[string]interface{}{
		"get":            h.Get,
		"put":            h.Put,
		"post":           h.Post,
		"ajax":           h.Ajax,
		"connect":        h.Connect,
		"md5Encode":      h.Md5Encode,
		"md5Encode16":    h.Md5Encode16,
		"base64Encode":   h.Base64Encode,
		"base64Decode":   h.Base64Decode,
		"encodeURI":      h.EncodeURI,
		"randomUUID":     h.RandomUUID,
		"timeFormat":     h.TimeFormat,
		"toNumChapter":   h.ToNumChapter,
		"t2s":            h.T2S,
		"toast":          h.Toast,
		"androidId":      h.AndroidId,
		"log":            h.Log,
		"getString":      h.GetString,
		"getElement":     h.GetElement,
		"getElements":    h.GetElements,
		"setContent":     h.SetContent,
		"HMacHex":        h.HMacHex,
		"decode":         h.Decode,
		"login":          h.Login,
		"refreshExplore": h.RefreshExplore,
	})
	sourceObj := make(map[string]interface{})
	if len(extra) > 0 {
		if metadata, ok := extra[0]["source"].(map[string]interface{}); ok {
			for key, value := range metadata {
				sourceObj[key] = value
			}
		}
	}
	for key, value := range vm.makeSourceObj(baseURL, sourceState) {
		sourceObj[key] = value
	}
	_ = rt.Set("source", sourceObj)
	_ = rt.Set("cookie", vm.makeCookieObj(sourceState))
	_ = rt.Set("cache", vm.makeCacheObj(sourceState))

	// Set extra bindings (key, page, book, chapter, etc.)
	if len(extra) > 0 {
		for k, v := range extra[0] {
			if k == "analyzer" || k == "source" {
				continue
			}
			_ = rt.Set(k, v)
		}
	}

	// Bind standalone helper functions that legado provides globally.
	// decode — used by some sources for base64-encoded content.
	_ = rt.Set("decode", h.Decode)

	// Declarations must be block-scoped because pooled runtimes retain global
	// lexical bindings across source evaluations.
	// Simple scripts without declarations preserve the final expression value.
	wrapped := script
	trimmedScript := strings.TrimSpace(script)
	if strings.Contains(script, "\nvar ") || strings.Contains(script, "\nlet ") || strings.Contains(script, "\nconst ") ||
		strings.HasPrefix(trimmedScript, "var ") || strings.HasPrefix(trimmedScript, "let ") || strings.HasPrefix(trimmedScript, "const ") {
		wrapped = "{ " + script + " }"
	}

	val, err := rt.RunString(wrapped)
	if err != nil {
		if ctx.Err() != nil {
			return "", fmt.Errorf("js eval: %w", ctx.Err())
		}
		return "", fmt.Errorf("js eval: %w", err)
	}
	if object, ok := val.(*goja.Object); ok {
		html := object.Get("__html")
		if html != nil && !goja.IsUndefined(html) && !goja.IsNull(html) {
			return map[string]interface{}{"__html": html.String()}, nil
		}
	}
	return val.Export(), nil
}

// EvalList evaluates JS and returns a string array.
func (vm *JSVM) EvalList(script string, content interface{}, baseURL string, extra ...map[string]interface{}) ([]string, error) {
	return vm.EvalListContext(context.Background(), script, content, baseURL, extra...)
}

func (vm *JSVM) EvalListContext(ctx context.Context, script string, content interface{}, baseURL string, extra ...map[string]interface{}) ([]string, error) {
	v, err := vm.EvalContext(ctx, script, content, baseURL, extra...)
	if err != nil {
		return nil, err
	}
	arr, ok := v.([]interface{})
	if !ok {
		return []string{fmt.Sprintf("%v", v)}, nil
	}
	result := make([]string, len(arr))
	for i, item := range arr {
		result[i] = fmt.Sprintf("%v", item)
	}
	return result, nil
}

// EvalElements evaluates JS and returns elements as interface{}.
func (vm *JSVM) EvalElements(script string, content interface{}, baseURL string, extra ...map[string]interface{}) ([]interface{}, error) {
	return vm.EvalElementsContext(context.Background(), script, content, baseURL, extra...)
}

func (vm *JSVM) EvalElementsContext(ctx context.Context, script string, content interface{}, baseURL string, extra ...map[string]interface{}) ([]interface{}, error) {
	v, err := vm.EvalContext(ctx, script, content, baseURL, extra...)
	if err != nil {
		return nil, err
	}
	arr, ok := v.([]interface{})
	if !ok {
		return []interface{}{v}, nil
	}
	return arr, nil
}

// makeSourceObj creates a JS object with source.key, source.getKey(), source.getVariable(), etc.
func (vm *JSVM) makeSourceObj(baseURL string, state SourceState) map[string]interface{} {
	src := &jsSource{baseURL: baseURL, vm: vm, state: state}
	return map[string]interface{}{
		"key":         baseURL,
		"getKey":      func() string { return baseURL },
		"getVariable": func() string { return src.GetVariable() },
		"putVariable": func(v string) { src.PutVariable(v) },
		"setVariable": func(v string) { src.PutVariable(v) },
		"get":         func(k string) string { return src.Get(k) },
		"put":         func(k, v string) string { return src.Put(k, v) },
	}
}

// makeCookieObj creates a JS object backed by the current source session.
func (vm *JSVM) makeCookieObj(state SourceState) map[string]interface{} {
	return map[string]interface{}{
		"removeCookie": func(url string) string {
			if state != nil {
				_ = state.RemoveCookies(url)
			}
			return ""
		},
		"getCookie": func(url string) string {
			if state == nil {
				return ""
			}
			return state.CookieHeader(url)
		},
		"getKey": func(url, key string) string {
			if state == nil {
				return ""
			}
			return state.GetCookie(url, key)
		},
		"setCookie": func(url, cookie string) {
			if state == nil {
				return
			}
			for _, pair := range strings.Split(cookie, ";") {
				parts := strings.SplitN(strings.TrimSpace(pair), "=", 2)
				if len(parts) == 2 {
					_ = state.SetCookie(url, strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
				}
			}
		},
	}
}

func (vm *JSVM) makeCacheObj(state SourceState) map[string]interface{} {
	put := func(key string, value interface{}) {
		if state != nil {
			state.PutMemory(key, value)
		}
	}
	get := func(key string) interface{} {
		if state == nil {
			return nil
		}
		return state.GetMemory(key)
	}
	return map[string]interface{}{
		"put": put, "get": get,
		"putMemory": put, "getFromMemory": get,
	}
}

// --- java.* bridge implementation ---

type jsHelpers struct {
	vm       *JSVM
	rt       *goja.Runtime
	hc       fetcher.HTTPClient
	ctx      context.Context
	analyzer *Analyzer
	state    SourceState
	baseURL  string
}

type responseCookieState interface {
	SetCookies(rawURL string, cookies []*http.Cookie) error
}

type responseURLState interface {
	SetLastURL(rawURL string)
}

func (h *jsHelpers) get(rawURL string, headers map[string]string) (*fetcher.Response, error) {
	if client, ok := h.hc.(fetcher.ContextHTTPClient); ok {
		return client.GetContext(h.ctx, rawURL, headers)
	}
	return h.hc.Get(rawURL, headers)
}

func (h *jsHelpers) getContext(ctx context.Context, rawURL string, headers map[string]string) (*fetcher.Response, error) {
	return h.getContextOptions(ctx, rawURL, headers, 0, "")
}

func (h *jsHelpers) getContextOptions(ctx context.Context, rawURL string, headers map[string]string, retry int, charset string) (*fetcher.Response, error) {
	if charset != "" {
		if client, ok := h.hc.(interface {
			GetContextWithCharset(context.Context, string, map[string]string, int, string) (*fetcher.Response, error)
		}); ok {
			return client.GetContextWithCharset(ctx, rawURL, headers, retry, charset)
		}
	}
	if client, ok := h.hc.(fetcher.ContextHTTPClient); ok {
		return client.GetContext(ctx, rawURL, headers, retry)
	}
	return h.hc.Get(rawURL, headers)
}

func (h *jsHelpers) getNoRedirect(rawURL string, headers map[string]string) (*fetcher.Response, error) {
	if client, ok := h.hc.(fetcher.ContextHTTPClient); ok {
		return client.GetContextNoRedirect(h.ctx, rawURL, headers)
	}
	return h.hc.GetContextNoRedirect(h.ctx, rawURL, headers)
}

func (h *jsHelpers) post(rawURL, body string, headers map[string]string) (*fetcher.Response, error) {
	return h.postContext(h.ctx, rawURL, body, headers)
}

func (h *jsHelpers) postContext(ctx context.Context, rawURL, body string, headers map[string]string) (*fetcher.Response, error) {
	return h.postContextOptions(ctx, rawURL, body, headers, 0, "")
}

func (h *jsHelpers) postContextOptions(ctx context.Context, rawURL, body string, headers map[string]string, retry int, charset string) (*fetcher.Response, error) {
	if charset != "" {
		if client, ok := h.hc.(interface {
			PostContextWithCharset(context.Context, string, string, map[string]string, int, string) (*fetcher.Response, error)
		}); ok {
			return client.PostContextWithCharset(ctx, rawURL, body, headers, retry, charset)
		}
	}
	if client, ok := h.hc.(fetcher.ContextHTTPClient); ok {
		return client.PostContext(ctx, rawURL, body, headers, retry)
	}
	contentType := ajaxHeader(headers, "Content-Type")
	if contentType == "" {
		contentType = "application/x-www-form-urlencoded"
	}
	return h.hc.Post(rawURL, contentType, body, headers)
}

func (h *jsHelpers) headContext(ctx context.Context, rawURL string, headers map[string]string, retry int) (*fetcher.Response, error) {
	client, ok := h.hc.(interface {
		HeadContextWithCharset(context.Context, string, map[string]string, int) (*fetcher.Response, error)
	})
	if !ok {
		return nil, fmt.Errorf("js ajax: HEAD is not supported by the configured client")
	}
	return client.HeadContextWithCharset(ctx, rawURL, headers, retry)
}

func jsDuration(value interface{}) time.Duration {
	var seconds float64
	switch v := value.(type) {
	case int:
		seconds = float64(v)
	case int64:
		seconds = float64(v)
	case float64:
		seconds = v
	default:
		return 0
	}
	if seconds <= 0 {
		return 0
	}
	return time.Duration(seconds) * time.Millisecond
}

func (h *jsHelpers) syncResponseCookies(rawURL string, headers http.Header) {
	if state, ok := h.state.(responseURLState); ok && state != nil && rawURL != "" {
		state.SetLastURL(rawURL)
	}
	state, ok := h.state.(responseCookieState)
	if !ok || state == nil {
		return
	}
	response := &http.Response{Header: headers}
	if err := state.SetCookies(rawURL, response.Cookies()); err != nil {
		slog.Debug("js: response cookie sync failed", "url", rawURL, "err", err)
	}
}

func responseURL(response *fetcher.Response, fallback string) string {
	if response != nil && response.URL != "" {
		return response.URL
	}
	return fallback
}

func (h *jsHelpers) responseObject(body string, status int, headers http.Header, finalURL, errorMessage string) interface{} {
	if h.rt == nil {
		return map[string]interface{}{"body": body, "bodyText": body, "statusCode": status, "error": errorMessage}
	}
	headerObject := h.rt.NewObject()
	getHeader := func(name string) string {
		if headers == nil {
			return ""
		}
		return headers.Get(name)
	}
	headerObject.Set("get", func(call goja.FunctionCall) goja.Value {
		return h.rt.ToValue(getHeader(call.Argument(0).String()))
	})
	responseObject := h.rt.NewObject()
	responseObject.Set("body", func() string { return body })
	responseObject.Set("bodyText", body)
	responseObject.Set("statusCode", status)
	responseObject.Set("code", func() int { return status })
	responseObject.Set("headers", func() *goja.Object { return headerObject })
	responseObject.Set("header", func(call goja.FunctionCall) goja.Value {
		return h.rt.ToValue(getHeader(call.Argument(0).String()))
	})
	if errorMessage != "" {
		responseObject.Set("error", errorMessage)
	}
	rawObject := h.rt.NewObject()
	rawObject.Set("body", func() string { return body })
	rawObject.Set("code", func() int { return status })
	rawObject.Set("headers", func() *goja.Object { return headerObject })
	rawObject.Set("request", func() *goja.Object {
		requestObject := h.rt.NewObject()
		requestObject.Set("url", finalURL)
		requestObject.Set("headers", headerObject)
		return requestObject
	})
	responseObject.Set("raw", func() *goja.Object { return rawObject })
	return responseObject
}

// Get performs HTTP GET or variable retrieval: java.get(url, headers?) or java.get(key)
func (h *jsHelpers) Get(arg1 string, args ...interface{}) interface{} {
	// Variable getter
	if len(args) == 0 && !strings.HasPrefix(arg1, "http") {
		h.vm.mu.Lock()
		defer h.vm.mu.Unlock()
		if h.vm.cacheData == nil {
			return ""
		}
		return h.vm.cacheData[arg1]
	}
	headers := make(map[string]string)
	if len(args) > 0 {
		if m, ok := args[0].(map[string]interface{}); ok {
			for k, v := range m {
				headers[k] = fmt.Sprint(v)
			}
		}
	}
	if h.hc == nil {
		return h.responseObject("", 0, nil, arg1, "")
	}
	resp, err := h.getNoRedirect(arg1, headers)
	if err != nil {
		return h.responseObject("", 0, nil, arg1, err.Error())
	}
	finalURL := responseURL(resp, arg1)
	h.syncResponseCookies(finalURL, resp.Headers)
	return h.responseObject(resp.Body, resp.StatusCode, resp.Headers, finalURL, "")
}

// Post performs HTTP POST: java.post(url, body, headers?)
func (h *jsHelpers) Post(urlStr, body string, args ...interface{}) interface{} {
	headers := make(map[string]string)
	if len(args) > 0 {
		if m, ok := args[0].(map[string]interface{}); ok {
			for k, v := range m {
				headers[k] = fmt.Sprint(v)
			}
		}
	}
	if h.hc == nil {
		return h.responseObject("", 0, nil, urlStr, "")
	}
	resp, err := h.post(urlStr, body, headers)
	if err != nil {
		return h.responseObject("", 0, nil, urlStr, err.Error())
	}
	finalURL := responseURL(resp, urlStr)
	h.syncResponseCookies(finalURL, resp.Headers)
	return h.responseObject(resp.Body, resp.StatusCode, resp.Headers, finalURL, "")
}

// connect performs HTTP and returns a chainable response object.
// Supports java.connect(url).raw().request().url() chain pattern.
func (h *jsHelpers) Connect(urlStr string, args ...interface{}) interface{} {
	headers := make(map[string]string)
	if len(args) > 0 {
		if s, ok := args[0].(string); ok && s != "" {
			// Parse header JSON string and forward to the fetch
			if err := json.Unmarshal([]byte(s), &headers); err != nil {
				slog.Debug("js: connect header parse failed", "err", err)
			}
		}
	}

	if h.hc == nil {
		return h.responseObject("", 0, nil, urlStr, "")
	}
	resp, err := h.get(urlStr, headers)
	if err != nil {
		return h.responseObject("", 0, nil, urlStr, err.Error())
	}
	finalURL := responseURL(resp, urlStr)
	h.syncResponseCookies(finalURL, resp.Headers)
	return h.responseObject(resp.Body, resp.StatusCode, resp.Headers, finalURL, "")
}

// --- Missing bridge methods ---

func (h *jsHelpers) TimeFormat(ts int64) string {
	return fmt.Sprintf("%d", ts) // ponytail: basic timestamp formatting
}

func (h *jsHelpers) AndroidId() string {
	return "goja-android-id" // ponytail: static value, used for API signing
}

func (h *jsHelpers) Log(msg interface{}) interface{} {
	slog.Info("js:log", "msg", fmt.Sprint(msg))
	return msg
}

var chapterNumberPattern = regexp.MustCompile(`第(.+?)章`)

func (h *jsHelpers) ToNumChapter(value interface{}) interface{} {
	if value == nil {
		return nil
	}
	title := fmt.Sprint(value)
	match := chapterNumberPattern.FindStringSubmatch(title)
	if len(match) != 2 {
		return title
	}
	return fmt.Sprintf("第%d章", chapterNumber(match[1]))
}

func (h *jsHelpers) Toast(msg interface{}) {
	slog.Info("js:toast", "message", fmt.Sprint(msg))
}

func chapterNumber(value string) int32 {
	var normalized strings.Builder
	for _, char := range value {
		switch {
		case char == '　':
			normalized.WriteByte(' ')
		case char >= '！' && char <= '～':
			normalized.WriteRune(char - 65248)
		default:
			normalized.WriteRune(char)
		}
	}
	compact := strings.Join(strings.Fields(normalized.String()), "")
	if number, err := strconv.ParseInt(compact, 10, 32); err == nil {
		return int32(number)
	}
	return chineseChapterNumber(compact)
}

var chineseChapterDigits = map[rune]int32{
	'〇': 0, '零': 0, '一': 1, '壹': 1, '二': 2, '贰': 2, '两': 2,
	'三': 3, '叁': 3, '四': 4, '肆': 4, '五': 5, '伍': 5, '六': 6,
	'陆': 6, '七': 7, '柒': 7, '八': 8, '捌': 8, '九': 9, '玖': 9,
	'十': 10, '拾': 10, '百': 100, '佰': 100, '千': 1000, '仟': 1000,
	'万': 10000, '亿': 100000000,
}

func chineseChapterNumber(value string) int32 {
	chars := []rune(value)
	var result, pending, billion int32
	for index, char := range chars {
		number, ok := chineseChapterDigits[char]
		if !ok {
			return -1
		}
		switch {
		case number == 100000000:
			result += pending
			result *= number
			billion = billion*number + result
			result, pending = 0, 0
		case number == 10000:
			result += pending
			result *= number
			pending = 0
		case number >= 10:
			if pending == 0 {
				pending = 1
			}
			result += number * pending
			pending = 0
		default:
			if index >= 2 && index == len(chars)-1 {
				previous := chineseChapterDigits[chars[index-1]]
				if previous > 10 {
					pending = number * previous / 10
					continue
				}
			}
			pending = pending*10 + number
		}
	}
	return result + pending + billion
}

// HMacHex computes HMAC hex digest: java.HMacHex(data, algorithm, key)
// Legado supports "HmacMD5", "HmacSHA1", "HmacSHA256"
func (h *jsHelpers) HMacHex(data, algorithm, key string) string {
	var mac hash.Hash
	switch strings.ToLower(algorithm) {
	case "hmacmd5", "hmac-md5", "md5":
		mac = hmac.New(md5.New, []byte(key))
	case "hmacsha1", "hmac-sha1", "sha1":
		mac = hmac.New(sha1.New, []byte(key))
	case "hmacsha256", "hmac-sha256", "sha256":
		mac = hmac.New(sha256.New, []byte(key))
	default:
		mac = hmac.New(md5.New, []byte(key))
	}
	mac.Write([]byte(data))
	return hex.EncodeToString(mac.Sum(nil))
}

// Decode decodes a base64 string: java.decode(str)
// Legado uses this to decode base64-encoded content in URL templates.
func (h *jsHelpers) Decode(str string) string {
	b, err := base64.StdEncoding.DecodeString(str)
	if err != nil {
		b, err = base64.URLEncoding.DecodeString(str)
		if err != nil {
			return str
		}
	}
	return string(b)
}

// Login is a stub for sources that require login.
func (h *jsHelpers) Login(args ...interface{}) string {
	return ""
}

func (h *jsHelpers) RefreshExplore() {
	if h.state != nil {
		h.state.PutMemory(RefreshExploreMemoryKey, true)
	}
}

func (h *jsHelpers) GetString(rule string, args ...interface{}) string {
	if h.analyzer == nil {
		return ""
	}
	value, err := h.analyzer.GetString(rule)
	if err != nil {
		return ""
	}
	return value
}

// GetElement evaluates a rule against current content and preserves element methods in JavaScript.
func (h *jsHelpers) GetElement(rule string, args ...interface{}) (interface{}, error) {
	if h.analyzer == nil {
		return nil, fmt.Errorf("java.getElement: analyzer unavailable")
	}
	values, err := h.analyzer.GetElements(rule)
	if errors.Is(err, ErrNoElements) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("java.getElement %q: %w", rule, err)
	}
	var html strings.Builder
	for _, value := range values {
		fragment := strings.TrimSpace(ToString(value))
		if !strings.HasPrefix(fragment, "<") {
			return collapseElementValues(values), nil
		}
		html.WriteString(fragment)
	}
	return makeJSoupSelectionFromHTML(h.rt, html.String())
}

func (h *jsHelpers) GetElements(rule string, args ...interface{}) []interface{} {
	if h.analyzer == nil {
		return nil
	}
	values, err := h.analyzer.GetElements(rule)
	if err != nil {
		return nil
	}
	return values
}

func (h *jsHelpers) SetContent(content interface{}) {
	if h.analyzer != nil {
		h.analyzer.SetContent(content)
	}
}

// Ajax executes a Legado URL, including its optional method/body/header suffix.
func (h *jsHelpers) Ajax(value interface{}, args ...interface{}) string {
	rawURL := firstAjaxURL(value)
	if h.hc == nil || rawURL == "" {
		return ""
	}
	meta := &URLMeta{Method: http.MethodGet, Headers: make(map[string]string)}
	if before, option, ok := extractJSONOption(rawURL); ok {
		rawURL = before
		if _, err := applyURLJSONOption(meta, option); err != nil {
			return ""
		}
	}
	resolved, err := resolveAjaxURL(rawURL, h.baseURL)
	if err != nil {
		return ""
	}
	meta.URL = resolved
	if strings.EqualFold(meta.Method, http.MethodPost) {
		prepareAjaxBody(meta)
	}
	requestCtx := h.ctx
	if len(args) > 0 {
		if timeout := jsDuration(args[0]); timeout > 0 {
			var cancel context.CancelFunc
			requestCtx, cancel = context.WithTimeout(h.ctx, timeout)
			defer cancel()
		}
	}
	var response *fetcher.Response
	switch strings.ToUpper(meta.Method) {
	case http.MethodGet:
		response, err = h.getContextOptions(requestCtx, meta.URL, meta.Headers, meta.Retry, meta.Charset)
	case http.MethodPost:
		response, err = h.postContextOptions(requestCtx, meta.URL, meta.Body, meta.Headers, meta.Retry, meta.Charset)
	case http.MethodHead:
		response, err = h.headContext(requestCtx, meta.URL, meta.Headers, meta.Retry)
	default:
		slog.Warn("js:ajax unsupported method", "method", meta.Method)
		return ""
	}
	if err != nil {
		return ""
	}
	h.syncResponseCookies(responseURL(response, meta.URL), response.Headers)
	return response.Body
}

func firstAjaxURL(value interface{}) string {
	switch values := value.(type) {
	case []interface{}:
		if len(values) > 0 {
			return fmt.Sprint(values[0])
		}
	case []string:
		if len(values) > 0 {
			return values[0]
		}
	default:
		return fmt.Sprint(value)
	}
	return ""
}

func prepareAjaxBody(meta *URLMeta) {
	contentType := ajaxHeader(meta.Headers, "Content-Type")
	if contentType == "" {
		if json.Valid([]byte(meta.Body)) {
			contentType = "application/json"
		} else {
			contentType = "application/x-www-form-urlencoded"
		}
		meta.Headers["Content-Type"] = contentType
	}
	if meta.Charset == "" || !strings.EqualFold(strings.TrimSpace(strings.SplitN(contentType, ";", 2)[0]), "application/x-www-form-urlencoded") {
		return
	}
	pairs := strings.Split(meta.Body, "&")
	for index, pair := range pairs {
		separator := strings.IndexByte(pair, '=')
		if separator >= 0 {
			pairs[index] = pair[:separator+1] + EncodeParamValue(pair[separator+1:], meta.Charset)
		}
	}
	meta.Body = strings.Join(pairs, "&")
}

func ajaxHeader(headers map[string]string, name string) string {
	for key, value := range headers {
		if strings.EqualFold(key, name) {
			return value
		}
	}
	return ""
}

func resolveAjaxURL(rawURL, baseURL string) (string, error) {
	reference, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return "", err
	}
	if reference.IsAbs() {
		return reference.String(), nil
	}
	base, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	return base.ResolveReference(reference).String(), nil
}

// put stores a value: java.put(key, value)
func (h *jsHelpers) Put(key, value string) string {
	// Store in a simple map on the JSVM
	h.vm.mu.Lock()
	if h.vm.cacheData == nil {
		h.vm.cacheData = make(map[string]string)
	}
	h.vm.cacheData[key] = value
	h.vm.mu.Unlock()
	return value
}

// md5Encode computes MD5 hex: java.md5Encode(str)
func (h *jsHelpers) Md5Encode(str string) string {
	hash := md5.Sum([]byte(str))
	return hex.EncodeToString(hash[:])
}

// md5Encode16 computes 16-char MD5: java.md5Encode16(str)
func (h *jsHelpers) Md5Encode16(str string) string {
	hash := md5.Sum([]byte(str))
	full := hex.EncodeToString(hash[:])
	return full[8:24]
}

// base64Encode: java.base64Encode(str)
func (h *jsHelpers) Base64Encode(str string) string {
	return base64.StdEncoding.EncodeToString([]byte(str))
}

// base64Decode: java.base64Decode(str)
func (h *jsHelpers) Base64Decode(str string) string {
	b, err := base64.StdEncoding.DecodeString(str)
	if err != nil {
		return ""
	}
	return string(b)
}

// randomUUID: java.randomUUID()
func (h *jsHelpers) RandomUUID() string {
	return fmt.Sprintf("%x-%x-%x-%x-%x",
		randUint64(), randUint64()&0xffff|0x4000,
		randUint64()&0x3fffffffffffffff|0x8000000000000000,
		randUint64(), randUint64())
}

// encodeURI: java.encodeURI(str)
func (h *jsHelpers) EncodeURI(str string) string {
	return url.QueryEscape(str)
}

// --- source.* bridge ---

type jsSource struct {
	baseURL string
	vm      *JSVM
	state   SourceState
	data    map[string]string
	mu      sync.Mutex
}

func (s *jsSource) GetKey() string { return s.baseURL }
func (s *jsSource) Key() string    { return s.baseURL }

func (s *jsSource) GetVariable() string {
	if s.state != nil {
		return s.state.GetVariable(s.baseURL)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	key := "sourceVariable_" + s.baseURL
	if s.vm != nil && s.vm.cacheData != nil {
		return s.vm.cacheData[key]
	}
	return ""
}

func (s *jsSource) PutVariable(v string) {
	if s.state != nil {
		s.state.PutVariable(s.baseURL, v)
		return
	}
	if s.vm == nil {
		return
	}
	s.vm.mu.Lock()
	defer s.vm.mu.Unlock()
	if s.vm.cacheData == nil {
		s.vm.cacheData = make(map[string]string)
	}
	s.vm.cacheData["sourceVariable_"+s.baseURL] = v
}

func (s *jsSource) Get(key string) string {
	if s.state != nil {
		if value, ok := s.state.GetMemory(key).(string); ok {
			return value
		}
		return ""
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.data == nil {
		return ""
	}
	return s.data[key]
}

func (s *jsSource) Put(key, value string) string {
	if s.state != nil {
		s.state.PutMemory(key, value)
		return value
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.data == nil {
		s.data = make(map[string]string)
	}
	s.data[key] = value
	return value
}

// --- cookie.* bridge ---

// ponytail: jsCookie struct removed — makeCookieObj returns an inline map.

// --- cache.* bridge ---

type jsCache struct {
	vm *JSVM
}

func (c *jsCache) PutMemory(key string, value interface{}) {
	c.vm.mu.Lock()
	defer c.vm.mu.Unlock()
	if c.vm.memoryCache == nil {
		c.vm.memoryCache = make(map[string]interface{})
	}
	c.vm.memoryCache[key] = value
}

func (c *jsCache) GetFromMemory(key string) interface{} {
	c.vm.mu.Lock()
	defer c.vm.mu.Unlock()
	if c.vm.memoryCache == nil {
		return nil
	}
	return c.vm.memoryCache[key]
}

// ponytail: simple rand for UUID, seeded from time to avoid deterministic sequences
var randSeed = uint64(time.Now().UnixNano())
var randMu sync.Mutex

func randUint64() uint64 {
	randMu.Lock()
	randSeed = randSeed*6364136223846793005 + 1442695040888963407
	randMu.Unlock()
	return randSeed
}

// newJSoupBridge creates the org.jsoup.Jsoup bridge for JS eval.
// Legado sources use:
//
//	var doc = org.jsoup.Jsoup.parse(html);
//	var el = doc.select(css).first();
//	var text = el.text();
//	var attr = el.attr('href');
//
// We delegate to goquery for CSS selection and provide the common element methods.
func newJSoupBridge(rt *goja.Runtime, baseURL string) map[string]interface{} {
	return map[string]interface{}{
		"jsoup": map[string]interface{}{
			"Jsoup": map[string]interface{}{
				"parse": func(html string) map[string]interface{} {
					doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
					if err != nil {
						return map[string]interface{}{"select": func(string) *goja.Object { return emptyJSoupSelection(rt) }}
					}
					return map[string]interface{}{
						"select": func(css string) *goja.Object {
							var elements []map[string]interface{}
							doc.Find(css).Each(func(_ int, s *goquery.Selection) {
								elements = append(elements, makeJSoupElement(rt, s))
							})
							return makeJSoupSelection(rt, elements)
						},
					}
				},
			},
		},
	}
}

func makeJSoupElement(rt *goja.Runtime, s *goquery.Selection) map[string]interface{} {
	outerHTML, _ := goquery.OuterHtml(s)
	return map[string]interface{}{
		"__html":    outerHTML,
		"text":      func() string { return s.Text() },
		"ownText":   func() string { return ownText(s) },
		"html":      func() string { h, _ := s.Html(); return h },
		"outerHtml": func() string { h, _ := goquery.OuterHtml(s); return h },
		"attr":      func(name string) string { v, _ := s.Attr(name); return v },
		"val":       func() string { v, _ := s.Attr("value"); return v },
		"data":      func(name string) string { v, _ := s.Attr("data-" + name); return v },
		"select":    func(css string) *goja.Object { return makeJSoupSelectionFromGoquery(rt, s.Find(css)) },
		"first":     func() interface{} { return makeJSoupElement(rt, s.First()) },
		"last":      func() interface{} { return makeJSoupElement(rt, s.Last()) },
		"size":      1,
	}
}

func ownText(selection *goquery.Selection) string {
	var result strings.Builder
	selection.Contents().Each(func(_ int, child *goquery.Selection) {
		if goquery.NodeName(child) == "#text" {
			result.WriteString(child.Text())
		}
	})
	return result.String()
}

func emptyJSoupSelection(rt *goja.Runtime) *goja.Object { return makeJSoupSelection(rt, nil) }

func makeJSoupSelectionFromHTML(rt *goja.Runtime, html string) (*goja.Object, error) {
	html, selector := contextualizeHTMLSelection(html)
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, fmt.Errorf("java.getElement: parse selection: %w", err)
	}
	return makeJSoupSelectionFromGoquery(rt, doc.Find(selector)), nil
}

func makeJSoupSelectionFromGoquery(rt *goja.Runtime, selection *goquery.Selection) *goja.Object {
	var elements []map[string]interface{}
	selection.Each(func(_ int, s *goquery.Selection) { elements = append(elements, makeJSoupElement(rt, s)) })
	return makeJSoupSelection(rt, elements)
}

func makeJSoupSelection(rt *goja.Runtime, elements []map[string]interface{}) *goja.Object {
	selection := rt.NewArray()
	fragments := make([]interface{}, 0, len(elements))
	for index, element := range elements {
		fragments = append(fragments, element["__html"])
		_ = selection.Set(strconv.Itoa(index), element)
	}
	properties := map[string]interface{}{
		"__html": serializeHTMLSelection(fragments),
		"size":   func() int { return len(elements) },
		"eq": func(index int) *goja.Object {
			if index < 0 {
				panic(rt.NewTypeError("Index must be greater than or equal to zero"))
			}
			if index >= len(elements) {
				return emptyJSoupSelection(rt)
			}
			return makeJSoupSelection(rt, elements[index:index+1])
		},
		"first": func() interface{} {
			if len(elements) == 0 {
				return nil
			}
			return elements[0]
		},
		"last": func() interface{} {
			if len(elements) == 0 {
				return nil
			}
			return elements[len(elements)-1]
		},
		"attr": func(name string) string {
			if len(elements) == 0 {
				return ""
			}
			return elements[0]["attr"].(func(string) string)(name)
		},
		"text": func() string {
			var result []string
			for _, element := range elements {
				result = append(result, element["text"].(func() string)())
			}
			return strings.Join(result, "")
		},
		"select": func(css string) *goja.Object {
			if len(elements) == 0 {
				return emptyJSoupSelection(rt)
			}
			return elements[0]["select"].(func(string) *goja.Object)(css)
		},
	}
	for name, value := range properties {
		_ = selection.DefineDataProperty(name, rt.ToValue(value), goja.FLAG_TRUE, goja.FLAG_TRUE, goja.FLAG_FALSE)
	}
	return selection
}
