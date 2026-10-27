package security

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func newTestSessions(t *testing.T) *Sessions {
	t.Helper()
	s, err := NewSessions([]byte("test-key-that-is-long-enough"), time.Hour, false)
	if err != nil {
		t.Fatalf("NewSessions: %v", err)
	}
	return s
}

func reqWithCookie(value string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	if value != "" {
		r.AddCookie(&http.Cookie{Name: CookieName, Value: value})
	}
	return r
}

func TestIssueAndParse(t *testing.T) {
	s := newTestSessions(t)
	c := s.Issue("alice", "fp123")
	if c.Value == "" {
		t.Fatal("empty cookie value")
	}
	if !c.HttpOnly {
		t.Error("cookie must be HttpOnly")
	}
	if c.SameSite != http.SameSiteLaxMode {
		t.Errorf("SameSite = %v", c.SameSite)
	}
	if c.Path != "/" {
		t.Errorf("Path = %q", c.Path)
	}

	sess, err := s.Parse(reqWithCookie(c.Value))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if sess.Username != "alice" || sess.Fingerprint != "fp123" {
		t.Errorf("session = %+v", sess)
	}
}

func TestSecureFlag(t *testing.T) {
	s, err := NewSessions([]byte("k"), time.Hour, true)
	if err != nil {
		t.Fatal(err)
	}
	if !s.Issue("a", "b").Secure {
		t.Error("expected Secure cookie")
	}
	if !s.Clear().Secure {
		t.Error("expected Secure clear cookie")
	}
}

func TestNoCookie(t *testing.T) {
	if _, err := newTestSessions(t).Parse(reqWithCookie("")); !errors.Is(err, ErrNoSession) {
		t.Fatalf("got %v", err)
	}
}

func TestTamperedPayload(t *testing.T) {
	s := newTestSessions(t)
	value := s.Issue("alice", "fp").Value
	parts := strings.Split(value, "|")

	// Swap the username, keep the original signature.
	tampered := b64("admin") + "|" + strings.Join(parts[1:], "|")
	if _, err := s.Parse(reqWithCookie(tampered)); !errors.Is(err, ErrBadSignature) {
		t.Fatalf("got %v, want ErrBadSignature", err)
	}
}

func TestTamperedSignature(t *testing.T) {
	s := newTestSessions(t)
	value := s.Issue("alice", "fp").Value
	bad := value[:len(value)-1] + flipHex(value[len(value)-1])
	if _, err := s.Parse(reqWithCookie(bad)); !errors.Is(err, ErrBadSignature) {
		t.Fatalf("got %v, want ErrBadSignature", err)
	}
}

func flipHex(c byte) string {
	if c == 'a' {
		return "b"
	}
	return "a"
}

func TestDifferentKeyRejected(t *testing.T) {
	a := newTestSessions(t)
	b, err := NewSessions([]byte("another-key"), time.Hour, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.Parse(reqWithCookie(a.Issue("x", "y").Value)); !errors.Is(err, ErrBadSignature) {
		t.Fatalf("got %v", err)
	}
}

func TestExpiredSession(t *testing.T) {
	s, err := NewSessions([]byte("k"), time.Nanosecond, false)
	if err != nil {
		t.Fatal(err)
	}
	c := s.Issue("alice", "fp")
	time.Sleep(2 * time.Millisecond)
	if _, err := s.Parse(reqWithCookie(c.Value)); !errors.Is(err, ErrSessionExpired) {
		t.Fatalf("got %v, want ErrSessionExpired", err)
	}
}

func TestMalformedCookies(t *testing.T) {
	s := newTestSessions(t)
	for _, v := range []string{"garbage", "a|b", "a|b|c", "a|b|c|zz", "a|b|notanumber|deadbeef"} {
		_, err := s.Parse(reqWithCookie(v))
		if err == nil {
			t.Errorf("%q accepted", v)
		}
	}
}

func TestClear(t *testing.T) {
	c := newTestSessions(t).Clear()
	if c.MaxAge != -1 {
		t.Errorf("MaxAge = %d", c.MaxAge)
	}
	if c.Value != "" {
		t.Errorf("Value = %q", c.Value)
	}
	if !c.HttpOnly {
		t.Error("clear cookie must be HttpOnly")
	}
}

func TestFingerprintChangesWithUserData(t *testing.T) {
	base := Fingerprint("alice", "hash1", "user", "/home/alice", "read|write")
	if base != Fingerprint("alice", "hash1", "user", "/home/alice", "read|write") {
		t.Error("fingerprint is not deterministic")
	}
	variants := []string{
		Fingerprint("alice", "hash2", "user", "/home/alice", "read|write"),
		Fingerprint("alice", "hash1", "admin", "/home/alice", "read|write"),
		Fingerprint("alice", "hash1", "user", "/home/bob", "read|write"),
		Fingerprint("alice", "hash1", "user", "/home/alice", "read"),
		Fingerprint("bob", "hash1", "user", "/home/alice", "read|write"),
	}
	for _, v := range variants {
		if v == base {
			t.Error("fingerprint did not change when user data changed")
		}
	}
}

func TestFingerprintFieldBoundaries(t *testing.T) {
	// Concatenation without length prefixes would collide here.
	a := Fingerprint("ab", "c", "user", "/", "")
	b := Fingerprint("a", "bc", "user", "/", "")
	if a == b {
		t.Error("fingerprint collides across field boundaries")
	}
}

func TestParseEmptyUsername(t *testing.T) {
	// Usernames containing the separator must round-trip safely because the
	// payload fields are base64 encoded.
	s := newTestSessions(t)
	sess, err := s.Parse(reqWithCookie(s.Issue("a|b|c", "f|p").Value))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if sess.Username != "a|b|c" || sess.Fingerprint != "f|p" {
		t.Errorf("session = %+v", sess)
	}
}

func TestNewSessionsValidation(t *testing.T) {
	if _, err := NewSessions(nil, time.Hour, false); err == nil {
		t.Error("empty key accepted")
	}
	if _, err := NewSessions([]byte("k"), 0, false); err == nil {
		t.Error("zero ttl accepted")
	}
}

func TestKeyHexRoundTrip(t *testing.T) {
	hexKey, err := CreateKeyHex()
	if err != nil {
		t.Fatal(err)
	}
	if len(hexKey) != 64 {
		t.Errorf("key length = %d, want 64 hex chars", len(hexKey))
	}
	if _, err := ParseKeyHex(hexKey); err != nil {
		t.Fatalf("ParseKeyHex: %v", err)
	}
	if _, err := ParseKeyHex("nothex"); err == nil {
		t.Error("invalid hex accepted")
	}
	if _, err := ParseKeyHex(""); err == nil {
		t.Error("empty key accepted")
	}
}

func TestCookiePathFollowsTheMountPoint(t *testing.T) {
	s := newTestSessions(t)

	// Default: the whole site.
	if got := s.Issue("alice", "fp").Path; got != "/" {
		t.Errorf("default path = %q", got)
	}

	// Mounted under a prefix: the cookie must not leak to sibling apps.
	s.SetPath("/browser/")
	cookie := s.Issue("alice", "fp")
	if cookie.Path != "/browser" {
		t.Errorf("section path = %q, want /browser", cookie.Path)
	}
	// Clearing must use the same path, or the browser keeps the old cookie.
	if got := s.Clear().Path; got != "/browser" {
		t.Errorf("clear path = %q", got)
	}
}

func TestCSRFCookiePath(t *testing.T) {
	c := NewCSRF(false)
	if got := c.Cookie("tok").Path; got != "/" {
		t.Errorf("default path = %q", got)
	}
	c.Path = "/browser"
	if got := c.Cookie("tok").Path; got != "/browser" {
		t.Errorf("path = %q", got)
	}
}
