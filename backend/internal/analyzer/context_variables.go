package analyzer

import "fmt"

type evaluationVariables struct {
	book           map[string]interface{}
	chapter        map[string]interface{}
	fallbackGet    func(string) string
	fallbackPut    func(string, string)
	bookCleanup    func()
	chapterCleanup func()
}

func newEvaluationVariables(book, chapter map[string]interface{}, analyzer *Analyzer, state SourceState) *evaluationVariables {
	variables := &evaluationVariables{book: book, chapter: chapter}
	if analyzer != nil && state != nil {
		variables.fallbackGet = func(key string) string {
			if value, ok := state.GetMemory(key).(string); ok {
				return value
			}
			return ""
		}
		variables.fallbackPut = func(key, value string) { state.PutMemory(key, value) }
	}
	variables.bookCleanup = bindVariableMethods(book)
	variables.chapterCleanup = bindVariableMethods(chapter)
	return variables
}

func (v *evaluationVariables) Close() {
	if v == nil {
		return
	}
	v.chapterCleanup()
	v.bookCleanup()
}

func (v *evaluationVariables) Get(key string) string {
	if v == nil {
		return ""
	}
	if key == "bookName" {
		return contextString(v.book, "name")
	}
	if key == "title" {
		return contextString(v.chapter, "title")
	}
	if value, ok := getContextVariable(v.chapter, key); ok {
		return value
	}
	if value, ok := getContextVariable(v.book, key); ok {
		return value
	}
	if v.fallbackGet != nil {
		return v.fallbackGet(key)
	}
	return ""
}

func (v *evaluationVariables) Put(key, value string) string {
	if v == nil {
		return value
	}
	if v.chapter != nil {
		putContextVariable(v.chapter, key, value)
	} else if v.book != nil {
		putContextVariable(v.book, key, value)
	} else if v.fallbackPut != nil {
		v.fallbackPut(key, value)
	}
	return value
}

func bindVariableMethods(values map[string]interface{}) func() {
	if values == nil {
		return func() {}
	}
	previousGet, hadGet := values["getVariable"]
	previousPut, hadPut := values["putVariable"]
	values["getVariable"] = func(key string) string {
		value, _ := getContextVariable(values, key)
		return value
	}
	values["putVariable"] = func(key string, value interface{}) bool {
		putContextVariable(values, key, fmt.Sprint(value))
		return true
	}
	return func() {
		if hadGet {
			values["getVariable"] = previousGet
		} else {
			delete(values, "getVariable")
		}
		if hadPut {
			values["putVariable"] = previousPut
		} else {
			delete(values, "putVariable")
		}
	}
}

func getContextVariable(values map[string]interface{}, key string) (string, bool) {
	if values == nil {
		return "", false
	}
	switch variables := values["variableMap"].(type) {
	case map[string]string:
		value, ok := variables[key]
		return value, ok
	case map[string]interface{}:
		value, ok := variables[key]
		if !ok {
			return "", false
		}
		return fmt.Sprint(value), true
	default:
		return "", false
	}
}

func putContextVariable(values map[string]interface{}, key, value string) {
	if values == nil {
		return
	}
	switch variables := values["variableMap"].(type) {
	case map[string]string:
		variables[key] = value
	case map[string]interface{}:
		variables[key] = value
	default:
		values["variableMap"] = map[string]string{key: value}
	}
}

func contextString(values map[string]interface{}, key string) string {
	if values == nil {
		return ""
	}
	value, _ := values[key].(string)
	return value
}
