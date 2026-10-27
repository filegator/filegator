package httpx

import (
	"errors"
	"net/http"

	"github.com/filegator/filegator/internal/auth"
	"github.com/filegator/filegator/internal/security"
)

// Sessions resolves the session cookie and the CSRF token for requests.
//
// Both the user API and the admin API need this, so it lives here rather than
// in either handler package.
type Sessions struct {
	// Store is the auth provider used to resolve the session's user.
	Store auth.Provider
	// Sessions issues and verifies session cookies.
	Sessions *security.Sessions
	// CSRF validates double-submit tokens.
	CSRF *security.CSRF
	// Enabled reports whether CSRF protection is active.
	Enabled func() bool
}

// CurrentUser resolves the session cookie into a user. The session is only
// accepted if the user still exists and their fingerprint matches, so changing
// a user's role, permissions, home or password logs them out.
func (s *Sessions) CurrentUser(r *http.Request) (*auth.User, error) {
	sess, err := s.Sessions.Parse(r)
	switch {
	case err == nil:
		// A session carrying its own claims belongs to an identity with no
		// stored account (an SSO "policy" login). The claims are covered by the
		// cookie signature, so they are as trustworthy as the username.
		if !sess.Claims.Empty() {
			return claimsUser(sess.Username, sess.Claims), nil
		}
		u, findErr := s.Store.Find(sess.Username)
		if findErr != nil {
			return s.guest("not logged in")
		}
		if s.Store.SessionFingerprint(u) != sess.Fingerprint {
			// An expired session is not a guest visit: the user was logged in
			// and should be told to log in again, not silently demoted.
			return nil, Errorf(http.StatusUnauthorized, CodeUnauthorized, "session expired")
		}
		return u, nil
	case errors.Is(err, security.ErrNoSession):
		// No cookie at all: an anonymous visitor.
		return s.guest("not logged in")
	case errors.Is(err, security.ErrSessionExpired):
		// A cookie that was valid and has aged out. Treating it as a guest
		// would leave a logged-out user browsing by accident; they should log
		// in again.
		return nil, Errorf(http.StatusUnauthorized, CodeUnauthorized, "session expired")
	default:
		// A cookie that fails verification was tampered with, signed with a
		// different key, or left over from a previous version. That is not a
		// guest visit: the browser still holds a session cookie, so treating
		// it as anonymous would make "guest mode" look broken to every
		// returning user until they clear cookies. Reporting 401 sends them to
		// the login screen, which rewrites the cookie.
		return nil, Errorf(http.StatusUnauthorized, CodeUnauthorized, "invalid session")
	}
}

// claimsUser builds a user from a session's own claims.
func claimsUser(username string, c security.Claims) *auth.User {
	perms := make([]auth.Permission, 0, len(c.Permissions))
	for _, p := range c.Permissions {
		perms = append(perms, auth.Permission(p))
	}
	home := c.Home
	if home == "" {
		home = "/"
	}
	return &auth.User{
		Username:    username,
		Name:        c.Name,
		Role:        auth.Role(c.Role),
		HomeDir:     home,
		Permissions: perms,
	}
}

// guest returns the anonymous viewer when the instance publishes one.
//
// v7 served anonymous requests as the guest account whenever it existed, so a
// repository could be published by giving that account permissions. The same
// rule applies here: no usable session and a guest with read access means the
// caller is the guest, and every endpoint then applies its normal permission
// checks. Guests are read-only (see auth.User.has).
func (s *Sessions) guest(reason string) (*auth.User, error) {
	if g := s.Store.Guest(); g != nil && g.Can(auth.PermRead) {
		return g, nil
	}
	return nil, Errorf(http.StatusUnauthorized, CodeUnauthorized, reason)
}

// RequireCSRF validates the double-submit token on state-changing requests.
func (s *Sessions) RequireCSRF(r *http.Request) error {
	if s.Enabled != nil && !s.Enabled() {
		return nil
	}
	if err := s.CSRF.Valid(r); err != nil {
		return Errorf(http.StatusForbidden, CodeCSRF, "csrf token missing or invalid")
	}
	return nil
}

// RequireAdmin resolves the session and rejects callers that are not
// administrators.
func (s *Sessions) RequireAdmin(r *http.Request) (*auth.User, error) {
	u, err := s.CurrentUser(r)
	if err != nil {
		return nil, err
	}
	if !u.IsAdmin() {
		return nil, Errorf(http.StatusForbidden, CodeForbidden, "administrator access required")
	}
	return u, nil
}
