package app

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/filegator/filegator/internal/version"
)

// versionFile names the metadata file an export writes and a custom frontend
// may carry. It is not required: a hand-made frontend without it is served,
// just without a version to compare against.
const versionFile = "version.json"

// FrontendRequired lists the files a directory must contain to be accepted as
// a frontend by -frontend-dir. ExportFrontend produces them.
func FrontendRequired() []string { return append([]string(nil), frontendRequired...) }

// ExportFrontend writes the interface compiled into this binary to dir.
//
// It exists so an administrator can obtain the exact assets this binary
// serves, edit them, and start it with -frontend-dir — without needing the Go
// source tree. The written files are therefore guaranteed to match the running
// version, which a copy from a repository at a different commit is not.
//
// Nothing is written into the current directory: the target is created if
// missing, and an existing directory is accepted only when it is empty or is a
// previous export. Refusing anything else prevents an export over a folder of
// hand-made edits.
func ExportFrontend(dir string) (int, error) {
	if dir == "" {
		return 0, errors.New("export: a target directory is required")
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return 0, fmt.Errorf("export: %w", err)
	}

	// Never export over an unrelated directory.
	if info, statErr := os.Stat(abs); statErr == nil {
		if !info.IsDir() {
			return 0, fmt.Errorf("export: %s is a file", abs)
		}
		entries, readErr := os.ReadDir(abs)
		if readErr != nil {
			return 0, fmt.Errorf("export: %w", readErr)
		}
		if len(entries) > 0 && !looksLikeFrontend(os.DirFS(abs)) {
			return 0, fmt.Errorf(
				"export: %s is not empty and does not look like a FileGator frontend; "+
					"choose an empty folder or delete the old export first", abs)
		}
		// A previous export is overwritten only when it still matches a
		// frontend this binary could have written. Anything else has been
		// customised, and replacing it would silently destroy hand-made edits —
		// the one way this command can lose work.
		if len(entries) > 0 {
			diff, diffErr := frontendDifferences(os.DirFS(abs))
			if diffErr != nil {
				return 0, fmt.Errorf("export: %w", diffErr)
			}
			if len(diff) > 0 {
				return 0, fmt.Errorf(
					"export: %s has been customised (%s); refusing to overwrite it. "+
						"Export into an empty folder, or delete the old one first",
					abs, summariseDifferences(diff))
			}
		}
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return 0, fmt.Errorf("export: %w", statErr)
	}

	written := 0
	err = fs.WalkDir(frontendRoot(), ".", func(name string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		target := filepath.Join(abs, filepath.FromSlash(name))
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, readErr := fs.ReadFile(frontendRoot(), name)
		if readErr != nil {
			return readErr
		}
		if writeErr := os.WriteFile(target, data, 0o644); writeErr != nil {
			return writeErr
		}
		written++
		return nil
	})
	if err != nil {
		return written, fmt.Errorf("export: %w", err)
	}

	// version.json records which release this interface belongs to, so a
	// later build can say whether the folder is stale instead of silently
	// serving an interface that predates it.
	meta := []byte("{\n  \"version\": \"" + version.Version + "\"\n}\n")
	if writeErr := os.WriteFile(filepath.Join(abs, versionFile), meta, 0o644); writeErr != nil {
		return written, fmt.Errorf("export: write %s: %w", versionFile, writeErr)
	}
	written++
	return written, nil
}

// looksLikeFrontend reports whether a directory contains a usable interface.
func looksLikeFrontend(root fs.FS) bool {
	for _, name := range frontendRequired {
		if _, err := fs.Stat(root, name); err != nil {
			return false
		}
	}
	return true
}

// differenceLimit bounds how many differing names are reported. A folder that
// diverged widely needs "delete it and export again" rather than a full listing.
const differenceLimit = 20

// frontendDifferences lists the files in root that do not match the embedded
// interface, in either direction: edited, missing, or added.
//
// It is the guard behind refusing to overwrite a customised export. Comparing
// contents rather than timestamps keeps the answer true across a copy, a
// checkout or a filesystem that rewrites mtimes.
func frontendDifferences(root fs.FS) ([]string, error) {
	var diff []string
	seen := make(map[string]bool)

	err := fs.WalkDir(frontendRoot(), ".", func(name string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if name == versionFile {
			// Written by the export, never part of the embedded interface.
			return nil
		}
		seen[name] = true
		want, readErr := fs.ReadFile(frontendRoot(), name)
		if readErr != nil {
			return readErr
		}
		got, readErr := fs.ReadFile(root, name)
		if readErr != nil || string(got) != string(want) {
			diff = append(diff, name)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Files that exist in the target but not in the binary are customisations
	// too: a stylesheet added by hand is exactly what must not be discarded.
	err = fs.WalkDir(root, ".", func(name string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !d.IsDir() && !seen[name] && name != versionFile {
			// version.json is the export's own metadata, not a customisation:
			// it changes whenever the binary's build does.
			diff = append(diff, name)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return diff, nil
}

// summariseDifferences renders a bounded, sorted list for an error message.
func summariseDifferences(diff []string) string {
	sorted := append([]string(nil), diff...)
	sort.Strings(sorted)
	if len(sorted) <= differenceLimit {
		return strings.Join(sorted, ", ")
	}
	return fmt.Sprintf("%s and %d more", strings.Join(sorted[:differenceLimit], ", "),
		len(sorted)-differenceLimit)
}
