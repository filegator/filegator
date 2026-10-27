package local

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/filegator/filegator/internal/storage"
)

// setupTree builds a repository with two user homes and an admin-only folder.
func setupTree(t *testing.T) (*Root, *Scope) {
	t.Helper()
	r := newRoot(t)
	writeFile(t, r.Dir(), "alice/docs/report.txt", "alice data")
	writeFile(t, r.Dir(), "bob/notes.txt", "bob data")
	writeFile(t, r.Dir(), "shared/common.txt", "shared data")
	writeFile(t, r.Dir(), "secret.txt", "admin only")

	s, err := r.Scope("/alice")
	if err != nil {
		t.Fatalf("Scope: %v", err)
	}
	return r, s.(*Scope)
}

func TestScopeReadsOwnHome(t *testing.T) {
	_, s := setupTree(t)

	entries, err := s.ReadDir("/")
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 1 || entries[0].Name != "docs" {
		t.Fatalf("entries = %+v", entries)
	}

	f, err := s.Open("/docs/report.txt")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer f.Close()
	if got := readAll(t, f); got != "alice data" {
		t.Errorf("content = %q", got)
	}
}

func TestScopeCannotEscapeUpward(t *testing.T) {
	_, s := setupTree(t)

	// Every attempt to leave the home folder must fail.
	attempts := []string{
		"..",
		"/..",
		"/../",
		"/../bob/notes.txt",
		"/docs/../../bob/notes.txt",
		"/../../etc/passwd",
		`/..\bob`,
	}
	for _, p := range attempts {
		if _, err := s.Stat(p); !errors.Is(err, storage.ErrOutsideScope) {
			t.Errorf("Stat(%q) err = %v, want storage.ErrOutsideScope", p, err)
		}
		if _, err := s.Open(p); !errors.Is(err, storage.ErrOutsideScope) {
			t.Errorf("Open(%q) err = %v, want storage.ErrOutsideScope", p, err)
		}
		if _, err := s.ReadDir(p); !errors.Is(err, storage.ErrOutsideScope) {
			t.Errorf("ReadDir(%q) err = %v, want storage.ErrOutsideScope", p, err)
		}
	}
}

func TestScopeCannotReadSiblingsByAbsolutePath(t *testing.T) {
	_, s := setupTree(t)

	// A leading slash is treated as "relative to my home", so this names
	// alice's own /bob, which does not exist — it must not reach bob's home.
	if s.Exists("/bob/notes.txt") {
		t.Error("scope reached a sibling home folder")
	}
	if _, err := s.Open("/secret.txt"); !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("got %v, want storage.ErrNotFound", err)
	}
}

func TestScopeCannotWriteOutside(t *testing.T) {
	r, s := setupTree(t)

	if _, err := s.Create("/../bob/evil.txt"); !errors.Is(err, storage.ErrOutsideScope) {
		t.Errorf("Create err = %v", err)
	}
	if err := s.Mkdir("/../bob/newdir"); !errors.Is(err, storage.ErrOutsideScope) {
		t.Errorf("Mkdir err = %v", err)
	}
	if err := s.Rename("/docs/report.txt", "/../bob/stolen.txt"); !errors.Is(err, storage.ErrOutsideScope) {
		t.Errorf("Rename err = %v", err)
	}
	if err := s.Copy("/docs/report.txt", "/../bob/copied.txt"); !errors.Is(err, storage.ErrOutsideScope) {
		t.Errorf("Copy err = %v", err)
	}
	if err := s.Remove("/../bob/notes.txt"); !errors.Is(err, storage.ErrOutsideScope) {
		t.Errorf("Remove err = %v", err)
	}

	// Bob's folder must be untouched.
	if !r.Exists("/bob/notes.txt") {
		t.Fatal("bob's file was removed")
	}
	if r.Exists("/bob/evil.txt") || r.Exists("/bob/copied.txt") || r.Exists("/bob/stolen.txt") {
		t.Error("scope wrote into a sibling home")
	}
}

func TestScopeCannotRemoveHome(t *testing.T) {
	r, s := setupTree(t)

	if err := s.Remove("/"); !errors.Is(err, storage.ErrOutsideScope) {
		t.Errorf("got %v, want storage.ErrOutsideScope", err)
	}
	if err := s.Remove("/."); !errors.Is(err, storage.ErrOutsideScope) {
		t.Errorf("got %v, want storage.ErrOutsideScope", err)
	}
	if !r.Exists("/alice/docs/report.txt") {
		t.Fatal("home was damaged")
	}
}

func TestScopeSymlinkEscapeRejected(t *testing.T) {
	r := newRoot(t)
	writeFile(t, r.Dir(), "alice/file.txt", "mine")
	writeFile(t, r.Dir(), "bob/secret.txt", "bob secret")

	// A symlink inside alice's home pointing at bob's folder.
	link := filepath.Join(r.Dir(), "alice", "escape")
	if err := os.Symlink(filepath.Join(r.Dir(), "bob"), link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	s, err := r.Scope("/alice")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.ReadDir("/escape"); !errors.Is(err, storage.ErrOutsideScope) {
		t.Errorf("ReadDir through symlink err = %v, want storage.ErrOutsideScope", err)
	}
	if _, err := s.Open("/escape/secret.txt"); !errors.Is(err, storage.ErrOutsideScope) {
		t.Errorf("Open through symlink err = %v, want storage.ErrOutsideScope", err)
	}
}

func TestScopeSymlinkInsideHomeAllowed(t *testing.T) {
	r := newRoot(t)
	writeFile(t, r.Dir(), "alice/real/file.txt", "data")
	link := filepath.Join(r.Dir(), "alice", "alias")
	if err := os.Symlink(filepath.Join(r.Dir(), "alice", "real"), link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	s, err := r.Scope("/alice")
	if err != nil {
		t.Fatal(err)
	}
	f, err := s.Open("/alias/file.txt")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer f.Close()
	if got := readAll(t, f); got != "data" {
		t.Errorf("content = %q", got)
	}
}

func TestScopeHomeMustNotEscapeRoot(t *testing.T) {
	r := newRoot(t)
	outside := filepath.Join(t.TempDir(), "outside")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(r.Dir(), "linked")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	if _, err := r.Scope("/linked"); !errors.Is(err, storage.ErrOutsideScope) {
		t.Fatalf("got %v, want storage.ErrOutsideScope", err)
	}
}

func TestScopeMissingHome(t *testing.T) {
	r := newRoot(t)
	if _, err := r.Scope("/does-not-exist"); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("got %v, want storage.ErrNotFound", err)
	}
}

func TestScopeSharedHome(t *testing.T) {
	r := newRoot(t)
	writeFile(t, r.Dir(), "shared/common.txt", "shared data")

	a, err := r.Scope("/shared")
	if err != nil {
		t.Fatal(err)
	}
	b, err := r.Scope("/shared")
	if err != nil {
		t.Fatal(err)
	}

	f, err := a.Open("/common.txt")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()

	if !b.Exists("/common.txt") {
		t.Error("second scope cannot see shared content")
	}

	// Both can write to the shared folder.
	w, err := b.Create("/from-b.txt")
	if err != nil {
		t.Fatal(err)
	}
	w.Close()
	if !a.Exists("/from-b.txt") {
		t.Error("shared write not visible")
	}
}

func TestRootScopeSeesEverything(t *testing.T) {
	r := newRoot(t)
	writeFile(t, r.Dir(), "alice/a.txt", "a")
	writeFile(t, r.Dir(), "bob/b.txt", "b")
	writeFile(t, r.Dir(), "top.txt", "t")

	// Administrators use home "/" and see the whole tree.
	admin, err := r.Scope("/")
	if err != nil {
		t.Fatalf("Scope(/): %v", err)
	}
	for _, p := range []string{"/alice/a.txt", "/bob/b.txt", "/top.txt"} {
		if !admin.Exists(p) {
			t.Errorf("admin cannot see %s", p)
		}
	}
	if admin.(*Scope).Home() != "/" {
		t.Errorf("Home() = %q", admin.(*Scope).Home())
	}
}

func TestAdminScopeCannotEscapeRoot(t *testing.T) {
	r := newRoot(t)
	admin, err := r.Scope("/")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Stat("/../../etc/passwd"); !errors.Is(err, storage.ErrOutsideScope) {
		t.Fatalf("got %v, want storage.ErrOutsideScope", err)
	}
}

func TestScopeNestedHome(t *testing.T) {
	r := newRoot(t)
	writeFile(t, r.Dir(), "team/alice/docs/a.txt", "deep")
	writeFile(t, r.Dir(), "team/bob/b.txt", "other")

	s, err := r.Scope("/team/alice")
	if err != nil {
		t.Fatalf("Scope: %v", err)
	}
	if !s.Exists("/docs/a.txt") {
		t.Error("nested home content not visible")
	}
	if s.Exists("/../bob/b.txt") {
		t.Error("escaped nested home")
	}
	if _, err := s.Open("/../../bob/b.txt"); !errors.Is(err, storage.ErrOutsideScope) {
		t.Errorf("got %v", err)
	}
}

func TestEnsureDir(t *testing.T) {
	r := newRoot(t)
	if err := r.EnsureDir("/newuser/docs"); err != nil {
		t.Fatalf("EnsureDir: %v", err)
	}
	if !r.Exists("/newuser/docs") {
		t.Fatal("directory not created")
	}
	if err := r.EnsureDir("/newuser/docs"); !errors.Is(err, storage.ErrExists) {
		t.Fatalf("got %v, want storage.ErrExists", err)
	}
	if err := r.EnsureDir("/../escape"); !errors.Is(err, storage.ErrOutsideRoot) {
		t.Fatalf("got %v, want storage.ErrOutsideRoot", err)
	}
}
