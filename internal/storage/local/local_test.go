package local

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/filegator/filegator/internal/storage"
)

func newRoot(t *testing.T) *Root {
	t.Helper()
	r, err := NewRoot(filepath.Join(t.TempDir(), "repository"))
	if err != nil {
		t.Fatalf("NewRoot: %v", err)
	}
	return r
}

// writeFile creates a file with content below dir, making parents as needed.
func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	full := filepath.Join(dir, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readAll(t *testing.T, f interface{ Read([]byte) (int, error) }) string {
	t.Helper()
	var sb strings.Builder
	buf := make([]byte, 1024)
	for {
		n, err := f.Read(buf)
		sb.Write(buf[:n])
		if err != nil {
			break
		}
	}
	return sb.String()
}

func TestReadDirListsAllFilesIncludingDotfiles(t *testing.T) {
	r := newRoot(t)
	writeFile(t, r.Dir(), ".gitkeep", "")
	writeFile(t, r.Dir(), ".hidden", "x")
	writeFile(t, r.Dir(), "visible.txt", "x")

	entries, err := r.ReadDir("/")
	if err != nil {
		t.Fatal(err)
	}
	// Every file is listed, including dotfiles: nothing is hidden from users.
	if len(entries) != 3 {
		t.Fatalf("entries = %+v", entries)
	}
}

func TestGitkeepIsDeletable(t *testing.T) {
	r := newRoot(t)
	writeFile(t, r.Dir(), ".gitkeep", "")

	// The placeholder is an ordinary file: users may remove it.
	if err := r.Remove("/.gitkeep"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if r.Exists("/.gitkeep") {
		t.Error(".gitkeep was not removed")
	}
}

func TestRootDirIsCreated(t *testing.T) {
	r := newRoot(t)
	info, err := os.Stat(r.Dir())
	if err != nil || !info.IsDir() {
		t.Fatalf("root dir not created: %v", err)
	}
}

func TestMkdirAndReadDir(t *testing.T) {
	r := newRoot(t)
	if err := r.Mkdir("/docs"); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}
	if err := r.Mkdir("/docs"); !errors.Is(err, storage.ErrExists) {
		t.Errorf("duplicate Mkdir err = %v, want storage.ErrExists", err)
	}

	for _, name := range []string{"b.txt", "a.txt"} {
		f, err := r.Create("/docs/" + name)
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		f.Close()
	}
	if err := r.Mkdir("/docs/sub"); err != nil {
		t.Fatal(err)
	}

	entries, err := r.ReadDir("/docs")
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	// Directories first, then case-insensitive name order.
	want := []string{"sub", "a.txt", "b.txt"}
	if len(entries) != len(want) {
		t.Fatalf("got %d entries", len(entries))
	}
	for i := range want {
		if entries[i].Name != want[i] {
			t.Errorf("entries[%d] = %q, want %q", i, entries[i].Name, want[i])
		}
	}
	if entries[0].Path != "/docs/sub" {
		t.Errorf("Path = %q", entries[0].Path)
	}
	if !entries[0].IsDir {
		t.Error("sub should be a directory")
	}
}

func TestReadDirMissing(t *testing.T) {
	if _, err := newRoot(t).ReadDir("/nope"); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("got %v", err)
	}
}

func TestWriteAndReadFile(t *testing.T) {
	r := newRoot(t)
	w, err := r.Create("/hello.txt")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := w.Write([]byte("hello world")); err != nil {
		t.Fatal(err)
	}
	w.Close()

	rd, err := r.Open("/hello.txt")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer rd.Close()
	if got := readAll(t, rd); got != "hello world" {
		t.Errorf("content = %q", got)
	}

	entry, err := r.Stat("/hello.txt")
	if err != nil {
		t.Fatal(err)
	}
	if entry.Size != 11 || entry.IsDir {
		t.Errorf("entry = %+v", entry)
	}
}

func TestRename(t *testing.T) {
	r := newRoot(t)
	writeFile(t, r.Dir(), "a.txt", "x")
	if err := r.Rename("/a.txt", "/b.txt"); err != nil {
		t.Fatalf("Rename: %v", err)
	}
	if r.Exists("/a.txt") || !r.Exists("/b.txt") {
		t.Error("rename did not move the file")
	}
}

func TestRenameRefusesOverwrite(t *testing.T) {
	r := newRoot(t)
	writeFile(t, r.Dir(), "a.txt", "x")
	writeFile(t, r.Dir(), "b.txt", "y")
	if err := r.Rename("/a.txt", "/b.txt"); !errors.Is(err, storage.ErrExists) {
		t.Fatalf("got %v, want storage.ErrExists", err)
	}
}

func TestCopyFile(t *testing.T) {
	r := newRoot(t)
	writeFile(t, r.Dir(), "a.txt", "content")
	if err := r.Copy("/a.txt", "/b.txt"); err != nil {
		t.Fatalf("Copy: %v", err)
	}
	f, _ := r.Open("/b.txt")
	defer f.Close()
	if got := readAll(t, f); got != "content" {
		t.Errorf("content = %q", got)
	}
}

func TestCopyDirectoryRecursive(t *testing.T) {
	r := newRoot(t)
	writeFile(t, r.Dir(), "src/a.txt", "one")
	writeFile(t, r.Dir(), "src/nested/b.txt", "two")

	if err := r.Copy("/src", "/dst"); err != nil {
		t.Fatalf("Copy: %v", err)
	}
	for _, p := range []string{"/dst/a.txt", "/dst/nested/b.txt"} {
		if !r.Exists(p) {
			t.Errorf("%s missing after copy", p)
		}
	}
}

func TestCopyIntoItselfRejected(t *testing.T) {
	r := newRoot(t)
	writeFile(t, r.Dir(), "src/a.txt", "x")
	if err := r.Copy("/src", "/src/inner"); !errors.Is(err, storage.ErrInvalidPath) {
		t.Fatalf("got %v, want storage.ErrInvalidPath", err)
	}
}

func TestRemove(t *testing.T) {
	r := newRoot(t)
	writeFile(t, r.Dir(), "dir/a.txt", "x")
	if err := r.Remove("/dir"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if r.Exists("/dir") {
		t.Error("directory not removed")
	}
}

func TestRemoveRootRefused(t *testing.T) {
	r := newRoot(t)
	if err := r.Remove("/"); !errors.Is(err, storage.ErrOutsideRoot) {
		t.Fatalf("got %v, want storage.ErrOutsideRoot", err)
	}
	if _, err := os.Stat(r.Dir()); err != nil {
		t.Fatal("root was deleted")
	}
}

func TestRootTraversalRejected(t *testing.T) {
	r := newRoot(t)
	for _, p := range []string{"/../etc", "..", "/a/../../b"} {
		if _, err := r.Stat(p); !errors.Is(err, storage.ErrOutsideRoot) {
			t.Errorf("Stat(%q) err = %v, want storage.ErrOutsideRoot", p, err)
		}
		if _, err := r.ReadDir(p); !errors.Is(err, storage.ErrOutsideRoot) {
			t.Errorf("ReadDir(%q) err = %v", p, err)
		}
	}
}

func TestCleanPath(t *testing.T) {
	cases := map[string]string{
		"":           "/",
		"/":          "/",
		"docs":       "/docs",
		"/docs/":     "/docs",
		"/docs/./a":  "/docs/a",
		"/docs/../a": "/a",
		`\docs\a`:    "/docs/a",
		"//docs//a":  "/docs/a",
	}
	for in, want := range cases {
		got, err := cleanPath(in)
		if err != nil {
			t.Errorf("cleanPath(%q): %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("cleanPath(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCleanPathRejects(t *testing.T) {
	for _, p := range []string{"..", "../x", "/../x", "a/../../b", "/a\x00b"} {
		if _, err := cleanPath(p); err == nil {
			t.Errorf("cleanPath(%q) accepted", p)
		}
	}
}

func TestValidateName(t *testing.T) {
	for _, ok := range []string{".gitignore", "a.txt", "with space", "Ünïcode"} {
		if err := storage.ValidateName(ok); err != nil {
			t.Errorf("%q rejected: %v", ok, err)
		}
	}
	for _, bad := range []string{"", ".", "..", "a/b", `a\b`, "a\x00b"} {
		if err := storage.ValidateName(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}
