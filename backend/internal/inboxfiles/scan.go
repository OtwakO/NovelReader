package inboxfiles

import (
	"context"
	"errors"
	"io"
	"os"
	"sort"
)

type Entry struct {
	Name       string
	Size       int64
	ModifiedAt int64
	Problem    string
}

// Scan retains one name-ordered page, not an unbounded directory listing.
// Listing is not acquisition approval; stores must inspect again before intake.
// The caller bounds limit and supplies the format's filename and size policy.
func Scan(ctx context.Context, root *os.Root, after string, limit int, maxBytes int64, validate func(string) error) ([]Entry, error) {
	directory, err := root.Open(".")
	if err != nil {
		return nil, err
	}
	defer directory.Close()
	result := make([]Entry, 0, limit)
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
			if name <= after || validate(name) != nil {
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
			value := Entry{Name: name, Size: info.Size(), ModifiedAt: info.ModTime().UnixMilli()}
			if !info.Mode().IsRegular() {
				value.Problem = "not_regular"
			} else if info.Size() > maxBytes {
				value.Problem = "too_large"
			}
			index := sort.Search(len(result), func(i int) bool { return result[i].Name >= name })
			result = append(result, Entry{})
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
	return result, nil
}
