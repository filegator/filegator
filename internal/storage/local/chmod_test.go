package local

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/filegator/filegator/internal/storage"
)

func TestChmod(t *testing.T) {
	r := newRoot(t)
	writeFile(t, r.Dir(), "a.txt", "x")
	if err := r.Chmod("/a.txt", 0o600); err != nil {
		t.Fatalf("Chmod: %v", err)
	}
	info, err := os.Stat(filepath.Join(r.Dir(), "a.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("mode = %o, want 600", perm)
	}
}

func TestChmodMissing(t *testing.T) {
	r := newRoot(t)
	if err := r.Chmod("/nope.txt", 0o600); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("got %v, want storage.ErrNotFound", err)
	}
}

func TestChmodStripsTypeBits(t *testing.T) {
	r := newRoot(t)
	writeFile(t, r.Dir(), "a.txt", "x")
	// Callers may pass os.Executable-style modes; only permission bits apply.
	if err := r.Chmod("/a.txt", 0o777|os.ModeDir); err != nil {
		t.Fatalf("Chmod: %v", err)
	}
	info, _ := os.Stat(filepath.Join(r.Dir(), "a.txt"))
	if perm := info.Mode().Perm(); perm != 0o777 {
		t.Errorf("mode = %o, want 777", perm)
	}
}

func TestScopeChmodConfined(t *testing.T) {
	r, s := setupTree(t)
	if err := s.Chmod("/docs/report.txt", 0o600); err != nil {
		t.Fatalf("Chmod: %v", err)
	}
	if err := s.Chmod("/", 0o700); !errors.Is(err, storage.ErrOutsideScope) {
		t.Errorf("chmod on home root err = %v, want storage.ErrOutsideScope", err)
	}
	if err := s.Chmod("/../bob/notes.txt", 0o777); !errors.Is(err, storage.ErrOutsideScope) {
		t.Errorf("chmod outside err = %v, want storage.ErrOutsideScope", err)
	}
	info, err := os.Stat(filepath.Join(r.Dir(), "bob", "notes.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm == 0o777 {
		t.Error("chmod affected a file outside the home")
	}
}
