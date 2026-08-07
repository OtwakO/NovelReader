package readerstore

import (
	"errors"
	"regexp"
)

var (
	ErrInvalidUserID = errors.New("readerstore: invalid user ID")

	canonicalUUIDv4 = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
)

// UserID is the immutable filesystem and storage identity of one reader.
type UserID string

func ParseUserID(value string) (UserID, error) {
	if !canonicalUUIDv4.MatchString(value) {
		return "", ErrInvalidUserID
	}
	return UserID(value), nil
}

func validateUserID(userID UserID) error {
	parsed, err := ParseUserID(string(userID))
	if err != nil || parsed != userID {
		return ErrInvalidUserID
	}
	return nil
}
