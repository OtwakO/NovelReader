// Conformance tests for JavaScript-to-rule-engine helper methods.
package analyzer

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/otwako/novelreader/internal/fetcher"
)

func TestJavaEncodeURIUsesOptionalCharset(t *testing.T) {
	vm := NewJSVM()

	value, err := vm.Eval(`java.encodeURI('凡人修仙传')`, "", "https://example.test/")
	if err != nil || ToString(value) != "%E5%87%A1%E4%BA%BA%E4%BF%AE%E4%BB%99%E4%BC%A0" {
		t.Fatalf("UTF-8 encodeURI=%q err=%v", ToString(value), err)
	}
	value, err = vm.Eval(`java.encodeURI('凡人修仙传', 'gb2312')`, "", "https://example.test/")
	if err != nil || ToString(value) != "%B7%B2%C8%CB%D0%DE%CF%C9%B4%AB" {
		t.Fatalf("GB2312 encodeURI=%q err=%v", ToString(value), err)
	}
	if _, err := vm.Eval(`java.encodeURI('凡人修仙传', 'unsupported-charset')`, "", "https://example.test/"); err == nil {
		t.Fatal("unsupported encodeURI charset did not return an evaluation error")
	}
}

func TestBuildURLUsesJavaEncodeURICharset(t *testing.T) {
	meta, err := BuildURL(
		`https://example.test/search?q={{java.encodeURI(key, 'gb2312')}}`,
		"凡人修仙传", 1, "", NewJSVM(),
	)
	if err != nil {
		t.Fatal(err)
	}
	if meta.URL != "https://example.test/search?q=%B7%B2%C8%CB%D0%DE%CF%C9%B4%AB" {
		t.Fatalf("URL=%q", meta.URL)
	}
}

func TestJavaTimeFormatUsesLegadoLocalDateFormat(t *testing.T) {
	previousLocal := time.Local
	time.Local = time.FixedZone("UTC+8", 8*60*60)
	t.Cleanup(func() { time.Local = previousLocal })

	value, err := NewJSVM().Eval(`java.timeFormat(1788011130000)`, "", "https://example.test/")
	if err != nil || ToString(value) != "2026/08/29 21:45" {
		t.Fatalf("timeFormat=%q err=%v", ToString(value), err)
	}
}

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

func TestJavaHexDecodeToString(t *testing.T) {
	vm := NewJSVM()

	value, err := vm.Eval(`java.hexDecodeToString('7b226b6579223a22e58991e69da5227d')`, "", "https://example.test/")
	if err != nil || ToString(value) != `{"key":"剑来"}` {
		t.Fatalf("hex decode=%q err=%v", ToString(value), err)
	}
	if _, err := vm.Eval(`java.hexDecodeToString('not-hex')`, "", "https://example.test/"); err == nil {
		t.Fatal("invalid hex did not return an evaluation error")
	}
}

func TestJavaSymmetricCryptoAndByteArrayBridge(t *testing.T) {
	vm := NewJSVM()
	script := `
var crypto=java.createSymmetricCrypto(
  'AES/CBC/PKCS5Padding',
  java.base64DecodeToByteArray('L6alxSR4ttjXvcGpZozYtdcJtG4l0tSnQplRUONIRsw='),
  java.base64DecodeToByteArray('AAAAAAAAAAAAAAAAAAAAAA==')
);
var encrypted=crypto.encryptBase64('Legado bridge');
crypto.decryptStr(encrypted);
`
	value, err := vm.Eval(script, "", "https://example.test/")
	if err != nil || ToString(value) != "Legado bridge" {
		t.Fatalf("crypto round trip=%q err=%v", ToString(value), err)
	}
	nullValue, err := vm.Eval(`java.base64DecodeToByteArray('')`, "", "https://example.test/")
	if err != nil || nullValue != nil {
		t.Fatalf("empty decode=%#v err=%v, want null", nullValue, err)
	}
}

func TestJavaAjaxExecutesLegadoRequestOptions(t *testing.T) {
	type request struct {
		method, path, body, header, contentType string
	}
	requests := make(chan request, 4)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		encoded, _ := io.ReadAll(r.Body)
		if json.Valid(encoded) {
			var body map[string]interface{}
			_ = json.Unmarshal(encoded, &body)
			encoded, _ = json.Marshal(body)
		}
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

	value, err = vm.Eval(`java.ajax(`+fmt.Sprintf("%q", server.URL+`/head,{"method":"HEAD","retry":1}`)+`)`, "", server.URL)
	if err != nil || ToString(value) != "" {
		t.Fatalf("HEAD ajax value=%q err=%v", ToString(value), err)
	}
	if got := <-requests; got.method != http.MethodHead || got.path != "/head" {
		t.Fatalf("HEAD request=%+v", got)
	}

	value, err = vm.Eval(`java.ajax(`+fmt.Sprintf("%q", server.URL+`/form,{"method":"POST","charset":"gbk","body":"q=中文"}`)+`)`, "", server.URL)
	if err != nil || ToString(value) != `{"ok":true}` {
		t.Fatalf("charset ajax value=%q err=%v", ToString(value), err)
	}
	if got := <-requests; got.body != "q=%D6%D0%CE%C4" || got.contentType != "application/x-www-form-urlencoded" {
		t.Fatalf("charset request=%+v", got)
	}
}

func TestJSoupSelectionForInEnumeratesOnlyElements(t *testing.T) {
	vm := NewJSVM()
	script := `var links=org.jsoup.Jsoup.parse(result).select('.list a'); var names=[]; for (var i in links) names.push(links[i].text()); names.join('|')`
	value, err := vm.Eval(script, `<div class="list"><a>One</a><a>Two</a></div>`, "https://example.test/")
	if err != nil || ToString(value) != "One|Two" {
		t.Fatalf("JSoup for-in value=%q err=%v", ToString(value), err)
	}
}

func TestJavaHelpersReevaluateCurrentAnalyzer(t *testing.T) {
	analyzer := New(`<div class="book"><a href="/first">First</a></div><div class="book"><a href="/second">Second</a></div>`, "https://example.test/", NewJSVM(), nil)

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

	value, err = analyzer.jsEval(`java.getElement('@@tag.a.0').text() + '|' + java.getElement('@@tag.a.1').attr('href')`, "")
	if err != nil || ToString(value) != "First|/second" {
		t.Fatalf("java.getElement = %q, err=%v", ToString(value), err)
	}
	if _, err := analyzer.jsEval(`java.getElement('$[')`, ""); err == nil {
		t.Fatal("java.getElement swallowed an invalid rule")
	}
	analyzer.SetContent(`<table><tbody><tr id="row"><td data-cell="name">Cell</td></tr></tbody></table>`)
	value, err = analyzer.jsEval(`java.getElement('@@tag.tr.0').attr('id') + '|' + java.getElement('@@tag.td.0').attr('data-cell')`, "")
	if err != nil || ToString(value) != "row|name" {
		t.Fatalf("table getElement = %q, err=%v", ToString(value), err)
	}

	analyzer.SetContent(map[string]interface{}{"ids": []interface{}{"1", "2", "3"}})
	value, err = analyzer.jsEval(`java.getString('$.ids[*]')`, "")
	if err != nil || ToString(value) != "1\n2\n3" {
		t.Fatalf("JSON list getString = %q, err=%v", ToString(value), err)
	}

	if _, err := analyzer.jsEval(`java.setContent('<p>updated</p>'); java.getString('@css:p@text')`, ""); err != nil {
		t.Fatal(err)
	}
	value, err = analyzer.jsEval(`java.getString('@css:p@text')`, "")
	if err != nil || ToString(value) != "updated" {
		t.Fatalf("setContent result = %q, err=%v", ToString(value), err)
	}
	value, err = analyzer.jsEval(`java.setContent('<a href="/updated">Updated</a>'); java.getElement('@@tag.a.0').attr('href')`, "")
	if err != nil || ToString(value) != "/updated" {
		t.Fatalf("getElement after setContent = %q, err=%v", ToString(value), err)
	}

	if _, err := analyzer.jsEval(`java.setContent({payload:{name:'structured'}})`, ""); err != nil {
		t.Fatal(err)
	}
	value, err = analyzer.GetString("$.payload.name")
	if err != nil || value != "structured" {
		t.Fatalf("structured setContent result = %q, err=%v", value, err)
	}
}
