package admin

import (
	"net/http"
	"testing"

	"github.com/filegator/filegator/internal/auth"
	"github.com/filegator/filegator/internal/storage"
)

// plainStorage hides the optional space reporter, the way an S3 or SFTP
// backend does.
type plainStorage struct{ storage.Storage }

func TestStorageSpaceReportsLocalDisk(t *testing.T) {
	w := adminEnv(t)
	rec := w.req(http.MethodGet, "/api/admin/storage/space", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body)
	}
	var info SpaceInfo
	decode(t, rec, &info)
	if !info.Supported {
		t.Fatal("local storage did not report free space")
	}
	if info.Total == 0 || info.Free > info.Total {
		t.Errorf("free/total = %d/%d", info.Free, info.Total)
	}
	if info.Percent < 0 || info.Percent > 100 {
		t.Errorf("percent = %f", info.Percent)
	}
}

func TestStorageSpaceUnsupportedBackend(t *testing.T) {
	w := adminEnv(t)
	w.srv.SetRoot(plainStorage{Storage: w.root}, w.root.Dir())
	rec := w.req(http.MethodGet, "/api/admin/storage/space", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body)
	}
	var info SpaceInfo
	decode(t, rec, &info)
	if info.Supported {
		t.Error("a backend without a space reporter was reported as supported")
	}
}

func TestStorageSpaceRequiresAdmin(t *testing.T) {
	e := newEnv(t)
	e.addUser("bob", "pw", "/bob", auth.RoleUser, auth.PermRead)
	session := e.login("bob", "pw")

	rec := e.do(http.MethodGet, "/api/admin/storage/space", "", session)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status %d, want 403", rec.Code)
	}
}
