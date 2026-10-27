package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/filegator/filegator/internal/auth"
	"github.com/filegator/filegator/internal/auth/jsonfile"
)

// newSeedUsers writes the shipped sample as a users file in a temp dir and
// returns the provider for it.
func newSeedUsers(t *testing.T) (*jsonfile.UsersFile, string) {
	t.Helper()
	dir := t.TempDir()
	sample := filepath.Join(dir, "users.json.sample")
	usersPath := filepath.Join(dir, "users.json")

	data, err := os.ReadFile(filepath.Join("..", "..", "private", "users.json.sample"))
	if err != nil {
		t.Fatalf("read the shipped sample: %v", err)
	}
	if err := os.WriteFile(sample, data, 0o644); err != nil {
		t.Fatal(err)
	}
	users := jsonfile.NewUsersFile(usersPath)
	if _, err := users.EnsureFromTemplate(sample); err != nil {
		t.Fatalf("create the users file: %v", err)
	}
	return users, usersPath
}

// TestAdminPasswordSeedAppliesOnce covers the whole point of the variable: it
// sets the first administrator's password, and then stops applying so it can
// stay in a compose file without resetting a password someone chose later.
func TestAdminPasswordSeedAppliesOnce(t *testing.T) {
	users, _ := newSeedUsers(t)
	t.Setenv(adminPasswordEnv, "correct-horse-battery")

	applied, err := applyAdminPasswordSeed(users)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	if !applied {
		t.Fatal("the seed did not apply to a fresh install")
	}

	// The new password signs in and the published default no longer does.
	if _, err := users.Authenticate("admin", "correct-horse-battery"); err != nil {
		t.Fatalf("the seeded password does not authenticate: %v", err)
	}
	if _, err := users.Authenticate("admin", jsonfile.DefaultAdminPassword); err == nil {
		t.Fatal("the default password still authenticates after seeding")
	}
	if users.DefaultAdminPassword() {
		t.Fatal("the provider still reports the default password")
	}

	// A second start with a different value must not reset the password.
	t.Setenv(adminPasswordEnv, "a-completely-different-one")
	applied, err = applyAdminPasswordSeed(users)
	if err != nil {
		t.Fatalf("second seed: %v", err)
	}
	if applied {
		t.Fatal("the seed applied on an install that no longer uses the default password")
	}
	if _, err := users.Authenticate("admin", "correct-horse-battery"); err != nil {
		t.Fatalf("the chosen password was changed by a stale environment variable: %v", err)
	}
	if _, err := users.Authenticate("admin", "a-completely-different-one"); err == nil {
		t.Fatal("a stale environment variable overrode the stored password")
	}
}

// TestAdminPasswordSeedLeavesTheRestAlone: only the password changes. A typo in
// one field of the account would be worse than the default it replaces.
func TestAdminPasswordSeedLeavesTheRestAlone(t *testing.T) {
	users, _ := newSeedUsers(t)
	before, err := users.Find("admin")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv(adminPasswordEnv, "a-long-enough-password")
	if _, err := applyAdminPasswordSeed(users); err != nil {
		t.Fatalf("seed: %v", err)
	}
	after, err := users.Find("admin")
	if err != nil {
		t.Fatal(err)
	}
	if after.Name != before.Name || after.Role != before.Role || after.HomeDir != before.HomeDir {
		t.Errorf("the account changed beyond its password: %+v -> %+v", before, after)
	}
	if len(after.Permissions) != len(before.Permissions) {
		t.Errorf("permissions changed: %v -> %v", before.Permissions, after.Permissions)
	}

	// The guest is not an administrator and must be untouched.
	guest, err := users.Find("guest")
	if err != nil {
		t.Fatal(err)
	}
	if guest.Role != auth.RoleGuest {
		t.Errorf("the guest account was promoted: %q", guest.Role)
	}
}

// TestAdminPasswordSeedRejectsBadValues: a password the administrator expects
// but the server silently replaces would leave them locked out, so a bad value
// is refused loudly instead.
func TestAdminPasswordSeedRejectsBadValues(t *testing.T) {
	cases := map[string]string{
		"too short":     "short",
		"whitespace":    " padded-password ",
		"only spaces":   "        ",
		"empty ignored": "",
	}
	for name, value := range cases {
		t.Run(name, func(t *testing.T) {
			users, _ := newSeedUsers(t)
			t.Setenv(adminPasswordEnv, value)
			applied, err := applyAdminPasswordSeed(users)
			if err != nil {
				// Refused: nothing was written, the default still stands.
				if !users.DefaultAdminPassword() {
					t.Fatal("a refused value still changed the password")
				}
				return
			}
			// An empty value means "not set" and is not an error.
			if applied {
				t.Fatalf("applied %q", value)
			}
			if !users.DefaultAdminPassword() {
				t.Fatal("the password changed without the seed reporting it")
			}
		})
	}
}

// TestAdminPasswordSeedIsOptional: with no variable the install behaves exactly
// as before, default password and all.
func TestAdminPasswordSeedIsOptional(t *testing.T) {
	users, _ := newSeedUsers(t)
	os.Unsetenv(adminPasswordEnv)
	applied, err := applyAdminPasswordSeed(users)
	if err != nil || applied {
		t.Fatalf("applied=%v err=%v with no variable set", applied, err)
	}
	if !users.DefaultAdminPassword() {
		t.Fatal("the default password was changed without being configured")
	}
}

// TestAdminPasswordSeedEndsWithBcrypt checks the stored value is a hash the
// login path accepts — the $2y$ prefix matters for v7 compatibility.
func TestAdminPasswordSeedEndsWithBcrypt(t *testing.T) {
	users, path := newSeedUsers(t)
	t.Setenv(adminPasswordEnv, "a-long-enough-password")
	if _, err := applyAdminPasswordSeed(users); err != nil {
		t.Fatalf("seed: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "a-long-enough-password") {
		t.Fatal("the password was stored in plain text")
	}
	if !strings.Contains(string(data), "$2y$") {
		t.Error("the stored hash is not the $2y$ form v7 uses")
	}
}
