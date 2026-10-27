package admin

import (
	"github.com/filegator/filegator/internal/api"

	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/filegator/filegator/internal/auth"
	"github.com/filegator/filegator/internal/security"
)

func TestAdminBrowseListsDirectories(t *testing.T) {
	w := adminEnv(t)
	if err := os.MkdirAll(filepath.Join(w.root.Dir(), "projects", "alpha"), 0o755); err != nil {
		t.Fatal(err)
	}

	rec := w.req(http.MethodGet, "/api/admin/browse?path="+w.root.Dir(), "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body)
	}
	var result struct {
		Data BrowseResult `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, d := range result.Data.Directories {
		names[d.Name] = true
	}
	if !names["projects"] {
		t.Errorf("directories = %+v", result.Data.Directories)
	}
	// Files must never be listed, even though they exist in this folder.
	if names["top.txt"] || names["secret.txt"] {
		t.Error("files appear in the folder picker")
	}
}

func TestAdminBrowseRequiresAdmin(t *testing.T) {
	e := newEnv(t)
	e.addUser("bob", "pw", "/bob", auth.RoleUser, auth.PermRead)
	session := e.login("bob", "pw")

	rec := e.do(http.MethodGet, "/api/admin/browse?path=/", "", session)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status %d, want 403", rec.Code)
	}
}

func TestAdminBrowseRequiresLogin(t *testing.T) {
	e := newEnv(t)
	rec := e.do(http.MethodGet, "/api/admin/browse?path=/", "", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d, want 401", rec.Code)
	}
}

func TestAdminSetRootSwitchesStorage(t *testing.T) {
	w := adminEnv(t)
	other := t.TempDir()
	if err := os.MkdirAll(filepath.Join(other, "outside-data"), 0o755); err != nil {
		t.Fatal(err)
	}

	rec := w.submit("/api/admin/root", `{"path":`+quote(other)+`}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body)
	}

	// The listing must show the new root immediately, without a restart.
	rec = w.req(http.MethodGet, "/api/files?path=/", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("list status %d body %s", rec.Code, rec.Body)
	}
	var listing api.DirListing
	decode(t, rec, &listing)
	if len(listing.Entries) != 1 || listing.Entries[0].Name != "outside-data" {
		t.Errorf("entries = %+v", listing.Entries)
	}

	if got := w.srv.Config.Get().Storage.Root; got != other {
		t.Errorf("config storage root = %q, want %q", got, other)
	}
	if got := w.srv.StorageDescription(); got != other {
		t.Errorf("active root = %q, want %q", got, other)
	}
}

func TestAdminSetRootRejectsFile(t *testing.T) {
	w := adminEnv(t)
	file := filepath.Join(t.TempDir(), "a.txt")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	rec := w.submit("/api/admin/root", `{"path":`+quote(file)+`}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d, want 400", rec.Code)
	}
}

func TestAdminSetRootRejectsMissing(t *testing.T) {
	w := adminEnv(t)
	rec := w.submit("/api/admin/root", `{"path":`+quote(filepath.Join(t.TempDir(), "nope"))+`}`)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status %d, want 404", rec.Code)
	}
}

func TestAdminSetRootRequiresAdmin(t *testing.T) {
	e := newEnv(t)
	e.addUser("bob", "pw", "/bob", auth.RoleUser, auth.PermRead, auth.PermWrite)
	session := e.login("bob", "pw")
	token, _, err := e.srv.CSRF.Token(httptest.NewRequest(http.MethodGet, "/", nil))
	if err != nil {
		t.Fatal(err)
	}

	r := httptest.NewRequest(http.MethodPost, "/api/admin/root", strings.NewReader(`{"path":"/tmp"}`))
	r.RemoteAddr = "127.0.0.1:1234"
	r.AddCookie(&http.Cookie{Name: security.CookieName, Value: session})
	r.AddCookie(&http.Cookie{Name: security.CSRFCookieName, Value: token})
	r.Header.Set(security.CSRFHeader, token)
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, r)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status %d, want 403", rec.Code)
	}
}

func TestAdminGetRoot(t *testing.T) {
	w := adminEnv(t)
	rec := w.req(http.MethodGet, "/api/admin/root", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	var result struct {
		Data RootConfig `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Data.Path != w.root.Dir() {
		t.Errorf("path = %q, want %q", result.Data.Path, w.root.Dir())
	}
}

func TestSetRootValidatesBeforeSwitching(t *testing.T) {
	w := adminEnv(t)
	before := w.srv.StorageDescription()

	rec := w.submit("/api/admin/root", `{"path":"/definitely/not/here"}`)
	if rec.Code == http.StatusOK {
		t.Fatal("invalid path accepted")
	}
	if after := w.srv.StorageDescription(); after != before {
		t.Errorf("root changed to %q after a failed update", after)
	}
}

func TestAdminSetRootRequiresCSRF(t *testing.T) {
	w := adminEnv(t)
	r := httptest.NewRequest(http.MethodPost, "/api/admin/root", strings.NewReader(`{"path":"/tmp"}`))
	r.RemoteAddr = "127.0.0.1:1234"
	r.AddCookie(&http.Cookie{Name: security.CookieName, Value: w.session})
	rec := httptest.NewRecorder()
	w.router.ServeHTTP(rec, r)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status %d, want 403", rec.Code)
	}
}

func quote(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
}
