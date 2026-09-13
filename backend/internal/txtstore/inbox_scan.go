package txtstore

import (
	"context"
	"errors"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/otwako/novelreader/internal/txt"
)

type InboxEntry struct {
	Name       string
	Size       int64
	ModifiedAt int64
	ReceiptID  string
	Problem    string
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
	directory, err := root.Open(".")
	if err != nil {
		return nil, err
	}
	defer directory.Close()
	result := make([]InboxEntry, 0, limit)
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		entries, readErr := directory.ReadDir(128)
		for _, entry := range entries {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			name := entry.Name()
			if name <= after || ValidateFilename(name) != nil {
				continue
			}
			if len(result) == limit && name >= result[len(result)-1].Name {
				continue
			}
			info, err := root.Lstat(name) // Keep metadata lookup anchored; never follow symlinks.
			if errors.Is(err, os.ErrNotExist) {
				continue
			} // Another admitted acquisition consumed it.
			if err != nil {
				return nil, err
			}
			value := InboxEntry{Name: name, Size: info.Size(), ModifiedAt: info.ModTime().UnixMilli()}
			if !info.Mode().IsRegular() {
				value.Problem = "not_regular"
			} else if info.Size() > txt.MaxInputBytes {
				value.Problem = "too_large"
			}
			index := sort.Search(len(result), func(i int) bool { return result[i].Name >= name })
			result = append(result, InboxEntry{})
			copy(result[index+1:], result[index:])
			result[index] = value
			if len(result) > limit {
				result = result[:limit]
			}
		}
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return nil, readErr
		}
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
	rows, err := s.db.QueryContext(ctx, `SELECT name,receipt_id FROM txt_inbox_claims WHERE name IN (`+placeholders+`)`, args...)
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
