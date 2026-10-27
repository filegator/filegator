package jsonfile

import (
	"errors"
	"strings"
	"testing"
)

func TestHashAndVerify(t *testing.T) {
	hash, err := HashPassword("s3cret")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if !IsBcryptHash(hash) {
		t.Fatalf("hash %q is not bcrypt", hash)
	}
	if !VerifyPassword("s3cret", hash) {
		t.Error("correct password rejected")
	}
	if VerifyPassword("wrong", hash) {
		t.Error("wrong password accepted")
	}
}

func TestVerifyPHPHash(t *testing.T) {
	// v7 hash for the password "admin123".
	const phpHash = "$2y$10$Nu35w4pteLfc7BDCIkDPkecjw8wsH8Y2GMfIewUbXLT7zzW6WOxwq"
	if !VerifyPassword("admin123", phpHash) {
		t.Error("v7 hash not accepted")
	}
	if VerifyPassword("notadmin", phpHash) {
		t.Error("wrong password accepted against v7 hash")
	}
}

func TestPlainTextPasswordRejected(t *testing.T) {
	for _, stored := range []string{"plain", "filegator", "", "sha256:abc"} {
		if VerifyPassword(stored, stored) {
			t.Errorf("plain-text hash %q accepted", stored)
		}
	}
}

func TestHashTooLong(t *testing.T) {
	if _, err := HashPassword(strings.Repeat("a", 73)); !errors.Is(err, ErrPasswordTooLong) {
		t.Fatalf("got %v", err)
	}
	if VerifyPassword(strings.Repeat("a", 100), "$2y$10$Nu35w4pteLfc7BDCIkDPkecjw8wsH8Y2GMfIewUbXLT7zzW6WOxwq") {
		t.Error("overlong password should not verify")
	}
}

func TestIsBcryptHashVariants(t *testing.T) {
	for _, h := range []string{"$2y$10$abc", "$2a$10$abc", "$2b$10$abc"} {
		if !IsBcryptHash(h) {
			t.Errorf("%q not detected", h)
		}
	}
	if IsBcryptHash("2a$10$abc") {
		t.Error("missing leading $ should be rejected")
	}
}
