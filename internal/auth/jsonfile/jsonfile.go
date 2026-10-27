package jsonfile

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"sync"

	"github.com/filegator/filegator/internal/auth"
	"github.com/filegator/filegator/internal/security"
)

// Record is the on-disk shape of one user in users.json. It mirrors v7.
type Record struct {
	Username    string `json:"username"`
	Name        string `json:"name"`
	Role        string `json:"role"`
	HomeDir     string `json:"homedir"`
	Permissions string `json:"permissions"`
	Password    string `json:"password"`
}

// UsersFile manages a users.json file.
type UsersFile struct {
	mu   sync.RWMutex
	path string
}

// NewUsersFile returns a store backed by path. A missing file yields the
// built-in default accounts; the shipped sample makes this a fallback only.
func NewUsersFile(path string) *UsersFile {
	return &UsersFile{path: path}
}

// EnsureFromTemplate creates the users file from templatePath when the live
// file does not exist yet. It reports whether the file was created. An
// existing file is never modified, so a real deployment's users survive
// restarts and upgrades.
func (f *UsersFile) EnsureFromTemplate(templatePath string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, err := os.Stat(f.path); err == nil {
		return false, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, fmt.Errorf("auth: stat users: %w", err)
	}

	data, err := os.ReadFile(templatePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, os.ErrNotExist
		}
		return false, fmt.Errorf("auth: read template: %w", err)
	}
	// Validate before writing so a broken template does not become the live
	// file: an invalid file would lock everyone out.
	var recs map[string]Record
	if err := json.Unmarshal(data, &recs); err != nil {
		return false, fmt.Errorf("auth: template %s is not valid JSON: %w", templatePath, err)
	}

	if err := os.MkdirAll(filepath.Dir(f.path), 0o755); err != nil {
		return false, fmt.Errorf("auth: create dir: %w", err)
	}
	// 0600: the file holds password hashes.
	if err := os.WriteFile(f.path, data, 0o600); err != nil {
		return false, fmt.Errorf("auth: write users: %w", err)
	}
	return true, nil
}

// DefaultAdminPassword is the password in the shipped users template. It is
// only used to warn administrators that the default is still in place.
// DefaultAdminPassword is the password in the shipped users template. It is
// only used to warn administrators that the default is still in place.
const DefaultAdminPassword = "admin123"

// DefaultAdminPassword reports whether the admin account still uses the
// shipped default password. The check is a real bcrypt comparison, so an
// administrator who deliberately set the same password is also flagged.
func (f *UsersFile) DefaultAdminPassword() bool {
	f.mu.RLock()
	defer f.mu.RUnlock()

	recs, err := f.read()
	if err != nil {
		return false
	}
	for _, r := range recs {
		if r.Role == string(auth.RoleAdmin) && VerifyPassword(DefaultAdminPassword, r.Password) {
			return true
		}
	}
	return false
}

// SessionFingerprint returns a value that changes when the user's password,
// role, home or permissions change, so stale sessions are rejected.
func (f *UsersFile) SessionFingerprint(u *auth.User) string {
	return security.Fingerprint(
		u.Username,
		u.Credential(),
		string(u.Role),
		u.HomeDir,
		auth.EncodePermissions(u.Permissions),
	)
}

// Writable reports whether the users file can be written.
func (f *UsersFile) Writable() bool {
	return f.CheckWritable() == nil
}

// Capabilities reports the operations the JSON file provider supports.
func (f *UsersFile) Capabilities() auth.Caps {
	return auth.Caps{ManageUsers: true, ChangePassword: true}
}

// Path returns the file path.
func (f *UsersFile) Path() string { return f.path }

// DefaultUsers returns the accounts used when users.json is missing. The
// shipped file contains the same accounts, so this is a safety net rather than
// the normal path.
// The admin password is "admin123", as in v7.
func DefaultUsers() map[string]Record {
	return map[string]Record{
		"1": {
			Username:    "admin",
			Name:        "Admin",
			Role:        string(auth.RoleAdmin),
			HomeDir:     "/",
			Permissions: auth.EncodePermissions(auth.AllPermissions),
			Password:    "$2y$10$Nu35w4pteLfc7BDCIkDPkecjw8wsH8Y2GMfIewUbXLT7zzW6WOxwq",
		},
		"2": {
			Username: "guest",
			Name:     "Guest",
			Role:     string(auth.RoleGuest),
			HomeDir:  "/",
			// Guest access is off by default: a fresh install must never
			// publish its storage before an administrator has decided to.
			// Giving the guest read turns it on.
			Permissions: "",
		},
	}
}

// read loads and decodes the file. Callers must hold the lock.
func (f *UsersFile) read() (map[string]Record, error) {
	data, err := os.ReadFile(f.path)
	if errors.Is(err, os.ErrNotExist) {
		return DefaultUsers(), nil
	}
	if err != nil {
		return nil, fmt.Errorf("auth: read users: %w", err)
	}
	var recs map[string]Record
	if err := json.Unmarshal(data, &recs); err != nil {
		return nil, fmt.Errorf("auth: parse users: %w", err)
	}
	return recs, nil
}

// ErrReadOnly is returned when the users file cannot be written. Losing user
// changes silently is unacceptable, so the server refuses to start instead.
var ErrReadOnly = errors.New("auth: users file is read-only")

// CheckWritable verifies that the users file can be written. It is called at
// startup so a read-only volume fails loudly rather than accepting password
// changes that would never persist.
func (f *UsersFile) CheckWritable() error {
	f.mu.RLock()
	dir := filepath.Dir(f.path)
	path := f.path
	f.mu.RUnlock()

	// A missing directory is not a read-only mount: create it first so a
	// fresh install can start.
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("%w: cannot create %s: %v", ErrReadOnly, dir, err)
	}

	probe, err := os.CreateTemp(dir, ".write-probe-*")
	if err != nil {
		return fmt.Errorf("%w: %s is not writable: %v", ErrReadOnly, dir, err)
	}
	name := probe.Name()
	probe.Close()
	os.Remove(name)

	// The file itself must also be replaceable, which fails when it is on a
	// read-only mount even if the directory probe somehow succeeded.
	if _, err := os.Stat(path); err == nil {
		fh, err := os.OpenFile(path, os.O_WRONLY, 0)
		if err != nil {
			return fmt.Errorf("%w: %s is not writable: %v", ErrReadOnly, path, err)
		}
		fh.Close()
	}
	return nil
}

// write persists records atomically. Callers must hold the lock.
func (f *UsersFile) write(recs map[string]Record) error {
	data, err := json.Marshal(recs)
	if err != nil {
		return fmt.Errorf("auth: marshal users: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(f.path), 0o755); err != nil {
		return fmt.Errorf("auth: create dir: %w", err)
	}
	tmp := f.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("auth: write temp: %w", err)
	}
	if err := os.Rename(tmp, f.path); err != nil {
		return fmt.Errorf("auth: rename: %w", err)
	}
	return nil
}

// toUser converts a record into a auth.User.
func toUser(id string, r Record) (*auth.User, error) {
	role := auth.Role(r.Role)
	if !auth.ValidRole(role) {
		return nil, fmt.Errorf("%w: %q", auth.ErrInvalidRole, r.Role)
	}
	perms, err := auth.ParsePermissions(r.Permissions)
	if err != nil {
		return nil, fmt.Errorf("user %q: %w", r.Username, err)
	}
	home := r.HomeDir
	if home == "" {
		home = "/"
	}
	user := &auth.User{
		Username:    r.Username,
		Name:        r.Name,
		Role:        role,
		HomeDir:     home,
		Permissions: perms,
	}
	user.SetCredential(r.Password)
	return user, nil
}

// List returns all users sorted by numeric ID.
func (f *UsersFile) List() ([]*auth.User, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	recs, err := f.read()
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(recs))
	for id := range recs {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		a, errA := strconv.Atoi(ids[i])
		b, errB := strconv.Atoi(ids[j])
		if errA == nil && errB == nil {
			return a < b
		}
		return ids[i] < ids[j]
	})

	users := make([]*auth.User, 0, len(ids))
	for _, id := range ids {
		u, err := toUser(id, recs[id])
		if err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, nil
}

// Find returns the user with the given username, or auth.ErrUserNotFound.
func (f *UsersFile) Find(username string) (*auth.User, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.findLocked(username)
}

func (f *UsersFile) findLocked(username string) (*auth.User, error) {
	recs, err := f.read()
	if err != nil {
		return nil, err
	}
	for id, r := range recs {
		if r.Username == username {
			return toUser(id, r)
		}
	}
	return nil, auth.ErrUserNotFound
}

// Authenticate verifies a username/password pair. It returns auth.ErrUserNotFound
// for unknown users and auth.ErrInvalidPassword for wrong credentials. A record
// whose password is not a bcrypt hash can never authenticate.
func (f *UsersFile) Authenticate(username, password string) (*auth.User, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	recs, err := f.read()
	if err != nil {
		return nil, err
	}
	for id, r := range recs {
		if r.Username != username {
			continue
		}
		if !VerifyPassword(password, r.Password) {
			return nil, auth.ErrInvalidPassword
		}
		return toUser(id, r)
	}
	return nil, auth.ErrUserNotFound
}

// Guest returns the guest account, or nil if none is defined.
func (f *UsersFile) Guest() *auth.User {
	f.mu.RLock()
	defer f.mu.RUnlock()
	recs, err := f.read()
	if err != nil {
		return nil
	}
	for id, r := range recs {
		if r.Role == string(auth.RoleGuest) || r.Username == "guest" {
			u, err := toUser(id, r)
			if err != nil {
				return nil
			}
			return u
		}
	}
	return nil
}

// Save writes recs to disk.
func (f *UsersFile) Save(recs map[string]Record) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.write(recs)
}

// Add creates a new user with a bcrypt-hashed password.
func (f *UsersFile) Add(u *auth.User, password string) (*auth.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := validateUser(u); err != nil {
		return nil, err
	}
	if password == "" {
		return nil, auth.ErrInvalidPassword
	}
	hash, err := HashPassword(password)
	if err != nil {
		return nil, err
	}

	recs, err := f.read()
	if err != nil {
		return nil, err
	}
	for _, r := range recs {
		if r.Username == u.Username {
			return nil, auth.ErrUsernameTaken
		}
	}

	id := nextID(recs)
	recs[id] = Record{
		Username:    u.Username,
		Name:        u.Name,
		Role:        string(u.Role),
		HomeDir:     normaliseHome(u.HomeDir),
		Permissions: auth.EncodePermissions(u.Permissions),
		Password:    toPHPBcrypt(hash),
	}
	if err := f.write(recs); err != nil {
		return nil, err
	}
	return f.findLocked(u.Username)
}

// Update changes an existing user identified by oldUsername. An empty
// password leaves the stored hash unchanged.
func (f *UsersFile) Update(oldUsername string, u *auth.User, password string) (*auth.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := validateUser(u); err != nil {
		return nil, err
	}

	recs, err := f.read()
	if err != nil {
		return nil, err
	}

	var id string
	for rid, r := range recs {
		if r.Username == oldUsername {
			id = rid
		}
		if r.Username == u.Username && rid != id {
			return nil, auth.ErrUsernameTaken
		}
	}
	if id == "" {
		return nil, auth.ErrUserNotFound
	}

	rec := recs[id]
	rec.Username = u.Username
	rec.Name = u.Name
	rec.Role = string(u.Role)
	rec.HomeDir = normaliseHome(u.HomeDir)
	rec.Permissions = auth.EncodePermissions(u.Permissions)

	if password != "" {
		hash, err := HashPassword(password)
		if err != nil {
			return nil, err
		}
		rec.Password = toPHPBcrypt(hash)
	}
	recs[id] = rec

	if err := f.write(recs); err != nil {
		return nil, err
	}
	return f.findLocked(u.Username)
}

// Delete removes the user with the given username.
func (f *UsersFile) Delete(username string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	recs, err := f.read()
	if err != nil {
		return err
	}
	for id, r := range recs {
		if r.Username == username {
			delete(recs, id)
			return f.write(recs)
		}
	}
	return auth.ErrUserNotFound
}

func validateUser(u *auth.User) error {
	if u.Username == "" {
		return errors.New("auth: username must not be empty")
	}
	if !auth.ValidRole(u.Role) {
		return fmt.Errorf("%w: %q", auth.ErrInvalidRole, u.Role)
	}
	for _, p := range u.Permissions {
		if !auth.ValidPermission(p) {
			return fmt.Errorf("%w: %q", auth.ErrInvalidPerm, p)
		}
	}
	return nil
}

func normaliseHome(h string) string {
	if h == "" {
		return "/"
	}
	return h
}

// nextID returns the smallest integer key not in use, as a string.
func nextID(recs map[string]Record) string {
	max := 0
	for id := range recs {
		if n, err := strconv.Atoi(id); err == nil && n > max {
			max = n
		}
	}
	return strconv.Itoa(max + 1)
}

// toPHPBcrypt rewrites a Go bcrypt hash ("$2a$") into the PHP variant
// ("$2y$") so files stay interchangeable with v7.
func toPHPBcrypt(hash string) string {
	if len(hash) > 3 && (hash[:4] == "$2a$" || hash[:4] == "$2b$") {
		return "$2y$" + hash[4:]
	}
	return hash
}

// Ensure UsersFile implements the provider interface at compile time.
var _ auth.Provider = (*UsersFile)(nil)
