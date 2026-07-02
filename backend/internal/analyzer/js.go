package analyzer

import (
	"fmt"
	"sync"

	"github.com/dop251/goja"
)

// JSVM wraps a goja runtime for evaluating JavaScript in book source rules.
// ponytail: single global runtime per source group. Legado creates per-source scopes,
// but for Go we reuse one runtime to avoid per-eval overhead. Reset state between calls.
type JSVM struct {
	mu       sync.Mutex
	runtime  *goja.Runtime
	initCode string // shared JS lib code loaded once
}

// NewJSVM creates a new JS evaluation engine.
func NewJSVM() *JSVM {
	return &JSVM{
		runtime: goja.New(),
	}
}

// LoadLib sets shared JavaScript library code (from jsLib field).
func (vm *JSVM) LoadLib(code string) error {
	if code == "" {
		return nil
	}
	vm.mu.Lock()
	defer vm.mu.Unlock()
	vm.initCode = code
	_, err := vm.runtime.RunString(code)
	return err
}

// Eval evaluates JS and returns a string result.
// The script gets access to `result` (content), `baseUrl`, and `java` (helper object).
func (vm *JSVM) Eval(script, content, baseURL string) (interface{}, error) {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	// Set up the environment
	_ = vm.runtime.Set("result", content)
	_ = vm.runtime.Set("baseUrl", baseURL)
	_ = vm.runtime.Set("java", &jsHelpers{vm: vm})

	val, err := vm.runtime.RunString(script)
	if err != nil {
		return "", fmt.Errorf("js eval: %w", err)
	}
	return val.Export(), nil
}

// EvalList evaluates JS and expects a string array result.
func (vm *JSVM) EvalList(script, content, baseURL string) ([]string, error) {
	v, err := vm.Eval(script, content, baseURL)
	if err != nil {
		return nil, err
	}
	switch arr := v.(type) {
	case []interface{}:
		var result []string
		for _, item := range arr {
			result = append(result, fmt.Sprintf("%v", item))
		}
		return result, nil
	case []string:
		return arr, nil
	default:
		return []string{fmt.Sprintf("%v", v)}, nil
	}
}

// EvalElements evaluates JS and returns elements as interface{}.
func (vm *JSVM) EvalElements(script, content, baseURL string) ([]interface{}, error) {
	v, err := vm.Eval(script, content, baseURL)
	if err != nil {
		return nil, err
	}
	switch arr := v.(type) {
	case []interface{}:
		return arr, nil
	case interface{}:
		return []interface{}{arr}, nil
	default:
		return nil, fmt.Errorf("js eval: unexpected result type %T", v)
	}
}

// jsHelpers exposes book-source-friendly helpers available in JS code.
type jsHelpers struct {
	vm *JSVM
}

func (h *jsHelpers) Get(key string) string {
	return ""
}

func (h *jsHelpers) Put(key, value string) string {
	return value
}

// ponytail: jsHelpers minimal. Add only when a source script actually needs it
// (ajax, cookie, cache, toast). Most sources use CSS/XPath only.
