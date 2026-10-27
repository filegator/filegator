package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/filegator/filegator/internal/api/httpx"
	"github.com/filegator/filegator/internal/auth"
	"github.com/filegator/filegator/internal/logger"
	"github.com/filegator/filegator/internal/router"
)

// ChangePasswordRequest changes the caller's own password.
type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// ChangePassword handles POST /api/auth/password. Users may change their own
// password; the current one must be given so a hijacked session cannot take
// the account over.
func (s *Server) ChangePassword(w http.ResponseWriter, r *http.Request, _ router.Params) {
	u, err := s.CurrentUser(r)
	if err != nil {
		WriteError(w, err)
		return
	}
	if !s.PasswordChangeAllowed() {
		// The administrator turned self-service off. Refusing here as well as
		// hiding the button means the rule cannot be bypassed by calling the
		// endpoint directly.
		WriteError(w, Errorf(http.StatusForbidden, CodeForbidden, "changing your own password is disabled"))
		return
	}
	if err := s.RequireCSRF(r); err != nil {
		WriteError(w, err)
		return
	}

	var req ChangePasswordRequest
	if err := DecodeJSON(w, r, &req, maxJSONBody); err != nil {
		WriteError(w, err)
		return
	}
	if strings.TrimSpace(req.NewPassword) == "" {
		WriteError(w, Errorf(http.StatusBadRequest, CodeBadRequest, "new password is required"))
		return
	}

	// Verify the current password against the stored hash.
	if _, err := s.Users.Authenticate(u.Username, req.CurrentPassword); err != nil {
		if errors.Is(err, auth.ErrInvalidPassword) || errors.Is(err, auth.ErrUserNotFound) {
			// A wrong current password on a live session is worth a warning:
			// it is what a stolen cookie being used to take over looks like.
			s.logEvent(r, u, logger.EventPasswordChanged, "password change refused: wrong current password", nil)
			WriteError(w, Errorf(http.StatusForbidden, CodeForbidden, "current password is incorrect"))
			return
		}
		WriteError(w, err)
		return
	}

	updated, err := s.Users.Update(u.Username, u, req.NewPassword)
	if err != nil {
		WriteError(w, httpx.MapUserError(err))
		return
	}

	s.logEvent(r, u, logger.EventPasswordChanged, "own password changed", nil)

	// The session fingerprint covers the password hash, so the existing cookie
	// is now stale. Issue a fresh one so the user is not logged out.
	http.SetCookie(w, s.Sessions.Issue(updated.Username, s.Users.SessionFingerprint(updated)))
	WriteJSON(w, http.StatusOK, toUserResponse(updated))
}
