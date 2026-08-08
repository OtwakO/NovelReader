package auth_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/otwako/novelreader/internal/auth"
)

func TestValidatePasswordUsesUnicodeCodePointLengthWithoutRewriting(t *testing.T) {
	valid := []string{
		"correct horse battery staple",
		"  leading and trailing spaces  ",
		strings.Repeat("界", 12),
		strings.Repeat("a", 128),
	}
	for _, password := range valid {
		if err := auth.ValidatePassword(password); err != nil {
			t.Errorf("ValidatePassword(%q): %v", password, err)
		}
	}

	invalid := []string{
		strings.Repeat("界", 11),
		strings.Repeat("a", 129),
		"12345678901\xff",
	}
	for _, password := range invalid {
		if err := auth.ValidatePassword(password); !errors.Is(err, auth.ErrInvalidPassword) {
			t.Errorf("ValidatePassword(length %d) error = %v", len([]rune(password)), err)
		}
	}
}
