// Package storage defines the backend-agnostic storage surface.
//
// It holds the shared types — Entry, the FS interface, the Storage interface,
// capabilities and the sentinel errors — while each backend lives in its own
// subpackage: storage/local for the local filesystem, storage/sftp for SSH
// file transfer.
package storage

// Caps describes which operations a storage backend supports. Operations are
// omitted (false) where they are meaningless or unimplemented, so callers can
// hide the corresponding UI instead of showing buttons that fail.
type Caps struct {
	// Chmod reports whether permission bits can be set. Local filesystems
	// support it; object stores such as S3 do not.
	Chmod bool
	// ServerBrowse reports whether the admin can pick a storage root by
	// browsing the host filesystem. True for local storage; false for remote
	// backends, where the root is connection configuration instead of a folder.
	ServerBrowse bool
}

// Storage is the backend-agnostic view of the file store. A backend that
// serves another filesystem (SFTP, S3, ...) implements this instead of using a
// local Root directly.
type Storage interface {
	// Scope returns a confined view rooted at home. "/" is the whole store.
	Scope(home string) (FS, error)
	// EnsureDir creates a directory, used to provision home folders.
	EnsureDir(path string) error
	// Description returns a human-readable identifier for the store: the
	// local path, or something like "sftp://host/prefix".
	Description() string
	// Capabilities reports which operations this backend supports.
	Capabilities() Caps
}

// DirInfo describes a directory offered to the admin folder picker.
type DirInfo struct {
	Name string `json:"name"`
	Path string `json:"path"`
}
