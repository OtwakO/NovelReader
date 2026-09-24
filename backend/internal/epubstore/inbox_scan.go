package epubstore

import (
	"context"
	"sort"
	"strings"

	"github.com/otwako/novelreader/internal/epub"
	"github.com/otwako/novelreader/internal/inboxfiles"
)

type InboxEntry struct {
	inboxfiles.Entry
	ReceiptID string
}

// ScanInbox keeps only one name-ordered page while reading the directory in
// chunks. Callers bound limit/deadline. This is a live listing, not a completion
// proof; producers must finish copying first and acquisition rechecks each file.
func (s *Store) ScanInbox(ctx context.Context, after string, limit int) ([]InboxEntry, error) {
	root, err := s.files.OpenInbox()
	if err != nil {
		return nil, err
	}
	defer root.Close()
	entries, err := inboxfiles.Scan(ctx, root, after, limit, epub.MaxInputBytes, ValidateFilename)
	if err != nil {
		return nil, err
	}
	result := make([]InboxEntry, len(entries))
	for index, entry := range entries {
		result[index].Entry = entry
	}

	if len(result) == 0 {
		return result, nil
	}
	// Join only the page's names, not the complete operational journal.
	args := make([]any, len(result))
	for index, entry := range result {
		args[index] = entry.Name
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(args)), ",")
	rows, err := s.db.QueryContext(ctx, `SELECT name,receipt_id FROM epub_inbox_claims WHERE name IN (`+placeholders+`)`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var name, id string
		if err := rows.Scan(&name, &id); err != nil {
			return nil, err
		}
		index := sort.Search(len(result), func(i int) bool { return result[i].Name >= name })
		result[index].ReceiptID = id // The query returns only names in this page.
	}
	return result, rows.Err()
}
