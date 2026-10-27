package search

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/filegator/filegator/internal/storage"
	"github.com/filegator/filegator/internal/storage/local"
)

// newFS returns a storage view over a temporary repository.
func newFS(t *testing.T) storage.FS {
	t.Helper()
	root, err := local.NewRoot(filepath.Join(t.TempDir(), "repo"))
	if err != nil {
		t.Fatal(err)
	}
	fsys, err := root.Scope("/")
	if err != nil {
		t.Fatal(err)
	}
	return fsys
}

// write creates a file, creating parent folders as needed.
func write(t *testing.T, fsys storage.FS, name, content string) {
	t.Helper()
	if dir := "/" + strings.TrimPrefix(filepath.Dir(name), "/"); dir != "/" {
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

// names lists the matched entry paths.
func names(results []Result) []string {
	out := make([]string, 0, len(results))
	for _, r := range results {
		out = append(out, r.Entry.Path)
	}
	return out
}

// find looks for a path in the results.
func find(results []Result, path string) *Result {
	for i := range results {
		if results[i].Entry.Path == path {
			return &results[i]
		}
	}
	return nil
}

func TestNamesFindsNestedFiles(t *testing.T) {
	fsys := newFS(t)
	write(t, fsys, "/docs/report.txt", "x")
	write(t, fsys, "/docs/sub/deep/report-final.txt", "x")
	write(t, fsys, "/other/notes.txt", "x")

	results, err := Names(context.Background(), fsys, "/", "report", Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("results = %v, want 2", names(results))
	}
	if find(results, "/docs/sub/deep/report-final.txt") == nil {
		t.Errorf("deep match missing from %v", names(results))
	}
}

func TestNamesIsCaseInsensitive(t *testing.T) {
	fsys := newFS(t)
	write(t, fsys, "/Reports/Annual.TXT", "x")

	results, err := Names(context.Background(), fsys, "/", "ANNUAL", Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Entry.Name != "Annual.TXT" {
		t.Errorf("results = %v", names(results))
	}
}

func TestNamesMatchesDirectories(t *testing.T) {
	fsys := newFS(t)
	write(t, fsys, "/photos/2024/img.jpg", "x")

	results, err := Names(context.Background(), fsys, "/", "photos", Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || !results[0].IsDir() {
		t.Fatalf("results = %v", names(results))
	}
}

func TestNamesReportsParent(t *testing.T) {
	fsys := newFS(t)
	write(t, fsys, "/docs/sub/report.txt", "x")

	results, err := Names(context.Background(), fsys, "/", "report", Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatal("no match")
	}
	if results[0].Parent != "/docs/sub" {
		t.Errorf("parent = %q, want /docs/sub", results[0].Parent)
	}
}

func TestNamesScopesToRoot(t *testing.T) {
	fsys := newFS(t)
	write(t, fsys, "/docs/report.txt", "x")
	write(t, fsys, "/other/report.txt", "x")

	results, err := Names(context.Background(), fsys, "/docs", "report", Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Entry.Path != "/docs/report.txt" {
		t.Errorf("results = %v", names(results))
	}
}

func TestNamesRejectsEmptyTerm(t *testing.T) {
	fsys := newFS(t)
	for _, term := range []string{"", "   "} {
		if _, err := Names(context.Background(), fsys, "/", term, Options{}); !errors.Is(err, ErrEmptyTerm) {
			t.Errorf("%q: got %v, want ErrEmptyTerm", term, err)
		}
	}
}

func TestNamesLimit(t *testing.T) {
	fsys := newFS(t)
	for _, name := range []string{"a1.txt", "a2.txt", "a3.txt", "a4.txt"} {
		write(t, fsys, "/"+name, "x")
	}

	results, err := Names(context.Background(), fsys, "/", "a", Options{Limit: 2})
	if !errors.Is(err, ErrLimited) {
		t.Errorf("err = %v, want ErrLimited", err)
	}
	// A capped search still returns what it found; callers show it with a
	// "refine your search" hint.
	if len(results) > 2 {
		t.Errorf("results = %d, want at most 2", len(results))
	}
	if len(results) == 0 {
		t.Error("limit dropped every result")
	}
}

func TestNamesMaxDepth(t *testing.T) {
	fsys := newFS(t)
	write(t, fsys, "/top.txt", "x")
	write(t, fsys, "/one/deep.txt", "x")
	write(t, fsys, "/one/two/two.txt", "x")

	// Depth 1 visits "/" and its children but does not descend into "/one".
	results, err := Names(context.Background(), fsys, "/", ".txt", Options{MaxDepth: 1})
	if err != nil {
		t.Fatal(err)
	}
	if find(results, "/top.txt") == nil {
		t.Error("top level file missing")
	}
	if find(results, "/one/deep.txt") != nil || find(results, "/one/two/two.txt") != nil {
		t.Errorf("depth limit ignored: %v", names(results))
	}
}

func TestNamesStopsWhenCancelled(t *testing.T) {
	fsys := newFS(t)
	write(t, fsys, "/a/deep.txt", "x")
	write(t, fsys, "/b/deep.txt", "x")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	results, err := Names(ctx, fsys, "/", "deep", Options{})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
	if len(results) != 0 {
		t.Errorf("cancelled search returned %v", names(results))
	}
}

func TestNamesMatchesEverythingAtRoot(t *testing.T) {
	fsys := newFS(t)
	write(t, fsys, "/docs/a.txt", "x")

	results, err := Names(context.Background(), fsys, "/", "docs", Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Errorf("results = %v", names(results))
	}
}

func TestNamesNoMatches(t *testing.T) {
	fsys := newFS(t)
	write(t, fsys, "/a.txt", "x")

	results, err := Names(context.Background(), fsys, "/", "zzz", Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Errorf("results = %v", names(results))
	}
}

func TestNamesSkipsUnreadableFolders(t *testing.T) {
	fsys := newFS(t)
	write(t, fsys, "/docs/report.txt", "x")
	if err := fsys.MkdirAll("/locked"); err != nil {
		t.Fatal(err)
	}

	// A backend that cannot list /locked must not hide the other matches.
	limited := &unreadable{FS: fsys, dir: "/locked"}
	results, err := Names(context.Background(), limited, "/", "report", Options{})
	if err != nil {
		t.Fatalf("unreadable folder failed the search: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("results = %v", names(results))
	}
}

// unreadable is a storage.FS that refuses to list one folder.
type unreadable struct {
	storage.FS
	dir string
}

func (u *unreadable) ReadDir(dir string) ([]*storage.Entry, error) {
	if dir == u.dir {
		return nil, storage.ErrPermission
	}
	return u.FS.ReadDir(dir)
}
