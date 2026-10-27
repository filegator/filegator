package admin

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/filegator/filegator/internal/auth"
	"github.com/filegator/filegator/internal/config"
)

// authBody builds a complete auth request from the current state.
func (w *adminTestEnv) authBody(mutate func(*AuthConfig)) string {
	w.t.Helper()
	rec := w.req(http.MethodGet, "/api/admin/auth", "")
	if rec.Code != http.StatusOK {
		w.t.Fatalf("get auth: status %d", rec.Code)
	}
	var envelope struct {
		Data AuthConfig `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		w.t.Fatal(err)
	}
	cfg := envelope.Data
	if mutate != nil {
		mutate(&cfg)
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		w.t.Fatal(err)
	}
	return string(data)
}

func TestGetAuthConfigDefaultsToJSON(t *testing.T) {
	w := adminEnv(t)
	rec := w.req(http.MethodGet, "/api/admin/auth", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	var result struct {
		Data AuthConfig `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Data.Type != config.AuthTypeJSON {
		t.Errorf("type = %q", result.Data.Type)
	}
	if result.Data.UsersFile == "" {
		t.Error("users_file missing")
	}
}

func TestAuthConfigNeverReturnsBindPassword(t *testing.T) {
	w := adminEnv(t)
	if err := w.srv.Config.Update(func(c *config.Config) error {
		c.Auth.LDAP.BindPass = "service-account-secret"
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	rec := w.req(http.MethodGet, "/api/admin/auth", "")
	body := rec.Body.String()
	if containsStr(body, "service-account-secret") {
		t.Errorf("bind password leaked: %s", body)
	}
	if !containsStr(body, `"ldap_has_bind_pass":true`) {
		t.Errorf("has_bind_pass flag missing: %s", body)
	}
}

func TestSetAuthConfigRejectsUnreachableLDAP(t *testing.T) {
	w := adminEnv(t)
	before := w.srv.Users.Capabilities()

	rec := w.submit("/api/admin/auth", w.authBody(func(c *AuthConfig) {
		c.Type = config.AuthTypeLDAP
		c.LDAPServer = "ldap://127.0.0.1:1"
		c.LDAPBaseDN = "dc=example,dc=com"
		c.LDAPUsernameAttribute = "uid"
		c.LDAPTimeout = 1
	}))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d, want 400 (body %s)", rec.Code, rec.Body)
	}
	// The previous provider must still be active.
	if after := w.srv.Users.Capabilities(); after.ManageUsers != before.ManageUsers {
		t.Error("auth provider changed after a failed switch")
	}
}

func TestSetAuthConfigValidatesLDAPFields(t *testing.T) {
	w := adminEnv(t)
	cases := map[string]func(*AuthConfig){
		"missing server": func(c *AuthConfig) {
			c.Type = config.AuthTypeLDAP
			c.LDAPBaseDN = "dc=x"
			c.LDAPUsernameAttribute = "uid"
		},
		"missing base dn": func(c *AuthConfig) {
			c.Type = config.AuthTypeLDAP
			c.LDAPServer = "ldap://h"
			c.LDAPUsernameAttribute = "uid"
		},
		"missing username attribute": func(c *AuthConfig) {
			c.Type = config.AuthTypeLDAP
			c.LDAPServer = "ldap://h"
			c.LDAPBaseDN = "dc=x"
		},
		"unknown type": func(c *AuthConfig) { c.Type = "wordpress" },
	}
	for name, mutate := range cases {
		rec := w.submit("/api/admin/auth", w.authBody(mutate))
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status %d, want 400", name, rec.Code)
		}
	}
}

func TestSetAuthConfigSwitchesBackToJSON(t *testing.T) {
	w := adminEnv(t)
	// Start from a config that claims LDAP but is actually serving JSON; the
	// switch back must work and keep the users file.
	if err := w.srv.Config.Update(func(c *config.Config) error {
		c.Auth.LDAP.BindPass = "stored"
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	rec := w.submit("/api/admin/auth", w.authBody(func(c *AuthConfig) {
		c.Type = config.AuthTypeJSON
	}))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body)
	}
	if got := w.srv.Config.Get().Auth.Type; got != config.AuthTypeJSON {
		t.Errorf("persisted type = %q", got)
	}
	// Login still works with the JSON provider.
	if session := w.login("admin", "admin123"); session == "" {
		t.Error("JSON login broken after switching providers")
	}
}

func TestSetAuthConfigKeepsBindPassWhenBlank(t *testing.T) {
	w := adminEnv(t)
	if err := w.srv.Config.Update(func(c *config.Config) error {
		c.Auth.LDAP.BindPass = "stored"
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	rec := w.submit("/api/admin/auth", w.authBody(func(c *AuthConfig) {
		c.Type = config.AuthTypeLDAP
		c.LDAPServer = "ldap://127.0.0.1:1"
		c.LDAPBaseDN = "dc=x"
		c.LDAPUsernameAttribute = "uid"
		c.LDAPBindPass = ""
		c.LDAPHasBindPass = true
		c.LDAPTimeout = 1
	}))
	// The failure must be the connection, not a missing password.
	if containsStr(rec.Body.String(), "password") {
		t.Errorf("stored bind password was not reused: %s", rec.Body)
	}
}

func TestSetAuthConfigRequiresAdmin(t *testing.T) {
	e := newEnv(t)
	e.addUser("bob", "pw", "/bob", auth.RoleUser, auth.PermRead)
	session := e.login("bob", "pw")

	rec := e.do(http.MethodGet, "/api/admin/auth", "", session)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status %d, want 403", rec.Code)
	}
}
