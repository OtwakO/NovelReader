package auth_test

import (
	"errors"
	"testing"

	"github.com/otwako/novelreader/internal/auth"
)

func TestNormalizeUsernameProducesDisplayAndUniqueLookupForms(t *testing.T) {
	username, err := auth.NormalizeUsername("  Ａlice  ")
	if err != nil {
		t.Fatal(err)
	}
	if username.Display != "Alice" {
		t.Fatalf("display = %q", username.Display)
	}
	if username.Normalized != "alice" {
		t.Fatalf("normalized = %q", username.Normalized)
	}

	equivalent, err := auth.NormalizeUsername("ALICE")
	if err != nil {
		t.Fatal(err)
	}
	if equivalent.Normalized != username.Normalized {
		t.Fatalf("equivalent normalized = %q, want %q", equivalent.Normalized, username.Normalized)
	}

	precomposed, err := auth.NormalizeUsername("ǰohn")
	if err != nil {
		t.Fatal(err)
	}
	decomposed, err := auth.NormalizeUsername("J\u030Cohn")
	if err != nil {
		t.Fatal(err)
	}
	if precomposed.Normalized != "ǰohn" || decomposed.Normalized != precomposed.Normalized {
		t.Fatalf("canonical usernames = %q and %q", precomposed.Normalized, decomposed.Normalized)
	}
}

func TestNormalizeUsernameValidatesNormalizedCharactersAndLength(t *testing.T) {
	valid := []string{"reader_1", "读者.一", "éclair-test"}
	for _, raw := range valid {
		if _, err := auth.NormalizeUsername(raw); err != nil {
			t.Errorf("NormalizeUsername(%q): %v", raw, err)
		}
	}

	invalid := []string{"ab", "reader name", "reader@example", "reader/one", "read̸er", "123456789012345678901234567890123", "reader\xffname"}
	for _, raw := range invalid {
		if _, err := auth.NormalizeUsername(raw); !errors.Is(err, auth.ErrInvalidUsername) {
			t.Errorf("NormalizeUsername(%q) error = %v", raw, err)
		}
	}
}
