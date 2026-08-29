// Package backup owns portable Reader-home archive creation and staged replacement.
package backup

import (
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/otwako/novelreader/internal/readerstore"
)

const (
	Format         = "novelreader-reader-home"
	CurrentVersion = 1
	ManifestPath   = "manifest.json"
	RestoreHelp    = "RESTORE.txt"
	PayloadRoot    = "reader-home"
)

type Manifest struct {
	Format               string `json:"format"`
	FormatVersion        int    `json:"formatVersion"`
	CreatedAt            string `json:"createdAt"`
	ExportedFromUsername string `json:"exportedFromUsername"`
	ReaderSchemaVersion  int    `json:"readerSchemaVersion"`
}

func NewManifest(username string, createdAt time.Time) Manifest {
	return Manifest{
		Format: Format, FormatVersion: CurrentVersion, CreatedAt: createdAt.Format(time.RFC3339),
		ExportedFromUsername: username, ReaderSchemaVersion: readerstore.CurrentReaderSchemaVersion,
	}
}

func Filename(username string, createdAt time.Time) string {
	return fmt.Sprintf("novelreader-%s-backup-%s.tar.gz", safeFilenamePart(username), createdAt.Format("20060102-150405-0700"))
}

func safeFilenamePart(value string) string {
	var result strings.Builder
	separator := false
	for _, character := range strings.TrimSpace(value) {
		if unicode.IsLetter(character) || unicode.IsDigit(character) || character == '-' || character == '_' {
			result.WriteRune(character)
			separator = false
		} else if !separator && result.Len() > 0 {
			result.WriteByte('-')
			separator = true
		}
		if result.Len() >= 60 {
			break
		}
	}
	cleaned := strings.Trim(result.String(), "-_")
	if cleaned == "" {
		return "reader"
	}
	return cleaned
}
