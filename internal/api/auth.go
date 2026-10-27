package api

import (
	"errors"
	"net/http"

	"github.com/filegator/filegator/internal/auth"
	"github.com/filegator/filegator/internal/logger"
	"github.com/filegator/filegator/internal/router"
)

// LoginRequest is the body of POST /api/auth/login.
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// UserResponse describes the logged-in user. It deliberately omits the
// password hash.
type UserResponse struct {
	Username    string   `json:"username"`
	Name        string   `json:"name"`
	Role        string   `json:"role"`
	HomeDir     string   `json:"homedir"`
	Permissions []string `json:"permissions"`
	// Guest marks an anonymous session, so the frontend shows "Log in"
	// instead of "Log out" and hides the admin button.
	Guest bool `json:"guest"`
}

func toUserResponse(u *auth.User) UserResponse {
	perms := make([]string, len(u.Permissions))
	for i, p := range u.Permissions {
		perms[i] = string(p)
	}
	return UserResponse{
		Username:    u.Username,
		Name:        u.Name,
		Role:        string(u.Role),
		HomeDir:     u.HomeDir,
		Permissions: perms,
		Guest:       u.IsGuest(),
	}
}

// Login handles POST /api/auth/login.
func (s *Server) Login(w http.ResponseWriter, r *http.Request, _ router.Params) {
	key := s.clientKey(r)
	var req LoginRequest
	if err := DecodeJSON(w, r, &req, 64*1024); err != nil {
		WriteError(w, err)
		return
	}
	if req.Username == "" || req.Password == "" {
		WriteError(w, Errorf(http.StatusBadRequest, CodeBadRequest, "username and password are required"))
		return
	}

	// The lockout is checked after the body is read so the log entry can name
	// the account being tried, which is what an administrator investigates.
	if s.Lockout.Blocked(key) {
		s.Log.Warn(logger.EventLoginLocked, "too many login attempts",
			logger.UserFields(logger.RequestFields(r), req.Username))
		WriteError(w, Errorf(http.StatusTooManyRequests, CodeTooManyLogin, "too many attempts, try again later"))
		return
	}

	u, err := s.Users.Authenticate(req.Username, req.Password)
	if err != nil {
		s.Lockout.Fail(key)
		// Both "unknown user" and "wrong password" return the same message so
		// the response does not reveal which usernames exist.
		if errors.Is(err, auth.ErrUserNotFound) || errors.Is(err, auth.ErrInvalidPassword) {
			s.Log.Warn(logger.EventLoginFailed, "login failed",
				logger.UserFields(logger.RequestFields(r), req.Username))
			WriteError(w, Errorf(http.StatusUnauthorized, CodeUnauthorized, "invalid username or password"))
			return
		}
		WriteError(w, err)
		return
	}

	s.Lockout.Reset(key)
	s.logEvent(r, u, logger.EventLogin, "logged in", nil)
	http.SetCookie(w, s.Sessions.Issue(u.Username, s.Users.SessionFingerprint(u)))
	if token, fresh, err := s.CSRF.Token(r); err == nil && fresh {
		http.SetCookie(w, s.CSRF.Cookie(token))
	}
	WriteJSON(w, http.StatusOK, toUserResponse(u))
}

// Logout handles POST /api/auth/logout.
func (s *Server) Logout(w http.ResponseWriter, r *http.Request, _ router.Params) {
	if !s.LogoutAllowed() {
		// The administrator turned logout off (usual with SSO, where the
		// identity provider holds the session). Refusing here as well as
		// hiding the button means the rule cannot be bypassed by calling the
		// endpoint directly.
		WriteError(w, Errorf(http.StatusForbidden, CodeForbidden, "logout is disabled"))
		return
	}
	// The session is resolved only to name the account in the log; a logout
	// with an already-invalid session is still a successful logout.
	if u, err := s.CurrentUser(r); err == nil {
		s.logEvent(r, u, logger.EventLogout, "logged out", nil)
	}
	http.SetCookie(w, s.Sessions.Clear())
	WriteNoContent(w)
}

// Me handles GET /api/auth/me. It returns 401 when nobody is logged in, which
// the frontend uses to decide whether to show the login screen.
func (s *Server) Me(w http.ResponseWriter, r *http.Request, _ router.Params) {
	u, err := s.CurrentUser(r)
	if err != nil {
		if token, fresh, csrfErr := s.CSRF.Token(r); csrfErr == nil && fresh {
			http.SetCookie(w, s.CSRF.Cookie(token))
		}
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, toUserResponse(u))
}

// Routes registers authentication endpoints.
func (s *Server) AuthRoutes(rt *router.Router) {
	rt.Post("/api/auth/login", s.Login)
	rt.Post("/api/auth/logout", s.Logout)
	rt.Post("/api/auth/password", s.ChangePassword)
	rt.Get("/api/auth/me", s.Me)
}
