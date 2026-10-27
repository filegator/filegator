package api

import "github.com/filegator/filegator/internal/storage"

// SetRoot installs the initial storage backend. New lets the caller fail fast
// at startup when the configured store is unusable.
func (s *Server) SetRoot(root storage.Storage, description string) {
	s.roots.set(root, description)
}

// activeStorage returns the current storage backend, or nil if none is set.
func (s *Server) ActiveStorage() storage.Storage {
	return s.roots.current()
}

// StorageDescription returns the human-readable identifier of the active
// storage backend (its path, or a bucket/connection label).
func (s *Server) StorageDescription() string {
	return s.roots.describe()
}
