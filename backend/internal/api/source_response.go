package api

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/otwako/novelreader/internal/booksource"
)

type sourceManagementResponse map[string]json.RawMessage

type sourceManagementSummary struct {
	SourceID          string   `json:"sourceId"`
	BookSourceURL     string   `json:"bookSourceUrl"`
	BookSourceName    string   `json:"bookSourceName"`
	BookSourceGroup   string   `json:"bookSourceGroup,omitempty"`
	BookSourceType    int      `json:"bookSourceType,omitempty"`
	Enabled           bool     `json:"enabled"`
	EnabledExplore    bool     `json:"enabledExplore"`
	ExploreURL        string   `json:"exploreUrl,omitempty"`
	BookSourceComment string   `json:"bookSourceComment,omitempty"`
	CollectionID      string   `json:"collectionId,omitempty"`
	Capabilities      []string `json:"capabilities"`
	Searchable        bool     `json:"searchable"`
}

func sourceManagementSummaries(sources []booksource.BookSource) []sourceManagementSummary {
	summaries := make([]sourceManagementSummary, len(sources))
	for index := range sources {
		source := sources[index]
		summaries[index] = sourceManagementSummary{
			SourceID: source.ID, BookSourceURL: source.BookSourceURL, BookSourceName: source.BookSourceName,
			BookSourceGroup: source.BookSourceGroup, BookSourceType: source.BookSourceType,
			Enabled: source.Enabled, EnabledExplore: source.EnabledExplore, ExploreURL: source.ExploreURL,
			BookSourceComment: source.BookSourceComment, CollectionID: source.CollectionID,
			Capabilities: booksource.CapabilityTags(source),
			Searchable:   source.Enabled && source.BookSourceType == 0 && strings.TrimSpace(source.SearchURL) != "" && strings.TrimSpace(source.RuleSearch) != "",
		}
	}
	return summaries
}

func sourceManagementResponseFor(source booksource.BookSource) (sourceManagementResponse, error) {
	encoded, err := source.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("encode BookSource management response: %w", err)
	}
	var response sourceManagementResponse
	if err := json.Unmarshal(encoded, &response); err != nil {
		return nil, fmt.Errorf("decode BookSource management response: %w", err)
	}
	encodedID, _ := json.Marshal(source.ID)
	response["sourceId"] = encodedID
	if source.CollectionID != "" {
		encodedID, _ := json.Marshal(source.CollectionID)
		response["collectionId"] = encodedID
	}
	return response, nil
}
