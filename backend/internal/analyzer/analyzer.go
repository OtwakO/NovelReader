// Package analyzer implements a legado-compatible rule parsing engine.
// It dispatches rule strings to the appropriate parser based on prefix:
// - CSS selectors (default / @CSS:)
// - XPath (/ or @XPath:)
// - JSONPath ($. or $[ or @Json:)
// - Regex (@js: or {{...}} or ##)
// - JavaScript (<js> or @js:)
package analyzer

import (
	"fmt"
	"strings"
)

// Mode indicates which parser to use for a rule.
type Mode int

const (
	ModeCSS    Mode = iota // Default / @CSS:
	ModeXPath             // / or @XPath:
	ModeJSON              // $. or $[ or @Json:
	ModeRegex             // ## patterns
	ModeJS                // <js> or @js:
)

// Result holds the output of a rule evaluation.
type Result struct {
	String string
	List   []string
	Elem   interface{} // single element (map, etc.)
	Elems  []interface{}
}

// Rule represents a single parsed rule segment with its mode and value.
type Rule struct {
	Mode       Mode
	Expression string
	// ReplaceRegex: post-extraction substitution via ##pattern##replacement### suffix
	ReplaceRegex string
	Replacement  string
	ReplaceFirst bool
}

// Analyzer evaluates rules against content (HTML or JSON string).
type Analyzer struct {
	content string
	baseURL string
	isJSON  bool
	jsVM    *JSVM
	cache   *CacheManager
}

// New creates an Analyzer for the given content.
func New(content, baseURL string, jsVM *JSVM, cache *CacheManager) *Analyzer {
	isJSON := looksLikeJSON(content)
	return &Analyzer{
		content: content,
		baseURL: baseURL,
		isJSON:  isJSON,
		jsVM:    jsVM,
		cache:   cache,
	}
}

// GetString evaluates a rule string and returns the first text result.
func (a *Analyzer) GetString(ruleStr string) (string, error) {
	rules, err := ParseRules(ruleStr, a.isJSON)
	if err != nil {
		return "", err
	}
	return a.evalString(rules)
}

// GetStringList evaluates a rule string and returns a list of text results.
func (a *Analyzer) GetStringList(ruleStr string) ([]string, error) {
	rules, err := ParseRules(ruleStr, a.isJSON)
	if err != nil {
		return nil, err
	}
	return a.evalStringList(rules)
}

// GetElement evaluates a rule and returns a single element.
func (a *Analyzer) GetElement(ruleStr string) (interface{}, error) {
	rules, err := ParseRules(ruleStr, a.isJSON)
	if err != nil {
		return nil, err
	}
	return a.evalElement(rules)
}

// GetElements evaluates a rule and returns a list of elements.
func (a *Analyzer) GetElements(ruleStr string) ([]interface{}, error) {
	rules, err := ParseRules(ruleStr, a.isJSON)
	if err != nil {
		return nil, err
	}
	return a.evalElements(rules)
}

// evalString evaluates a chain of rules returning a single string.
func (a *Analyzer) evalString(rules []Rule) (string, error) {
	current := a.content
	for _, rule := range rules {
		var err error
		current, err = a.applyRuleString(current, rule)
		if err != nil {
			return "", err
		}
	}
	return current, nil
}

// evalStringList evaluates a chain of rules returning a string list.
func (a *Analyzer) evalStringList(rules []Rule) ([]string, error) {
	current := a.content
	for i, rule := range rules {
		if i == len(rules)-1 {
			return a.applyRuleStringList(current, rule)
		}
		var err error
		current, err = a.applyRuleString(current, rule)
		if err != nil {
			return nil, err
		}
	}
	return []string{current}, nil
}

func (a *Analyzer) evalElement(rules []Rule) (interface{}, error) {
	current := interface{}(a.content)
	for _, rule := range rules {
		var err error
		current, err = a.applyRuleElement(current, rule)
		if err != nil {
			return nil, err
		}
	}
	return current, nil
}

func (a *Analyzer) evalElements(rules []Rule) ([]interface{}, error) {
	current := interface{}(a.content)
	for i, rule := range rules {
		if i == len(rules)-1 {
			return a.applyRuleElements(current, rule)
		}
		var err error
		current, err = a.applyRuleElement(current, rule)
		if err != nil {
			return nil, err
		}
	}
	return nil, fmt.Errorf("analyzer: no terminal rule in chain")
}

// applyRuleString runs a single rule against string content, returns string.
func (a *Analyzer) applyRuleString(content string, rule Rule) (string, error) {
	result, err := a.dispatch(rule.Mode, content, rule.Expression)
	if err != nil {
		return "", err
	}
	s := ToString(result)
	if rule.ReplaceRegex != "" {
		return applyReplace(s, rule.ReplaceRegex, rule.Replacement, rule.ReplaceFirst)
	}
	return s, nil
}

// applyRuleStringList runs a single rule returning a string list.
func (a *Analyzer) applyRuleStringList(content string, rule Rule) ([]string, error) {
	result, err := a.dispatchList(rule.Mode, content, rule.Expression)
	if err != nil {
		return nil, err
	}
	if rule.ReplaceRegex != "" {
		for i, s := range result {
			result[i], _ = applyReplace(s, rule.ReplaceRegex, rule.Replacement, rule.ReplaceFirst)
		}
	}
	return result, nil
}

func (a *Analyzer) applyRuleElement(content interface{}, rule Rule) (interface{}, error) {
	s := ToString(content)
	result, err := a.dispatch(rule.Mode, s, rule.Expression)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (a *Analyzer) applyRuleElements(content interface{}, rule Rule) ([]interface{}, error) {
	s := ToString(content)
	return a.dispatchElements(rule.Mode, s, rule.Expression)
}

// dispatch routes to the correct parser mode and returns a single result.
func (a *Analyzer) dispatch(mode Mode, content, expr string) (interface{}, error) {
	switch mode {
	case ModeCSS:
		return cssQuery(content, expr)
	case ModeXPath:
		return xpathQuery(content, expr)
	case ModeJSON:
		return jsonQuery(content, expr)
	case ModeRegex:
		return regexQuery(content, expr)
	case ModeJS:
		return a.jsEval(expr, content)
	default:
		return content, nil
	}
}

// dispatchList routes and returns a list of strings.
func (a *Analyzer) dispatchList(mode Mode, content, expr string) ([]string, error) {
	switch mode {
	case ModeCSS:
		return cssQueryList(content, expr)
	case ModeXPath:
		return xpathQueryList(content, expr)
	case ModeJSON:
		return jsonQueryList(content, expr)
	case ModeRegex:
		return regexQueryList(content, expr)
	case ModeJS:
		return a.jsEvalList(expr, content)
	default:
		return []string{content}, nil
	}
}

// dispatchElements routes and returns a list of elements.
func (a *Analyzer) dispatchElements(mode Mode, content, expr string) ([]interface{}, error) {
	switch mode {
	case ModeCSS:
		return cssQueryElements(content, expr)
	case ModeXPath:
		return xpathQueryElements(content, expr)
	case ModeJSON:
		return jsonQueryElements(content, expr)
	case ModeRegex:
		return regexQueryElements(content, expr)
	case ModeJS:
		return a.jsEvalElements(expr, content)
	default:
		return nil, fmt.Errorf("analyzer: unsupported mode for elements: %v", mode)
	}
}

func (a *Analyzer) jsEval(expr, content string) (interface{}, error) {
	if a.jsVM == nil {
		return "", fmt.Errorf("analyzer: JS engine not available")
	}
	return a.jsVM.Eval(expr, content, a.baseURL)
}

func (a *Analyzer) jsEvalList(expr, content string) ([]string, error) {
	if a.jsVM == nil {
		return nil, fmt.Errorf("analyzer: JS engine not available")
	}
	return a.jsVM.EvalList(expr, content, a.baseURL)
}

func (a *Analyzer) jsEvalElements(expr, content string) ([]interface{}, error) {
	if a.jsVM == nil {
		return nil, fmt.Errorf("analyzer: JS engine not available")
	}
	return a.jsVM.EvalElements(expr, content, a.baseURL)
}

// ToString converts a value to string.
func ToString(v interface{}) string {
	if v == nil {
		return ""
	}
	switch s := v.(type) {
	case string:
		return s
	case fmt.Stringer:
		return s.String()
	default:
		return fmt.Sprintf("%v", v)
	}
}

// looksLikeJSON checks if the content looks like JSON.
func looksLikeJSON(s string) bool {
	s = strings.TrimSpace(s)
	return strings.HasPrefix(s, "{") || strings.HasPrefix(s, "[")
}
