package local

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/filegator/filegator/internal/storage"
)

func TestBrowseListsOnlyDirectories(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"alpha", "Beta", "gamma"} {
		if err := os.Mkdir(filepath.Join(dir, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "secret.txt"), []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}

	dirs, err := Browse(dir)
	if err != nil {
		t.Fatalf("Browse: %v", err)
	}
	if len(dirs) != 3 {
		t.Fatalf("got %d entries, want 3: %+v", len(dirs), dirs)
	}
	// Case-insensitive sort: alpha, Beta, gamma.
	want := []string{"alpha", "Beta", "gamma"}
	for i, w := range want {
		if dirs[i].Name != w {
			t.Errorf("dirs[%d] = %q, want %q", i, dirs[i].Name, w)
		}
	}
	if dirs[0].Path != filepath.Join(dir, "alpha") {
		t.Errorf("path = %q", dirs[0].Path)
	}
}

func TestBrowseMissingDirectory(t *testing.T) {
	if _, err := Browse(filepath.Join(t.TempDir(), "nope")); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("got %v, want storage.ErrNotFound", err)
	}
}

func TestBrowseFileRejected(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Browse(file); !errors.Is(err, storage.ErrNotDir) {
		t.Fatalf("got %v, want storage.ErrNotDir", err)
	}
}

func TestBrowseSkipsBrokenSymlinks(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "real"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(dir, "missing"), filepath.Join(dir, "broken")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := os.Symlink(filepath.Join(dir, "real"), filepath.Join(dir, "good")); err != nil {
		t.Skip()
	}

	dirs, err := Browse(dir)
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, d := range dirs {
		names[d.Name] = true
	}
	if names["broken"] {
		t.Error("broken symlink listed")
	}
	if !names["good"] || !names["real"] {
		t.Errorf("dirs = %+v", dirs)
	}
}

func TestBrowseRoot(t *testing.T) {
	// The filesystem root must be browsable, so the admin can start anywhere.
	dirs, err := Browse("/")
	if err != nil {
		t.Fatalf("Browse(/): %v", err)
	}
	if len(dirs) == 0 {
		t.Error("expected some directories at /")
	}
}

func TestResolveDir(t *testing.T) {
	dir := t.TempDir()
	got, err := ResolveDir(dir)
	if err != nil {
		t.Fatalf("ResolveDir: %v", err)
	}
	if !filepath.IsAbs(got) {
		t.Errorf("path %q is not absolute", got)
	}

	if _, err := ResolveDir(""); !errors.Is(err, storage.ErrInvalidPath) {
		t.Errorf("empty dir: got %v", err)
	}
	if _, err := ResolveDir(filepath.Join(dir, "missing")); !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("missing dir: got %v", err)
	}

	file := filepath.Join(dir, "f.txt")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ResolveDir(file); !errors.Is(err, storage.ErrNotDir) {
		t.Errorf("file: got %v", err)
	}
}
