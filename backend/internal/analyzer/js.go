package analyzer

import (
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"
	"sync"

	"github.com/dop251/goja"
	"github.com/otwako/novelreader/internal/fetcher"
)

// JSVM provides a pool of goja runtimes for JavaScript evaluation in book source rules.
// Each eval borrows a runtime, executes, and returns it.
type JSVM struct {
	pool        chan *goja.Runtime
	initCode    string
	mu          sync.Mutex
	hc          *fetcher.Client      // for java.get/java.post from JS
	cacheData   map[string]string    // java.put/java.get storage
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
// key/page are optional — set when evaluating URL templates from search context.
func (vm *JSVM) Eval(script, content, baseURL string, extra ...map[string]interface{}) (interface{}, error) {
	rt := <-vm.pool
	defer func() { vm.pool <- rt }()

	// Bind standard objects (use JS objects for property access patterns like source.key)
	_ = rt.Set("result", content)
	_ = rt.Set("baseUrl", baseURL)
	_ = rt.Set("java", &jsHelpers{vm: vm, rt: rt})
	_ = rt.Set("source", vm.makeSourceObj(baseURL))
	_ = rt.Set("cookie", vm.makeCookieObj())
	_ = rt.Set("cache", &jsCache{vm: vm})

	// Set extra bindings (key, page, etc.)
	if len(extra) > 0 {
		for k, v := range extra[0] {
			_ = rt.Set(k, v)
		}
	}

	val, err := rt.RunString(script)
	if err != nil {
		return "", fmt.Errorf("js eval: %w", err)
	}
	return val.Export(), nil
}

// EvalList evaluates JS and returns a string array.
func (vm *JSVM) EvalList(script, content, baseURL string) ([]string, error) {
	v, err := vm.Eval(script, content, baseURL)
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
func (vm *JSVM) EvalElements(script, content, baseURL string) ([]interface{}, error) {
	v, err := vm.Eval(script, content, baseURL)
	if err != nil {
		return nil, err
	}
	arr, ok := v.([]interface{})
	if !ok {
		return []interface{}{v}, nil
	}
	return arr, nil
}

// makeSourceObj creates a JS object with source.key and source.getKey() for goja.
func (vm *JSVM) makeSourceObj(baseURL string) map[string]interface{} {
	return map[string]interface{}{
		"key":     baseURL,
		"getKey": func() string { return baseURL },
	}
}

// makeCookieObj creates a JS object with cookie.removeCookie/getCookie/setCookie for goja.
func (vm *JSVM) makeCookieObj() map[string]interface{} {
	return map[string]interface{}{
		"removeCookie": func(url string) string { return "" },
		"getCookie":    func(url string) string { return "" },
		"getKey":       func(url, key string) string { return "" },
		"setCookie":    func(url, cookie string) {},
	}
}

// --- java.* bridge implementation ---

type jsHelpers struct {
	vm *JSVM
	rt *goja.Runtime
}

// Get performs HTTP GET or variable retrieval from JS: java.get(url, headers?) or java.get(key)
// If the first argument looks like a URL, it's an HTTP request; otherwise it's a variable lookup.
func (h *jsHelpers) Get(arg1 string, args ...interface{}) interface{} {
	// Variable getter: java.get(key)
	if len(args) == 0 && !strings.HasPrefix(arg1, "http") {
		h.vm.mu.Lock()
		defer h.vm.mu.Unlock()
		if h.vm.cacheData == nil {
			return ""
		}
		return h.vm.cacheData[arg1]
	}
	// HTTP GET: java.get(url, headers?)
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

// Post performs HTTP POST from JS: java.post(url, body, headers?)
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

// connect is similar to get but returns {body, code, headers}
func (h *jsHelpers) Connect(urlStr string, args ...interface{}) map[string]interface{} {
	headers := make(map[string]string)
	if len(args) > 0 {
		if s, ok := args[0].(string); ok && s != "" {
			// Header can be a JSON string: java.connect(url, '{"Referer":"..."}')
			_ = s // ponytail: JSON header parsing deferred
		}
	}
	if h.vm.hc == nil {
		return map[string]interface{}{"body": "", "code": 0}
	}
	resp, err := h.vm.hc.Get(urlStr, headers)
	if err != nil {
		return map[string]interface{}{"body": "", "code": 0, "error": err.Error()}
	}
	return map[string]interface{}{"body": resp.Body, "code": resp.StatusCode}
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
	data    map[string]string
	mu      sync.Mutex
}

func (s *jsSource) GetKey() string  { return s.baseURL }
func (s *jsSource) Key() string     { return s.baseURL }
func (s *jsSource) Get(key string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.data == nil {
		return ""
	}
	return s.data[key]
}
func (s *jsSource) Put(key, value string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.data == nil {
		s.data = make(map[string]string)
	}
	s.data[key] = value
	return value
}

// --- cookie.* bridge ---

type jsCookie struct{}

func (c *jsCookie) RemoveCookie(url string) string {
	// ponytail: cookie removal is a no-op in single-user mode.
	// In legado this deletes cookies for the domain from the persistent store.
	return ""
}

func (c *jsCookie) GetCookie(url string) string  { return "" }
func (c *jsCookie) GetKey(url, key string) string { return "" }
func (c *jsCookie) SetCookie(url, cookie string)  {}

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

// ponytail: simple rand for UUID, not crypto-safe
var randSeed = uint64(0)
var randMu sync.Mutex

func randUint64() uint64 {
	randMu.Lock()
	if randSeed == 0 {
		randSeed = 123456789
	}
	randSeed = randSeed*6364136223846793005 + 1442695040888963407
	randMu.Unlock()
	return randSeed
}
