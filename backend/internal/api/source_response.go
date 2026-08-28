package api

import "github.com/otwako/novelreader/internal/booksource"

type sourceManagementResponse struct {
	booksource.BookSource
	CollectionID string `json:"collectionId,omitempty"`
}

func sourceManagementResponses(sources []booksource.BookSource) []sourceManagementResponse {
	responses := make([]sourceManagementResponse, len(sources))
	for index := range sources {
		responses[index] = sourceManagementResponse{BookSource: sources[index], CollectionID: sources[index].CollectionID}
	}
	return responses
}
