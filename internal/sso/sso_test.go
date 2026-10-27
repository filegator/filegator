package sso

import (
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/filegator/filegator/internal/config"
)

const secret = "0123456789abcdef0123456789abcdef"

// testConfig returns an SSO block with one application per mode.
func testConfig() config.SSO {
	return config.SSO{Applications: []config.SSOApplication{
		{Name: "panel", Secret: secret, Mode: config.SSOModeCreate, Home: "/clients/%s",
			Permissions: []string{"read", "download"}, Expiry: 60},
		{Name: "portal", Secret: secret + "-portal", Mode: config.SSOModePolicy, Home: "/s/%s",
			Permissions: []string{"read"}},
		{Name: "admin-panel", Secret: secret + "-admin", Mode: config.SSOModeExisting,
			AdminUsernames: []string{"jane"}},
	}}
}

// future returns an expiry far enough ahead to be valid.
func future() int64 { return time.Now().Add(time.Minute).Unix() }

func TestVerifyAcceptsAValidHandoff(t *testing.T) {
	v := NewVerifier()
	exp := future()
	sig := Signature(secret, "alice", "panel", exp, "nonce1")

	h, err := v.Verify(testConfig(), "alice", "panel", exp, "nonce1", sig)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if h.Username != "alice" || h.Application.Name != "panel" {
		t.Errorf("handoff = %+v", h)
	}
}

func TestVerifyRefusesAReplay(t *testing.T) {
	v := NewVerifier()
	exp := future()
	sig := Signature(secret, "alice", "panel", exp, "nonce1")

	if _, err := v.Verify(testConfig(), "alice", "panel", exp, "nonce1", sig); err != nil {
		t.Fatalf("first use: %v", err)
	}
	// The same URL must never work twice, or a leaked link in browser history
	// would sign someone in.
	if _, err := v.Verify(testConfig(), "alice", "panel", exp, "nonce1", sig); !errors.Is(err, ErrReplayed) {
		t.Fatalf("second use: got %v, want ErrReplayed", err)
	}
}

func TestVerifyRefusesExpired(t *testing.T) {
	v := NewVerifier()
	exp := time.Now().Add(-time.Second).Unix()
	sig := Signature(secret, "alice", "panel", exp, "n")

	if _, err := v.Verify(testConfig(), "alice", "panel", exp, "n", sig); !errors.Is(err, ErrExpired) {
		t.Fatalf("got %v, want ErrExpired", err)
	}
	// An expired handoff must not consume its nonce: nothing to protect, and
	// it keeps the cache honest.
	if !v.spend("n", time.Now().Add(time.Minute)) {
		t.Error("expired handoff consumed its nonce")
	}
}

func TestVerifyRefusesUnknownApplication(t *testing.T) {
	v := NewVerifier()
	exp := future()
	// Signed with a valid secret, but for an application that does not exist.
	sig := Signature(secret, "alice", "ghost", exp, "n")

	if _, err := v.Verify(testConfig(), "alice", "ghost", exp, "n", sig); !errors.Is(err, ErrInvalid) {
		t.Fatalf("got %v, want ErrInvalid", err)
	}
}

func TestVerifyRefusesTamperedFields(t *testing.T) {
	v := NewVerifier()
	exp := future()
	sig := Signature(secret, "alice", "panel", exp, "nonce")

	cases := []struct{ user, app string }{
		{"bob", "panel"},       // changed user
		{"alice", "portal"},    // changed application
		{"alice-ext", "panel"}, // longer user
	}
	for _, c := range cases {
		if _, err := v.Verify(testConfig(), c.user, c.app, exp, "nonce", sig); err == nil {
			t.Errorf("tampered handoff accepted: %+v", c)
		}
	}
}

func TestVerifyRefusesTamperedExpiry(t *testing.T) {
	v := NewVerifier()
	exp := future()
	sig := Signature(secret, "alice", "panel", exp, "nonce")

	// Extending the expiry breaks the signature, so a short-lived link cannot
	// be made long-lived by editing it.
	if _, err := v.Verify(testConfig(), "alice", "panel", exp+3600, "nonce", sig); err == nil {
		t.Fatal("extended expiry accepted")
	}
}

func TestVerifyRefusesEmptyFields(t *testing.T) {
	v := NewVerifier()
	exp := future()
	for _, c := range []struct{ user, nonce string }{
		{"", "n"}, {"alice", ""}, {"   ", "n"},
	} {
		sig := Signature(secret, c.user, "panel", exp, c.nonce)
		if _, err := v.Verify(testConfig(), c.user, "panel", exp, c.nonce, sig); err == nil {
			t.Errorf("accepted %+v", c)
		}
	}
}

func TestVerifyIsCaseInsensitiveForTheSignature(t *testing.T) {
	v := NewVerifier()
	exp := future()
	sig := Signature(secret, "alice", "panel", exp, "n")

	// Hex signatures may arrive upper- or lower-case from different languages.
	if _, err := v.Verify(testConfig(), "alice", "panel", exp, "n", upper(sig)); err != nil {
		t.Fatalf("uppercase signature rejected: %v", err)
	}
}

func upper(s string) string {
	out := []byte(s)
	for i, c := range out {
		if c >= 'a' && c <= 'f' {
			out[i] = c - 32
		}
	}
	return string(out)
}

func TestExpiryDefaults(t *testing.T) {
	if got := Expiry(config.SSOApplication{}); got != time.Minute {
		t.Errorf("default expiry = %v", got)
	}
	if got := Expiry(config.SSOApplication{Expiry: 120}); got != 2*time.Minute {
		t.Errorf("configured expiry = %v", got)
	}
}

func TestNonceCacheIsBounded(t *testing.T) {
	v := NewVerifier()
	exp := time.Now().Add(time.Hour)
	for i := 0; i < maxNonces; i++ {
		if !v.spend("nonce-"+strconv.Itoa(i), exp) {
			// Distinct nonces must all be accepted until the cap.
			t.Fatalf("refused a distinct nonce at %d", i)
		}
	}
	// Beyond the cap the verifier refuses rather than growing without bound;
	// reaching this means handoffs are generated far faster than used.
	if v.spend("one-too-many", exp) {
		t.Error("cache grew past its limit")
	}
}

func TestHomeFor(t *testing.T) {
	app := config.SSOApplication{Home: "/clients/%s"}
	if got := HomeFor(app, "alice"); got != "/clients/alice" {
		t.Errorf("HomeFor = %q", got)
	}
	// A name with a separator must not climb out of the template.
	if got := HomeFor(app, "../../etc"); got == "/clients/../../etc" {
		t.Errorf("separator not encoded: %q", got)
	}
	// An empty template means the storage root.
	if got := HomeFor(config.SSOApplication{}, "alice"); got != "/" {
		t.Errorf("empty template = %q", got)
	}
}

func TestSignatureMatchesThePHPHelper(t *testing.T) {
	// The PHP helper in docs/content/sso.md and this package must produce the
	// same bytes, or no other application can sign a handoff. The value below
	// was produced by that code.
	got := Signature("0123456789abcdef0123456789abcdef", "alice", "panel", 1750000000, "abc123")
	const want = "f95a7b0a7bab98d99767f52ab977631628d63445af48343bb79b2e645aa3f64f"
	if got != want {
		t.Fatalf("Signature = %s, want %s (PHP helper disagrees)", got, want)
	}
}
