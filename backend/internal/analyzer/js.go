package analyzer

import (
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"log/slog"
	"net/url"
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
	hc          *fetcher.Client        // for java.get/java.post from JS
	cacheData   map[string]string      // java.put/java.get storage
	memoryCache map[string]interface{} // cache.putMemory/cache.getFromMemory
}

// NewJSVM creates a JSVM with a pool of 16 runtimes.
func NewJSVM() *JSVM {
	const poolSize = 16
	pool := make(chan *goja.Runtime, poolSize)
	for range poolSize {
		pool <- goja.New()
	}
	return &JSVM{pool: pool}
}

// SourceState is the session surface exposed to Legado JavaScript bindings.
// It is intentionally defined here so analyzer does not depend on sourceexec.
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
func (vm *JSVM) SetFetcher(hc *fetcher.Client) {
	vm.hc = hc
}

// LoadLib sets shared JavaScript library code, reloading into all pool runtimes.
func (vm *JSVM) LoadLib(code string) error {
	if code == "" {
		return nil
	}
	vm.mu.Lock()
	vm.initCode = code
	vm.mu.Unlock()

	n := len(vm.pool)
	for range n {
		rt := <-vm.pool
		if _, err := rt.RunString(code); err != nil {
			vm.pool <- rt
			return fmt.Errorf("js: load lib: %w", err)
		}
		vm.pool <- rt
	}
	return nil
}

// Eval evaluates JS on a borrowed runtime with standard bindings.
// extra can contain: key, page, book (map), chapter (map), src (alias for content).
func (vm *JSVM) Eval(script, content, baseURL string, extra ...map[string]interface{}) (interface{}, error) {
	rt := <-vm.pool
	defer func() { vm.pool <- rt }()

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
	if len(extra) > 0 {
		if state, ok := extra[0]["sourceState"].(SourceState); ok {
			sourceState = state
		}
	}

	// Bind standard objects
	// java MUST be a map with lowercase keys — goja exposes Go struct methods capitalized
	// (EncodeURI, not encodeURI). Following legado's JsExtensions naming convention.
	h := &jsHelpers{vm: vm}
	_ = rt.Set("result", content)
	_ = rt.Set("src", content) // alias matching legado's `src` variable
	_ = rt.Set("baseUrl", baseURL)
	_ = rt.Set("java", map[string]interface{}{
		"get":          h.Get,
		"put":          h.Put,
		"post":         h.Post,
		"ajax":         h.Ajax,
		"connect":      h.Connect,
		"md5Encode":    h.Md5Encode,
		"md5Encode16":  h.Md5Encode16,
		"base64Encode": h.Base64Encode,
		"base64Decode": h.Base64Decode,
		"encodeURI":    h.EncodeURI,
		"randomUUID":   h.RandomUUID,
		"timeFormat":   h.TimeFormat,
		"androidId":    h.AndroidId,
		"log":          h.Log,
		"getString":    h.GetString,
		"getElements":  h.GetElements,
		"setContent":   h.SetContent,
		"HMacHex":      h.HMacHex,
		"decode":       h.Decode,
		"login":        h.Login,
	})
	_ = rt.Set("source", vm.makeSourceObj(baseURL, sourceState))
	_ = rt.Set("cookie", vm.makeCookieObj(sourceState))
	_ = rt.Set("cache", vm.makeCacheObj(sourceState))

	// Set extra bindings (key, page, book, chapter, etc.)
	if len(extra) > 0 {
		for k, v := range extra[0] {
			_ = rt.Set(k, v)
		}
	}

	// Bind standalone helper functions that legado provides globally.
	// decode — used by some sources for base64-encoded content.
	_ = rt.Set("decode", h.Decode)

	// Run the script. For scripts that use `var` at the top level (which can
	// collide across evals on the same pooled runtime), wrap in a block scope.
	// We detect top-level `var` by checking for `var ` at the start of a line.
	// Simple scripts without `var` are evaluated directly so the return value
	// from the last expression is preserved (critical for @js: URL builders).
	wrapped := script
	if strings.Contains(script, "\nvar ") || strings.HasPrefix(strings.TrimSpace(script), "var ") {
		wrapped = "{ " + script + " }"
	}

	val, err := rt.RunString(wrapped)
	if err != nil {
		return "", fmt.Errorf("js eval: %w", err)
	}
	return val.Export(), nil
}

// EvalList evaluates JS and returns a string array.
func (vm *JSVM) EvalList(script, content, baseURL string, extra ...map[string]interface{}) ([]string, error) {
	v, err := vm.Eval(script, content, baseURL, extra...)
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
func (vm *JSVM) EvalElements(script, content, baseURL string, extra ...map[string]interface{}) ([]interface{}, error) {
	v, err := vm.Eval(script, content, baseURL, extra...)
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
	return map[string]interface{}{
		"putMemory": func(key string, value interface{}) {
			if state != nil {
				state.PutMemory(key, value)
			}
		},
		"getFromMemory": func(key string) interface{} {
			if state == nil {
				return nil
			}
			return state.GetMemory(key)
		},
	}
}

// --- java.* bridge implementation ---

type jsHelpers struct {
	vm *JSVM
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
	if h.vm.hc == nil {
		return map[string]interface{}{"body": "", "statusCode": 0}
	}
	resp, err := h.vm.hc.Get(arg1, headers)
	if err != nil {
		return map[string]interface{}{"body": "", "statusCode": 0, "error": err.Error()}
	}
	result := make(map[string]interface{})
	result["body"] = resp.Body
	result["statusCode"] = resp.StatusCode
	hdr := make(map[string]string)
	for k, v := range resp.Headers {
		if len(v) > 0 {
			hdr[k] = v[0]
		}
	}
	result["headers"] = hdr
	return result
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
	if h.vm.hc == nil {
		return map[string]interface{}{"body": "", "statusCode": 0}
	}
	resp, err := h.vm.hc.Post(urlStr, "application/x-www-form-urlencoded", body, headers)
	if err != nil {
		return map[string]interface{}{"body": "", "statusCode": 0, "error": err.Error()}
	}
	return map[string]interface{}{"body": resp.Body, "statusCode": resp.StatusCode}
}

// connect performs HTTP and returns a chainable response object.
// Supports java.connect(url).raw().request().url() chain pattern.
func (h *jsHelpers) Connect(urlStr string, args ...interface{}) map[string]interface{} {
	headers := make(map[string]string)
	if len(args) > 0 {
		if s, ok := args[0].(string); ok && s != "" {
			// Parse header JSON string and forward to the fetch
			if err := json.Unmarshal([]byte(s), &headers); err != nil {
				slog.Debug("js: connect header parse failed", "err", err)
			}
		}
	}

	finalURL := urlStr
	body := ""
	code := 0
	respHeaders := make(map[string]interface{})

	if h.vm.hc != nil {
		resp, err := h.vm.hc.Get(urlStr, headers)
		if err == nil {
			body = resp.Body
			code = resp.StatusCode
			finalURL = resp.URL
			for k, vals := range resp.Headers {
				if len(vals) > 0 {
					respHeaders[k] = vals[0]
				}
			}
		}
	}

	return map[string]interface{}{
		"body":    body,
		"code":    code,
		"headers": respHeaders,
		// ponytail: raw.request.url chain matches legado's StrResponse shape.
		// .body(), .code(), .headers() also work as direct properties.
		"raw": func() map[string]interface{} {
			return map[string]interface{}{
				"body":    body,
				"code":    code,
				"headers": respHeaders,
				"request": func() map[string]interface{} {
					return map[string]interface{}{
						"url":     func() string { return finalURL },
						"headers": respHeaders,
					}
				},
			}
		},
	}
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

func (h *jsHelpers) GetString(rule string, args ...interface{}) string {
	// ponytail: basic rule evaluation in JS context — minimal implementation
	return ""
}

func (h *jsHelpers) GetElements(rule string, args ...interface{}) []interface{} {
	return nil
}

func (h *jsHelpers) SetContent(content string) {
	// ponytail: content switching in JS context — no-op for now
}

// ajax is like get but simpler: java.ajax(url)
func (h *jsHelpers) Ajax(urlStr interface{}, args ...interface{}) string {
	s := fmt.Sprint(urlStr)
	if h.vm.hc == nil {
		return ""
	}
	headers := make(map[string]string)
	if len(args) > 0 {
		if timeout, ok := args[0].(int64); ok {
			_ = timeout // ponytail: per-request timeout deferred
		}
	}
	resp, err := h.vm.hc.Get(s, headers)
	if err != nil {
		return ""
	}
	return resp.Body
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
						return map[string]interface{}{
							"select": func(string) []interface{} { return nil },
						}
					}
					return map[string]interface{}{
						"select": func(css string) []interface{} {
							var results []interface{}
							doc.Find(css).Each(func(_ int, s *goquery.Selection) {
								el := make(map[string]interface{})
								el["text"] = func() string { return s.Text() }
								el["ownText"] = s.Contents().Not("script").Not("style").Text()
								el["html"] = func() string {
									h, _ := s.Html()
									return h
								}
								el["outerHtml"] = func() string {
									h, _ := goquery.OuterHtml(s)
									return h
								}
								el["attr"] = func(name string) string {
									v, _ := s.Attr(name)
									return v
								}
								el["val"] = func() string {
									v, _ := s.Attr("value")
									return v
								}
								el["data"] = func(name string) string {
									v, _ := s.Attr("data-" + name)
									return v
								}
								// first/last/size are used for iteration
								el["first"] = func() interface{} { return nil } // ponytail: single element
								el["last"] = func() interface{} { return nil }
								el["size"] = 1
								results = append(results, el)
							})
							return results
						},
					}
				},
			},
		},
	}
}
