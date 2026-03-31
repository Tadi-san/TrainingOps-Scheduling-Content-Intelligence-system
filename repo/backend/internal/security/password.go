package security

import (
	"errors"
	"unicode"
)

var ErrPasswordTooWeak = errors.New("password must be at least 12 characters and include upper, lower, number, and symbol")

func ValidatePassword(password string) error {
	if len(password) < 12 {
		return ErrPasswordTooWeak
	}

	var hasUpper, hasLower, hasNumber, hasSymbol bool
	for _, r := range password {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsDigit(r):
			hasNumber = true
		case unicode.IsPunct(r) || unicode.IsSymbol(r):
			hasSymbol = true
		}
	}

	if !hasUpper || !hasLower || !hasNumber || !hasSymbol {
		return ErrPasswordTooWeak
	}

	return nil
}
