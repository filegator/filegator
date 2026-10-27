// Package auth defines what a FileGator user is and what an auth provider must
// do. It holds the shared types - User, roles, permissions, the Provider
// interface and the errors callers match on - and nothing about how any
// particular provider stores accounts.
//
// Implementations live in subpackages: internal/auth/jsonfile for the users
// file and internal/auth/ldap for directory authentication.
package auth

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

// Role is a user role.
type Role string

// Supported roles, matching v7.
const (
	RoleGuest Role = "guest"
	RoleUser  Role = "user"
	RoleAdmin Role = "admin"
)

// Permission is a single capability.
type Permission string

// Supported permissions, matching v7.
const (
	PermRead          Permission = "read"
	PermWrite         Permission = "write"
	PermUpload        Permission = "upload"
	PermDownload      Permission = "download"
	PermBatchDownload Permission = "batchdownload"
	PermZip           Permission = "zip"
	PermChmod         Permission = "chmod"
)

// GuestPermissions are the only permissions a guest account can hold, in v7
// order. The rest are refused by User.has anyway, so offering them would only
// let an administrator configure something that silently does nothing.
var GuestPermissions = []Permission{PermRead, PermDownload, PermBatchDownload}

// IsGuestPermission reports whether a guest may hold p.
func IsGuestPermission(p Permission) bool {
	return slices.Contains(GuestPermissions, p)
}

// AllPermissions lists every known permission in v7 order.
var AllPermissions = []Permission{
	PermRead, PermWrite, PermUpload, PermDownload,
	PermBatchDownload, PermZip, PermChmod,
}

// Errors returned by this package.
var (
	ErrUserNotFound  = errors.New("auth: user not found")
	ErrUsernameTaken = errors.New("auth: username already taken")
	ErrInvalidRole   = errors.New("auth: invalid role")
	ErrInvalidPerm   = errors.New("auth: invalid permission")
	// ErrInvalidPassword reports a failed credential check. Every provider
	// returns it so callers do not need to know how credentials are stored.
	ErrInvalidPassword = errors.New("auth: invalid credentials")
	// ErrUnsupported is returned by providers that cannot perform an
	// operation, such as user management on a read-only directory.
	ErrUnsupported = errors.New("auth: operation not supported by this provider")
)

// User is a FileGator account.
type User struct {
	// Username is the login name.
	Username string
	// Name is the display name.
	Name string
	// Role determines access to admin areas.
	Role Role
	// HomeDir is the user's root path inside the storage, e.g. "/team/alice".
	HomeDir string
	// Permissions are the capabilities granted to this user.
	Permissions []Permission
	// credential is an opaque value each provider chooses: the stored password
	// hash for a users file, or something like an LDAP modify timestamp. It
	// never leaves the server, but it feeds session fingerprints so a
	// credential change logs the user out.
	credential string
}

// Credential returns the provider's opaque credential marker. It is used only
// to build a session fingerprint, never displayed or compared.
func (u *User) Credential() string { return u.credential }

// SetCredential records the provider's opaque credential marker. Providers
// outside this package need it when building a User; it is never exposed to
// HTTP handlers.
func (u *User) SetCredential(v string) { u.credential = v }

// IsGuest reports whether the user is the guest account.
func (u *User) IsGuest() bool { return u.Role == RoleGuest }

// IsAdmin reports whether the user is an administrator.
func (u *User) IsAdmin() bool { return u.Role == RoleAdmin }

// Can reports whether the user holds every permission in want. An empty list
// always returns true, mirroring v7 behavior.
func (u *User) Can(want ...Permission) bool {
	for _, p := range want {
		if !u.has(p) {
			return false
		}
	}
	return true
}

func (u *User) has(p Permission) bool {
	if !slices.Contains(u.Permissions, p) {
		return false
	}
	// Guests are read-only, always. v7 honoured any permission the guest
	// account held, which made anonymous uploads and deletes possible; there
	// is no login behind a guest session, so a CSRF token proves nothing and
	// write access would effectively be unauthenticated.
	if u.IsGuest() {
		return IsGuestPermission(p)
	}
	return true
}

// ValidRole reports whether r is a known role.
func ValidRole(r Role) bool {
	switch r {
	case RoleGuest, RoleUser, RoleAdmin:
		return true
	}
	return false
}

// ValidPermission reports whether p is a known permission.
func ValidPermission(p Permission) bool {
	return slices.Contains(AllPermissions, p)
}

// ParsePermissions decodes a "read|write|zip" string. Empty segments are
// ignored so an empty string yields no permissions.
func ParsePermissions(s string) ([]Permission, error) {
	if strings.TrimSpace(s) == "" {
		return nil, nil
	}
	parts := strings.Split(s, "|")
	out := make([]Permission, 0, len(parts))
	for _, raw := range parts {
		p := Permission(strings.TrimSpace(raw))
		if p == "" {
			continue
		}
		if !ValidPermission(p) {
			return nil, fmt.Errorf("%w: %q", ErrInvalidPerm, p)
		}
		out = append(out, p)
	}
	return out, nil
}

// EncodePermissions turns permissions back into the "read|write" form.
func EncodePermissions(ps []Permission) string {
	parts := make([]string, len(ps))
	for i, p := range ps {
		parts[i] = string(p)
	}
	return strings.Join(parts, "|")
}
