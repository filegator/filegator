package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/filegator/filegator/internal/auth"
)

// readFile reads a file from the repository, relative to the root.
func (e *env) readFile(rel string) string {
	e.t.Helper()
	data, err := os.ReadFile(filepath.Join(e.root.Dir(), filepath.FromSlash(rel)))
	if err != nil {
		e.t.Fatalf("read %s: %v", rel, err)
	}
	return string(data)
}

// guestEnv returns an environment where the guest account may read.
func guestEnv(t *testing.T, perms ...auth.Permission) *env {
	t.Helper()
	e := newEnv(t)
	// users.json.sample already ships a guest account with no permissions;
	// granting it read is all it takes to publish the repository, which is
	// v7's rule.
	guest, err := e.srv.Users.Find("guest")
	if err != nil {
		e.t.Fatalf("no guest in the sample: %v", err)
	}
	guest.HomeDir = "/alice"
	guest.Permissions = perms
	if _, err := e.srv.Users.Update("guest", guest, ""); err != nil {
		e.t.Fatalf("grant guest permissions: %v", err)
	}
	return e
}

// anon issues a request with no session cookie.
func (e *env) anon(method, path, body string) *httptest.ResponseRecorder {
	e.t.Helper()
	return e.do(method, path, body, "")
}

func TestGuestBrowsesWhenItCanRead(t *testing.T) {
	e := guestEnv(t, auth.PermRead, auth.PermDownload)

	rec := e.anon(http.MethodGet, "/api/files?path=/", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body)
	}
	var listing DirListing
	decode(t, rec, &listing)
	if len(listing.Entries) == 0 {
		t.Error("guest saw an empty listing")
	}
}

func TestGuestMeReportsGuestFlag(t *testing.T) {
	e := guestEnv(t, auth.PermRead)

	rec := e.anon(http.MethodGet, "/api/auth/me", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body)
	}
	var user UserResponse
	decode(t, rec, &user)
	if !user.Guest {
		t.Error("guest flag missing, so the UI would show Log out")
	}
	if user.Role != "guest" {
		t.Errorf("role = %q", user.Role)
	}
}

func TestAnonymousRefusedWithoutGuestRead(t *testing.T) {
	// No guest account at all.
	e := newEnv(t)
	if rec := e.anon(http.MethodGet, "/api/files", ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("no guest: status %d, want 401", rec.Code)
	}

	// A guest that exists but may not read.
	e2 := guestEnv(t)
	if rec := e2.anon(http.MethodGet, "/api/files", ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("readless guest: status %d, want 401", rec.Code)
	}
}

func TestGuestIsForcedReadOnly(t *testing.T) {
	// Permissions are deliberately over-granted: the write gate must still
	// hold, because a guest session has no login behind it.
	e := guestEnv(t,
		auth.PermRead, auth.PermDownload, auth.PermWrite,
		auth.PermUpload, auth.PermZip, auth.PermChmod)

	rec := e.anon(http.MethodPost, "/api/files/delete", `{"paths":["/notes.md"]}`)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("delete status %d, want 403", rec.Code)
	}
	rec = e.anon(http.MethodPost, "/api/files/mkdir", `{"parent":"/","name":"new"}`)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("mkdir status %d, want 403", rec.Code)
	}
	rec = e.anon(http.MethodPost, "/api/files/save", `{"path":"/notes.md","content":"x"}`)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("save status %d, want 403", rec.Code)
	}
	rec = e.anon(http.MethodPost, "/api/files/zip", `{"paths":["/notes.md"]}`)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("zip status %d, want 403", rec.Code)
	}

	// The files are untouched.
	if got := e.readFile("alice/notes.md"); got != "notes" {
		t.Errorf("notes.md = %q", got)
	}
}

func TestGuestCanDownload(t *testing.T) {
	e := guestEnv(t, auth.PermRead, auth.PermDownload)

	rec := e.anon(http.MethodGet, "/api/files/download?path=/notes.md", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body)
	}
}

func TestGuestDownloadRefusedWithoutPermission(t *testing.T) {
	// Read allows browsing but not downloading, exactly as for a real user.
	e := guestEnv(t, auth.PermRead)

	rec := e.anon(http.MethodGet, "/api/files/download?path=/notes.md", "")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status %d, want 403", rec.Code)
	}
}

func TestGuestCannotReachAdmin(t *testing.T) {
	// The admin routes are registered by the admin package, which this
	// environment does not wire; the guard itself lives in CurrentUser and is
	// covered by the role check below plus the admin package's own tests.
	e := guestEnv(t, auth.PermRead, auth.PermDownload)

	rec := e.anon(http.MethodGet, "/api/auth/me", "")
	var user UserResponse
	decode(t, rec, &user)
	if user.Role == "admin" {
		t.Fatal("guest is an administrator")
	}

	// A guest is refused the same capability check the admin routes use.
	g, err := e.srv.Users.Find("guest")
	if err != nil {
		t.Fatal(err)
	}
	if g.IsAdmin() {
		t.Error("guest account reports administrator rights")
	}
}

func TestGuestIsConfinedToItsHome(t *testing.T) {
	e := guestEnv(t, auth.PermRead, auth.PermDownload)
	e.write("bob/secret.txt", "bob only")

	for _, path := range []string{"/api/files?path=/bob", "/api/files?path=/../bob", "/api/files/download?path=/bob/secret.txt"} {
		rec := e.anon(http.MethodGet, path, "")
		if rec.Code == http.StatusOK {
			t.Errorf("%s: guest escaped its home", path)
		}
	}
	// A search must not list anything outside it either.
	rec := e.anon(http.MethodGet, "/api/files/search?q=secret", "")
	if rec.Code == http.StatusOK {
		var body SearchResponse
		decode(t, rec, &body)
		if len(body.Results) != 0 {
			t.Errorf("guest search found %+v", body.Results)
		}
	}
}

func TestLoginStillWorksAlongsideGuest(t *testing.T) {
	e := guestEnv(t, auth.PermRead)
	e.addUser("alice", "pw", "/alice", auth.RoleUser,
		auth.PermRead, auth.PermWrite)

	// A real user's session must win over the guest fallback.
	session := e.login("alice", "pw")
	rec := e.do(http.MethodGet, "/api/auth/me", "", session)
	var user UserResponse
	decode(t, rec, &user)
	if user.Guest || user.Username != "alice" {
		t.Fatalf("user = %+v", user)
	}

	// And they may write.
	rec = e.do(http.MethodPost, "/api/files/create", `{"parent":"/","name":"mine.txt"}`, session)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status %d body %s", rec.Code, rec.Body)
	}
}

func TestGuestPermissionsAreClamped(t *testing.T) {
	// The stored record may claim writes; Can must not honour them.
	g := &auth.User{Role: auth.RoleGuest, Permissions: []auth.Permission{
		auth.PermRead, auth.PermDownload, auth.PermWrite, auth.PermUpload,
	}}
	if !g.Can(auth.PermRead) {
		t.Error("guest read refused")
	}
	if g.Can(auth.PermWrite) || g.Can(auth.PermUpload) || g.Can(auth.PermChmod) {
		t.Error("guest write permission honoured")
	}
	// A normal user is unaffected.
	u := &auth.User{Role: auth.RoleUser, Permissions: []auth.Permission{auth.PermWrite}}
	if !u.Can(auth.PermWrite) {
		t.Error("user write refused")
	}
}

func TestGuestPermissionsUseTheStoredPipeFormat(t *testing.T) {
	// v7 stores permissions as "read|write|...". A comma-separated value is
	// not a valid record at all, and the users file refuses to load it — which
	// looks like "guest access is broken" rather than a bad setting, so the
	// parser behaviour is pinned here.
	perms, err := auth.ParsePermissions("read|download")
	if err != nil {
		t.Fatalf("pipe-separated permissions rejected: %v", err)
	}
	if len(perms) != 2 {
		t.Errorf("perms = %v", perms)
	}
	if _, err := auth.ParsePermissions("read,download"); err == nil {
		t.Error("comma-separated permissions accepted; records must use the v7 pipe format")
	}
}

func TestExpiredSessionDoesNotBecomeGuest(t *testing.T) {
	// A user whose session aged out must be sent to the login screen, not
	// silently demoted to guest browsing.
	e := guestEnv(t, auth.PermRead)
	e.addUser("alice", "pw", "/alice", auth.RoleUser, auth.PermRead)
	session := e.login("alice", "pw")

	// Change the password: the store's fingerprint changes, so the cookie no
	// longer matches even though it has not expired.
	alice, _ := e.srv.Users.Find("alice")
	if _, err := e.srv.Users.Update("alice", alice, "different"); err != nil {
		t.Fatal(err)
	}

	rec := e.do(http.MethodGet, "/api/auth/me", "", session)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d, want 401 rather than a guest fallback", rec.Code)
	}
	var body struct {
		Error struct{ Code string } `json:"error"`
	}
	_ = body
	if strings.Contains(rec.Body.String(), `"guest"`) {
		t.Error("stale session was served as the guest")
	}
}

func TestGuestReadWithoutDownloadCannotDownload(t *testing.T) {
	// Decided deliberately: read and download stay independent, so a guest
	// that may list a folder cannot necessarily fetch its contents. An
	// administrator who wants downloads adds the permission.
	e := guestEnv(t, auth.PermRead)

	rec := e.anon(http.MethodGet, "/api/files?path=/", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("listing status %d, want 200", rec.Code)
	}

	rec = e.anon(http.MethodGet, "/api/files/download?path=/notes.md", "")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("download status %d, want 403 without the download permission", rec.Code)
	}
}
