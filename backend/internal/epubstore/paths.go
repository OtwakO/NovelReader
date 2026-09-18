package epubstore

import (
	"errors"
	"path"
	"strings"
	"unicode/utf8"

	"github.com/otwako/novelreader/internal/readerstore"
)

var ErrInvalidFilename = errors.New("epubstore: expected an EPUB filename without directory components, at most 1024 bytes")

func ValidateFilename(name string) error {
	if len(name) > 1024 || !utf8.ValidString(name) || strings.ContainsAny(name, "/\\\x00") || !strings.EqualFold(path.Ext(name), ".epub") {
		return ErrInvalidFilename
	}
	return nil
}

func validateID(id string) error {
	if id == "" {
		return readerstore.ErrInvalidFilePath
	}
	for _, c := range id {
		if !(c >= 'A' && c <= 'Z' || c >= '2' && c <= '7') {
			return readerstore.ErrInvalidFilePath
		}
	}
	return nil
}
func originalPath(id string) string { return path.Join("epub", id, "original.epub") }
func transferPath(id string) string { return path.Join(readerstore.WorkDirectory, "epub", id+".part") }
