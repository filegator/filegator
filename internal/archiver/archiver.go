// Package archiver creates and extracts zip archives over a storage backend.
//
// It works through the storage.FS interface, so archives can be built from and
// extracted into local, SFTP or S3 storage without knowing which is in use.
//
// Extraction validates every entry name: an archive containing "../" entries
// must not be able to write outside the destination folder (the "zip slip"
// vulnerability).
package archiver

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"

	"github.com/filegator/filegator/internal/storage"
)

// maxEntryName bounds an entry name. Zip files can claim enormous names and
// the limit keeps a hostile archive from exhausting memory during validation.
const maxEntryName = 4096

// Errors returned by this package.
var (
	// ErrNoEntries is returned when a call would produce an empty archive.
	ErrNoEntries = errors.New("archiver: no entries to archive")
	// ErrUnsafeEntry is returned when an archive entry would escape the
	// destination folder.
	ErrUnsafeEntry = errors.New("archiver: archive entry is outside the destination")
	// ErrInvalidArchive is returned when the payload is not a usable zip.
	ErrInvalidArchive = errors.New("archiver: not a valid zip archive")
)

// Write serialises the given paths into w as a zip.
//
// Each path is relative to the storage view and is stored under the same name,
// so archiving "/docs/a.txt" produces an entry "docs/a.txt". Directories are
// stored recursively, and empty directories are preserved.
func Write(fsys storage.FS, w io.Writer, paths []string) error {
	if len(paths) == 0 {
		return ErrNoEntries
	}
	zw := zip.NewWriter(w)
	for _, p := range paths {
		clean, err := storage.CleanPath(p)
		if err != nil {
			return err
		}
		if err := addPath(zw, fsys, clean, strings.TrimPrefix(clean, "/")); err != nil {
			return err
		}
	}
	return zw.Close()
}

// addPath adds one file or directory tree to the archive.
func addPath(zw *zip.Writer, fsys storage.FS, virtual, name string) error {
	entry, err := fsys.Stat(virtual)
	if err != nil {
		return err
	}

	if !entry.IsDir {
		return addFile(zw, fsys, virtual, name)
	}

	// A trailing slash marks a directory entry, which preserves empty folders.
	if _, err := zw.Create(name + "/"); err != nil {
		return err
	}
	children, err := fsys.ReadDir(virtual)
	if err != nil {
		return err
	}
	for _, child := range children {
		if err := addPath(zw, fsys, child.Path, strings.TrimPrefix(child.Path, "/")); err != nil {
			return err
		}
	}
	return nil
}

// addFile copies one file into the archive.
func addFile(zw *zip.Writer, fsys storage.FS, virtual, name string) error {
	src, err := fsys.Open(virtual)
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := zw.Create(name)
	if err != nil {
		return err
	}
	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("archiver: write %s: %w", name, err)
	}
	return nil
}

// Extract unpacks the zip read from r into destDir, returning the entries it
// created.
//
// Every entry name is validated: absolute paths, ".." segments and names that
// climb above destDir are rejected before anything is written.
func Extract(fsys storage.FS, r io.ReaderAt, size int64, destDir string) ([]*storage.Entry, error) {
	cleanDest, err := storage.CleanPath(destDir)
	if err != nil {
		return nil, err
	}

	zr, err := zip.NewReader(r, size)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidArchive, err)
	}

	created := make([]*storage.Entry, 0, len(zr.File))
	for _, file := range zr.File {
		target, err := entryTarget(cleanDest, file.Name)
		if err != nil {
			return nil, err
		}
		if target == cleanDest {
			// A top-level entry with no name; skip it.
			continue
		}

		if file.FileInfo().IsDir() {
			if err := ensureDir(fsys, target); err != nil {
				return nil, err
			}
			continue
		}

		// Directories implied by the file path may not exist yet.
		if err := ensureDir(fsys, path.Dir(target)); err != nil {
			return nil, err
		}
		if err := extractFile(fsys, file, target); err != nil {
			return nil, err
		}

		entry, err := fsys.Stat(target)
		if err != nil {
			return nil, err
		}
		created = append(created, entry)
	}
	return created, nil
}

// entryTarget resolves an archive entry name inside destDir, rejecting any
// attempt to escape it.
func entryTarget(destDir, name string) (string, error) {
	if len(name) > maxEntryName {
		return "", fmt.Errorf("%w: entry name is too long", ErrUnsafeEntry)
	}
	// Backslashes are treated as separators, NUL bytes are rejected.
	if strings.ContainsRune(name, 0) {
		return "", fmt.Errorf("%w: %q", ErrUnsafeEntry, name)
	}
	normalised := strings.ReplaceAll(name, `\`, "/")

	// An absolute entry name would ignore the destination entirely.
	if strings.HasPrefix(normalised, "/") {
		return "", fmt.Errorf("%w: absolute entry %q", ErrUnsafeEntry, name)
	}

	// Count depth so a name cannot climb above the destination.
	depth := 0
	for _, seg := range strings.Split(normalised, "/") {
		switch seg {
		case "", ".":
			continue
		case "..":
			if depth == 0 {
				return "", fmt.Errorf("%w: entry %q climbs above the destination", ErrUnsafeEntry, name)
			}
			depth--
		default:
			depth++
		}
	}

	// path.Join drops the trailing slash of a directory entry, so "docs/"
	// resolves to the same path "docs" would.
	cleaned := path.Clean(path.Join(destDir, normalised))
	// Extracting into "/" has no prefix to compare against: every absolute
	// path is already inside the root.
	if destDir != "/" && cleaned != destDir && !strings.HasPrefix(cleaned, destDir+"/") {
		return "", fmt.Errorf("%w: entry %q", ErrUnsafeEntry, name)
	}
	return cleaned, nil
}

// ensureDir creates a directory, tolerating one that already exists.
func ensureDir(fsys storage.FS, dir string) error {
	if dir == "/" || dir == "." {
		return nil
	}
	if fsys.Exists(dir) {
		return nil
	}
	if err := fsys.MkdirAll(dir); err != nil && !errors.Is(err, storage.ErrExists) {
		return err
	}
	return nil
}

// extractFile writes one archive entry to the storage.
func extractFile(fsys storage.FS, file *zip.File, target string) error {
	src, err := file.Open()
	if err != nil {
		return fmt.Errorf("archiver: open %s: %w", file.Name, err)
	}
	defer src.Close()

	dst, err := fsys.Create(target)
	if err != nil {
		return err
	}
	if _, err := io.Copy(dst, src); err != nil {
		dst.Close()
		return fmt.Errorf("archiver: write %s: %w", target, err)
	}
	return dst.Close()
}

// SuggestFolder returns a folder name derived from an archive name, used when
// extracting into the archive's own directory: "photos.zip" gives "photos".
func SuggestFolder(archiveName string) string {
	base := path.Base(archiveName)
	// A leading dot is part of the name, not an extension marker, so ".zip"
	// yields nothing usable and falls back below.
	if i := strings.LastIndex(base, "."); i > 0 {
		base = base[:i]
	}
	// A non-empty, safe folder name is required; fall back when the name has
	// nothing usable left.
	if err := storage.ValidateName(base); err != nil {
		return "extracted"
	}
	return base
}
