// Package security provides session management and request protection for
// FileGator.
//
// Sessions are stored in a single signed cookie: the cookie payload is the
// username plus a fingerprint of the user record. Because the fingerprint
// covers role, permissions, home directory and password hash, any change to a
// user invalidates their existing sessions automatically — the same guarantee
// FileGator v7 provides with its SESSION_HASH check.
//
// The cookie is signed with HMAC-SHA256 and carries an expiry timestamp. No
// server-side session state is kept, so restarts do not log users out and no
// cleanup is required.
package security

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// CookieName is the name of the session cookie.
const CookieName = "filegator_session"

// Errors returned by this package.
var (
	ErrNoSession       = errors.New("security: no session")
	ErrSessionExpired  = errors.New("security: session expired")
	ErrBadSignature    = errors.New("security: invalid session signature")
	ErrMalformedCookie = errors.New("security: malformed session cookie")
)

// Session is the data stored in the session cookie.
type Session struct {
	// Username identifies the logged-in user.
	Username string
	// Fingerprint changes whenever the user record changes.
	Fingerprint string
	// Expires is the absolute expiry time.
	Expires time.Time
	// Claims are set for identities that have no stored account, such as an
	// SSO "policy" login that exists only for this session. Stored users leave
	// this zero and are read from the auth provider on every request.
	Claims Claims
}

// Claims carries the authorisation of a session-only identity. It is empty for
// accounts that exist in the users file.
type Claims struct {
	// Name is the display name.
	Name string
	// Role is the account role.
	Role string
	// Home is the storage folder the session is confined to.
	Home string
	// Permissions are the granted permissions.
	Permissions []Permission
}

// Empty reports whether the claims carry nothing.
func (c Claims) Empty() bool { return c.Role == "" && c.Home == "" && len(c.Permissions) == 0 }

// Permission is one capability, mirrored here so the session package does not
// depend on the auth package.
type Permission = string

// Sessions issues and verifies signed session cookies.
type Sessions struct {
	key    []byte
	ttl    time.Duration
	secure bool
	// path scopes the cookie to a mount point such as "/browser"; "" means
	// the whole site.
	path string
}

// NewSessions returns a session manager. key must be a non-empty secret; ttl
// is the session lifetime. When secure is true the cookie is marked
// Secure, which requires HTTPS.
func NewSessions(key []byte, ttl time.Duration, secure bool) (*Sessions, error) {
	if len(key) == 0 {
		return nil, errors.New("security: session key must not be empty")
	}
	if ttl <= 0 {
		return nil, errors.New("security: session ttl must be positive")
	}
	return &Sessions{key: key, ttl: ttl, secure: secure}, nil
}

// GenerateKey returns a cryptographically random session key.
func GenerateKey() ([]byte, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return nil, fmt.Errorf("security: generate key: %w", err)
	}
	return b, nil
}

// Fingerprint derives a stable value from the mutable parts of a user record.
// Changing any of them invalidates existing sessions.
func Fingerprint(username, passwordHash, role, homeDir, permissions string) string {
	h := sha256.New()
	for _, part := range []string{username, passwordHash, role, homeDir, permissions} {
		// Length-prefix so field boundaries cannot be shifted.
		fmt.Fprintf(h, "%d:%s", len(part), part)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// Issue creates a signed cookie for the session.
func (s *Sessions) Issue(username, fingerprint string) *http.Cookie {
	return s.IssueWithClaims(username, fingerprint, Claims{})
}

// IssueWithClaims creates a cookie for an identity that carries its own
// authorisation, used by SSO sessions that have no stored account.
func (s *Sessions) IssueWithClaims(username, fingerprint string, claims Claims) *http.Cookie {
	exp := time.Now().Add(s.ttl)
	value := s.sign(Session{
		Username:    username,
		Fingerprint: fingerprint,
		Expires:     exp.UTC(),
		Claims:      claims,
	})
	return &http.Cookie{
		Name:     CookieName,
		Value:    value,
		Path:     s.cookiePath(),
		Expires:  exp,
		MaxAge:   int(s.ttl.Seconds()),
		HttpOnly: true,
		Secure:   s.secure,
		SameSite: http.SameSiteLaxMode,
	}
}

// Clear returns a cookie that deletes the session.
// cookiePath returns the path to use, defaulting to the whole site.
func (s *Sessions) cookiePath() string {
	if s.path == "" {
		return "/"
	}
	return s.path
}

// SetPath scopes the cookies to a URL prefix such as "/browser", so they are
// not sent to other applications on the same domain. Empty or "/" means the
// whole site.
func (s *Sessions) SetPath(prefix string) { s.path = normalizeCookiePath(prefix) }

// normalizeCookiePath returns a cookie Path value.
func normalizeCookiePath(prefix string) string {
	p := strings.TrimSpace(prefix)
	if p == "" || p == "/" {
		return "/"
	}
	return "/" + strings.Trim(p, "/")
}

func (s *Sessions) Clear() *http.Cookie {
	return &http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     s.cookiePath(),
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
		Secure:   s.secure,
		SameSite: http.SameSiteLaxMode,
	}
}

// Parse verifies the session cookie on r and returns its contents.
func (s *Sessions) Parse(r *http.Request) (*Session, error) {
	c, err := r.Cookie(CookieName)
	if err != nil {
		return nil, ErrNoSession
	}
	sess, err := s.verify(c.Value)
	if err != nil {
		return nil, err
	}
	if time.Now().After(sess.Expires) {
		return nil, ErrSessionExpired
	}
	return sess, nil
}

// CreateKeyHex returns a hex-encoded random key suitable for a config file.
func CreateKeyHex() (string, error) {
	k, err := GenerateKey()
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(k), nil
}

// ParseKeyHex decodes a hex-encoded session key.
func ParseKeyHex(s string) ([]byte, error) {
	k, err := hex.DecodeString(strings.TrimSpace(s))
	if err != nil {
		return nil, fmt.Errorf("security: decode session key: %w", err)
	}
	if len(k) == 0 {
		return nil, errors.New("security: session key must not be empty")
	}
	return k, nil
}

// sign serialises and authenticates a session.
//
// The claims segment is empty for ordinary accounts, so an existing cookie
// stays valid across this change as long as its signature still matches — it
// does not, because the payload string changed; sessions are simply re-issued
// on the next login.
func (s *Sessions) sign(sess Session) string {
	payload := fmt.Sprintf("%s|%s|%d|%s",
		b64(sess.Username),
		b64(sess.Fingerprint),
		sess.Expires.UTC().Unix(),
		b64(encodeClaims(sess.Claims)),
	)
	sig := s.mac(payload)
	return payload + "|" + sig
}

// encodeClaims renders claims as a compact string for the cookie.
//
// JSON rather than a custom format: it is unambiguous, and the claims are
// signed so their content cannot be edited by the client.
func encodeClaims(c Claims) string {
	if c.Empty() {
		return ""
	}
	data, err := json.Marshal(c)
	if err != nil {
		// Claims are plain strings and a permission list; a failure here means
		// the type changed in a way that must not be ignored.
		panic("security: cannot encode session claims: " + err.Error())
	}
	return string(data)
}

// decodeClaims reverses encodeClaims.
func decodeClaims(s string) Claims {
	if s == "" {
		return Claims{}
	}
	var c Claims
	if err := json.Unmarshal([]byte(s), &c); err != nil {
		return Claims{}
	}
	return c
}

// verify checks the signature and decodes the session.
func (s *Sessions) verify(value string) (*Session, error) {
	parts := strings.Split(value, "|")
	if len(parts) != 5 {
		return nil, ErrMalformedCookie
	}
	payload := strings.Join(parts[:4], "|")
	gotSig, err := hex.DecodeString(parts[4])
	if err != nil {
		return nil, ErrMalformedCookie
	}
	wantSig, err := hex.DecodeString(s.mac(payload))
	if err != nil {
		return nil, ErrMalformedCookie
	}
	if subtle.ConstantTimeCompare(gotSig, wantSig) != 1 {
		return nil, ErrBadSignature
	}

	username, err := unb64(parts[0])
	if err != nil {
		return nil, ErrMalformedCookie
	}
	fingerprint, err := unb64(parts[1])
	if err != nil {
		return nil, ErrMalformedCookie
	}
	var unix int64
	if _, err := fmt.Sscanf(parts[2], "%d", &unix); err != nil {
		return nil, ErrMalformedCookie
	}
	claimsRaw, err := unb64(parts[3])
	if err != nil {
		return nil, ErrMalformedCookie
	}
	return &Session{
		Username:    username,
		Fingerprint: fingerprint,
		Expires:     time.Unix(unix, 0).UTC(),
		Claims:      decodeClaims(claimsRaw),
	}, nil
}

func (s *Sessions) mac(payload string) string {
	m := hmac.New(sha256.New, s.key)
	m.Write([]byte(payload))
	return hex.EncodeToString(m.Sum(nil))
}

func b64(s string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(s))
}

func unb64(s string) (string, error) {
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
