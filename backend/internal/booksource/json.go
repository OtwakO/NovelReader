package booksource

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// rawBookSource mirrors BookSource but uses RawMessage for rule fields and IntOrString for numeric fields.
type rawBookSource struct {
	BookSourceURL     string          `json:"bookSourceUrl"`
	BookSourceName    string          `json:"bookSourceName"`
	BookSourceGroup   string          `json:"bookSourceGroup,omitempty"`
	BookSourceType    int             `json:"bookSourceType"`
	BookURLPattern    string          `json:"bookUrlPattern,omitempty"`
	CustomOrder       int             `json:"customOrder,omitempty"`
	Enabled           bool            `json:"enabled"`
	EnabledExplore    *bool           `json:"enabledExplore"`
	EnabledCookieJar  *bool           `json:"enabledCookieJar,omitempty"`
	SearchURL         string          `json:"searchUrl,omitempty"`
	ExploreURL        string          `json:"exploreUrl,omitempty"`
	ExploreScreen     string          `json:"exploreScreen,omitempty"`
	RuleSearch        json.RawMessage `json:"ruleSearch,omitempty"`
	RuleBookInfo      json.RawMessage `json:"ruleBookInfo,omitempty"`
	RuleToc           json.RawMessage `json:"ruleToc,omitempty"`
	RuleContent       json.RawMessage `json:"ruleContent,omitempty"`
	RuleExplore       json.RawMessage `json:"ruleExplore,omitempty"`
	RuleReview        json.RawMessage `json:"ruleReview,omitempty"`
	JSLib             string          `json:"jsLib,omitempty"`
	Header            string          `json:"header,omitempty"`
	LoginURL          string          `json:"loginUrl,omitempty"`
	LoginUI           string          `json:"loginUi,omitempty"`
	LoginCheckJS      string          `json:"loginCheckJs,omitempty"`
	CoverDecodeJS     string          `json:"coverDecodeJs,omitempty"`
	ConcurrentRate    string          `json:"concurrentRate,omitempty"`
	BookSourceComment string          `json:"bookSourceComment,omitempty"`
	VariableComment   string          `json:"variableComment,omitempty"`
	LastUpdateTime    json.RawMessage `json:"lastUpdateTime,omitempty"`
	RespondTime       json.RawMessage `json:"respondTime,omitempty"`
	Weight            json.RawMessage `json:"weight,omitempty"`
}

func (s *BookSource) UnmarshalJSON(data []byte) error {
	*s = BookSource{sourceJSON: string(data)}
	var raw rawBookSource
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	s.BookSourceURL = raw.BookSourceURL
	s.BookSourceName = raw.BookSourceName
	s.BookSourceGroup = raw.BookSourceGroup
	s.BookSourceType = raw.BookSourceType
	s.BookURLPattern = raw.BookURLPattern
	s.CustomOrder = raw.CustomOrder
	s.Enabled = raw.Enabled
	s.EnabledExplore = true
	if raw.EnabledExplore != nil {
		s.EnabledExplore = *raw.EnabledExplore
	}
	s.EnabledCookieJar = raw.EnabledCookieJar
	s.SearchURL = raw.SearchURL
	s.ExploreURL = raw.ExploreURL
	s.ExploreScreen = raw.ExploreScreen
	s.JSLib = raw.JSLib
	s.Header = raw.Header
	s.LoginURL = raw.LoginURL
	s.LoginUI = raw.LoginUI
	s.LoginCheckJS = raw.LoginCheckJS
	s.CoverDecodeJS = raw.CoverDecodeJS
	s.ConcurrentRate = raw.ConcurrentRate
	s.BookSourceComment = raw.BookSourceComment
	s.VariableComment = raw.VariableComment

	// Convert raw rule messages to JSON strings
	if raw.RuleSearch != nil {
		s.RuleSearch = string(raw.RuleSearch)
	}
	if raw.RuleBookInfo != nil {
		s.RuleBookInfo = string(raw.RuleBookInfo)
	}
	if raw.RuleToc != nil {
		s.RuleToc = string(raw.RuleToc)
	}
	if raw.RuleContent != nil {
		s.RuleContent = string(raw.RuleContent)
	}
	if raw.RuleExplore != nil {
		s.RuleExplore = string(raw.RuleExplore)
	}
	if raw.RuleReview != nil {
		s.RuleReview = string(raw.RuleReview)
	}

	// Parse numeric fields that may be strings or numbers in JSON
	s.LastUpdateTime = parseIntOrZero(raw.LastUpdateTime)
	s.RespondTime = parseIntOrZero(raw.RespondTime)
	if s.RespondTime == 0 {
		s.RespondTime = 180000
	}
	s.Weight = int(parseIntOrZero(raw.Weight))
	return nil
}

// MarshalJSON restores the imported representation when fields are unchanged, and
// merges preserved fields when application code updates a typed value.
func (s BookSource) MarshalJSON() ([]byte, error) {
	current, err := json.Marshal(bookSourceWire(s))
	if err != nil {
		return nil, err
	}
	if s.sourceJSON == "" {
		return current, nil
	}

	var original map[string]json.RawMessage
	if err := json.Unmarshal([]byte(s.sourceJSON), &original); err != nil {
		return nil, fmt.Errorf("booksource: restore source JSON: %w", err)
	}
	var baseline BookSource
	if err := json.Unmarshal([]byte(s.sourceJSON), &baseline); err != nil {
		return nil, fmt.Errorf("booksource: compare source JSON: %w", err)
	}
	baselineBytes, err := json.Marshal(bookSourceWire(baseline))
	if err != nil {
		return nil, err
	}
	if bytes.Equal(current, baselineBytes) {
		return []byte(s.sourceJSON), nil
	}

	var currentFields, baselineFields map[string]json.RawMessage
	if err := json.Unmarshal(current, &currentFields); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(baselineBytes, &baselineFields); err != nil {
		return nil, err
	}
	changedRules := make(map[string]bool, len(ruleJSONFields))
	for key := range ruleJSONFields {
		changedRules[key] = !bytes.Equal(currentFields[key], baselineFields[key])
	}
	for key, value := range original {
		if _, known := bookSourceJSONFields[key]; !known {
			currentFields[key] = value
			continue
		}
		if bytes.Equal(currentFields[key], baselineFields[key]) {
			currentFields[key] = value
		}
	}
	for key, changed := range changedRules {
		if changed {
			normalizeRuleField(currentFields, key)
		}
	}
	return json.Marshal(currentFields)
}

type bookSourceWire BookSource

var ruleJSONFields = map[string]struct{}{
	"ruleSearch": {}, "ruleBookInfo": {}, "ruleToc": {}, "ruleContent": {}, "ruleExplore": {}, "ruleReview": {},
}

var bookSourceJSONFields = map[string]struct{}{
	"bookSourceUrl": {}, "bookSourceName": {}, "bookSourceGroup": {}, "bookSourceType": {},
	"bookUrlPattern": {}, "customOrder": {}, "enabled": {}, "enabledExplore": {}, "enabledCookieJar": {},
	"searchUrl": {}, "exploreUrl": {}, "exploreScreen": {}, "ruleSearch": {}, "ruleBookInfo": {},
	"ruleToc": {}, "ruleContent": {}, "ruleExplore": {}, "ruleReview": {}, "jsLib": {}, "header": {},
	"loginUrl": {}, "loginUi": {}, "loginCheckJs": {}, "coverDecodeJs": {}, "concurrentRate": {},
	"bookSourceComment": {}, "variableComment": {}, "lastUpdateTime": {}, "respondTime": {}, "weight": {},
	"createdAt": {}, "updatedAt": {},
}

func normalizeRuleField(fields map[string]json.RawMessage, key string) {
	var value string
	if err := json.Unmarshal(fields[key], &value); err == nil && json.Valid([]byte(value)) {
		fields[key] = json.RawMessage(value)
	}
}

// parseIntOrZero parses a json.RawMessage as int64, accepting both number and string formats.
func parseIntOrZero(raw json.RawMessage) int64 {
	if len(raw) == 0 {
		return 0
	}
	// Try direct number first
	var n int64
	if err := json.Unmarshal(raw, &n); err == nil {
		return n
	}
	// Try string
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		fmt.Sscanf(s, "%d", &n)
	}
	return n
}

// ImportSources parses a JSON array (or single object) of book sources.
func ImportSources(data []byte) ([]*BookSource, error) {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return nil, fmt.Errorf("booksource: empty import data")
	}

	var sources []*BookSource
	// Try array first, then single object.
	if err := json.Unmarshal([]byte(trimmed), &sources); err == nil {
		return sources, nil
	}

	// Single source.
	var single BookSource
	if err := json.Unmarshal([]byte(trimmed), &single); err != nil {
		return nil, fmt.Errorf("booksource: import: %w", err)
	}
	return []*BookSource{&single}, nil
}
