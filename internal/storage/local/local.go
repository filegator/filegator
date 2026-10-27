// Package local implements the storage backend for a directory tree on the
// local filesystem.
//
// A Root is the administrator's view of the whole storage. A Scope is a
// user's confined view: every path is interpreted relative to the scope's
// home folder, and requests that try to leave it — via ".." or symlinks — are
// rejected with storage.ErrOutsideScope.
package local

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/filegator/filegator/internal/storage"
)

// Root grants unrestricted access to a directory tree.
type Root struct {
	view
	dir string
}

// NewRoot returns a Root for dir, creating the directory if needed.
func NewRoot(dir string) (*Root, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("storage: resolve root: %w", err)
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return nil, fmt.Errorf("storage: create root: %w", err)
	}
	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return nil, fmt.Errorf("storage: resolve root symlinks: %w", err)
	}
	return &Root{
		view: view{base: abs, real: real},
		dir:  abs,
	}, nil
}

// Dir returns the absolute path of the root directory.
func (r *Root) Dir() string { return r.dir }

// Description returns the absolute path of the root directory. It satisfies
// the Storage interface; a non-local backend returns a different label.
func (r *Root) Description() string { return r.dir }

// Capabilities reports the operations a local filesystem supports.
func (r *Root) Capabilities() storage.Caps {
	return storage.Caps{Chmod: true, ServerBrowse: true}
}

// Scope returns a user's confined view of the tree. home is relative to the
// root; "/" gives the whole tree, which is appropriate for administrators.
func (r *Root) Scope(home string) (storage.FS, error) {
	clean, err := r.clean(home)
	if err != nil {
		return nil, err
	}
	abs, err := r.abs(clean)
	if err != nil {
		return nil, err
	}
	// The home folder must exist and must not itself escape via symlinks.
	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, storage.ErrNotFound
		}
		return nil, storage.ErrInvalidPath
	}
	if !withinRoot(r.real, real) {
		return nil, storage.ErrOutsideScope
	}
	return &Scope{view: view{base: abs, real: real, confined: true}, home: clean}, nil
}

// EnsureDir creates a directory inside the root. It returns storage.ErrExists if the
// directory is already present, so callers can tell "created" from "reused".
// It is used when provisioning a home folder for a new user.
func (r *Root) EnsureDir(p string) error {
	clean, err := r.clean(p)
	if err != nil {
		return err
	}
	abs, err := r.abs(clean)
	if err != nil {
		return err
	}
	if _, err := os.Lstat(abs); err == nil {
		return storage.ErrExists
	}

	// Parent folders may not exist yet, so walk up to the first existing
	// ancestor and verify that it resolves inside the root. This blocks
	// creating a directory through a symlink that points outside.
	existing := filepath.Dir(abs)
	for {
		if _, err := os.Lstat(existing); err == nil {
			break
		}
		parent := filepath.Dir(existing)
		if parent == existing {
			return storage.ErrNotFound
		}
		existing = parent
	}
	if _, err := r.realWithin(existing); err != nil {
		return err
	}
	return os.MkdirAll(abs, 0o755)
}

// Scope is a user's confined view of the storage.
type Scope struct {
	view
	home string
}

// Home returns the home folder path relative to the storage root.
func (s *Scope) Home() string { return s.home }

// Ensure the concrete types satisfy the interfaces.
var (
	_ storage.FS      = (*Root)(nil)
	_ storage.FS      = (*Scope)(nil)
	_ storage.Storage = (*Root)(nil)
)
