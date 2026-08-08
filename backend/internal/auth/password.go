package auth

import (
	"errors"
	"unicode/utf8"
)

var ErrInvalidPassword = errors.New("auth: invalid password")

// ValidatePassword checks length without trimming or normalizing the password.
func ValidatePassword(password string) error {
	if !utf8.ValidString(password) {
		return ErrInvalidPassword
	}
	length := utf8.RuneCountInString(password)
	if length < 12 || length > 128 {
		return ErrInvalidPassword
	}
	return nil
}
