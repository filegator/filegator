package admin

import (
	"net/http"
	"strings"
	"testing"

	"github.com/filegator/filegator/internal/config"
)

// TestAdminSetsPasswordForAnyUser checks the administrator's override.
//
// It is the way someone who cannot sign in is helped: an administrator sets a
// new password from Admin -> Users, and that must keep working even when
// self-service password changes are switched off for everybody else. The two
// are deliberately independent — turning users' own password changes off
// removes the profile button, not the administrator's ability to help.
func TestAdminSetsPasswordForAnyUser(t *testing.T) {
	e := adminEnv(t)

	if rec := e.submit("/api/admin/users", `{
		"username":"carol","name":"Carol","role":"user","homedir":"/carol",
		"permissions":["read"],"password":"firstpass1","create_home":true
	}`); !created(rec.Code) {
		t.Fatalf("create status %d body %s", rec.Code, rec.Body)
	}

	// Self-service is off: carol could not have changed this herself.
	if err := e.srv.Config.Update(func(c *config.Config) error {
		c.Security.AllowPasswordChange = false
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if rec := e.submit("/api/admin/users/carol", `{
		"username":"carol","name":"Carol","role":"user","homedir":"/carol",
		"permissions":["read"],"password":"secondpass2"
	}`); rec.Code != http.StatusOK {
		t.Fatalf("update status %d body %s", rec.Code, rec.Body)
	}

	// The new password works...
	if session := e.login("carol", "secondpass2"); session == "" {
		t.Error("administrator's new password was not accepted")
	}
	// ...and the old one does not.
	if rec := e.do(http.MethodPost, "/api/auth/login",
		`{"username":"carol","password":"firstpass1"}`, ""); rec.Code == http.StatusOK {
		t.Error("old password still accepted after the administrator replaced it")
	}

	// Carol herself still cannot change it, because the setting is off.
	session := e.login("carol", "secondpass2")
	rec := e.post("/api/auth/password",
		`{"current_password":"secondpass2","new_password":"thirdpass3"}`, session, "")
	if rec.Code != http.StatusForbidden {
		t.Errorf("self-service status %d, want 403 while disabled", rec.Code)
	}
}

// TestAdminPasswordFieldIsOptional checks that editing a user without the
// password field leaves the password alone, so saving a permission change does
// not silently lock someone out.
func TestAdminPasswordFieldIsOptional(t *testing.T) {
	e := adminEnv(t)

	if rec := e.submit("/api/admin/users", `{
		"username":"dave","name":"Dave","role":"user","homedir":"/dave",
		"permissions":["read"],"password":"keeppass1","create_home":true
	}`); !created(rec.Code) {
		t.Fatalf("create status %d body %s", rec.Code, rec.Body)
	}

	// No "password" key at all: the same shape the interface sends when the
	// field is left blank.
	if rec := e.submit("/api/admin/users/dave", `{
		"username":"dave","name":"Dave","role":"user","homedir":"/dave",
		"permissions":["read","write"]
	}`); rec.Code != http.StatusOK {
		t.Fatalf("update status %d body %s", rec.Code, rec.Body)
	}

	if session := e.login("dave", "keeppass1"); session == "" {
		t.Fatal("password changed by an edit that did not set one")
	}

	// The permission change did apply.
	rec := e.req(http.MethodGet, "/api/admin/users", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("list status %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"username":"dave"`) || !strings.Contains(body, `"write"`) {
		t.Errorf("permissions were not updated: %s", body)
	}
}

// created reports whether a create request succeeded: 201 by convention, but a
// route that answers 200 is not a failure worth stopping a test for.
func created(code int) bool {
	return code == http.StatusOK || code == http.StatusCreated
}

// TestSetPasswordEndpoint checks the per-row action: an administrator sets one
// user's password without touching anything else about the account.
func TestSetPasswordEndpoint(t *testing.T) {
	e := adminEnv(t)

	if rec := e.submit("/api/admin/users", `{
		"username":"erin","name":"Erin","role":"user","homedir":"/erin",
		"permissions":["read"],"password":"firstpass1","create_home":true
	}`); !created(rec.Code) {
		t.Fatalf("create status %d body %s", rec.Code, rec.Body)
	}

	// Self-service is off, so this is the only route that can help her.
	if err := e.srv.Config.Update(func(c *config.Config) error {
		c.Security.AllowPasswordChange = false
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if rec := e.submit("/api/admin/users/erin/password",
		`{"password":"resetpass2"}`); rec.Code != http.StatusNoContent {
		t.Fatalf("set password status %d body %s", rec.Code, rec.Body)
	}

	if session := e.login("erin", "resetpass2"); session == "" {
		t.Fatal("new password was not accepted")
	}
	if rec := e.do(http.MethodPost, "/api/auth/login",
		`{"username":"erin","password":"firstpass1"}`, ""); rec.Code == http.StatusOK {
		t.Error("old password still accepted")
	}

	// Only the password moved: role, home and permissions are untouched.
	rec := e.req(http.MethodGet, "/api/admin/users", "")
	body := rec.Body.String()
	if !strings.Contains(body, `"username":"erin"`) ||
		!strings.Contains(body, `"homedir":"/erin"`) ||
		!strings.Contains(body, `"permissions":["read"]`) {
		t.Errorf("account was altered beyond the password: %s", body)
	}
}

// TestSetPasswordRefusals covers the cases that must not succeed.
func TestSetPasswordRefusals(t *testing.T) {
	e := adminEnv(t)

	t.Run("empty password", func(t *testing.T) {
		rec := e.submit("/api/admin/users/admin/password", `{"password":"   "}`)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("status %d, want 400", rec.Code)
		}
	})

	t.Run("unknown user", func(t *testing.T) {
		rec := e.submit("/api/admin/users/nobody/password", `{"password":"whatever1"}`)
		if rec.Code != http.StatusNotFound {
			t.Errorf("status %d, want 404", rec.Code)
		}
	})

	t.Run("guest has no password", func(t *testing.T) {
		rec := e.submit("/api/admin/users/guest/password", `{"password":"whatever1"}`)
		if rec.Code != http.StatusConflict {
			t.Errorf("status %d, want 409", rec.Code)
		}
	})

	t.Run("non-administrator is refused", func(t *testing.T) {
		if rec := e.submit("/api/admin/users", `{
			"username":"frank","name":"Frank","role":"user","homedir":"/frank",
			"permissions":["read"],"password":"userpass1","create_home":true
		}`); !created(rec.Code) {
			t.Fatalf("create status %d", rec.Code)
		}
		session := e.login("frank", "userpass1")
		rec := e.post("/api/admin/users/admin/password", `{"password":"hijack123"}`, session, "")
		if rec.Code != http.StatusForbidden {
			t.Errorf("status %d, want 403", rec.Code)
		}
	})
}
