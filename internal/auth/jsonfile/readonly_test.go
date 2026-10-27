package jsonfile

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// lockedDir returns a directory that cannot be written to, skipping as root.
func lockedDir(t *testing.T) string {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("permission checks do not apply to root")
	}
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o700) })
	return dir
}

func TestCheckWritableAcceptsWritableFile(t *testing.T) {
	f := newTestFile(t)
	if err := f.CheckWritable(); err != nil {
		t.Fatalf("writable users file rejected: %v", err)
	}
}

func TestCheckWritableRejectsReadOnlyDirectory(t *testing.T) {
	dir := lockedDir(t)
	f := NewUsersFile(filepath.Join(dir, "users.json"))
	err := f.CheckWritable()
	if !errors.Is(err, ErrReadOnly) {
		t.Fatalf("got %v, want ErrReadOnly", err)
	}
}

func TestCheckWritableRejectsReadOnlyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "users.json")
	f := NewUsersFile(path)
	if _, err := f.EnsureFromTemplate(writeSample(t, dir)); err != nil {
		t.Fatal(err)
	}
	// Directory stays writable but the file itself does not.
	if err := os.Chmod(path, 0o400); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(path, 0o600) })

	// Only meaningful when the process cannot bypass permissions.
	if os.Geteuid() == 0 {
		t.Skip("permission checks do not apply to root")
	}
	if err := f.CheckWritable(); !errors.Is(err, ErrReadOnly) {
		t.Fatalf("got %v, want ErrReadOnly", err)
	}
}

// writeSample creates a users template inside dir and returns its path.
func writeSample(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "users.json.sample")
	if err := os.WriteFile(path, []byte(`{"1":{"username":"admin","role":"admin","homedir":"/","permissions":"","password":""}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
