package api

import (
	"net/http"

	"github.com/filegator/filegator/internal/router"
)

// Capabilities describes what the active auth provider and storage backend
// support. It is public because the flags are not sensitive and the frontend
// uses them to hide unsupported actions.
type Capabilities struct {
	// AuthManageUsers reports whether the admin can add, edit and delete users.
	AuthManageUsers bool `json:"auth_manage_users"`
	// AuthChangePassword reports whether a user can change their own password.
	AuthChangePassword bool `json:"auth_change_password"`
	// AuthLogout reports whether the Log out button is offered. False when an
	// administrator disabled it, which suits an SSO deployment where the
	// identity provider holds the real session.
	AuthLogout bool `json:"auth_logout"`
	// StorageChmod reports whether permission bits can be set.
	StorageChmod bool `json:"storage_chmod"`
	// StorageServerBrowse reports whether the admin can pick a local folder.
	StorageServerBrowse bool `json:"storage_server_browse"`
}

// GetCapabilities handles GET /api/capabilities.
func (s *Server) GetCapabilities(w http.ResponseWriter, r *http.Request, _ router.Params) {
	authCaps := s.Users.Capabilities()
	storageCaps := s.ActiveStorage().Capabilities()
	WriteJSON(w, http.StatusOK, Capabilities{
		AuthManageUsers: authCaps.ManageUsers,
		// Both conditions must hold: the provider must support it and the
		// administrator must have left it enabled.
		AuthChangePassword:  authCaps.ChangePassword && s.PasswordChangeAllowed(),
		AuthLogout:          s.LogoutAllowed(),
		StorageChmod:        storageCaps.Chmod,
		StorageServerBrowse: storageCaps.ServerBrowse,
	})
}

// PasswordChangeAllowed reports whether signed-in users may change their own
// password. The administrator's setting gates this on top of whatever the auth
// provider supports.
func (s *Server) PasswordChangeAllowed() bool {
	return s.Config.Get().Security.AllowPasswordChange
}

// LogoutAllowed reports whether the Log out button is offered and the endpoint
// accepts requests. The administrator's setting decides it.
func (s *Server) LogoutAllowed() bool {
	return s.Config.Get().Security.AllowLogout
}

// CapabilitiesRoutes registers the public capabilities endpoint.
func (s *Server) CapabilitiesRoutes(rt *router.Router) {
	rt.Get("/api/capabilities", s.GetCapabilities)
}
