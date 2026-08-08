package auth

import (
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"
)

var ErrInvalidUsername = errors.New("auth: invalid username")

type Username struct {
	Display    string
	Normalized string
}

// NormalizeUsername returns the display spelling and case-insensitive unique lookup form.
func NormalizeUsername(raw string) (Username, error) {
	if !utf8.ValidString(raw) {
		return Username{}, ErrInvalidUsername
	}
	display := norm.NFKC.String(strings.TrimSpace(raw))
	normalized := norm.NFKC.String(cases.Fold().String(display))
	length := utf8.RuneCountInString(normalized)
	if length < 3 || length > 32 {
		return Username{}, ErrInvalidUsername
	}
	for _, character := range normalized {
		if unicode.IsLetter(character) || unicode.IsNumber(character) || character == '_' || character == '-' || character == '.' {
			continue
		}
		return Username{}, ErrInvalidUsername
	}
	return Username{Display: display, Normalized: normalized}, nil
}
