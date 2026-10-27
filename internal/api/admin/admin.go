package admin

import (
	"errors"
	"net/http"

	"github.com/filegator/filegator/internal/api/httpx"

	"github.com/filegator/filegator/internal/config"
	"github.com/filegator/filegator/internal/logger"
	"github.com/filegator/filegator/internal/router"
	"github.com/filegator/filegator/internal/storage"
	"github.com/filegator/filegator/internal/storage/local"
	"github.com/filegator/filegator/internal/version"
)

// BrowseRequest names a directory to list.
type BrowseRequest struct {
	Path string `json:"path"`
}

// RootConfig is the current storage root.
type RootConfig struct {
	// Path is the absolute path of the active root.
	Path string `json:"path"`
}

// BrowseHost handles GET /api/admin/browse?path=/srv/data and lists the
// directories inside path. Only administrators may call it, and only
// directories are returned, never file contents.
func (h *Handler) BrowseHost(w http.ResponseWriter, r *http.Request, _ router.Params) {
	dir := r.URL.Query().Get("path")
	if dir == "" {
		dir = "/"
	}
	dirs, err := local.Browse(dir)
	if err != nil {
		httpx.WriteError(w, mapBrowseError(err))
		return
	}
	httpx.WriteJSON(w, http.StatusOK, BrowseResult{
		Path:        dir,
		Directories: dirs,
	})
}

// BrowseResult is the response for the folder picker.
type BrowseResult struct {
	Path        string            `json:"path"`
	Directories []storage.DirInfo `json:"directories"`
}

// BrowseStorage handles GET /api/admin/storage/browse?path=/team and lists
// directories inside the storage root only.
//
// This backs the home-folder picker. Unlike BrowseHost it can never walk the
// host filesystem: paths are relative to the root and climbing above it is
// rejected.
func (h *Handler) BrowseStorage(w http.ResponseWriter, r *http.Request, _ router.Params) {
	dir := r.URL.Query().Get("path")
	if dir == "" {
		dir = "/"
	}

	root := h.Core.ActiveStorage()
	if root == nil {
		httpx.WriteError(w, httpx.Errorf(http.StatusServiceUnavailable, httpx.CodeInternal, "storage is not configured"))
		return
	}

	// A scope over "/" exposes the whole store through the same confined view
	// the user handlers use, so the picker sees exactly what a user would.
	fsys, err := root.Scope("/")
	if err != nil {
		httpx.WriteError(w, httpx.MapStorageError(err))
		return
	}
	entries, err := fsys.ReadDir(dir)
	if err != nil {
		httpx.WriteError(w, httpx.MapStorageError(err))
		return
	}

	clean, err := httpx.CleanPath(dir)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}

	dirs := make([]storage.DirInfo, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir {
			dirs = append(dirs, storage.DirInfo{Name: entry.Name, Path: entry.Path})
		}
	}
	httpx.WriteJSON(w, http.StatusOK, BrowseResult{Path: clean, Directories: dirs})
}

// GetRoot handles GET /api/admin/root.
func (h *Handler) GetRoot(w http.ResponseWriter, r *http.Request, _ router.Params) {
	httpx.WriteJSON(w, http.StatusOK, RootConfig{Path: h.Core.StorageDescription()})
}

// SetRoot handles POST /api/admin/root and switches the storage root
// without a restart. Existing sessions keep working.
//
// This endpoint is local-filesystem specific: it accepts a host folder. A
// non-local backend (S3, FTP) is configured elsewhere, so this endpoint is not
// offered for it.
func (h *Handler) SetRoot(w http.ResponseWriter, r *http.Request, _ router.Params) {
	if !h.Core.ActiveStorage().Capabilities().ServerBrowse {
		httpx.WriteError(w, httpx.Errorf(http.StatusBadRequest, httpx.CodeBadRequest,
			"this storage backend does not use a local folder"))
		return
	}

	var req BrowseRequest
	if err := httpx.DecodeJSON(w, r, &req, httpx.MaxJSONBody); err != nil {
		httpx.WriteError(w, err)
		return
	}

	abs, err := local.ResolveDir(req.Path)
	if err != nil {
		httpx.WriteError(w, mapBrowseError(err))
		return
	}

	// Opening the root validates that it is readable and usable.
	root, err := local.NewRoot(abs)
	if err != nil {
		httpx.WriteError(w, httpx.Errorf(http.StatusBadRequest, httpx.CodeBadRequest, "cannot use this folder: "+err.Error()))
		return
	}
	if _, err := root.Scope("/"); err != nil {
		httpx.WriteError(w, httpx.Errorf(http.StatusBadRequest, httpx.CodeBadRequest, "cannot use this folder: it is not readable"))
		return
	}

	// Persist first so a restart keeps the same root. A read-only store is
	// reported clearly: the change still applies in memory for this run.
	if err := h.Core.Config.Update(func(c *config.Config) error {
		c.Storage.Root = abs
		return nil
	}); err != nil {
		if !errors.Is(err, config.ErrReadOnly) {
			httpx.WriteError(w, err)
			return
		}
		h.Core.SetRoot(root, abs)
		httpx.WriteError(w, httpx.Errorf(http.StatusConflict, httpx.CodeReadOnlyConfig,
			"the storage root was changed for this session but cannot be saved: the configuration file is read-only"))
		return
	}
	h.Core.SetRoot(root, abs)
	h.Core.LogAdmin(r, logger.EventAdminStorageChanged, "storage root changed", logger.Fields{"root": abs})
	httpx.WriteJSON(w, http.StatusOK, RootConfig{Path: abs})
}

// readOnlyReason explains why the configuration cannot be saved.
//
// The store's own error only says "read-only"; the administrator needs the
// actual cause (a folder that cannot be created, a read-only mount, a declined
// first start) to fix it.
func readOnlyReason(store *config.Store) string {
	if why := store.Why(); why != "" {
		return why
	}
	return "the configuration could not be written"
}

// mapBrowseError converts a storage browse error into a client-safe error.
func mapBrowseError(err error) error {
	switch {
	case errors.Is(err, storage.ErrNotFound):
		return httpx.Errorf(http.StatusNotFound, httpx.CodeNotFound, "folder not found")
	case errors.Is(err, storage.ErrNotDir):
		return httpx.Errorf(http.StatusBadRequest, httpx.CodeBadRequest, "not a folder")
	case errors.Is(err, storage.ErrInvalidPath):
		return httpx.Errorf(http.StatusBadRequest, httpx.CodeBadRequest, "invalid path")
	}
	// Permission problems are common here and should be reported plainly.
	return httpx.Errorf(http.StatusForbidden, httpx.CodeForbidden, "cannot read this folder")
}

// SystemStatus reports conditions administrators should know about. It is
// admin-only because it reveals how the instance is configured.
type SystemStatus struct {
	// ConfigReadOnly is true when settings changes will not be saved.
	ConfigReadOnly bool `json:"config_read_only"`
	// UsersFileReadOnly is true when the users store cannot be written.
	UsersFileReadOnly bool `json:"users_file_read_only"`
	// DefaultAdminPassword is true while an administrator still uses the
	// shipped default password.
	DefaultAdminPassword bool `json:"default_admin_password"`
	// StorageRoot is the active storage root path.
	StorageRoot string `json:"storage_root"`
	// FrontendSource reports where the interface is served from: "embedded"
	// or "directory". The settings form uses it to tell the administrator that
	// a newly saved frontend_dir needs a restart.
	FrontendSource string `json:"frontend_source"`
	// Version is the running binary's version.
	Version string `json:"version"`
	// FrontendVersion is the version.json of a custom interface, when it has
	// one. Empty for the built-in copy, which always matches.
	FrontendVersion string `json:"frontend_version,omitempty"`
	// FrontendVersionMismatch is true when a custom interface was exported
	// for a different release than the binary.
	FrontendVersionMismatch bool `json:"frontend_version_mismatch"`
	// AuthManageUsers reports whether the admin can add/edit/delete users.
	AuthManageUsers bool `json:"auth_manage_users"`
	// AuthChangePassword reports whether a user can change their own password.
	AuthChangePassword bool `json:"auth_change_password"`
	// StorageChmod reports whether permission bits can be set.
	StorageChmod bool `json:"storage_chmod"`
	// StorageServerBrowse reports whether the admin can pick a local folder.
	StorageServerBrowse bool `json:"storage_server_browse"`
}

// Status handles GET /api/admin/status.
// frontendVersion reports the version of a custom interface, or "".
func (h *Handler) frontendVersion() string { return h.Core.StaleFrontendVersion() }

// frontendStale reports whether a custom interface is from another release.
func (h *Handler) frontendStale() bool { return h.Core.StaleFrontendVersion() != "" }

func (h *Handler) Status(w http.ResponseWriter, r *http.Request, _ router.Params) {
	authCaps := h.Core.Users.Capabilities()
	storageCaps := h.Core.ActiveStorage().Capabilities()
	httpx.WriteJSON(w, http.StatusOK, SystemStatus{
		ConfigReadOnly:          h.Core.Config.ReadOnly(),
		UsersFileReadOnly:       !h.Core.Users.Writable(),
		DefaultAdminPassword:    h.Core.Users.DefaultAdminPassword(),
		StorageRoot:             h.Core.StorageDescription(),
		FrontendSource:          h.Core.FrontendSource(),
		Version:                 version.Version,
		FrontendVersion:         h.frontendVersion(),
		FrontendVersionMismatch: h.frontendStale(),
		AuthManageUsers:         authCaps.ManageUsers,
		AuthChangePassword:      authCaps.ChangePassword,
		StorageChmod:            storageCaps.Chmod,
		StorageServerBrowse:     storageCaps.ServerBrowse,
	})
}
