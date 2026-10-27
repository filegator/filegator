package jsonfile

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/filegator/filegator/internal/auth"
)

func newTestFile(t *testing.T) *UsersFile {
	t.Helper()
	return NewUsersFile(filepath.Join(t.TempDir(), "users.json"))
}

func TestEnsureFromTemplateCreatesFile(t *testing.T) {
	dir := t.TempDir()
	template := filepath.Join(dir, "users.json.sample")
	live := filepath.Join(dir, "users.json")

	contents := `{"1":{"username":"admin","name":"Admin","role":"admin","homedir":"/","permissions":"read","password":"$2y$10$Nu35w4pteLfc7BDCIkDPkecjw8wsH8Y2GMfIewUbXLT7zzW6WOxwq"}}`
	if err := os.WriteFile(template, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}

	f := NewUsersFile(live)
	created, err := f.EnsureFromTemplate(template)
	if err != nil {
		t.Fatalf("EnsureFromTemplate: %v", err)
	}
	if !created {
		t.Fatal("expected the file to be created")
	}

	data, err := os.ReadFile(live)
	if err != nil {
		t.Fatalf("live file missing: %v", err)
	}
	if string(data) != contents {
		t.Errorf("contents differ from the template")
	}

	info, err := os.Stat(live)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm&0o077 != 0 {
		t.Errorf("mode = %o, want no group/world access", perm)
	}
}

func TestEnsureFromTemplateNeverOverwrites(t *testing.T) {
	dir := t.TempDir()
	template := filepath.Join(dir, "users.json.sample")
	live := filepath.Join(dir, "users.json")

	if err := os.WriteFile(template, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	existing := `{"1":{"username":"real","role":"admin","homedir":"/","permissions":"","password":""}}`
	if err := os.WriteFile(live, []byte(existing), 0o600); err != nil {
		t.Fatal(err)
	}

	f := NewUsersFile(live)
	created, err := f.EnsureFromTemplate(template)
	if err != nil {
		t.Fatal(err)
	}
	if created {
		t.Error("existing file must not be replaced")
	}
	data, _ := os.ReadFile(live)
	if string(data) != existing {
		t.Error("existing file was modified")
	}
}

func TestEnsureFromTemplateMissingTemplate(t *testing.T) {
	dir := t.TempDir()
	f := NewUsersFile(filepath.Join(dir, "users.json"))
	created, err := f.EnsureFromTemplate(filepath.Join(dir, "missing.sample"))
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("got %v, want os.ErrNotExist", err)
	}
	if created {
		t.Error("nothing should be created")
	}
	// The store still works on built-in defaults.
	if _, err := f.Find("admin"); err != nil {
		t.Errorf("built-in defaults unavailable: %v", err)
	}
}

func TestEnsureFromTemplateRejectsInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	template := filepath.Join(dir, "users.json.sample")
	live := filepath.Join(dir, "users.json")
	if err := os.WriteFile(template, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := NewUsersFile(live).EnsureFromTemplate(template); err == nil {
		t.Fatal("expected an error for an invalid template")
	}
	if _, err := os.Stat(live); err == nil {
		t.Error("an invalid template must not become the live file")
	}
}

func TestEnsureFromTemplateUnwritableDir(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("cannot test permission errors as root")
	}
	dir := t.TempDir()
	template := filepath.Join(dir, "users.json.sample")
	if err := os.WriteFile(template, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	locked := filepath.Join(dir, "locked")
	if err := os.Mkdir(locked, 0o500); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(locked, 0o700)

	f := NewUsersFile(filepath.Join(locked, "users.json"))
	if _, err := f.EnsureFromTemplate(template); err == nil {
		t.Error("expected an error for an unwritable directory")
	}
}

func TestHasDefaultAdminPassword(t *testing.T) {
	f := newTestFile(t) // built-in defaults use admin123
	if !f.DefaultAdminPassword() {
		t.Error("shipped default password not detected")
	}

	u, err := f.Find("admin")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Update("admin", u, "a-strong-password"); err != nil {
		t.Fatal(err)
	}
	if f.DefaultAdminPassword() {
		t.Error("changed password still flagged as default")
	}
}

func TestHasDefaultAdminPasswordIgnoresNonAdmins(t *testing.T) {
	f := newTestFile(t)
	// A regular user with the default password must not trigger the warning.
	if _, err := f.Add(&auth.User{Username: "normal", Role: auth.RoleUser}, DefaultAdminPassword); err != nil {
		t.Fatal(err)
	}
	u, _ := f.Find("admin")
	if _, err := f.Update("admin", u, "changed"); err != nil {
		t.Fatal(err)
	}
	if f.DefaultAdminPassword() {
		t.Error("a non-admin with the default password must not be flagged")
	}
}

func TestMissingFileYieldsDefaults(t *testing.T) {
	f := newTestFile(t)
	users, err := f.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(users) != 2 {
		t.Fatalf("got %d users, want 2", len(users))
	}
	if users[0].Username != "admin" || users[0].Role != auth.RoleAdmin {
		t.Errorf("first user = %+v", users[0])
	}
	if users[1].Username != "guest" || users[1].Role != auth.RoleGuest {
		t.Errorf("second user = %+v", users[1])
	}
}

func TestDefaultAdminLogsIn(t *testing.T) {
	f := newTestFile(t)
	u, err := f.Authenticate("admin", "admin123")
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	if !u.IsAdmin() {
		t.Error("admin should have admin role")
	}
	if !u.Can(auth.PermRead, auth.PermWrite, auth.PermZip) {
		t.Error("admin should have all default permissions")
	}
}

func TestReadsV7File(t *testing.T) {
	path := filepath.Join(t.TempDir(), "users.json")
	// Verbatim v7 users.json.blank.
	data := `{"1":{"username":"admin","name":"Admin","role":"admin","homedir":"\/","permissions":"read|write|upload|download|batchdownload|zip|chmod","password":"$2y$10$Nu35w4pteLfc7BDCIkDPkecjw8wsH8Y2GMfIewUbXLT7zzW6WOxwq"},"2":{"username":"guest","name":"Guest","role":"guest","homedir":"\/","permissions":"","password":""}}`
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}

	f := NewUsersFile(path)
	u, err := f.Authenticate("admin", "admin123")
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	if u.HomeDir != "/" {
		t.Errorf("HomeDir = %q", u.HomeDir)
	}
	if len(u.Permissions) != 7 {
		t.Errorf("Permissions = %v", u.Permissions)
	}

	guest := f.Guest()
	if guest == nil || guest.Username != "guest" {
		t.Fatalf("guest = %+v", guest)
	}
}

func TestAuthenticateUnknownUser(t *testing.T) {
	f := newTestFile(t)
	if _, err := f.Authenticate("nobody", "x"); !errors.Is(err, auth.ErrUserNotFound) {
		t.Fatalf("got %v", err)
	}
}

func TestAuthenticateWrongPassword(t *testing.T) {
	f := newTestFile(t)
	if _, err := f.Authenticate("admin", "wrong"); !errors.Is(err, auth.ErrInvalidPassword) {
		t.Fatalf("got %v", err)
	}
}

func TestAuthenticatePlainTextRecordFails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "users.json")
	data := `{"1":{"username":"bob","name":"Bob","role":"user","homedir":"/","permissions":"read","password":"hunter2"}}`
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	f := NewUsersFile(path)
	if _, err := f.Authenticate("bob", "hunter2"); !errors.Is(err, auth.ErrInvalidPassword) {
		t.Fatalf("plain-text password must not authenticate, got %v", err)
	}
}

func TestAddWritesCombinedFile(t *testing.T) {
	f := newTestFile(t)
	_, err := f.Add(&auth.User{
		Username:    "alice",
		Name:        "Alice",
		Role:        auth.RoleUser,
		HomeDir:     "/home/alice",
		Permissions: []auth.Permission{auth.PermRead, auth.PermWrite},
	}, "secret123")
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	data, err := os.ReadFile(f.Path())
	if err != nil {
		t.Fatal(err)
	}
	var recs map[string]Record
	if err := json.Unmarshal(data, &recs); err != nil {
		t.Fatalf("written file is not the v7 map shape: %v", err)
	}
	rec, ok := recs["3"]
	if !ok {
		t.Fatalf("record 3 missing: %s", data)
	}
	if rec.Username != "alice" || rec.Permissions != "read|write" || rec.HomeDir != "/home/alice" {
		t.Errorf("record = %+v", rec)
	}
	if !strings.HasPrefix(rec.Password, "$2y$") {
		t.Errorf("password not stored as PHP bcrypt: %q", rec.Password)
	}

	if _, err := f.Authenticate("alice", "secret123"); err != nil {
		t.Errorf("new user cannot log in: %v", err)
	}
}

func TestAddDuplicateUsername(t *testing.T) {
	f := newTestFile(t)
	_, err := f.Add(&auth.User{Username: "admin", Role: auth.RoleUser}, "x")
	if !errors.Is(err, auth.ErrUsernameTaken) {
		t.Fatalf("got %v", err)
	}
}

func TestAddRequiresBcryptCapablePassword(t *testing.T) {
	f := newTestFile(t)
	if _, err := f.Add(&auth.User{Username: "eve", Role: auth.RoleUser}, ""); !errors.Is(err, auth.ErrInvalidPassword) {
		t.Fatalf("got %v", err)
	}
	if _, err := f.Add(&auth.User{Username: "eve", Role: "root"}, "pw"); !errors.Is(err, auth.ErrInvalidRole) {
		t.Fatalf("got %v", err)
	}
}

func TestUpdateKeepsPasswordWhenEmpty(t *testing.T) {
	f := newTestFile(t)
	u, err := f.Find("admin")
	if err != nil {
		t.Fatal(err)
	}
	u.Name = "Renamed"
	if _, err := f.Update("admin", u, ""); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if _, err := f.Authenticate("admin", "admin123"); err != nil {
		t.Errorf("password changed unexpectedly: %v", err)
	}
	got, _ := f.Find("admin")
	if got.Name != "Renamed" {
		t.Errorf("Name = %q", got.Name)
	}
}

func TestUpdateChangesPassword(t *testing.T) {
	f := newTestFile(t)
	u, err := f.Find("admin")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Update("admin", u, "newpassword"); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if _, err := f.Authenticate("admin", "admin123"); err == nil {
		t.Error("old password still works")
	}
	if _, err := f.Authenticate("admin", "newpassword"); err != nil {
		t.Errorf("new password rejected: %v", err)
	}
}

func TestUpdateRenameAndCollision(t *testing.T) {
	f := newTestFile(t)
	u, _ := f.Find("admin")
	u.Username = "guest"
	if _, err := f.Update("admin", u, ""); !errors.Is(err, auth.ErrUsernameTaken) {
		t.Fatalf("got %v", err)
	}
	u.Username = "administrator"
	if _, err := f.Update("admin", u, ""); err != nil {
		t.Fatalf("rename: %v", err)
	}
	if _, err := f.Find("administrator"); err != nil {
		t.Errorf("renamed user missing: %v", err)
	}
}

func TestUpdateUnknownUser(t *testing.T) {
	f := newTestFile(t)
	if _, err := f.Update("ghost", &auth.User{Username: "ghost", Role: auth.RoleUser}, ""); !errors.Is(err, auth.ErrUserNotFound) {
		t.Fatalf("got %v", err)
	}
}

func TestDelete(t *testing.T) {
	f := newTestFile(t)
	if err := f.Delete("guest"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := f.Find("guest"); !errors.Is(err, auth.ErrUserNotFound) {
		t.Fatalf("got %v", err)
	}
	if err := f.Delete("guest"); !errors.Is(err, auth.ErrUserNotFound) {
		t.Fatalf("got %v", err)
	}
}

func TestListSortsNumerically(t *testing.T) {
	f := newTestFile(t)
	for _, name := range []string{"b", "c", "d"} {
		if _, err := f.Add(&auth.User{Username: name, Role: auth.RoleUser}, "pw"); err != nil {
			t.Fatal(err)
		}
	}
	users, err := f.List()
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"admin", "guest", "b", "c", "d"}
	if len(users) != len(want) {
		t.Fatalf("got %d users", len(users))
	}
	for i := range want {
		if users[i].Username != want[i] {
			t.Errorf("users[%d] = %q, want %q", i, users[i].Username, want[i])
		}
	}
}

func TestInvalidRoleRecordFails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "users.json")
	data := `{"1":{"username":"x","role":"root","homedir":"/","permissions":"","password":""}}`
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewUsersFile(path).List(); !errors.Is(err, auth.ErrInvalidRole) {
		t.Fatalf("got %v", err)
	}
}

func TestSaveFileIsNotWorldReadable(t *testing.T) {
	f := newTestFile(t)
	if _, err := f.Add(&auth.User{Username: "permy", Role: auth.RoleUser}, "pw"); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(f.Path())
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm&0o077 != 0 {
		t.Errorf("users.json permissions = %o, should not be group/world accessible", perm)
	}
}
