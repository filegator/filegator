package archiver

import (
	"archive/zip"
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/filegator/filegator/internal/storage"
	"github.com/filegator/filegator/internal/storage/local"
)

// newFS returns a local storage view over a temporary directory.
func newFS(t *testing.T) (storage.FS, string) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "repo")
	root, err := local.NewRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	fsys, err := root.Scope("/")
	if err != nil {
		t.Fatal(err)
	}
	return fsys, dir
}

// write creates a file with content.
func write(t *testing.T, fsys storage.FS, name, content string) {
	t.Helper()
	dir := "/" + strings.TrimPrefix(filepath.Dir(name), "/")
	if dir != "/" {
		if err := fsys.MkdirAll(dir); err != nil {
			t.Fatal(err)
		}
	}
	w, err := fsys.Create("/" + strings.TrimPrefix(name, "/"))
	if err != nil {
		t.Fatalf("create %s: %v", name, err)
	}
	if _, err := w.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
}

// readAll reads a file back.
func readAll(t *testing.T, fsys storage.FS, name string) string {
	t.Helper()
	r, err := fsys.Open(name)
	if err != nil {
		t.Fatalf("open %s: %v", name, err)
	}
	defer r.Close()
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// names lists the entries in a zip payload.
func names(t *testing.T, data []byte) []string {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("zip is not readable: %v", err)
	}
	out := make([]string, 0, len(zr.File))
	for _, f := range zr.File {
		out = append(out, f.Name)
	}
	return out
}

func TestWriteSingleFile(t *testing.T) {
	fsys, _ := newFS(t)
	write(t, fsys, "/notes.txt", "hello")

	var buf bytes.Buffer
	if err := Write(fsys, &buf, []string{"/notes.txt"}); err != nil {
		t.Fatalf("Write: %v", err)
	}

	if got := names(t, buf.Bytes()); len(got) != 1 || got[0] != "notes.txt" {
		t.Errorf("entries = %v", got)
	}
}

func TestWriteDirectoryRecursively(t *testing.T) {
	fsys, _ := newFS(t)
	write(t, fsys, "/docs/a.txt", "one")
	write(t, fsys, "/docs/sub/b.txt", "two")

	var buf bytes.Buffer
	if err := Write(fsys, &buf, []string{"/docs"}); err != nil {
		t.Fatalf("Write: %v", err)
	}

	// Order follows ReadDir (directories first), so compare as a set.
	got := names(t, buf.Bytes())
	want := map[string]bool{
		"docs/": true, "docs/a.txt": true,
		"docs/sub/": true, "docs/sub/b.txt": true,
	}
	if len(got) != len(want) {
		t.Fatalf("entries = %v, want %d entries", got, len(want))
	}
	for _, name := range got {
		if !want[name] {
			t.Errorf("unexpected entry %q", name)
		}
	}
}

func TestWritePreservesEmptyDirectories(t *testing.T) {
	fsys, _ := newFS(t)
	if err := fsys.MkdirAll("/empty"); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if err := Write(fsys, &buf, []string{"/empty"}); err != nil {
		t.Fatal(err)
	}
	if got := names(t, buf.Bytes()); len(got) != 1 || got[0] != "empty/" {
		t.Errorf("entries = %v", got)
	}
}

func TestWriteMultiplePaths(t *testing.T) {
	fsys, _ := newFS(t)
	write(t, fsys, "/a.txt", "a")
	write(t, fsys, "/b.txt", "b")

	var buf bytes.Buffer
	if err := Write(fsys, &buf, []string{"/a.txt", "/b.txt"}); err != nil {
		t.Fatal(err)
	}
	if got := names(t, buf.Bytes()); len(got) != 2 {
		t.Errorf("entries = %v", got)
	}
}

func TestWriteNoEntries(t *testing.T) {
	fsys, _ := newFS(t)
	var buf bytes.Buffer
	if err := Write(fsys, &buf, nil); !errors.Is(err, ErrNoEntries) {
		t.Fatalf("got %v, want ErrNoEntries", err)
	}
}

func TestWriteMissingPath(t *testing.T) {
	fsys, _ := newFS(t)
	var buf bytes.Buffer
	if err := Write(fsys, &buf, []string{"/nope.txt"}); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("got %v, want ErrNotFound", err)
	}
}

func TestExtractRoundTrip(t *testing.T) {
	fsys, _ := newFS(t)
	write(t, fsys, "/docs/a.txt", "alpha")
	write(t, fsys, "/docs/sub/b.txt", "beta")

	var buf bytes.Buffer
	if err := Write(fsys, &buf, []string{"/docs"}); err != nil {
		t.Fatal(err)
	}

	destFS, _ := newFS(t)
	created, err := Extract(destFS, bytes.NewReader(buf.Bytes()), int64(buf.Len()), "/")
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(created) != 2 {
		t.Errorf("created %d entries, want 2", len(created))
	}
	if got := readAll(t, destFS, "/docs/a.txt"); got != "alpha" {
		t.Errorf("content = %q", got)
	}
	if got := readAll(t, destFS, "/docs/sub/b.txt"); got != "beta" {
		t.Errorf("content = %q", got)
	}
}

func TestExtractIntoSubfolder(t *testing.T) {
	fsys, _ := newFS(t)
	write(t, fsys, "/a.txt", "x")

	var buf bytes.Buffer
	if err := Write(fsys, &buf, []string{"/a.txt"}); err != nil {
		t.Fatal(err)
	}

	if _, err := Extract(fsys, bytes.NewReader(buf.Bytes()), int64(buf.Len()), "/out"); err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if got := readAll(t, fsys, "/out/a.txt"); got != "x" {
		t.Errorf("content = %q", got)
	}
}

func TestExtractRejectsTraversal(t *testing.T) {
	cases := []string{
		"../evil.txt",
		"../../evil.txt",
		"a/../../evil.txt",
		"/absolute.txt",
		`..\evil.txt`,
	}
	for _, name := range cases {
		var buf bytes.Buffer
		zw := zip.NewWriter(&buf)
		if _, err := zw.Create(name); err != nil {
			t.Fatal(err)
		}
		zw.Close()

		destFS, dir := newFS(t)
		_, err := Extract(destFS, bytes.NewReader(buf.Bytes()), int64(buf.Len()), "/out")
		if !errors.Is(err, ErrUnsafeEntry) {
			t.Errorf("%q: got %v, want ErrUnsafeEntry", name, err)
		}
		// Nothing may be written outside the destination.
		if _, statErr := fsysStat(filepath.Join(dir, "evil.txt")); statErr == nil {
			t.Errorf("%q: wrote outside the destination", name)
		}
	}
}

// fsysStat checks a path on the real filesystem.
func fsysStat(p string) (any, error) {
	return osStat(p)
}

func TestEntryTargetAcceptsNested(t *testing.T) {
	got, err := entryTarget("/out", "a/b/c.txt")
	if err != nil {
		t.Fatalf("rejected: %v", err)
	}
	if got != "/out/a/b/c.txt" {
		t.Errorf("got %q", got)
	}

	// Leading "./" is harmless.
	if _, err := entryTarget("/out", "./a.txt"); err != nil {
		t.Errorf("dot prefix rejected: %v", err)
	}
}

func TestEntryTargetRejectsNul(t *testing.T) {
	if _, err := entryTarget("/out", "a\x00b"); !errors.Is(err, ErrUnsafeEntry) {
		t.Fatal("NUL byte accepted")
	}
}

func TestEntryTargetRejectsLongNames(t *testing.T) {
	long := strings.Repeat("a", maxEntryName+1)
	if _, err := entryTarget("/out", long); !errors.Is(err, ErrUnsafeEntry) {
		t.Fatal("overlong name accepted")
	}
}

func TestSuggestFolder(t *testing.T) {
	cases := map[string]string{
		"photos.zip":     "photos",
		"/path/docs.zip": "docs",
		"archive.tar.gz": "archive.tar",
		"noext":          "noext",
		".zip":           ".zip", // a dotfile name is valid as-is
		"weird/..":       "extracted",
	}
	for in, want := range cases {
		if got := SuggestFolder(in); got != want {
			t.Errorf("SuggestFolder(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestExtractInvalidZip(t *testing.T) {
	fsys, _ := newFS(t)
	_, err := Extract(fsys, strings.NewReader("not a zip"), int64(len("not a zip")), "/")
	if err == nil {
		t.Fatal("invalid zip accepted")
	}
}

// osStat is a tiny indirection so the test file has one import list.
func osStat(p string) (any, error) { return os.Stat(p) }
