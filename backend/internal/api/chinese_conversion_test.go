package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/otwako/novelreader/internal/chineseconv"
)

type conversionStub struct {
	capability chineseconv.Capability
	converted  []string
	err        error
}

func (s conversionStub) Capability() chineseconv.Capability { return s.capability }
func (s conversionStub) Convert(context.Context, string, []string) ([]string, error) {
	return s.converted, s.err
}
func (s conversionStub) Close() error { return nil }

func TestChineseConversionCapabilityAndBatch(t *testing.T) {
	service := conversionStub{
		capability: chineseconv.Capability{Available: true, Engine: "BYVoid/OpenCC", Version: "1.4.2", Presets: map[string]string{"simplified": "tw2sp_jieba.json", "traditional": "s2twp_jieba.json"}, Modes: []string{"simplified", "traditional"}},
		converted:  []string{"軟體", "滑鼠"},
	}
	server := &Server{chineseConversion: service, mux: http.NewServeMux()}
	server.registerRoutesWithoutHealth()

	capability := httptest.NewRecorder()
	server.ServeHTTP(capability, httptest.NewRequest(http.MethodGet, "/api/system/chinese-conversion", nil))
	if capability.Code != http.StatusOK || !strings.Contains(capability.Body.String(), `"traditional":"s2twp_jieba.json"`) {
		t.Fatalf("status=%d body=%s", capability.Code, capability.Body.String())
	}

	conversion := httptest.NewRecorder()
	server.ServeHTTP(conversion, httptest.NewRequest(http.MethodPost, "/api/system/chinese-conversion", strings.NewReader(`{"mode":"traditional","texts":["软件","鼠标"]}`)))
	if conversion.Code != http.StatusOK || conversion.Body.String() != `{"texts":["軟體","滑鼠"]}`+"\n" {
		t.Fatalf("status=%d body=%s", conversion.Code, conversion.Body.String())
	}
}

func TestChineseConversionUnavailableIsExplicit(t *testing.T) {
	server := &Server{chineseConversion: conversionStub{capability: chineseconv.Capability{Available: false, Modes: []string{}}, err: errors.New("unused")}, mux: http.NewServeMux()}
	server.registerRoutesWithoutHealth()
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/system/chinese-conversion", strings.NewReader(`{"mode":"traditional","texts":["软件"]}`)))
	if response.Code != http.StatusServiceUnavailable || !strings.Contains(response.Body.String(), `"code":"chinese_conversion_unavailable"`) {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestChineseConversionRejectsTrailingJSON(t *testing.T) {
	server := &Server{chineseConversion: conversionStub{capability: chineseconv.Capability{Available: true, Modes: []string{"simplified", "traditional"}}}, mux: http.NewServeMux()}
	server.registerRoutesWithoutHealth()
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/system/chinese-conversion", strings.NewReader(`{"mode":"traditional","texts":[]} {}`)))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestChineseConversionRejectsUnknownMode(t *testing.T) {
	server := &Server{chineseConversion: conversionStub{capability: chineseconv.Capability{Available: true, Modes: []string{"simplified", "traditional"}}}, mux: http.NewServeMux()}
	server.registerRoutesWithoutHealth()
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/system/chinese-conversion", strings.NewReader(`{"mode":"regional","texts":[]}`)))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}
