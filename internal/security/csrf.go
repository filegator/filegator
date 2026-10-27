package security

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"net/http"
)

// CSRF implements double-submit-cookie protection.
//
// A random token is stored in a non-HttpOnly cookie so the frontend can read
// it, and the same token must be sent back in the CSRFHeader header on
// state-changing requests. No server-side state is needed: an attacker on a
// different origin can make the browser send the cookie but cannot read it to
// populate the header.
type CSRF struct {
	// CookieName is the cookie holding the token.
	CookieName string
	// Header is the request header carrying the token.
	Header string
	// Secure marks cookies Secure; requires HTTPS.
	Secure bool
	// Path scopes the cookie to a mount point; empty means the whole site.
	Path string
}

// Default CSRF settings, matching the names used by v7's frontend.
const (
	CSRFCookieName = "filegator_csrf"
	CSRFHeader     = "X-CSRF-Token"
)

// ErrCSRF is returned when validation fails.
var ErrCSRF = errors.New("security: invalid csrf token")

// NewCSRF returns a CSRF protector with the default cookie and header names.
func NewCSRF(secure bool) *CSRF {
	return &CSRF{CookieName: CSRFCookieName, Header: CSRFHeader, Secure: secure}
}

// NewToken returns a random, URL-safe CSRF token.
func NewToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// Token returns the current token for r, or issues a new one. The returned
// boolean reports whether the caller must set the cookie on the response.
func (c *CSRF) Token(r *http.Request) (token string, fresh bool, err error) {
	if cookie, err := r.Cookie(c.CookieName); err == nil && cookie.Value != "" {
		return cookie.Value, false, nil
	}
	token, err = NewToken()
	if err != nil {
		return "", false, err
	}
	return token, true, nil
}

// Cookie builds the cookie that stores token on the client.
func (c *CSRF) Cookie(token string) *http.Cookie {
	path := c.Path
	if path == "" {
		path = "/"
	}
	return &http.Cookie{
		Name:     c.CookieName,
		Value:    token,
		Path:     path,
		HttpOnly: false, // must be readable by the frontend
		Secure:   c.Secure,
		SameSite: http.SameSiteLaxMode,
	}
}

// Valid reports whether r carries a matching token in both the cookie and
// the header.
func (c *CSRF) Valid(r *http.Request) error {
	cookie, err := r.Cookie(c.CookieName)
	if err != nil || cookie.Value == "" {
		return ErrCSRF
	}
	header := r.Header.Get(c.Header)
	if header == "" {
		return ErrCSRF
	}
	if subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(header)) != 1 {
		return ErrCSRF
	}
	return nil
}
