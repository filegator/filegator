package admin

import (
	"github.com/filegator/filegator/internal/api"

	"net/http"
	"strings"
	"testing"

	"github.com/filegator/filegator/internal/auth"
)

// TestAdminIgnoresHomedir guards the lockout regression: an administrator
// whose homedir is wrong must still reach the whole repository, otherwise
// nobody can repair the configuration.
func TestAdminIgnoresHomedir(t *testing.T) {
	e := newEnv(t)
	e.addUser("root", "pw-root", "/does-not-exist", auth.RoleAdmin,
		auth.PermRead, auth.PermWrite)
	session := e.login("root", "pw-root")

	rec := e.do(http.MethodGet, "/api/files?path=/", "", session)
	if rec.Code != http.StatusOK {
		t.Fatalf("admin with a bad homedir got status %d (body %s)", rec.Code, rec.Body)
	}

	var listing api.DirListing
	decode(t, rec, &listing)
	names := map[string]bool{}
	for _, entry := range listing.Entries {
		names[entry.Name] = true
	}
	for _, want := range []string{"alice", "bob", "top.txt", "secret.txt"} {
		if !names[want] {
			t.Errorf("admin cannot see %q at the storage root", want)
		}
	}
}

func TestAdminSeesRootAfterHomedirChange(t *testing.T) {
	e := newEnv(t)
	e.addUser("root", "pw-root", "/", auth.RoleAdmin, auth.PermRead)
	session := e.login("root", "pw-root")

	// Point the admin somewhere invalid after logging in; access must survive
	// once the session is refreshed.
	u, err := e.srv.Users.Find("root")
	if err != nil {
		t.Fatal(err)
	}
	u.HomeDir = "/gone"
	if _, err := e.srv.Users.Update("root", u, ""); err != nil {
		t.Fatal(err)
	}
	session = e.login("root", "pw-root")

	if rec := e.do(http.MethodGet, "/api/files?path=/", "", session); rec.Code != http.StatusOK {
		t.Fatalf("status %d after homedir change", rec.Code)
	}
}

func TestUserWithMissingHomedirGetsClearError(t *testing.T) {
	e := newEnv(t)
	e.addUser("alice2", "pw", "/missing", auth.RoleUser, auth.PermRead)
	session := e.login("alice2", "pw")

	rec := e.do(http.MethodGet, "/api/files?path=/", "", session)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status %d, want 403", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "home folder does not exist") {
		t.Errorf("error should explain the problem: %s", body)
	}
	if !strings.Contains(body, "administrator") {
		t.Errorf("error should say who can fix it: %s", body)
	}
}

func TestUserWithExistingHomedirWorks(t *testing.T) {
	e := newEnv(t)
	e.addUser("bob2", "pw", "/bob", auth.RoleUser, auth.PermRead)
	session := e.login("bob2", "pw")

	rec := e.do(http.MethodGet, "/api/files?path=/", "", session)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d (body %s)", rec.Code, rec.Body)
	}
	var listing api.DirListing
	decode(t, rec, &listing)
	if len(listing.Entries) != 1 || listing.Entries[0].Name != "secret.txt" {
		t.Errorf("bob should only see his own files, got %+v", listing.Entries)
	}
}
