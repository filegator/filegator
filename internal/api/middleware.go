package api

import (
	"net/http"
	"net/url"
	"strconv"

	"github.com/filegator/filegator/internal/api/httpx"
	"github.com/filegator/filegator/internal/auth"
	"github.com/filegator/filegator/internal/logger"
	"github.com/filegator/filegator/internal/router"
)

// UserHandler is a handler that runs for an authenticated user.
type UserHandler func(w http.ResponseWriter, r *http.Request, u *auth.User)

// withUser resolves the session for a request and rejects unauthenticated
// callers.
func (s *Server) withUser(h UserHandler) router.Handler {
	return func(w http.ResponseWriter, r *http.Request, p router.Params) {
		u, err := s.CurrentUser(r)
		if err != nil {
			WriteError(w, err)
			return
		}
		h(w, r, u)
	}
}

// session returns the shared session resolver for this server.
func (s *Server) session() *httpx.Sessions {
	return &httpx.Sessions{
		Store:    s.Users,
		Sessions: s.Sessions,
		CSRF:     s.CSRF,
		Enabled:  func() bool { return s.Config.Get().Security.CSRFProtection },
	}
}

// CurrentUser resolves the session cookie into a user.
func (s *Server) CurrentUser(r *http.Request) (*auth.User, error) {
	return s.session().CurrentUser(r)
}

// logEvent records an action by the given user, tagged with the client
// address and the request path. It keeps every handler's logging call to one
// line and guarantees the user is always present.
func (s *Server) logEvent(r *http.Request, u *auth.User, event, message string, fields logger.Fields) {
	// The handler's own fields come first so the request context cannot
	// overwrite them, and none of the caller's data is lost: an earlier
	// version replaced the whole map with the request fields, which silently
	// dropped the target of every admin event.
	merged := make(logger.Fields, len(fields)+4)
	for key, value := range fields {
		merged[key] = value
	}
	for key, value := range logger.RequestFields(r) {
		merged[key] = value
	}
	if u != nil {
		merged = logger.UserFields(merged, u.Username)
	}
	s.Log.Info(event, message, merged)
}

// LogAdmin records an administrator action for the api/admin package. The
// acting administrator is resolved from the request; if the session vanished
// between the permission check and the handler it is logged anonymously rather
// than lost.
func (s *Server) LogAdmin(r *http.Request, event, message string, fields logger.Fields) {
	var u *auth.User
	if current, err := s.CurrentUser(r); err == nil {
		u = current
	}
	s.logEvent(r, u, event, message, fields)
}

// itoa is a small helper to avoid importing strconv at call sites.
func itoa(n int64) string { return strconv.FormatInt(n, 10) }

// urlPathEscape percent-encodes s for use in a Content-Disposition filename*.
func urlPathEscape(s string) string { return url.PathEscape(s) }

// clientKey returns a stable key for lockout accounting.
func (s *Server) clientKey(r *http.Request) string {
	addr, ok := s.IPFilter.ClientIP(r)
	if !ok {
		return r.RemoteAddr
	}
	return addr.String()
}

// RequireCSRF validates the double-submit token on state-changing requests.
//
// A failure is logged: a burst of CSRF rejections is a useful signal that
// somebody is trying to drive a logged-in browser, or that the frontend broke.
func (s *Server) RequireCSRF(r *http.Request) error {
	err := s.session().RequireCSRF(r)
	if err != nil {
		s.logEvent(r, nil, logger.EventCSRFFailed, "csrf token rejected", nil)
	}
	return err
}
