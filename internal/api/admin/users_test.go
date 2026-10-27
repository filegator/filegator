package admin

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/filegator/filegator/internal/auth"
)

func TestListUsers(t *testing.T) {
	w := adminEnv(t)
	e := w
	e.addUser("alice", "pw", "/alice", auth.RoleUser, auth.PermRead)

	rec := w.req(http.MethodGet, "/api/admin/users", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body)
	}
	var users []AdminUser
	decode(t, rec, &users)

	if len(users) != 4 { // default admin + guest, root, alice
		t.Fatalf("got %d users: %+v", len(users), users)
	}
	for _, u := range users {
		if u.Username == "root" {
			if u.Role != "admin" {
				t.Errorf("role = %q", u.Role)
			}
			if len(u.Permissions) != 2 {
				t.Errorf("permissions = %v", u.Permissions)
			}
		}
	}
}

func TestListUsersOmitsPassword(t *testing.T) {
	w := adminEnv(t)
	rec := w.req(http.MethodGet, "/api/admin/users", "")
	body := rec.Body.String()
	for _, bad := range []string{"password", "$2y$", "$2a$"} {
		if containsStr(body, bad) {
			t.Errorf("user list leaks %q: %s", bad, body)
		}
	}
}

func containsStr(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}

func TestCreateUser(t *testing.T) {
	w := adminEnv(t)
	e := w
	if err := os.MkdirAll(filepath.Join(e.root.Dir(), "team"), 0o755); err != nil {
		t.Fatal(err)
	}

	body := `{"username":"alice","name":"Alice","role":"user","homedir":"/team","permissions":["read","write"],"password":"secret123"}`
	rec := w.submit("/api/admin/users", body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status %d body %s", rec.Code, rec.Body)
	}
	var created AdminUser
	decode(t, rec, &created)
	if created.Username != "alice" || created.HomeDir != "/team" {
		t.Errorf("created = %+v", created)
	}

	// The new user must be able to log in.
	if session := e.login("alice", "secret123"); session == "" {
		t.Error("new user cannot log in")
	}
}

func TestCreateUserWithHomeOnTheFly(t *testing.T) {
	w := adminEnv(t)
	e := w

	body := `{"username":"bob","role":"user","homedir":"/clients/bob","permissions":["read"],"password":"pw","create_home":true}`
	rec := w.submit("/api/admin/users", body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status %d body %s", rec.Code, rec.Body)
	}
	if _, err := os.Stat(filepath.Join(e.root.Dir(), "clients", "bob")); err != nil {
		t.Fatalf("home folder was not created: %v", err)
	}
}

func TestCreateUserMissingHomeRejected(t *testing.T) {
	w := adminEnv(t)
	body := `{"username":"carol","role":"user","homedir":"/nope","permissions":["read"],"password":"pw"}`
	rec := w.submit("/api/admin/users", body)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d, want 400", rec.Code)
	}
	if !containsStr(rec.Body.String(), "home folder does not exist") {
		t.Errorf("unhelpful error: %s", rec.Body)
	}
}

func TestCreateUserDuplicate(t *testing.T) {
	w := adminEnv(t)
	body := `{"username":"root","role":"user","homedir":"/","password":"pw"}`
	rec := w.submit("/api/admin/users", body)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status %d, want 409", rec.Code)
	}
}

func TestCreateUserRequiresPassword(t *testing.T) {
	w := adminEnv(t)
	rec := w.submit("/api/admin/users", `{"username":"dave","role":"user","homedir":"/"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d, want 400", rec.Code)
	}
}

func TestCreateUserInvalidRoleAndPermission(t *testing.T) {
	w := adminEnv(t)
	for _, body := range []string{
		`{"username":"x","role":"root","homedir":"/","password":"pw"}`,
		`{"username":"x","role":"user","homedir":"/","password":"pw","permissions":["teleport"]}`,
		`{"username":"","role":"user","homedir":"/","password":"pw"}`,
	} {
		if rec := w.submit("/api/admin/users", body); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status %d, want 400", body, rec.Code)
		}
	}
}

func TestUpdateUser(t *testing.T) {
	w := adminEnv(t)
	e := w
	e.addUser("alice", "pw", "/alice", auth.RoleUser, auth.PermRead)

	body := `{"name":"Alice A","role":"user","homedir":"/alice","permissions":["read","write"]}`
	rec := w.submit("/api/admin/users/alice", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body)
	}
	var updated AdminUser
	decode(t, rec, &updated)
	if updated.Name != "Alice A" || len(updated.Permissions) != 2 {
		t.Errorf("updated = %+v", updated)
	}

	// The password is unchanged, so the old one still works.
	if session := e.login("alice", "pw"); session == "" {
		t.Error("password changed unexpectedly")
	}
}

func TestUpdateUserChangesPassword(t *testing.T) {
	w := adminEnv(t)
	e := w
	e.addUser("alice", "oldpw", "/alice", auth.RoleUser, auth.PermRead)

	body := `{"role":"user","homedir":"/alice","permissions":["read"],"password":"newpw"}`
	if rec := w.submit("/api/admin/users/alice", body); rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body)
	}

	r := e.do(http.MethodPost, "/api/auth/login", `{"username":"alice","password":"oldpw"}`, "")
	if r.Code != http.StatusUnauthorized {
		t.Error("old password still works")
	}
	if session := e.login("alice", "newpw"); session == "" {
		t.Error("new password rejected")
	}
}

func TestUpdateUserRename(t *testing.T) {
	w := adminEnv(t)
	e := w
	e.addUser("alice", "pw", "/alice", auth.RoleUser, auth.PermRead)

	body := `{"username":"alicia","role":"user","homedir":"/alice","permissions":["read"]}`
	if rec := w.submit("/api/admin/users/alice", body); rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body)
	}
	if _, err := e.srv.Users.Find("alicia"); err != nil {
		t.Errorf("renamed user missing: %v", err)
	}
}

func TestUpdateUserNotFound(t *testing.T) {
	w := adminEnv(t)
	rec := w.submit("/api/admin/users/ghost", `{"role":"user","homedir":"/","permissions":["read"]}`)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status %d, want 404", rec.Code)
	}
}

func TestDeleteUser(t *testing.T) {
	w := adminEnv(t)
	e := w
	e.addUser("alice", "pw", "/alice", auth.RoleUser, auth.PermRead)

	rec := w.submit("/api/admin/users/alice/delete", "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status %d body %s", rec.Code, rec.Body)
	}
	if _, err := e.srv.Users.Find("alice"); err == nil {
		t.Error("user still exists")
	}
}

func TestDeleteLastAdminRefused(t *testing.T) {
	w := adminEnv(t)
	e := w

	// Remove the default admin so "root" is the only administrator left.
	if err := e.srv.Users.Delete("admin"); err != nil {
		t.Fatal(err)
	}

	rec := w.submit("/api/admin/users/root/delete", "")
	if rec.Code != http.StatusConflict {
		t.Fatalf("status %d, want 409", rec.Code)
	}
	if !containsStr(rec.Body.String(), "last administrator") {
		t.Errorf("body = %s", rec.Body)
	}
	if _, err := e.srv.Users.Find("root"); err != nil {
		t.Error("last administrator was deleted")
	}
}

func TestDeleteAdminAllowedWhenAnotherExists(t *testing.T) {
	w := adminEnv(t)
	e := w
	e.addUser("root2", "pw", "/", auth.RoleAdmin, auth.PermRead)

	if rec := w.submit("/api/admin/users/root2/delete", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("status %d body %s", rec.Code, rec.Body)
	}
}

// The same lockout as deleting the last administrator: the account survives
// but can no longer administer anything, and nobody can promote it back. The
// delete path refused this from the start; the edit path did not, which left a
// one-request way to reach an instance with no administrator.
func TestDemoteLastAdminRefused(t *testing.T) {
	w := adminEnv(t)
	if err := w.srv.Users.Delete("admin"); err != nil {
		t.Fatal(err)
	}

	rec := w.submit("/api/admin/users/root",
		`{"username":"root","name":"Root","role":"user","permissions":["read"],"homedir":"/"}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status %d, want 409 (body %s)", rec.Code, rec.Body)
	}
	if !containsStr(rec.Body.String(), "last administrator") {
		t.Errorf("body = %s", rec.Body)
	}

	// And nothing was written: the account is still an administrator.
	root, err := w.srv.Users.Find("root")
	if err != nil {
		t.Fatal(err)
	}
	if !root.IsAdmin() {
		t.Error("the last administrator was demoted anyway")
	}
}

// Demoting one administrator is ordinary when another remains. The rule is
// "at least one admin", not "never demote yourself": refusing this would
// forbid handover between colleagues.
func TestDemoteAdminAllowedWhenAnotherExists(t *testing.T) {
	w := adminEnv(t)
	w.addUser("root2", "pw", "/", auth.RoleAdmin, auth.PermRead)

	rec := w.submit("/api/admin/users/root2",
		`{"username":"root2","role":"user","permissions":["read"],"homedir":"/"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, want 200 (body %s)", rec.Code, rec.Body)
	}
	demoted, err := w.srv.Users.Find("root2")
	if err != nil {
		t.Fatal(err)
	}
	if demoted.IsAdmin() {
		t.Error("the role change did not apply")
	}
}

// Editing an administrator without touching the role must keep working: the
// guard is about the role the request asks for, not about which account it is.
func TestUpdateLastAdminKeepsItsRole(t *testing.T) {
	w := adminEnv(t)
	if err := w.srv.Users.Delete("admin"); err != nil {
		t.Fatal(err)
	}

	rec := w.submit("/api/admin/users/root",
		`{"username":"root","name":"Renamed","role":"admin","permissions":["read"],"homedir":"/"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, want 200 (body %s)", rec.Code, rec.Body)
	}
	root, err := w.srv.Users.Find("root")
	if err != nil {
		t.Fatal(err)
	}
	if root.Name != "Renamed" || !root.IsAdmin() {
		t.Errorf("name = %q admin = %v; the edit should have applied", root.Name, root.IsAdmin())
	}
}

// A normal account is always editable, however many administrators there are,
// and demoting it can never remove the last administrator.
func TestDemoteUserWhenNoAdminIsTargeted(t *testing.T) {
	w := adminEnv(t)
	w.addUser("bob", "pw", "/", auth.RoleAdmin, auth.PermRead)
	w.addUser("carol", "pw", "/", auth.RoleUser, auth.PermRead)

	rec := w.submit("/api/admin/users/carol",
		`{"username":"carol","role":"user","permissions":["read"],"homedir":"/"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, want 200 (body %s)", rec.Code, rec.Body)
	}
}

func TestUserEndpointsRequireAdmin(t *testing.T) {
	e := newEnv(t)
	e.addUser("bob", "pw", "/bob", auth.RoleUser, auth.PermRead, auth.PermWrite)
	session := e.login("bob", "pw")

	for _, path := range []string{"/api/admin/users"} {
		rec := e.do(http.MethodGet, path, "", session)
		if rec.Code != http.StatusForbidden {
			t.Errorf("%s: status %d, want 403", path, rec.Code)
		}
	}
}

func TestCreateUserCannotEscapeRoot(t *testing.T) {
	w := adminEnv(t)
	body := `{"username":"evil","role":"user","homedir":"/../..","permissions":["read"],"password":"pw","create_home":true}`
	rec := w.submit("/api/admin/users", body)
	if rec.Code == http.StatusCreated {
		t.Fatal("created a user with a home outside the root")
	}
}
