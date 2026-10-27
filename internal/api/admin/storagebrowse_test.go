package admin

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/filegator/filegator/internal/auth"
)

func TestBrowseStorageDirListsOnlyDirectories(t *testing.T) {
	w := adminEnv(t)
	root := w.root.Dir()
	for _, dir := range []string{"team", "team/alice", "clients"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "team", "notes.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	rec := w.req(http.MethodGet, "/api/admin/storage/browse?path=/team", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body)
	}

	var result struct {
		Data BrowseResult `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Data.Path != "/team" {
		t.Errorf("path = %q", result.Data.Path)
	}
	if len(result.Data.Directories) != 1 || result.Data.Directories[0].Name != "alice" {
		t.Errorf("directories = %+v", result.Data.Directories)
	}
	// Paths returned are storage-relative, not absolute host paths.
	if result.Data.Directories[0].Path != "/team/alice" {
		t.Errorf("path = %q, want /team/alice", result.Data.Directories[0].Path)
	}
}

// TestBrowseStorageDirCannotLeaveRoot is the important one: the home-folder
// picker must never expose the host filesystem.
func TestBrowseStorageDirCannotLeaveRoot(t *testing.T) {
	w := adminEnv(t)
	for _, p := range []string{"/..", "/../..", "/../etc", "/team/../../etc"} {
		rec := w.req(http.MethodGet, "/api/admin/storage/browse?path="+p, "")
		if rec.Code == http.StatusOK {
			t.Errorf("path %q was accepted", p)
			continue
		}
		if rec.Code != http.StatusForbidden {
			t.Errorf("path %q: status %d, want 403", p, rec.Code)
		}
	}
}

func TestBrowseStorageDirRootStartsAtStorage(t *testing.T) {
	w := adminEnv(t)
	// A folder outside the storage root must not be reachable from "/".
	outside := t.TempDir()
	if err := os.MkdirAll(filepath.Join(outside, "secret"), 0o755); err != nil {
		t.Fatal(err)
	}

	rec := w.req(http.MethodGet, "/api/admin/storage/browse?path=/", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	if containsStr(rec.Body.String(), "secret") {
		t.Error("browse exposed a directory outside the storage root")
	}
}

func TestBrowseStorageDirRequiresAdmin(t *testing.T) {
	e := newEnv(t)
	e.addUser("bob", "pw", "/bob", auth.RoleUser, auth.PermRead)
	session := e.login("bob", "pw")

	rec := e.do(http.MethodGet, "/api/admin/storage/browse?path=/", "", session)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status %d, want 403", rec.Code)
	}
}

func TestBrowseStorageDirMissing(t *testing.T) {
	w := adminEnv(t)
	rec := w.req(http.MethodGet, "/api/admin/storage/browse?path=/nope", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status %d, want 404", rec.Code)
	}
}
