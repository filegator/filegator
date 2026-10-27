package admin

import (
	"encoding/json"
	"net/http"
	"os"
	"testing"

	"github.com/filegator/filegator/internal/auth"
	"github.com/filegator/filegator/internal/auth/jsonfile"
)

func TestAdminStatusReportsDefaultPassword(t *testing.T) {
	w := adminEnv(t)
	// The default admin account still uses admin123.
	rec := w.req(http.MethodGet, "/api/admin/status", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body)
	}

	var result struct {
		Data SystemStatus `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if !result.Data.DefaultAdminPassword {
		t.Error("default password not reported (the admin UI banner depends on it)")
	}
	if result.Data.StorageRoot == "" {
		t.Error("storage root not reported")
	}
	if result.Data.ConfigReadOnly {
		t.Error("config should be writable in tests")
	}
}

func TestAdminStatusClearsAfterPasswordChange(t *testing.T) {
	w := adminEnv(t)
	e := w

	u, err := e.srv.Users.Find("admin")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.srv.Users.Update("admin", u, "a-strong-password"); err != nil {
		t.Fatal(err)
	}

	var result struct {
		Data SystemStatus `json:"data"`
	}
	rec := w.req(http.MethodGet, "/api/admin/status", "")
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Data.DefaultAdminPassword {
		t.Error("warning should disappear once the password is changed")
	}
}

func TestAdminStatusRequiresAdmin(t *testing.T) {
	e := newEnv(t)
	e.addUser("bob", "pw", "/bob", auth.RoleUser, auth.PermRead)
	session := e.login("bob", "pw")

	rec := e.do(http.MethodGet, "/api/admin/status", "", session)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status %d, want 403", rec.Code)
	}
}

func TestAdminStatusDetectsReadOnlyUsersFile(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission checks do not apply to root")
	}
	w := adminEnv(t)
	path := w.srv.Users.(*jsonfile.UsersFile).Path()
	if err := os.Chmod(path, 0o400); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(path, 0o600) })

	var result struct {
		Data SystemStatus `json:"data"`
	}
	rec := w.req(http.MethodGet, "/api/admin/status", "")
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if !result.Data.UsersFileReadOnly {
		t.Error("read-only users file not reported")
	}
}
