package sso

import (
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/filegator/filegator/internal/config"
)

const (
	// The login the linked application signed. It belongs to the visitor.
	resplitLogin = "admin|panel|9999999999"
	resplitNonce = "cf9683ba7d1e4a0b9c2f5e8a1d3b6c40"
)

// movedNonce is the nonce field once the boundaries are moved: everything the
// application signed after the login.
func movedNonce(issued int64) string {
	return "panel|" + strconv.FormatInt(issued, 10) + "|" + resplitNonce
}

// A handoff must not verify when the bytes that were signed are split at
// different "|" positions.
func TestVerifyRefusesAHandoffReadWithDifferentFieldBoundaries(t *testing.T) {
	v := NewVerifier()
	issued := future()
	sig := Signature(secret, resplitLogin, "panel", issued, resplitNonce)

	h, err := v.Verify(testConfig(), "admin", "panel", 9999999999, movedNonce(issued), sig)
	if err == nil {
		t.Fatalf("a handoff signed for %q was accepted as user %q", resplitLogin, h.Username)
	}
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("err = %v, want ErrInvalid", err)
	}
}

// Moving the boundaries must not make an expired handoff valid again.
func TestVerifyRefusesAnExpiredHandoffReadWithDifferentFieldBoundaries(t *testing.T) {
	v := NewVerifier()
	// The ticket the application issued ran out an hour ago.
	issued := time.Now().Add(-time.Hour).Unix()
	sig := Signature(secret, resplitLogin, "panel", issued, resplitNonce)

	// Read the way it was signed, it is refused as expired.
	if _, err := v.Verify(testConfig(), resplitLogin, "panel", issued, resplitNonce, sig); !errors.Is(err, ErrExpired) {
		t.Fatalf("as signed: err = %v, want ErrExpired", err)
	}

	if h, err := v.Verify(testConfig(), "admin", "panel", 9999999999, movedNonce(issued), sig); err == nil {
		t.Fatalf("a handoff that expired an hour ago was accepted as user %q", h.Username)
	}
}

// The application name is the other field a separator could hide in. Such a
// name cannot be saved, but config.Load does not validate, so Verify is what
// stands between a hand-edited entry and a second reading.
func TestVerifyRefusesAnApplicationNameContainingTheSeparator(t *testing.T) {
	cfg := config.SSO{Applications: []config.SSOApplication{
		{Name: "portal", Secret: secret, Mode: config.SSOModePolicy, Home: "/clients/%s"},
		{Name: "ops|portal", Secret: secret, Mode: config.SSOModeExisting},
	}}
	exp := future()
	// Signed for the login "admin|ops" in "portal": the same bytes as the
	// login "admin" in "ops|portal".
	sig := Signature(secret, "admin|ops", "portal", exp, "n1")

	h, err := NewVerifier().Verify(cfg, "admin", "ops|portal", exp, "n1", sig)
	if err == nil {
		t.Fatalf("a handoff signed for %q in %q was accepted as %q in %q",
			"admin|ops", "portal", h.Username, h.Application.Name)
	}
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("err = %v, want ErrInvalid", err)
	}
}

// The rule is about the nonce itself, not only about a nonce that was put
// together from other fields: one the application chose is refused as well.
func TestVerifyRefusesANonceContainingTheSeparator(t *testing.T) {
	exp := future()
	sig := Signature(secret, "alice", "panel", exp, "session|42")
	if _, err := NewVerifier().Verify(testConfig(), "alice", "panel", exp, "session|42", sig); !errors.Is(err, ErrInvalid) {
		t.Fatalf("err = %v, want ErrInvalid", err)
	}
}

// A username may still contain the separator: read the way it was signed, the
// handoff is accepted for exactly that name.
func TestVerifyAcceptsAUsernameContainingTheSeparator(t *testing.T) {
	exp := future()
	sig := Signature(secret, resplitLogin, "panel", exp, resplitNonce)

	h, err := NewVerifier().Verify(testConfig(), resplitLogin, "panel", exp, resplitNonce, sig)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if h.Username != resplitLogin {
		t.Fatalf("username = %q, want %q", h.Username, resplitLogin)
	}
}
