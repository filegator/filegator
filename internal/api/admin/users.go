package admin

import (
	"errors"
	"net/http"

	"github.com/filegator/filegator/internal/api/httpx"
	"strings"

	"github.com/filegator/filegator/internal/auth"
	"github.com/filegator/filegator/internal/logger"
	"github.com/filegator/filegator/internal/router"
	"github.com/filegator/filegator/internal/storage"
)

// AdminUser is the representation of a user returned to administrators. It
// includes the home folder (which regular users do not need to see) and never
// any password material.
type AdminUser struct {
	Username    string   `json:"username"`
	Name        string   `json:"name"`
	Role        string   `json:"role"`
	HomeDir     string   `json:"homedir"`
	Permissions []string `json:"permissions"`
}

// UserRequest is the body for creating or updating a user.
type UserRequest struct {
	Username    string   `json:"username"`
	Name        string   `json:"name"`
	Role        string   `json:"role"`
	HomeDir     string   `json:"homedir"`
	Permissions []string `json:"permissions"`
	// Password is required on create; empty keeps the current password on
	// update.
	Password string `json:"password"`
	// CreateHome creates the home folder if it does not exist. Nested parents
	// are created too.
	CreateHome bool `json:"create_home"`
}

func toAdminUser(u *auth.User) AdminUser {
	perms := make([]string, len(u.Permissions))
	for i, p := range u.Permissions {
		perms[i] = string(p)
	}
	return AdminUser{
		Username:    u.Username,
		Name:        u.Name,
		Role:        string(u.Role),
		HomeDir:     u.HomeDir,
		Permissions: perms,
	}
}

// ListUsers handles GET /api/admin/users.
func (h *Handler) ListUsers(w http.ResponseWriter, r *http.Request, _ router.Params) {
	users, err := h.Core.Users.List()
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	out := make([]AdminUser, 0, len(users))
	for _, u := range users {
		out = append(out, toAdminUser(u))
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

// CreateUser handles POST /api/admin/users.
func (h *Handler) CreateUser(w http.ResponseWriter, r *http.Request, _ router.Params) {
	var req UserRequest
	if err := httpx.DecodeJSON(w, r, &req, httpx.MaxJSONBody); err != nil {
		httpx.WriteError(w, err)
		return
	}
	user, err := h.buildUser(req, true)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	if user == nil {
		httpx.WriteError(w, httpx.Errorf(http.StatusBadRequest, httpx.CodeBadRequest, "password is required"))
		return
	}
	created, err := h.Core.Users.Add(user.value, req.Password)
	if err != nil {
		httpx.WriteError(w, httpx.MapUserError(err))
		return
	}
	h.Core.LogAdmin(r, logger.EventAdminUserCreated, "user created",
		logger.Fields{"target_user": created.Username, "role": created.Role, "homedir": created.HomeDir})
	httpx.WriteJSON(w, http.StatusCreated, toAdminUser(created))
}

// UpdateUser handles POST /api/admin/users/{username}.
func (h *Handler) UpdateUser(w http.ResponseWriter, r *http.Request, params router.Params) {
	oldUsername := params["username"]

	var req UserRequest
	if err := httpx.DecodeJSON(w, r, &req, httpx.MaxJSONBody); err != nil {
		httpx.WriteError(w, err)
		return
	}
	if req.Username == "" {
		req.Username = oldUsername
	}
	built, err := h.buildUser(req, false)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	if built == nil {
		httpx.WriteError(w, httpx.Errorf(http.StatusBadRequest, httpx.CodeBadRequest, "invalid user"))
		return
	}
	// Demoting the last administrator is the same lockout as deleting it: the
	// account remains but can no longer administer anything. Checked here so
	// the two paths cannot drift apart.
	if err := h.keepAnAdmin(oldUsername, string(built.value.Role)); err != nil {
		httpx.WriteError(w, err)
		return
	}
	updated, err := h.Core.Users.Update(oldUsername, built.value, req.Password)
	if err != nil {
		httpx.WriteError(w, httpx.MapUserError(err))
		return
	}
	h.Core.LogAdmin(r, logger.EventAdminUserUpdated, "user updated",
		logger.Fields{
			"target_user":  updated.Username,
			"previous":     oldUsername,
			"password_set": req.Password != "",
		})
	httpx.WriteJSON(w, http.StatusOK, toAdminUser(updated))
}

// DeleteUser handles POST /api/admin/users/{username}/delete.
//
// A dedicated path is used because the project router has no DELETE method.
func (h *Handler) DeleteUser(w http.ResponseWriter, r *http.Request, params router.Params) {
	username := params["username"]

	// The guest account is the switch for public access. Deleting it would
	// silently unpublish the repository and lose its homedir, so the
	// administrator is told to revoke it instead.
	if target, err := h.Core.Users.Find(username); err == nil && target.IsGuest() {
		httpx.WriteError(w, httpx.Errorf(http.StatusConflict, httpx.CodeConflict,
			"the guest account cannot be deleted; clear its permissions to stop guest access"))
		return
	}

	// Removing the last administrator would leave nobody able to sign in and
	// manage the instance.
	if err := h.keepAnAdmin(username, ""); err != nil {
		httpx.WriteError(w, err)
		return
	}

	if err := h.Core.Users.Delete(username); err != nil {
		httpx.WriteError(w, httpx.MapUserError(err))
		return
	}
	h.Core.LogAdmin(r, logger.EventAdminUserDeleted, "user deleted",
		logger.Fields{"target_user": username})
	httpx.WriteNoContent(w)
}

// keepAnAdmin refuses a change that would leave the instance with no
// administrator. It is called before deleting an account and before changing
// one's role, because both are the same problem: the account exists but can no
// longer administer anything.
//
// newRole is the role the account is about to have; "" means the account is
// being removed. An account that is not an administrator today cannot cause
// the problem, so it is always allowed.
//
// The rule is deliberately "at least one", not "not me": an administrator may
// delete or demote themselves as long as someone else can still administer the
// instance. Refusing that would forbid ordinary handover between colleagues.
func (h *Handler) keepAnAdmin(username, newRole string) error {
	if auth.Role(newRole) == auth.RoleAdmin {
		return nil
	}
	target, err := h.Core.Users.Find(username)
	if err != nil || !target.IsAdmin() {
		// Not an administrator (or already gone, which the caller's own
		// operation will report): nothing to protect.
		return nil
	}
	users, err := h.Core.Users.List()
	if err != nil {
		return err
	}
	admins := 0
	for _, u := range users {
		if u.IsAdmin() {
			admins++
		}
	}
	if admins <= 1 {
		if newRole == "" {
			return httpx.Errorf(http.StatusConflict, httpx.CodeConflict, "cannot delete the last administrator")
		}
		return httpx.Errorf(http.StatusConflict, httpx.CodeConflict, "cannot change the role of the last administrator")
	}
	return nil
}

// SetPasswordRequest carries a new password for another user.
type SetPasswordRequest struct {
	Password string `json:"password"`
}

// SetPassword handles POST /api/admin/users/{username}/password.
//
// This is the administrator's override, separate from a user's own password
// change: it works even when self-service changes are disabled, and it does not
// require the old password because the administrator does not know it. It is
// the way someone who cannot sign in is helped.
//
// Only the password is touched. The rest of the account is read and written
// back unchanged, so a password reset cannot quietly alter permissions, the
// role or the home folder.
func (h *Handler) SetPassword(w http.ResponseWriter, r *http.Request, params router.Params) {
	username := params["username"]

	var req SetPasswordRequest
	if err := httpx.DecodeJSON(w, r, &req, httpx.MaxJSONBody); err != nil {
		httpx.WriteError(w, err)
		return
	}
	if strings.TrimSpace(req.Password) == "" {
		httpx.WriteError(w, httpx.Errorf(http.StatusBadRequest, httpx.CodeBadRequest, "password is required"))
		return
	}

	target, err := h.Core.Users.Find(username)
	if err != nil {
		httpx.WriteError(w, httpx.MapUserError(err))
		return
	}
	// The guest has no password to set: it is entered without signing in, and
	// "changing" one would suggest the account can be logged into.
	if target.IsGuest() {
		httpx.WriteError(w, httpx.Errorf(http.StatusConflict, httpx.CodeConflict,
			"the guest account has no password"))
		return
	}

	// Copy so the write-back cannot be an alias of the stored record.
	value := *target
	if _, err := h.Core.Users.Update(username, &value, req.Password); err != nil {
		httpx.WriteError(w, httpx.MapUserError(err))
		return
	}
	h.Core.LogAdmin(r, logger.EventAdminUserUpdated, "user password set by administrator",
		logger.Fields{"target_user": username, "password_set": true})
	httpx.WriteNoContent(w)
}

// builtUser carries the parsed user plus the requested home directory.
type builtUser struct {
	value *auth.User
}

// buildUser validates a request and converts it into a User. The boolean says
// whether a password is required.
func (h *Handler) buildUser(req UserRequest, requirePassword bool) (*builtUser, error) {
	username := strings.TrimSpace(req.Username)
	if username == "" {
		return nil, httpx.Errorf(http.StatusBadRequest, httpx.CodeBadRequest, "username is required")
	}

	role := auth.Role(req.Role)
	if role == "" {
		role = auth.RoleUser
	}
	if !auth.ValidRole(role) {
		return nil, httpx.Errorf(http.StatusBadRequest, httpx.CodeBadRequest, "invalid role")
	}

	perms := make([]auth.Permission, 0, len(req.Permissions))
	for _, raw := range req.Permissions {
		p := auth.Permission(raw)
		if !auth.ValidPermission(p) {
			return nil, httpx.Errorf(http.StatusBadRequest, httpx.CodeBadRequest, "invalid permission: "+raw)
		}
		perms = append(perms, p)
	}

	// A guest can only ever read and download. Refusing the rest here means a
	// client cannot store a permission that would silently do nothing.
	// A guest with no permissions at all is accepted on purpose: that is how
	// the account is left switched off.
	if role == auth.RoleGuest {
		for _, p := range perms {
			if !auth.IsGuestPermission(p) {
				return nil, httpx.Errorf(http.StatusBadRequest, httpx.CodeBadRequest,
					"a guest account cannot have the "+string(p)+" permission")
			}
		}
	}

	home := strings.TrimSpace(req.HomeDir)
	if home == "" {
		home = "/"
	}
	if !strings.HasPrefix(home, "/") {
		home = "/" + home
	}

	// Optionally provision the home folder. Refusing to do this silently is
	// important: a missing home locks the user out.
	root := h.Core.ActiveStorage()
	if root == nil {
		return nil, httpx.Errorf(http.StatusServiceUnavailable, httpx.CodeInternal, "storage is not configured")
	}
	if req.CreateHome {
		if err := createHome(root, home); err != nil {
			return nil, err
		}
	} else if _, err := root.Scope(home); err != nil {
		// Only report this when the folder is genuinely missing; other
		// failures are reported by the next login attempt.
		if errors.Is(err, storage.ErrNotFound) {
			return nil, httpx.Errorf(http.StatusBadRequest, httpx.CodeBadRequest,
				"home folder does not exist; enable create_home or pick an existing folder")
		}
	}

	if requirePassword && req.Password == "" {
		return nil, nil
	}
	return &builtUser{value: &auth.User{
		Username:    username,
		Name:        strings.TrimSpace(req.Name),
		Role:        role,
		HomeDir:     home,
		Permissions: perms,
	}}, nil
}

// createHome creates a home folder inside the storage root. A folder that
// already exists is not an error.
func createHome(root storage.Storage, home string) error {
	err := root.EnsureDir(home)
	if err == nil || errors.Is(err, storage.ErrExists) {
		return nil
	}
	if errors.Is(err, storage.ErrOutsideRoot) || errors.Is(err, storage.ErrInvalidPath) {
		return httpx.Errorf(http.StatusBadRequest, httpx.CodeBadRequest, "invalid home folder")
	}
	return httpx.Errorf(http.StatusBadRequest, httpx.CodeBadRequest, "cannot create the home folder")
}
