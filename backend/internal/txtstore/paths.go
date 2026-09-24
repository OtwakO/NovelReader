package txtstore

import (
	"errors"
	"path"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/otwako/novelreader/internal/readerstore"
)

var ErrInvalidFilename = errors.New("txtstore: expected a TXT filename without directory components, at most 1024 bytes")

func ValidateFilename(name string) error {
	if len(name) > 1024 || !utf8.ValidString(name) || strings.ContainsAny(name, "/\\\x00") || !strings.EqualFold(path.Ext(name), ".txt") {
		return ErrInvalidFilename
	}
	return nil
}

func managedPath(name, id string) (string, error) {
	if err := ValidateFilename(name); err != nil {
		return "", err
	}
	var title strings.Builder
	for _, character := range strings.TrimSuffix(name, path.Ext(name)) {
		if !(unicode.IsLetter(character) || unicode.IsDigit(character) || unicode.IsMark(character) || character == '-' || character == '_' || character == ' ') {
			continue
		}
		if title.Len()+utf8.RuneLen(character) > 80 {
			break
		}
		title.WriteRune(character)
	}
	label := strings.TrimSpace(title.String())
	if label == "" {
		label = "TXT"
	}
	return path.Join("txt", label+"--"+id, "original.txt"), nil
}

func workPath(id string) string { return path.Join(readerstore.WorkDirectory, "txt", id+".part") }

// Persisted paths cross the restore boundary: they may only identify this
// receipt's managed directory, never another reader file or temporary work.
func validateReceiptPath(value Receipt) error {
	if value.ID == "" {
		return readerstore.ErrInvalidFilePath
	}
	for _, character := range value.ID {
		if !(character >= 'A' && character <= 'Z' || character >= '2' && character <= '7') {
			return readerstore.ErrInvalidFilePath
		}
	}
	parts := strings.Split(value.Path, "/")
	if len(parts) != 3 || parts[0] != "txt" || parts[2] != "original.txt" || strings.ContainsAny(value.Path, "\\\x00") || path.Clean(value.Path) != value.Path || !strings.HasSuffix(parts[1], "--"+value.ID) {
		return readerstore.ErrInvalidFilePath
	}
	return nil
}
