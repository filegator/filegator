package api

import (
	"net/http"
	"testing"

	"github.com/filegator/filegator/internal/auth"
)

func TestFolderSizeSumsFilesRecursively(t *testing.T) {
	e := newEnv(t)
	e.addUser("alice", "pw", "/alice", auth.RoleUser, auth.PermRead)
	session := e.login("alice", "pw")

	rec := e.do(http.MethodGet, "/api/files/size?path=/", "", session)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body)
	}
	var result SizeResponse
	decode(t, rec, &result)
	// "alice report" (12) + "notes" (5).
	if result.Size != 17 {
		t.Errorf("size = %d, want 17", result.Size)
	}
	if result.Path != "/" {
		t.Errorf("path = %q", result.Path)
	}
}

func TestFolderSizeRejectsAFile(t *testing.T) {
	e := newEnv(t)
	e.addUser("alice", "pw", "/alice", auth.RoleUser, auth.PermRead)
	session := e.login("alice", "pw")

	rec := e.do(http.MethodGet, "/api/files/size?path=/notes.md", "", session)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d, want 400", rec.Code)
	}
}

func TestFolderSizeMissingFolder(t *testing.T) {
	e := newEnv(t)
	e.addUser("alice", "pw", "/alice", auth.RoleUser, auth.PermRead)
	session := e.login("alice", "pw")

	rec := e.do(http.MethodGet, "/api/files/size?path=/nope", "", session)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status %d, want 404", rec.Code)
	}
}

func TestFolderSizeCannotLeaveHome(t *testing.T) {
	e := newEnv(t)
	e.addUser("alice", "pw", "/alice", auth.RoleUser, auth.PermRead)
	session := e.login("alice", "pw")

	rec := e.do(http.MethodGet, "/api/files/size?path=/../bob", "", session)
	if rec.Code == http.StatusOK {
		t.Fatalf("size of a folder outside the home was returned: %s", rec.Body)
	}
}
