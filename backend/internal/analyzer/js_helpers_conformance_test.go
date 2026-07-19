// Conformance tests for JavaScript-to-rule-engine helper methods.
package analyzer

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/otwako/novelreader/internal/fetcher"
)

func TestJavaChapterAndToastHelpers(t *testing.T) {
	vm := NewJSVM()
	for input, want := range map[string]string{
		"第１２章":         "第12章",
		"第二十一章":        "第21章",
		"第一千二百三十四章":    "第1234章",
		"第壹佰零贰章":       "第102章",
		"第2147483648章": "第-1章",
		"第萬章":          "第-1章",
		"序章":           "序章",
	} {
		value, err := vm.Eval(`java.toNumChapter(`+fmt.Sprintf("%q", input)+`)`, "", "https://example.test/")
		if err != nil || ToString(value) != want {
			t.Errorf("toNumChapter(%q)=%q err=%v, want %q", input, ToString(value), err, want)
		}
	}
	nullValue, err := vm.Eval(`java.toNumChapter(null)`, "", "https://example.test/")
	if err != nil || nullValue != nil {
		t.Fatalf("toNumChapter(null)=%#v err=%v, want null", nullValue, err)
	}
	value, err := vm.Eval(`java.toast('notice'); 'continued'`, "", "https://example.test/")
	if err != nil || ToString(value) != "continued" {
		t.Fatalf("toast continuation=%q err=%v", ToString(value), err)
	}
}

func TestJavaAjaxExecutesLegadoRequestOptions(t *testing.T) {
	type request struct {
		method, path, body, header, contentType string
	}
	requests := make(chan request, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		encoded, _ := json.Marshal(body)
		requests <- request{r.Method, r.URL.Path, string(encoded), r.Header.Get("X-Source"), r.Header.Get("Content-Type")}
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	vm := NewJSVMWithPoolSize(1)
	vm.SetFetcher(fetcher.NewWithTimeout(5 * time.Second))
	optionURL := `/graphql,{"method":"POST","body":{"query":"works","variables":{"ids":["1","2"]}},"headers":{"X-Source":"raw920"}}`
	value, err := vm.Eval(`java.ajax(`+fmt.Sprintf("%q", optionURL)+`)`, "", server.URL+"/base/")
	if err != nil || ToString(value) != `{"ok":true}` {
		t.Fatalf("ajax value=%q err=%v", ToString(value), err)
	}
	select {
	case got := <-requests:
		if got.method != http.MethodPost || got.path != "/graphql" || got.header != "raw920" || got.contentType != "application/json" || got.body != `{"query":"works","variables":{"ids":["1","2"]}}` {
			t.Fatalf("request=%+v", got)
		}
	case <-time.After(time.Second):
		t.Fatal("ajax request was not received")
	}

	value, err = vm.Eval(`java.ajax([`+fmt.Sprintf("%q", server.URL+"/list")+`])`, "", server.URL+"/base/")
	if err != nil || ToString(value) != `{"ok":true}` {
		t.Fatalf("list ajax value=%q err=%v", ToString(value), err)
	}
	select {
	case got := <-requests:
		if got.method != http.MethodGet || got.path != "/list" {
			t.Fatalf("list request=%+v", got)
		}
	case <-time.After(time.Second):
		t.Fatal("list ajax request was not received")
	}
}

func TestJavaHelpersReevaluateCurrentAnalyzer(t *testing.T) {
	analyzer := New(`<div class="book">First</div><div class="book">Second</div>`, "https://example.test/", NewJSVM(), nil)

	value, err := analyzer.jsEval(`java.getString('@css:.book@text')`, "")
	if err != nil || ToString(value) != "FirstSecond" {
		t.Fatalf("java.getString = %q, err=%v", ToString(value), err)
	}

	value, err = analyzer.jsEval(`java.getElements('@css:.book')`, "")
	if err != nil {
		t.Fatal(err)
	}
	if values, ok := value.([]interface{}); !ok || len(values) != 2 {
		t.Fatalf("java.getElements = %#v, want two elements", value)
	}

	if _, err := analyzer.jsEval(`java.setContent('<p>updated</p>'); java.getString('@css:p@text')`, ""); err != nil {
		t.Fatal(err)
	}
	value, err = analyzer.jsEval(`java.getString('@css:p@text')`, "")
	if err != nil || ToString(value) != "updated" {
		t.Fatalf("setContent result = %q, err=%v", ToString(value), err)
	}

	if _, err := analyzer.jsEval(`java.setContent({payload:{name:'structured'}})`, ""); err != nil {
		t.Fatal(err)
	}
	value, err = analyzer.GetString("$.payload.name")
	if err != nil || value != "structured" {
		t.Fatalf("structured setContent result = %q, err=%v", value, err)
	}
}
