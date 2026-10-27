package storage

import (
	"errors"
	"io"
	"io/fs"
	"time"
)

// Errors returned by storage backends. Backends map their own errors onto
// these so handlers can respond identically regardless of where files live.
var (
	// ErrNotFound is returned when a path does not exist.
	ErrNotFound = errors.New("storage: path not found")
	// ErrExists is returned when a target already exists.
	ErrExists = errors.New("storage: target already exists")
	// ErrOutsideRoot is returned for paths that escape the storage root.
	ErrOutsideRoot = errors.New("storage: path is outside the root")
	// ErrOutsideScope is returned for paths that escape the user's home.
	ErrOutsideScope = errors.New("storage: path is outside the user's home")
	// ErrInvalidPath is returned for malformed paths or names.
	ErrInvalidPath = errors.New("storage: invalid path")
	// ErrPermission is returned when the backend refuses access to a path.
	// Local filesystems report it when the OS denies a call; remote backends
	// map their own permission errors onto it.
	ErrPermission = errors.New("storage: permission denied")
	// ErrNotDir is returned when a directory was expected but a file was found.
	ErrNotDir = errors.New("storage: not a directory")
	// ErrUnsupported is returned when a backend cannot perform an operation,
	// such as chmod on an object store.
	ErrUnsupported = errors.New("storage: operation not supported by this backend")
)

// Entry describes a single file or directory.
type Entry struct {
	// Name is the base name, without any directory part.
	Name string
	// Path is the path relative to the user's home (or the storage root for
	// administrators), always starting with "/".
	Path string
	// Size is the file size in bytes; zero for directories.
	Size int64
	// Modified is the last modification time.
	Modified time.Time
	// IsDir reports whether the entry is a directory.
	IsDir bool
	// Mode is the permission bits.
	Mode fs.FileMode
}

// FS is the read/write surface shared by every storage backend.
type FS interface {
	Stat(p string) (*Entry, error)
	ReadDir(dir string) ([]*Entry, error)
	Open(p string) (io.ReadCloser, error)
	// Create creates a new file and fails with ErrExists when the path is
	// already taken. Saving over an existing file uses OpenWrite.
	Create(p string) (io.WriteCloser, error)
	// OpenWrite opens an existing file for writing from the start,
	// truncating it. It fails with ErrNotFound when the path is missing.
	OpenWrite(p string) (io.WriteCloser, error)
	Mkdir(p string) error
	MkdirAll(p string) error
	Rename(src, dst string) error
	Copy(src, dst string) error
	Remove(p string) error
	Chmod(p string, mode fs.FileMode) error
	Exists(p string) bool
}
