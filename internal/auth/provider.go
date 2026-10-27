package auth

// Caps describes which operations an auth provider supports. Operations are
// omitted (false) where they are meaningless or unsupported, so callers can
// hide the corresponding UI instead of showing buttons that fail.
type Caps struct {
	// ManageUsers reports whether the admin can add, edit and delete users.
	// False for read-only providers such as LDAP or a WordPress integration.
	ManageUsers bool
	// ChangePassword reports whether a signed-in user can change their own
	// password through the provider.
	ChangePassword bool
}

// Provider is the backend-agnostic auth surface. UsersFile implements it for
// the local JSON file; a database, LDAP or WordPress provider implements it
// instead, so the rest of the application never cares where users come from.
type Provider interface {
	Authenticate(username, password string) (*User, error)
	Find(username string) (*User, error)
	List() ([]*User, error)
	Add(u *User, password string) (*User, error)
	Update(oldUsername string, u *User, password string) (*User, error)
	Delete(username string) error
	Guest() *User

	// SessionFingerprint returns a value that changes whenever the user's
	// session-relevant fields (password, role, home, permissions) change.
	// Providers choose the source: the file provider hashes the stored
	// password; LDAP might use a modify timestamp.
	SessionFingerprint(u *User) string

	// Writable reports whether the backing store accepts writes.
	Writable() bool

	// DefaultAdminPassword reports whether any admin still uses the default.
	DefaultAdminPassword() bool

	// Capabilities reports which operations the provider supports.
	Capabilities() Caps
}
