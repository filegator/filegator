package local

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/filegator/filegator/internal/storage"
)

// Browse lists the immediate subdirectories of an absolute server path.
//
// It exists so administrators can pick a storage root or a user's home folder
// anywhere on the machine. Only directories are returned, never file contents,
// so the endpoint cannot be used to read data.
func Browse(dir string) ([]storage.DirInfo, error) {
	if dir == "" {
		dir = string(filepath.Separator)
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, storage.ErrInvalidPath
	}
	info, err := os.Stat(abs)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, storage.ErrNotFound
		}
		return nil, err
	}
	if !info.IsDir() {
		return nil, storage.ErrNotDir
	}
	items, err := os.ReadDir(abs)
	if err != nil {
		return nil, err
	}

	out := make([]storage.DirInfo, 0, len(items))
	for _, item := range items {
		full := filepath.Join(abs, item.Name())
		// DirEntry.IsDir reports false for symlinks, so resolve them: a link
		// to a directory is a valid choice, a broken link is not.
		isDir := item.IsDir()
		if !isDir && item.Type()&fs.ModeSymlink != 0 {
			if target, err := os.Stat(full); err == nil && target.IsDir() {
				isDir = true
			}
		}
		if !isDir {
			continue
		}
		out = append(out, storage.DirInfo{Name: item.Name(), Path: full})
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out, nil
}

// ResolveDir validates that dir exists and is a directory, returning its
// absolute path. It is used when the admin selects a storage root.
func ResolveDir(dir string) (string, error) {
	if strings.TrimSpace(dir) == "" {
		return "", storage.ErrInvalidPath
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", storage.ErrInvalidPath
	}
	info, err := os.Stat(abs)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", storage.ErrNotFound
		}
		return "", err
	}
	if !info.IsDir() {
		return "", storage.ErrNotDir
	}
	return abs, nil
}
