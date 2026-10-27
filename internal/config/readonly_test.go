package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// lockedDir returns a directory that cannot be written to. It skips the test
// when running as root, where permissions are not enforced.
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

func TestReadOnlyDetectedForUnwritableDir(t *testing.T) {
	dir := lockedDir(t)
	s, err := Load(filepath.Join(dir, "config.json"))
	if err != nil {
		t.Fatalf("Load must not fail on a read-only directory: %v", err)
	}
	if !s.ReadOnly() {
		t.Error("store should be read-only")
	}
	// Defaults are still available so the server can run.
	if s.Get().Frontend.AppName != "FileGator" {
		t.Error("defaults unavailable")
	}
}

func TestWritableDirIsNotReadOnly(t *testing.T) {
	s, err := Load(filepath.Join(t.TempDir(), "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	if s.ReadOnly() {
		t.Error("writable directory reported as read-only")
	}
}

func TestUpdateOnReadOnlyKeepsValuesInMemory(t *testing.T) {
	dir := lockedDir(t)
	s, err := Load(filepath.Join(dir, "config.json"))
	if err != nil {
		t.Fatal(err)
	}

	err = s.Update(func(c *Config) error {
		c.Frontend.AppName = "Changed"
		return nil
	})
	if !errors.Is(err, ErrReadOnly) {
		t.Fatalf("got %v, want ErrReadOnly", err)
	}
	// The change still applies so the current session behaves as requested.
	if s.Get().Frontend.AppName != "Changed" {
		t.Error("in-memory value not applied")
	}
}

func TestUpdateValidationStillRejectsBeforeReadOnly(t *testing.T) {
	dir := lockedDir(t)
	s, err := Load(filepath.Join(dir, "config.json"))
	if err != nil {
		t.Fatal(err)
	}

	err = s.Update(func(c *Config) error {
		c.Frontend.AppName = ""
		return nil
	})
	if err == nil || errors.Is(err, ErrReadOnly) {
		t.Fatalf("got %v, want a validation error", err)
	}
	if s.Get().Frontend.AppName == "" {
		t.Error("invalid config was applied")
	}
}

func TestReadOnlySurvivesExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	// Write a config first, then make the directory read-only.
	s, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Update(func(c *Config) error {
		c.Frontend.AppName = "Existing"
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o700) })

	reloaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !reloaded.ReadOnly() {
		t.Error("existing read-only config not detected")
	}
	if reloaded.Get().Frontend.AppName != "Existing" {
		t.Error("existing values were not loaded")
	}
}

// TestMissingDirectoryCountsAsWritable covers the first start of a binary in an
// empty folder: private/ does not exist yet, and the writer creates it. Probing
// it as if it did marked the very first start read-only — no session key, no
// config.json — while the second start succeeded, because the users provider
// had created private/ in the meantime. That made "created config.json" appear
// on a restart instead of on the first start.
func TestMissingDirectoryCountsAsWritable(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "private", "config.json")

	s, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s.ReadOnly() {
		t.Fatal("a missing private/ should count as writable: the writer creates it")
	}
	if why := s.Why(); why != "" {
		t.Errorf("Why() = %q for a writable store", why)
	}

	// And the write really happens, folder included.
	if err := s.Update(func(c *Config) error {
		c.Frontend.AppName = "First Start"
		return nil
	}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("Update did not create %s: %v", path, err)
	}
}

// TestMissingDirectoryBelowALockedParentIsReadOnly keeps the relaxed check
// honest: the folder is only creatable when its nearest existing parent accepts
// new entries.
func TestMissingDirectoryBelowALockedParentIsReadOnly(t *testing.T) {
	locked := lockedDir(t)
	path := filepath.Join(locked, "private", "config.json")

	s, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !s.ReadOnly() {
		t.Fatal("a folder that cannot be created below an unwritable parent must be read-only")
	}
	why := s.Why()
	if why == "" || why == ErrReadOnly.Error() {
		t.Errorf("Why() = %q; it should name the folder and the reason", why)
	}
	if !strings.Contains(why, "cannot be written") {
		t.Errorf("Why() = %q; it should say the folder cannot be written to", why)
	}
}

// TestWhyNamesAFolderThatIsAFile covers the other cause a bare "read-only"
// hides: the configuration path's parent exists but is a file.
func TestWhyNamesAFolderThatIsAFile(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "private")
	if err := os.WriteFile(blocker, []byte("not a folder"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(filepath.Join(blocker, "config.json")); err == nil {
		t.Fatal("a configuration path below a file must be an error, not a silent fallback")
	}
	// The reason is what a caller building a message has to say out loud.
	if why := writeReason(blocker); !strings.Contains(why, "not a folder") {
		t.Errorf("writeReason(%q) = %q; it should say that is not a folder", blocker, why)
	}
}
