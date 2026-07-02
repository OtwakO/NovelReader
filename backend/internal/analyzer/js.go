package analyzer

import (
	"fmt"
	"sync"

	"github.com/dop251/goja"
)

// JSVM provides a pool of goja runtimes for evaluating JavaScript in book source rules.
// Each eval borrows a runtime from the pool, executes, and returns it.
// The pool avoids the overhead of creating runtimes per-eval while allowing
// concurrent evaluations across goroutines.
type JSVM struct {
	pool     chan *goja.Runtime
	initCode string
	mu       sync.Mutex
}

// NewJSVM creates a JSVM with a pool of runtimes.
// ponytail: 16 runtimes — JS eval is fast (ms-scale) but 50 goroutines contend for it.
// Increased from 4 for search throughput; ~15/256 sources use JS URLs.
func NewJSVM() *JSVM {
	const poolSize = 16
	pool := make(chan *goja.Runtime, poolSize)
	for range poolSize {
		pool <- goja.New()
	}
	return &JSVM{
		pool: pool,
	}
}

// LoadLib sets shared JavaScript library code and reloads it into all pool runtimes.
func (vm *JSVM) LoadLib(code string) error {
	if code == "" {
		return nil
	}
	vm.mu.Lock()
	vm.initCode = code
	vm.mu.Unlock()

	// Reload into all runtimes
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

// Eval evaluates JS on a borrowed runtime.
// initCode is loaded once into each runtime by LoadLib.
// Per-eval we only set bindings — no re-init.
func (vm *JSVM) Eval(script, content, baseURL string) (interface{}, error) {
	rt := <-vm.pool
	defer func() { vm.pool <- rt }()

	_ = rt.Set("result", content)
	_ = rt.Set("baseUrl", baseURL)
	_ = rt.Set("java", &jsHelpers{rt: rt})

	val, err := rt.RunString(script)
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

// jsHelpers exposes helpers available in JS code.
type jsHelpers struct {
	rt *goja.Runtime
}

func (h *jsHelpers) Get(key string) string {
	return ""
}

func (h *jsHelpers) Put(key, value string) string {
	return value
}

// ponytail: pool of 4 runtimes is a guess. Tune based on real usage.
// ponytail: jsHelpers stubs. Add ajax/cookie/cache when sources actually need them.
