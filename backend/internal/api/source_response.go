package api

import (
	"encoding/json"
	"fmt"

	"github.com/otwako/novelreader/internal/booksource"
)

type sourceManagementResponse map[string]json.RawMessage

func sourceManagementResponses(sources []booksource.BookSource) ([]sourceManagementResponse, error) {
	responses := make([]sourceManagementResponse, len(sources))
	for index := range sources {
		encoded, err := sources[index].MarshalJSON()
		if err != nil {
			return nil, fmt.Errorf("encode BookSource management response: %w", err)
		}
		if err := json.Unmarshal(encoded, &responses[index]); err != nil {
			return nil, fmt.Errorf("decode BookSource management response: %w", err)
		}
		encodedID, _ := json.Marshal(sources[index].ID)
		responses[index]["sourceId"] = encodedID
		if sources[index].CollectionID != "" {
			encodedID, _ := json.Marshal(sources[index].CollectionID)
			responses[index]["collectionId"] = encodedID
		}
	}
	return responses, nil
}
