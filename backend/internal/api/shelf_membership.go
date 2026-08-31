package api

import (
	"log/slog"

	"github.com/otwako/novelreader/internal/book"
)

type shelfMembership map[string]string

func (s *Server) loadShelfMembership() shelfMembership {
	if s.bookStore == nil {
		return nil
	}
	identities, err := s.bookStore.ListShelfBookIdentities()
	if err != nil {
		slog.Warn("api: could not annotate discovery results with shelf membership", "error", err)
		return nil
	}
	membership := make(shelfMembership, len(identities))
	for _, identity := range identities {
		membership[logicalBookKey(identity.Name, identity.Author)] = identity.ID
	}
	return membership
}

func (membership shelfMembership) annotate(results []book.SearchResult) {
	for index := range results {
		name, author := book.NormalizeBookIdentity(results[index].Name, results[index].Author)
		results[index].ShelfBookID = membership[logicalBookKey(name, author)]
	}
}

func logicalBookKey(name, author string) string {
	return name + "\x00" + author
}
