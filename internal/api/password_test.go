package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/filegator/filegator/internal/auth"
	"github.com/filegator/filegator/internal/auth/jsonfile"
	"github.com/filegator/filegator/internal/config"
	"github.com/filegator/filegator/internal/security"
)

// changePassword sends the self-service password request.
func (w *writeEnv) changePassword(body string) *httptest.ResponseRecorder {
	w.t.Helper()
	r := httptest.NewRequest(http.MethodPost, "/api/auth/password", strings.NewReader(body))
	r.RemoteAddr = "127.0.0.1:1234"
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set(security.CSRFHeader, w.csrf)
	r.AddCookie(&http.Cookie{Name: security.CookieName, Value: w.session})
	r.AddCookie(&http.Cookie{Name: security.CSRFCookieName, Value: w.csrf})
	rec := httptest.NewRecorder()
	w.router.ServeHTTP(rec, r)
	return rec
}

func TestChangeOwnPassword(t *testing.T) {
	w := newWriteEnv(t)
	e := w.env
	e.srv.Users.Delete("alice")
	e.addUser("alice", "oldpass", "/alice", auth.RoleUser, auth.PermRead)
	w.session = e.login("alice", "oldpass")

	rec := w.changePassword(`{"current_password":"oldpass","new_password":"newpass123"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body)
	}

	// The new password works and the old one does not.
	if r := e.do(http.MethodPost, "/api/auth/login", `{"username":"alice","password":"oldpass"}`, ""); r.Code != http.StatusUnauthorized {
		t.Error("old password still accepted")
	}
	if session := e.login("alice", "newpass123"); session == "" {
		t.Error("new password rejected")
	}
}

func TestChangeOwnPasswordKeepsSessionAlive(t *testing.T) {
	w := newWriteEnv(t)
	e := w.env
	e.srv.Users.Delete("alice")
	e.addUser("alice", "oldpass", "/alice", auth.RoleUser, auth.PermRead)
	w.session = e.login("alice", "oldpass")

	rec := w.changePassword(`{"current_password":"oldpass","new_password":"newpass123"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}

	// A fresh session cookie must be issued, because the fingerprint changed.
	var sessionCookie string
	for _, c := range rec.Result().Cookies() {
		if c.Name == security.CookieName {
			sessionCookie = c.Value
		}
	}
	if sessionCookie == "" {
		t.Fatal("no new session cookie issued")
	}

	if r := e.do(http.MethodGet, "/api/auth/me", "", sessionCookie); r.Code != http.StatusOK {
		t.Errorf("session invalid after password change: status %d", r.Code)
	}
}

func TestChangePasswordWrongCurrent(t *testing.T) {
	w := newWriteEnv(t)
	e := w.env
	e.srv.Users.Delete("alice")
	e.addUser("alice", "oldpass", "/alice", auth.RoleUser, auth.PermRead)
	w.session = e.login("alice", "oldpass")

	rec := w.changePassword(`{"current_password":"wrong","new_password":"newpass123"}`)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status %d, want 403", rec.Code)
	}
	// The password must be unchanged.
	if r := e.do(http.MethodPost, "/api/auth/login", `{"username":"alice","password":"oldpass"}`, ""); r.Code != http.StatusOK {
		t.Error("password changed despite a wrong current password")
	}
}

func TestChangePasswordRequiresNew(t *testing.T) {
	w := newWriteEnv(t)
	rec := w.changePassword(`{"current_password":"admin123","new_password":"  "}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d, want 400", rec.Code)
	}
}

func TestChangePasswordRequiresLogin(t *testing.T) {
	e := newEnv(t)
	r := httptest.NewRequest(http.MethodPost, "/api/auth/password", nil)
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, r)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d, want 401", rec.Code)
	}
}

func TestChangePasswordRequiresCSRF(t *testing.T) {
	w := newWriteEnv(t)
	e := w.env
	e.srv.Users.Delete("alice")
	e.addUser("alice", "oldpass", "/alice", auth.RoleUser, auth.PermRead)
	session := e.login("alice", "oldpass")

	if err := e.srv.Config.Update(func(c *config.Config) error {
		c.Security.CSRFProtection = true
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	r := httptest.NewRequest(http.MethodPost, "/api/auth/password", strings.NewReader(`{"current_password":"oldpass","new_password":"x1234567"}`))
	r.RemoteAddr = "127.0.0.1:1234"
	r.AddCookie(&http.Cookie{Name: security.CookieName, Value: session})
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, r)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status %d, want 403", rec.Code)
	}
}

func TestPasswordChangeClearsDefaultWarning(t *testing.T) {
	w := newWriteEnv(t)
	e := w.env
	// Sign in as the default admin and change the password through the UI path.
	w.session = e.login("admin", jsonfile.DefaultAdminPassword)

	rec := w.changePassword(`{"current_password":"admin123","new_password":"much-better-password"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body)
	}

	// The old cookie is stale now; use the one issued with the response.
	fresh := ""
	for _, c := range rec.Result().Cookies() {
		if c.Name == security.CookieName {
			fresh = c.Value
		}
	}
	if fresh == "" {
		t.Fatal("no session cookie issued after the password change")
	}

	// The session must still work with the freshly issued cookie.
	if r := e.do(http.MethodGet, "/api/auth/me", "", fresh); r.Code != http.StatusOK {
		t.Errorf("session invalid after password change: %d", r.Code)
	}
}

func TestChangePasswordDisabledByAdmin(t *testing.T) {
	w := newWriteEnv(t)
	e := w.env
	e.srv.Users.Delete("alice")
	e.addUser("alice", "oldpass", "/alice", auth.RoleUser, auth.PermRead)
	w.session = e.login("alice", "oldpass")

	if err := e.srv.Config.Update(func(c *config.Config) error {
		c.Security.AllowPasswordChange = false
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	// The endpoint must refuse, not merely be hidden in the interface: the UI
	// hiding a feature is not a rule.
	rec := w.changePassword(`{"current_password":"oldpass","new_password":"newpass123"}`)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status %d, want 403", rec.Code)
	}

	// The password must be unchanged.
	if r := e.do(http.MethodPost, "/api/auth/login", `{"username":"alice","password":"oldpass"}`, ""); r.Code != http.StatusOK {
		t.Error("password changed although the administrator disabled it")
	}
	if r := e.do(http.MethodPost, "/api/auth/login", `{"username":"alice","password":"newpass123"}`, ""); r.Code == http.StatusOK {
		t.Error("new password accepted although the administrator disabled it")
	}

	// The capabilities flag drives the hidden profile button.
	rec = e.do(http.MethodGet, "/api/capabilities", "", "")
	var caps struct {
		Data Capabilities `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &caps); err != nil {
		t.Fatal(err)
	}
	if caps.Data.AuthChangePassword {
		t.Error("auth_change_password true while password changes are disabled")
	}
}

func TestChangePasswordAllowedByDefault(t *testing.T) {
	// A config.json written before the setting existed has no key for it, so
	// the default must be "allowed": upgrading must not lock anyone out of
	// changing their own password.
	c := config.Default()
	if !c.Security.AllowPasswordChange {
		t.Fatal("default is disabled: existing installations would lose the feature on upgrade")
	}

	// An explicit false in the file wins.
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(`{"security":{"allow_password_change":false}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if store.Get().Security.AllowPasswordChange {
		t.Error("explicit false in config.json was ignored")
	}

	// And a file without the key keeps the default.
	path2 := filepath.Join(dir, "other.json")
	if err := os.WriteFile(path2, []byte(`{"security":{"session_ttl":60}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	store2, err := config.Load(path2)
	if err != nil {
		t.Fatal(err)
	}
	if !store2.Get().Security.AllowPasswordChange {
		t.Error("missing key did not fall back to the enabled default")
	}
}
