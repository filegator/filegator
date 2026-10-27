package api

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/filegator/filegator/internal/auth"
	"github.com/filegator/filegator/internal/config"
	"github.com/filegator/filegator/internal/logger"
	"github.com/filegator/filegator/internal/router"
	"github.com/filegator/filegator/internal/security"
	"github.com/filegator/filegator/internal/sso"
	"github.com/filegator/filegator/internal/storage"
)

// Handoff handles GET /auth/sso and signs a visitor in from another
// application.
//
// The URL is signed with a per-application secret:
//
//	/auth/sso?user=alice&app=panel&exp=1750000000&nonce=<hex>&sig=<hex>
//	sig = HMAC-SHA256(secret, "user|app|exp|nonce")
//
// It is single-use and short-lived: the nonce is spent on first use, so a URL
// that leaks through browser history or a proxy log is worthless afterwards.
// On success the normal session cookie is issued and the browser is redirected
// to the file browser, which means from then on this is an ordinary session
// and nothing in the application needs to know how it started.
func (s *Server) Handoff(w http.ResponseWriter, r *http.Request, _ router.Params) {
	if s.SSO == nil {
		http.NotFound(w, r)
		return
	}
	query := r.URL.Query()

	// The handoff runs without a session, so state-changing protections do not
	// apply; the signature is the authorisation. It is still a GET because a
	// browser must be able to follow it as a redirect.
	handoff, err := s.SSO.Verify(
		s.Config.Get().SSO,
		query.Get("user"),
		query.Get("app"),
		parseInt64(query.Get("exp")),
		query.Get("nonce"),
		query.Get("sig"),
	)
	if err != nil {
		s.ssoFailure(r, query.Get("app"), err)
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		return
	}

	resolved, err := sso.Resolve(handoff.Application, handoff.Username, s.Users, s.ensureHome)
	if err != nil {
		s.ssoFailure(r, handoff.Application.Name, err)
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		return
	}

	user := resolved.User
	fingerprint := s.ssoFingerprint(handoff.Application, user)
	cookie := s.Sessions.Issue(user.Username, fingerprint)
	if resolved.Ephemeral {
		// No account to read on the next request, so the authorisation travels
		// inside the signed cookie.
		cookie = s.Sessions.IssueWithClaims(user.Username, fingerprint, security.Claims{
			Name:        user.Name,
			Role:        string(user.Role),
			Home:        user.HomeDir,
			Permissions: permissionStrings(user.Permissions),
		})
	}
	http.SetCookie(w, cookie)
	if token, fresh, csrfErr := s.CSRF.Token(r); csrfErr == nil && fresh {
		http.SetCookie(w, s.CSRF.Cookie(token))
	}

	fields := logger.Fields{
		"user":   user.Username,
		"app":    handoff.Application.Name,
		"mode":   handoff.Application.Mode,
		"target": user.HomeDir,
	}
	if resolved.Created {
		fields["created"] = true
	}
	if resolved.Ephemeral {
		fields["ephemeral"] = true
	}
	s.logEvent(r, user, logger.EventSSOLogin, "single sign-on", fields)

	// Redirect to the file browser, under the mount point when there is one.
	http.Redirect(w, r, s.BasePath()+"/", http.StatusSeeOther)
}

// ssoFingerprint binds a session to the authorisation it was issued under.
//
// For a stored account this is the provider's fingerprint, so changing the
// user's permissions still logs them out. For an ephemeral one it is derived
// from the application's policy, so editing that policy does the same.
func (s *Server) ssoFingerprint(app config.SSOApplication, u *auth.User) string {
	if stored, err := s.Users.Find(u.Username); err == nil && u.HomeDir == stored.HomeDir &&
		len(u.Permissions) == len(stored.Permissions) {
		return s.Users.SessionFingerprint(stored)
	}
	h := sha256.New()
	for _, part := range []string{
		app.Name, app.Mode, u.Username, u.HomeDir, string(u.Role),
		strings.Join(permissionStrings(u.Permissions), ","),
	} {
		h.Write([]byte(strconv.Itoa(len(part)) + ":" + part))
	}
	return hex.EncodeToString(h.Sum(nil))
}

// ensureHome creates a home folder for an account the handoff just created.
//
// Without it a newly created account would sign in and then be refused with
// "your home folder does not exist". A folder that already exists is fine: the
// template may point several users at one shared place.
func (s *Server) ensureHome(home string) error {
	root := s.ActiveStorage()
	if root == nil {
		return errors.New("sso: storage is not configured")
	}
	err := root.EnsureDir(home)
	if err != nil && !errors.Is(err, storage.ErrExists) {
		return err
	}
	return nil
}

// SSOFailure records a refused handoff.
//
// The reason is logged but never returned: an attacker must not be able to tell
// an unknown application from a bad signature, or an unknown user from a
// replayed URL.
func (s *Server) ssoFailure(r *http.Request, appName string, err error) {
	reason := "invalid"
	switch {
	case errors.Is(err, sso.ErrExpired):
		reason = "expired"
	case errors.Is(err, sso.ErrReplayed):
		reason = "replayed"
	case errors.Is(err, sso.ErrUnknownUser):
		reason = "unknown user"
	}
	s.Log.Warn(logger.EventSSOFailed, "single sign-on refused",
		logger.Fields{"app": appName, "reason": reason})
}

// SSOAuthRoutes registers the handoff endpoint.
func (s *Server) SSOAuthRoutes(rt *router.Router) {
	rt.Get("/auth/sso", s.Handoff)
}

// parseInt64 parses a decimal timestamp, returning 0 on failure so a malformed
// value can never be mistaken for a valid expiry.
func parseInt64(v string) int64 {
	n, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
	if err != nil {
		return 0
	}
	return n
}

// permissionStrings renders permissions for a session claim.
func permissionStrings(perms []auth.Permission) []string {
	out := make([]string, len(perms))
	for i, p := range perms {
		out[i] = string(p)
	}
	return out
}
