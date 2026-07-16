// Package analyzer implements a legado-compatible rule parsing engine.
// It dispatches rule strings to the appropriate parser based on prefix:
// - CSS selectors (default / @CSS:)
// - XPath (/ or @XPath:)
// - JSONPath ($. or $[ or @Json:)
// - Regex (@js: or {{...}} or ##)
// - JavaScript (<js> or @js:)
package analyzer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// ErrNoListValues marks a list rule that matched no values; callers may treat
// it as a normal terminal condition when a list is optional.
var ErrNoListValues = errors.New("analyzer: no list values matched")

// splitTopLevel splits a rule string on a separator (||, &&, %%) at top level,
// respecting <js>...</js> blocks and {{...}} expressions.
func splitTopLevel(s, sep string) []string {
	var result []string
	depth := 0
	doubleBrace := false
	inJS := false
	start := 0
	for i := 0; i < len(s); i++ {
		switch {
		case i+3 < len(s) && s[i:i+4] == "<js>":
			inJS = true
			i += 3
		case i+4 < len(s) && s[i:i+5] == "</js>":
			inJS = false
			i += 4
		case s[i] == '{' && !inJS:
			depth++
			if i+1 < len(s) && s[i+1] == '{' {
				doubleBrace = true
			}
		case s[i] == '}' && !inJS:
			if doubleBrace && i+1 < len(s) && s[i+1] == '}' {
				doubleBrace = false
				i++
			}
			depth--
		case depth == 0 && !inJS && !doubleBrace && i+len(sep) <= len(s) && s[i:i+len(sep)] == sep:
			result = append(result, strings.TrimSpace(s[start:i]))
			start = i + len(sep)
			i += len(sep) - 1
		}
	}
	result = append(result, strings.TrimSpace(s[start:]))
	return result
}

// Mode indicates which parser to use for a rule.
type Mode int

const (
	ModeCSS     Mode = iota // @CSS: prefix
	ModeXPath               // / or @XPath:
	ModeJSON                // $. or $[ or @Json:
	ModeRegex               // ## patterns
	ModeJS                  // <js> or @js:
	ModeDefault             // legado Default: class.odd.0@tag.a.0@text
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
	Template   string
	PutRules   map[string]string
	HasGet     bool
	Literal    bool
	// ReplaceRegex: post-extraction substitution via ##pattern##replacement### suffix
	ReplaceRegex string
	Replacement  string
	ReplaceFirst bool
}

// Analyzer evaluates rules against content (HTML or JSON string).
type Analyzer struct {
	content     interface{}
	baseURL     string
	isJSON      bool
	jsVM        *JSVM
	cache       *CacheManager
	jsLib       string                 // prepended to every JS eval (from source's jsLib field)
	book        map[string]interface{} // legado's `book` object: name, author, bookUrl, etc.
	chapter     map[string]interface{} // legado's `chapter` object: url, title, index, etc.
	nextChapter map[string]interface{} // the exact next TOC chapter, when known
	ruleVars    map[string]string
	sourceState SourceState
	ctx         context.Context
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
		ctx:     context.Background(),
	}
}

// SetJSLib sets the JS library code to prepend to every JS evaluation.
// legado evaluates jsLib once before source-specific JS; we prepend it per-eval.
func (a *Analyzer) SetJSLib(jsLib string) { a.jsLib = jsLib }

// SetBookData sets the `book` JS context object using string-valued fields.
func (a *Analyzer) SetBookData(b map[string]string) {
	values := make(map[string]interface{}, len(b))
	for key, value := range b {
		values[key] = value
	}
	a.SetBookDataValues(values)
}

// SetBookDataValues binds complete typed book fields for JavaScript rules.
func (a *Analyzer) SetBookDataValues(b map[string]interface{}) { a.book = b }

// SetChapterData sets the `chapter` JS context object using string-valued fields.
func (a *Analyzer) SetChapterData(c map[string]string) {
	values := make(map[string]interface{}, len(c))
	for key, value := range c {
		values[key] = value
	}
	a.SetChapterDataValues(values)
}

// SetChapterDataValues binds complete typed current-chapter fields.
func (a *Analyzer) SetChapterDataValues(c map[string]interface{}) { a.chapter = c }

// SetNextChapterDataValues binds the exact next chapter for content rules.
func (a *Analyzer) SetNextChapterDataValues(c map[string]interface{}) { a.nextChapter = c }

// SetSourceState binds the source session used by cookie, source, and cache JS objects.
func (a *Analyzer) SetSourceState(state SourceState) { a.sourceState = state }

// SetContext binds cancellation to JavaScript HTTP helpers used by this analyzer.
func (a *Analyzer) SetContext(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}
	a.ctx = ctx
}

// SetContent replaces the active content for JavaScript re-analysis.
func (a *Analyzer) SetContent(content interface{}) {
	a.content = content
	switch value := content.(type) {
	case map[string]interface{}:
		_, isHTML := value["__html"]
		a.isJSON = !isHTML
	case map[string]string, []interface{}, []string:
		a.isJSON = true
	default:
		a.isJSON = looksLikeJSON(ToString(content))
	}
}

// GetString evaluates a rule string and returns the first text result.
// Supports && (concatenate) and || (OR-first) between entire rule expressions.
func (a *Analyzer) GetString(ruleStr string) (string, error) {
	// Handle && concatenation at the top level
	if andParts := splitTopLevel(ruleStr, "&&"); len(andParts) > 1 {
		var parts []string
		for _, part := range andParts {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			// Each &&-separated part may contain || chains
			v, err := a.getFirstOr(part)
			if err == nil && v != "" {
				parts = append(parts, v)
			}
		}
		return strings.Join(parts, " "), nil
	}
	return a.getFirstOr(ruleStr)
}

// getFirstOr evaluates a rule string handling || (try first non-empty).
func (a *Analyzer) getFirstOr(ruleStr string) (string, error) {
	segments := splitTopLevel(ruleStr, "||")
	for _, seg := range segments {
		seg = strings.TrimSpace(seg)
		if seg == "" {
			continue
		}
		rules, err := ParseRules(seg, a.isJSON)
		if err != nil {
			continue
		}
		result, err := a.evalString(rules)
		if err == nil && result != "" {
			return result, nil
		}
	}
	// If all || options failed, try the last one even if empty
	if len(segments) > 0 {
		seg := strings.TrimSpace(segments[len(segments)-1])
		rules, err := ParseRules(seg, a.isJSON)
		if err == nil {
			return a.evalString(rules)
		}
	}
	return "", nil
}

// GetStringList evaluates a rule string and returns a list of text results.
func (a *Analyzer) GetStringList(ruleStr string) ([]string, error) {
	if parts := splitTopLevel(ruleStr, "&&"); len(parts) > 1 {
		var combined []string
		var firstErr error
		for _, part := range parts {
			values, err := a.getStringListOR(part)
			if err != nil {
				if !errors.Is(err, ErrNoListValues) && firstErr == nil {
					firstErr = err
				}
				continue
			}
			combined = append(combined, values...)
		}
		if len(combined) == 0 {
			if firstErr != nil {
				return nil, firstErr
			}
			return nil, ErrNoListValues
		}
		return combined, nil
	}
	if parts := splitTopLevel(ruleStr, "%%"); len(parts) > 1 {
		lists := make([][]string, 0, len(parts))
		maxLen := 0
		var firstErr error
		for _, part := range parts {
			values, err := a.getStringListOR(part)
			if err != nil {
				if !errors.Is(err, ErrNoListValues) && firstErr == nil {
					firstErr = err
				}
				continue
			}
			lists = append(lists, values)
			if len(values) > maxLen {
				maxLen = len(values)
			}
		}
		var interleaved []string
		for index := 0; index < maxLen; index++ {
			for _, values := range lists {
				if index < len(values) {
					interleaved = append(interleaved, values[index])
				}
			}
		}
		if len(interleaved) == 0 {
			if firstErr != nil {
				return nil, firstErr
			}
			return nil, ErrNoListValues
		}
		return interleaved, nil
	}
	return a.getStringListOR(ruleStr)
}

func (a *Analyzer) getStringListOR(ruleStr string) ([]string, error) {
	var firstErr error
	for _, segment := range splitTopLevel(ruleStr, "||") {
		rules, err := ParseRules(strings.TrimSpace(segment), a.isJSON)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		values, err := a.evalStringList(rules)
		if err == nil && len(values) > 0 {
			return values, nil
		}
		if err != nil && !errors.Is(err, ErrNoListValues) && firstErr == nil {
			firstErr = err
		}
	}
	if firstErr != nil {
		return nil, firstErr
	}
	return nil, ErrNoListValues
}

// GetElement evaluates a rule and returns a single element.
func (a *Analyzer) GetElement(ruleStr string) (interface{}, error) {
	if strings.HasPrefix(strings.TrimSpace(ruleStr), ":") {
		rules, err := ParseRules(strings.TrimSpace(ruleStr)[1:], false)
		if err != nil {
			return nil, err
		}
		for index := range rules {
			if rules[index].Mode != ModeJS {
				rules[index].Mode = ModeRegex
				rules[index].Literal = false
			}
		}
		return a.evalElement(rules)
	}
	if len(splitTopLevel(ruleStr, "&&")) > 1 || len(splitTopLevel(ruleStr, "||")) > 1 {
		values, err := a.GetElements(ruleStr)
		if err != nil {
			return nil, err
		}
		return collapseElementValues(values), nil
	}
	rules, err := ParseRules(ruleStr, a.isJSON)
	if err != nil {
		return nil, err
	}
	return a.evalElement(rules)
}

// GetElements evaluates a rule and returns a list of elements.
// || means OR (try each, return first with results).
// && means merge (return all results from all branches).
func (a *Analyzer) GetElements(ruleStr string) ([]interface{}, error) {
	// Handle && merge at top level
	if andParts := splitTopLevel(ruleStr, "&&"); len(andParts) > 1 {
		var all []interface{}
		for _, part := range andParts {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			result, err := a.getElementsOR(part)
			if err == nil && len(result) > 0 {
				all = append(all, result...)
			}
		}
		if len(all) > 0 {
			return all, nil
		}
		return nil, fmt.Errorf("analyzer: no elements matched")
	}
	return a.getElementsOR(ruleStr)
}

// getElementsOR handles || (OR) semantics for element extraction.
func (a *Analyzer) getElementsOR(ruleStr string) ([]interface{}, error) {
	segments := splitTopLevel(ruleStr, "||")
	for _, seg := range segments {
		seg = strings.TrimSpace(seg)
		if seg == "" {
			continue
		}
		rules, err := ParseRules(seg, a.isJSON)
		if err != nil {
			continue
		}
		result, err := a.evalElements(rules)
		if err == nil && len(result) > 0 {
			return result, nil
		}
	}
	return nil, fmt.Errorf("analyzer: no elements matched")
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
	return ToString(current), nil
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
	return []string{ToString(current)}, nil
}

func (a *Analyzer) evalElement(rules []Rule) (interface{}, error) {
	current := a.content
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
	current := a.content
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
func (a *Analyzer) applyRuleString(content interface{}, rule Rule) (string, error) {
	rule, err := a.prepareRuleForEvaluation(rule)
	if err != nil {
		return "", err
	}
	var result interface{}
	switch {
	case rule.Literal:
		result = rule.Expression
	case strings.TrimSpace(rule.Expression) == "":
		result = content
	case rule.Mode == ModeJS:
		result, err = a.jsEval(stripModePrefix(rule.Mode, rule.Expression), content)
	default:
		result, err = a.dispatch(rule.Mode, ToString(content), rule.Expression)
	}
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
func (a *Analyzer) applyRuleStringList(content interface{}, rule Rule) ([]string, error) {
	rule, err := a.prepareRuleForEvaluation(rule)
	if err != nil {
		return nil, err
	}
	var result []string
	switch {
	case rule.Literal:
		result = []string{rule.Expression}
	case strings.TrimSpace(rule.Expression) == "":
		result = []string{ToString(content)}
	case rule.Mode == ModeJS:
		result, err = a.jsEvalList(stripModePrefix(rule.Mode, rule.Expression), content)
	default:
		result, err = a.dispatchList(rule.Mode, ToString(content), rule.Expression)
	}
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
	rule, err := a.prepareRuleForEvaluation(rule)
	if err != nil {
		return nil, err
	}
	var result interface{}
	switch {
	case rule.Literal:
		result = rule.Expression
	case strings.TrimSpace(rule.Expression) == "":
		result = content
	default:
		result, err = a.dispatchElement(rule.Mode, content, rule.Expression)
	}
	if err != nil || rule.ReplaceRegex == "" {
		return result, err
	}
	return applyReplace(ToString(result), rule.ReplaceRegex, rule.Replacement, rule.ReplaceFirst)
}

func (a *Analyzer) applyRuleElements(content interface{}, rule Rule) ([]interface{}, error) {
	rule, err := a.prepareRuleForEvaluation(rule)
	if err != nil {
		return nil, err
	}
	if rule.Literal {
		return []interface{}{rule.Expression}, nil
	}
	if strings.TrimSpace(rule.Expression) == "" {
		return []interface{}{content}, nil
	}
	return a.dispatchElements(rule.Mode, content, rule.Expression)
}

// dispatchElement preserves structured JSON/JS values and complete HTML selections.
func (a *Analyzer) dispatchElement(mode Mode, content interface{}, expr string) (interface{}, error) {
	expr = stripModePrefix(mode, expr)
	text := ToString(content)
	switch mode {
	case ModeJSON:
		return jsonQueryElement(text, expr)
	case ModeJS:
		return a.jsEval(expr, content)
	case ModeCSS, ModeDefault, ModeXPath:
		values, err := a.dispatchElements(mode, text, expr)
		if err != nil {
			return nil, err
		}
		return serializeHTMLSelection(values), nil
	case ModeRegex:
		return regexQueryElement(text, expr)
	default:
		return content, nil
	}
}

func collapseElementValues(values []interface{}) interface{} {
	if len(values) == 0 {
		return ""
	}
	allHTML := true
	for _, value := range values {
		if !strings.HasPrefix(strings.TrimSpace(ToString(value)), "<") {
			allHTML = false
			break
		}
	}
	if allHTML {
		return serializeHTMLSelection(values)
	}
	if len(values) == 1 {
		return values[0]
	}
	return values
}

func serializeHTMLSelection(values []interface{}) string {
	var joined strings.Builder
	for _, value := range values {
		joined.WriteString(ToString(value))
	}
	content := joined.String()
	lower := strings.ToLower(strings.TrimSpace(content))
	switch {
	case strings.HasPrefix(lower, "<tr"):
		return "<table><tbody>" + content + "</tbody></table>"
	case strings.HasPrefix(lower, "<td"), strings.HasPrefix(lower, "<th"):
		return "<table><tbody><tr>" + content + "</tr></tbody></table>"
	case strings.HasPrefix(lower, "<thead"), strings.HasPrefix(lower, "<tbody"), strings.HasPrefix(lower, "<tfoot"), strings.HasPrefix(lower, "<caption"), strings.HasPrefix(lower, "<colgroup"):
		return "<table>" + content + "</table>"
	case strings.HasPrefix(lower, "<option"), strings.HasPrefix(lower, "<optgroup"):
		return "<select>" + content + "</select>"
	default:
		return content
	}
}

// dispatch routes to the correct parser mode and returns a single result.
func (a *Analyzer) dispatch(mode Mode, content, expr string) (interface{}, error) {
	expr = stripModePrefix(mode, expr)
	switch mode {
	case ModeCSS:
		return cssQuery(content, expr)
	case ModeDefault:
		return defaultQuery(content, expr)
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
	expr = stripModePrefix(mode, expr)
	switch mode {
	case ModeCSS:
		return cssQueryList(content, expr)
	case ModeDefault:
		return defaultQueryList(content, expr)
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
func (a *Analyzer) dispatchElements(mode Mode, content interface{}, expr string) ([]interface{}, error) {
	expr = stripModePrefix(mode, expr)
	text := ToString(content)
	switch mode {
	case ModeCSS:
		return cssQueryElements(text, expr)
	case ModeDefault:
		return defaultQueryElements(text, expr)
	case ModeXPath:
		return xpathQueryElements(text, expr)
	case ModeJSON:
		return jsonQueryElements(text, expr)
	case ModeRegex:
		return regexQueryElements(text, expr)
	case ModeJS:
		return a.jsEvalElements(expr, content)
	default:
		return nil, fmt.Errorf("analyzer: unsupported mode for elements: %v", mode)
	}
}

// prependJSLib prepends the source's JSLib code before the script.
// ponytail: simple concat, no caching — JSLib is tiny for most sources.
func (a *Analyzer) prependJSLib(script string) string {
	if a.jsLib == "" {
		return script
	}
	return a.jsLib + "\n" + script
}

// jsBindings returns extra bindings for JS eval (book, chapter, etc.)
func (a *Analyzer) jsBindings() map[string]interface{} {
	b := make(map[string]interface{})
	if a.book != nil {
		b["book"] = a.book
	}
	if a.chapter != nil {
		b["chapter"] = a.chapter
	}
	b["nextChapter"] = nil
	b["nextChapterUrl"] = nil
	if a.nextChapter != nil {
		b["nextChapter"] = a.nextChapter
		b["nextChapterUrl"] = a.nextChapter["url"]
	}
	if a.sourceState != nil {
		b["sourceState"] = a.sourceState
	}
	b["analyzer"] = a
	return b
}

func (a *Analyzer) jsEval(expr string, content interface{}) (interface{}, error) {
	if a.jsVM == nil {
		return "", fmt.Errorf("analyzer: JS engine not available")
	}
	return a.jsVM.EvalContext(a.ctx, a.prependJSLib(expr), content, a.baseURL, a.jsBindings())
}

func (a *Analyzer) jsEvalList(expr string, content interface{}) ([]string, error) {
	if a.jsVM == nil {
		return nil, fmt.Errorf("analyzer: JS engine not available")
	}
	return a.jsVM.EvalListContext(a.ctx, a.prependJSLib(expr), content, a.baseURL, a.jsBindings())
}

func (a *Analyzer) jsEvalElements(expr string, content interface{}) ([]interface{}, error) {
	if a.jsVM == nil {
		return nil, fmt.Errorf("analyzer: JS engine not available")
	}
	return a.jsVM.EvalElementsContext(a.ctx, a.prependJSLib(expr), content, a.baseURL, a.jsBindings())
}

func stripModePrefix(mode Mode, expr string) string {
	expr = strings.TrimSpace(expr)
	if mode == ModeJS && strings.HasPrefix(strings.ToLower(expr), "<js>") {
		expr = strings.TrimSpace(expr[len("<js>"):])
		if strings.HasSuffix(strings.ToLower(expr), "</js>") {
			expr = strings.TrimSpace(expr[:len(expr)-len("</js>")])
		}
		return expr
	}
	upper := strings.ToUpper(expr)
	prefix := ""
	switch mode {
	case ModeCSS:
		prefix = "@CSS:"
	case ModeXPath:
		prefix = "@XPATH:"
	case ModeJSON:
		prefix = "@JSON:"
	case ModeJS:
		prefix = "@JS:"
	}
	if strings.HasPrefix(upper, prefix) {
		return strings.TrimSpace(expr[len(prefix):])
	}
	return expr
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
	case map[string]interface{}:
		if html, ok := s["__html"].(string); ok {
			return html
		}
		encoded, err := json.Marshal(s)
		if err == nil {
			return string(encoded)
		}
		return fmt.Sprintf("%v", s)
	case map[string]string, []interface{}, []string:
		encoded, err := json.Marshal(s)
		if err == nil {
			return string(encoded)
		}
		return fmt.Sprintf("%v", s)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// looksLikeJSON checks if the content looks like JSON.
func looksLikeJSON(s string) bool {
	s = strings.TrimSpace(s)
	return strings.HasPrefix(s, "{") || strings.HasPrefix(s, "[")
}
