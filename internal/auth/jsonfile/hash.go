package jsonfile

import (
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"github.com/filegator/filegator/internal/auth"
)

// bcryptMaxPassword is the maximum password length bcrypt accepts.
const bcryptMaxPassword = 72

// ErrPasswordTooLong is returned when a password exceeds bcrypt's limit.
var ErrPasswordTooLong = errors.New("auth: password exceeds 72 bytes")

// IsBcryptHash reports whether s looks like a bcrypt hash. Both the "$2y$"
// variant produced by PHP and the "$2a$"/"$2b$" variants produced by Go are
// accepted.
func IsBcryptHash(s string) bool {
	return strings.HasPrefix(s, "$2y$") || strings.HasPrefix(s, "$2a$") || strings.HasPrefix(s, "$2b$")
}

// VerifyPassword checks password against a bcrypt hash. Plain-text values are
// rejected, never compared directly.
func VerifyPassword(password, hash string) bool {
	if !IsBcryptHash(hash) {
		return false
	}
	// PHP writes "$2y$"; Go's bcrypt accepts that body when prefixed with
	// "$2a$" or "$2b$", so normalise before comparison.
	normalised := "$2a$" + hash[4:]
	if len(password) > bcryptMaxPassword {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(normalised), []byte(password)) == nil
}

// HashPassword returns a bcrypt hash for password, compatible with v7.
func HashPassword(password string) (string, error) {
	if len(password) > bcryptMaxPassword {
		return "", ErrPasswordTooLong
	}
	h, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("auth: hash password: %w", err)
	}
	return string(h), nil
}

// Ensure the provider still satisfies the shared interface after the move.
var _ auth.Provider = (*UsersFile)(nil)
