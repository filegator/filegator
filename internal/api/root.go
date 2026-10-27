package api

import (
	"sync"

	"github.com/filegator/filegator/internal/storage"
)

// rootHolder stores the active storage backend. The administrator can change
// it while the server is running, so access is synchronized.
type rootHolder struct {
	mu          sync.RWMutex
	storage     storage.Storage
	description string
}

// current returns the active backend, or nil when none has been set.
func (h *rootHolder) current() storage.Storage {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.storage
}

// set replaces the active backend and records its description.
func (h *rootHolder) set(s storage.Storage, description string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.storage = s
	h.description = description
}

// description returns the human-readable identifier of the active backend.
func (h *rootHolder) describe() string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.description
}
