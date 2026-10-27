package sso

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/filegator/filegator/internal/auth"
	"github.com/filegator/filegator/internal/config"
)

// Resolved is the identity a handoff signs in as.
type Resolved struct {
	// User is the account to act as. In "policy" mode it exists only for this
	// session: it is never written to the users file.
	User *auth.User
	// Created reports that an account was added to the users file.
	Created bool
	// Ephemeral reports that the identity is not backed by a stored account,
	// so its home and permissions must be carried in the session.
	Ephemeral bool
}

// Resolve turns a handoff into the identity to sign in as.
//
// users is the configured auth provider; it is used to look names up and, in
// "create" mode, to add them. ensureHome creates the folder for a new account;
// it may be nil when the caller does not create accounts.
func Resolve(app config.SSOApplication, username string, users auth.Provider, ensureHome func(string) error) (*Resolved, error) {
	existing, findErr := users.Find(username)
	if findErr != nil && !errors.Is(findErr, auth.ErrUserNotFound) {
		return nil, findErr
	}

	switch app.Mode {
	case config.SSOModeExisting:
		if existing == nil {
			// The application may only sign in users the administrator
			// created, and may never widen what they can do.
			return nil, ErrUnknownUser
		}
		return &Resolved{User: applyAdminList(existing, app)}, nil

	case config.SSOModePolicy:
		// The application's policy wins even for an existing account: this mode
		// is for handing someone into one folder, not for impersonating
		// accounts it does not own.
		return &Resolved{
			User:      policyUser(app, username, existing),
			Ephemeral: true,
		}, nil

	case config.SSOModeCreate:
		if existing != nil {
			return &Resolved{User: applyAdminList(existing, app)}, nil
		}
		created, err := createUser(app, username, users, ensureHome)
		if err != nil {
			return nil, err
		}
		return &Resolved{User: created, Created: true}, nil

	default:
		return nil, ErrInvalid
	}
}

// policyUser builds the identity described by the application's policy.
//
// A stored record contributes only its display name; home and permissions are
// always the policy's, so an application cannot inherit someone else's access.
func policyUser(app config.SSOApplication, username string, existing *auth.User) *auth.User {
	name := username
	if existing != nil && existing.Name != "" {
		name = existing.Name
	}
	u := &auth.User{
		Username:    username,
		Name:        name,
		Role:        auth.RoleUser,
		HomeDir:     HomeFor(app, username),
		Permissions: permissions(app),
	}
	if slices.Contains(app.AdminUsernames, username) {
		u.Role = auth.RoleAdmin
		// An administrator needs the whole tree; a policy home would confine
		// it to a folder its role says it owns everywhere.
		u.HomeDir = "/"
	}
	return u
}

// applyAdminList promotes a stored user to administrator for this application.
//
// The promotion is applied to a copy: it never reaches the users file.
func applyAdminList(u *auth.User, app config.SSOApplication) *auth.User {
	if !slices.Contains(app.AdminUsernames, u.Username) {
		return u
	}
	copied := *u
	copied.Role = auth.RoleAdmin
	copied.Permissions = slices.Clone(auth.AllPermissions)
	copied.HomeDir = "/"
	return &copied
}

// createUser adds an account described by the application's policy.
func createUser(app config.SSOApplication, username string, users auth.Provider, ensureHome func(string) error) (*auth.User, error) {
	if !users.Capabilities().ManageUsers {
		// A read-only provider such as LDAP cannot store the account, so
		// creating one would only look like it worked.
		return nil, fmt.Errorf("%w: this authentication provider cannot create users", ErrUnknownUser)
	}
	home := HomeFor(app, username)
	candidate := &auth.User{
		Username:    username,
		Name:        username,
		Role:        auth.RoleUser,
		HomeDir:     home,
		Permissions: permissions(app),
	}
	if slices.Contains(app.AdminUsernames, username) {
		candidate.Role = auth.RoleAdmin
		candidate.HomeDir = "/"
	}

	// The folder must exist before the account does, or the first request from
	// the new user is refused for a missing home.
	if candidate.HomeDir != "/" && ensureHome != nil {
		if err := ensureHome(candidate.HomeDir); err != nil {
			return nil, err
		}
	}

	// The account is password-less: it can only be entered through this
	// application, never by typing a password on the login form. A random
	// secret nobody knows satisfies providers that demand one.
	created, err := users.Add(candidate, randomPassword())
	if err != nil {
		return nil, err
	}
	return created, nil
}

// HomeFor expands the application's home template.
func HomeFor(app config.SSOApplication, username string) string {
	home := strings.TrimSpace(app.Home)
	if home == "" {
		return "/"
	}
	// The username becomes a path segment, so a name containing a separator or
	// ".." would climb elsewhere. Such names are encoded instead of rejected,
	// because the remote application may legitimately use them.
	safe := username
	if strings.ContainsAny(username, `/\`) || username == "." || username == ".." ||
		strings.ContainsRune(username, 0) {
		safe = hex.EncodeToString([]byte(username))
	}
	if strings.Contains(home, "%s") {
		return strings.ReplaceAll(home, "%s", safe)
	}
	return home
}

// permissions converts the configured list, defaulting to read-only.
func permissions(app config.SSOApplication) []auth.Permission {
	if len(app.Permissions) == 0 {
		// An application that lists no permissions grants the least: it can
		// look but not change anything.
		return []auth.Permission{auth.PermRead}
	}
	out := make([]auth.Permission, 0, len(app.Permissions))
	for _, raw := range app.Permissions {
		out = append(out, auth.Permission(strings.TrimSpace(raw)))
	}
	return out
}

// randomPassword returns a value no one knows, so the account cannot be
// entered from the login form.
func randomPassword() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		// A predictable password would be worse than none, so fail loudly.
		panic("sso: cannot generate a password: " + err.Error())
	}
	return "sso-" + base64.RawURLEncoding.EncodeToString(b)
}
