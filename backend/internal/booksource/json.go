package booksource

import (
	"encoding/json"
	"fmt"
	"strings"
)

// rawBookSource mirrors BookSource but uses RawMessage for rule fields so JSON objects are accepted.
type rawBookSource struct {
	BookSourceURL    string          `json:"bookSourceUrl"`
	BookSourceName   string          `json:"bookSourceName"`
	BookSourceGroup  string          `json:"bookSourceGroup,omitempty"`
	BookSourceType   int             `json:"bookSourceType"`
	BookURLPattern   string          `json:"bookUrlPattern,omitempty"`
	CustomOrder      int             `json:"customOrder,omitempty"`
	Enabled          bool            `json:"enabled"`
	EnabledExplore   bool            `json:"enabledExplore"`
	EnabledCookieJar *bool           `json:"enabledCookieJar,omitempty"`
	SearchURL        string          `json:"searchUrl,omitempty"`
	ExploreURL       string          `json:"exploreUrl,omitempty"`
	ExploreScreen    string          `json:"exploreScreen,omitempty"`
	RuleSearch       json.RawMessage `json:"ruleSearch,omitempty"`
	RuleBookInfo     json.RawMessage `json:"ruleBookInfo,omitempty"`
	RuleToc          json.RawMessage `json:"ruleToc,omitempty"`
	RuleContent      json.RawMessage `json:"ruleContent,omitempty"`
	RuleExplore      json.RawMessage `json:"ruleExplore,omitempty"`
	RuleReview       json.RawMessage `json:"ruleReview,omitempty"`
	JSLib            string          `json:"jsLib,omitempty"`
	Header           string          `json:"header,omitempty"`
	LoginURL         string          `json:"loginUrl,omitempty"`
	LoginUI          string          `json:"loginUi,omitempty"`
	LoginCheckJS     string          `json:"loginCheckJs,omitempty"`
	CoverDecodeJS    string          `json:"coverDecodeJs,omitempty"`
	ConcurrentRate   string          `json:"concurrentRate,omitempty"`
	BookSourceComment string         `json:"bookSourceComment,omitempty"`
	VariableComment  string          `json:"variableComment,omitempty"`
	LastUpdateTime   int64           `json:"lastUpdateTime,omitempty"`
	RespondTime      int64           `json:"respondTime,omitempty"`
	Weight           int             `json:"weight,omitempty"`
}

func (s *BookSource) UnmarshalJSON(data []byte) error {
	var raw rawBookSource
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	*s = BookSource{
		BookSourceURL:    raw.BookSourceURL,
		BookSourceName:   raw.BookSourceName,
		BookSourceGroup:  raw.BookSourceGroup,
		BookSourceType:   raw.BookSourceType,
		BookURLPattern:   raw.BookURLPattern,
		CustomOrder:      raw.CustomOrder,
		Enabled:          raw.Enabled,
		EnabledExplore:   raw.EnabledExplore,
		EnabledCookieJar: raw.EnabledCookieJar,
		SearchURL:        raw.SearchURL,
		ExploreURL:       raw.ExploreURL,
		ExploreScreen:    raw.ExploreScreen,
		JSLib:            raw.JSLib,
		Header:           raw.Header,
		LoginURL:         raw.LoginURL,
		LoginUI:          raw.LoginUI,
		LoginCheckJS:     raw.LoginCheckJS,
		CoverDecodeJS:    raw.CoverDecodeJS,
		ConcurrentRate:   raw.ConcurrentRate,
		BookSourceComment: raw.BookSourceComment,
		VariableComment:  raw.VariableComment,
		LastUpdateTime:   raw.LastUpdateTime,
		RespondTime:      raw.RespondTime,
		Weight:           raw.Weight,
	}

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

	if s.RespondTime == 0 {
		s.RespondTime = 180000
	}
	return nil
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
