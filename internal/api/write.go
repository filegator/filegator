package api

import (
	"io"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/filegator/filegator/internal/api/httpx"
	"github.com/filegator/filegator/internal/auth"
	"github.com/filegator/filegator/internal/logger"
	"github.com/filegator/filegator/internal/router"
	"github.com/filegator/filegator/internal/storage"
)

// maxJSONBody bounds state-changing request bodies.
const maxJSONBody = httpx.MaxJSONBody

// maxSaveSize bounds the content accepted by the editor endpoint.
const maxSaveSize = 8 * 1024 * 1024

// PathRequest names a single target.
type PathRequest struct {
	Path string `json:"path"`
}

// MoveRequest names a source and destination.
type MoveRequest struct {
	From string `json:"from"`
	To   string `json:"to"`
	// OnCollision is "refuse" (default), "replace" or "upcount".
	OnCollision string `json:"on_collision"`
}

// CreateRequest creates a file or directory.
type CreateRequest struct {
	// Parent is the directory to create in; defaults to "/".
	Parent string `json:"parent"`
	// Name is the new entry's name.
	Name string `json:"name"`
	// OnCollision is "refuse" (default), "replace" or "upcount".
	OnCollision string `json:"on_collision"`
}

// ChmodRequest changes permission bits.
type ChmodRequest struct {
	Paths []string `json:"paths"`
	// Mode is an octal string such as "755" or "0644".
	Mode string `json:"mode"`
	// Recursive applies the mode to a folder's contents too: "all", "folders"
	// or "files". Empty changes only the listed entries. Ported from v7.
	Recursive string `json:"recursive"`
}

// SaveRequest writes text content to a file.
type SaveRequest struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// write creates a state-changing handler that validates CSRF and requires
// the given permissions before calling fn.
func (s *Server) write(perms []auth.Permission, fn func(w http.ResponseWriter, r *http.Request, u *auth.User, fsys storage.FS)) router.Handler {
	return func(w http.ResponseWriter, r *http.Request, p router.Params) {
		u, err := s.CurrentUser(r)
		if err != nil {
			WriteError(w, err)
			return
		}
		if err := s.RequireCSRF(r); err != nil {
			WriteError(w, err)
			return
		}
		fsys, err := s.requirePermission(u, perms...)
		if err != nil {
			WriteError(w, err)
			return
		}
		fn(w, r, u, fsys)
	}
}

// decodePath reads a PathRequest and rejects empty paths.
func decodePath(w http.ResponseWriter, r *http.Request) (string, bool) {
	var req PathRequest
	if err := DecodeJSON(w, r, &req, maxJSONBody); err != nil {
		WriteError(w, err)
		return "", false
	}
	if strings.TrimSpace(req.Path) == "" {
		WriteError(w, Errorf(http.StatusBadRequest, CodeBadRequest, "path is required"))
		return "", false
	}
	return req.Path, true
}

// Mkdir handles POST /api/files/mkdir.
func (s *Server) Mkdir(w http.ResponseWriter, r *http.Request, u *auth.User, fsys storage.FS) {
	var req CreateRequest
	if err := DecodeJSON(w, r, &req, maxJSONBody); err != nil {
		WriteError(w, err)
		return
	}
	target, err := targetPath(req.Parent, req.Name)
	if err != nil {
		WriteError(w, err)
		return
	}
	policy, err := normalizeCollision(req.OnCollision)
	if err != nil {
		WriteError(w, err)
		return
	}
	// "replace" is meaningless for a folder: deleting a whole tree to create
	// one in its place is never what the caller means.
	if policy == CollisionReplace {
		WriteError(w, Errorf(http.StatusBadRequest, CodeBadRequest,
			"on_collision replace cannot be used for a folder"))
		return
	}
	target, renamedFrom, err := resolveTarget(fsys, target, policy)
	if err != nil {
		WriteError(w, err)
		return
	}
	if err := fsys.Mkdir(target); err != nil {
		WriteError(w, mapStorageError(err))
		return
	}
	e, err := fsys.Stat(target)
	if err != nil {
		WriteError(w, mapStorageError(err))
		return
	}
	s.logEvent(r, u, logger.EventFileCreated, "folder created",
		logger.Fields{"target": target, "renamed_from": renamedFrom})
	WriteJSON(w, http.StatusCreated, toEntry(e))
}

// Create handles POST /api/files/create and returns an empty file.
func (s *Server) Create(w http.ResponseWriter, r *http.Request, u *auth.User, fsys storage.FS) {
	var req CreateRequest
	if err := DecodeJSON(w, r, &req, maxJSONBody); err != nil {
		WriteError(w, err)
		return
	}
	target, err := targetPath(req.Parent, req.Name)
	if err != nil {
		WriteError(w, err)
		return
	}
	policy, err := normalizeCollision(req.OnCollision)
	if err != nil {
		WriteError(w, err)
		return
	}
	target, renamedFrom, err := resolveTarget(fsys, target, policy)
	if err != nil {
		WriteError(w, err)
		return
	}
	f, err := fsys.Create(target)
	if err != nil {
		WriteError(w, mapStorageError(err))
		return
	}
	if err := f.Close(); err != nil {
		WriteError(w, err)
		return
	}
	e, err := fsys.Stat(target)
	if err != nil {
		WriteError(w, mapStorageError(err))
		return
	}
	s.logEvent(r, u, logger.EventFileCreated, "file created",
		logger.Fields{"target": target, "renamed_from": renamedFrom})
	WriteJSON(w, http.StatusCreated, toEntry(e))
}

// Rename handles POST /api/files/rename. It also performs moves when the
// destination is in a different directory.
func (s *Server) Rename(w http.ResponseWriter, r *http.Request, u *auth.User, fsys storage.FS) {
	var req MoveRequest
	if err := DecodeJSON(w, r, &req, maxJSONBody); err != nil {
		WriteError(w, err)
		return
	}
	if strings.TrimSpace(req.From) == "" || strings.TrimSpace(req.To) == "" {
		WriteError(w, Errorf(http.StatusBadRequest, CodeBadRequest, "from and to are required"))
		return
	}
	policy, err := normalizeCollision(req.OnCollision)
	if err != nil {
		WriteError(w, err)
		return
	}
	to, renamedFrom, err := resolveTarget(fsys, req.To, policy)
	if err != nil {
		WriteError(w, err)
		return
	}
	if err := fsys.Rename(req.From, to); err != nil {
		WriteError(w, mapStorageError(err))
		return
	}
	e, err := fsys.Stat(to)
	if err != nil {
		WriteError(w, mapStorageError(err))
		return
	}
	s.logEvent(r, u, logger.EventFileRenamed, "entry renamed or moved",
		logger.Fields{"from": req.From, "to": to, "renamed_from": renamedFrom})
	WriteJSON(w, http.StatusOK, toEntry(e))
}

// MoveInRequest describes moving entries into a directory.
type MoveInRequest struct {
	Paths []string `json:"paths"`
	// Target is the destination directory.
	Target string `json:"target"`
	// OnCollision is "refuse" (default), "replace" or "upcount".
	OnCollision string `json:"on_collision"`
}

// Move handles POST /api/files/move, moving entries into a directory.
func (s *Server) Move(w http.ResponseWriter, r *http.Request, u *auth.User, fsys storage.FS) {
	var req MoveInRequest
	if err := DecodeJSON(w, r, &req, maxJSONBody); err != nil {
		WriteError(w, err)
		return
	}
	target := req.Target
	if target == "" {
		target = "/"
	}
	if len(req.Paths) == 0 {
		WriteError(w, Errorf(http.StatusBadRequest, CodeBadRequest, "paths is required"))
		return
	}
	policy, err := normalizeCollision(req.OnCollision)
	if err != nil {
		WriteError(w, err)
		return
	}
	moved := make([]Entry, 0, len(req.Paths))
	for _, src := range req.Paths {
		dst, _, err := resolveTarget(fsys, path.Join(target, path.Base(src)), policy)
		if err != nil {
			WriteError(w, err)
			return
		}
		if err := fsys.Rename(src, dst); err != nil {
			WriteError(w, mapStorageError(err))
			return
		}
		e, err := fsys.Stat(dst)
		if err != nil {
			WriteError(w, mapStorageError(err))
			return
		}
		moved = append(moved, toEntry(e))
	}
	s.logEvent(r, u, logger.EventFileRenamed, "entries moved",
		logger.Fields{"paths": req.Paths, "target": target})
	WriteJSON(w, http.StatusOK, moved)
}

// CopyItemsRequest describes copying entries into a directory.
type CopyItemsRequest = MoveInRequest

// Copy handles POST /api/files/copy.
func (s *Server) Copy(w http.ResponseWriter, r *http.Request, u *auth.User, fsys storage.FS) {
	var req CopyItemsRequest
	if err := DecodeJSON(w, r, &req, maxJSONBody); err != nil {
		WriteError(w, err)
		return
	}
	target := req.Target
	if target == "" {
		target = "/"
	}
	if len(req.Paths) == 0 {
		WriteError(w, Errorf(http.StatusBadRequest, CodeBadRequest, "paths is required"))
		return
	}
	policy, err := normalizeCollision(req.OnCollision)
	if err != nil {
		WriteError(w, err)
		return
	}
	copied := make([]Entry, 0, len(req.Paths))
	for _, src := range req.Paths {
		dst, _, err := resolveTarget(fsys, path.Join(target, path.Base(src)), policy)
		if err != nil {
			WriteError(w, err)
			return
		}
		if err := fsys.Copy(src, dst); err != nil {
			WriteError(w, mapStorageError(err))
			return
		}
		e, err := fsys.Stat(dst)
		if err != nil {
			WriteError(w, mapStorageError(err))
			return
		}
		copied = append(copied, toEntry(e))
	}
	s.logEvent(r, u, logger.EventFileCopied, "entries copied",
		logger.Fields{"paths": req.Paths, "target": target})
	WriteJSON(w, http.StatusOK, copied)
}

// DeleteRequest deletes one or more entries.
type DeleteRequest struct {
	Paths []string `json:"paths"`
}

// Delete handles POST /api/files/delete. Deletion is permanent.
func (s *Server) Delete(w http.ResponseWriter, r *http.Request, u *auth.User, fsys storage.FS) {
	var req DeleteRequest
	if err := DecodeJSON(w, r, &req, maxJSONBody); err != nil {
		WriteError(w, err)
		return
	}
	if len(req.Paths) == 0 {
		WriteError(w, Errorf(http.StatusBadRequest, CodeBadRequest, "paths is required"))
		return
	}
	for _, p := range req.Paths {
		if err := fsys.Remove(p); err != nil {
			WriteError(w, mapStorageError(err))
			return
		}
	}
	s.logEvent(r, u, logger.EventFileDeleted, "entries deleted", logger.Fields{"paths": req.Paths})
	WriteNoContent(w)
}

// Chmod handles POST /api/files/chmod. The chmod permission is required in
// addition to write.
func (s *Server) Chmod(w http.ResponseWriter, r *http.Request, u *auth.User, fsys storage.FS) {
	var req ChmodRequest
	if err := DecodeJSON(w, r, &req, maxJSONBody); err != nil {
		WriteError(w, err)
		return
	}
	mode, err := parseMode(req.Mode)
	if err != nil {
		WriteError(w, err)
		return
	}
	if len(req.Paths) == 0 {
		WriteError(w, Errorf(http.StatusBadRequest, CodeBadRequest, "paths is required"))
		return
	}
	recursive := strings.ToLower(strings.TrimSpace(req.Recursive))
	switch recursive {
	case "", "all", "folders", "files":
	default:
		WriteError(w, Errorf(http.StatusBadRequest, CodeBadRequest,
			"recursive must be empty, all, folders or files"))
		return
	}

	for _, p := range req.Paths {
		// With a recursive scope the listed entry follows that scope too:
		// "files" must not change the folder the user picked, or selecting a
		// folder would silently change the folder itself.
		applyToListed := recursive == ""
		if recursive == "all" || recursive == "folders" {
			applyToListed = true
		}
		if applyToListed {
			if err := fsys.Chmod(p, mode); err != nil {
				WriteError(w, mapStorageError(err))
				return
			}
		}
		if recursive != "" {
			if err := chmodTree(fsys, p, mode, recursive); err != nil {
				WriteError(w, mapStorageError(err))
				return
			}
		}
	}
	s.logEvent(r, u, logger.EventFileChmod, "permission changed",
		logger.Fields{"paths": req.Paths, "mode": req.Mode, "recursive": recursive})
	WriteNoContent(w)
}

// chmodTree applies a mode to everything below dir.
//
// which decides what is touched: "all", "folders" or "files". A failure on one
// entry is skipped rather than aborting the walk, matching v7: changing a
// whole tree must not stop at the first file the server refuses to change.
func chmodTree(fsys storage.FS, dir string, mode os.FileMode, which string) error {
	entries, err := fsys.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		// The mode is applied before descending: removing read permission from
		// a folder first would make its contents unreachable a moment later.
		wanted := which == "all" ||
			(which == "folders" && entry.IsDir) ||
			(which == "files" && !entry.IsDir)
		if entry.IsDir && which != "files" {
			// Descend first for folders so the walk is unaffected by the new
			// mode on the folder itself.
			_ = chmodTree(fsys, entry.Path, mode, which)
		}
		if wanted {
			_ = fsys.Chmod(entry.Path, mode)
		}
	}
	return nil
}

// parseMode accepts a three or four digit octal string.
func parseMode(s string) (os.FileMode, error) {
	raw := strings.TrimSpace(s)
	if raw == "" {
		return 0, Errorf(http.StatusBadRequest, CodeBadRequest, "mode is required")
	}
	if len(raw) == 4 && raw[0] == '0' {
		raw = raw[1:]
	}
	if len(raw) != 3 {
		return 0, Errorf(http.StatusBadRequest, CodeBadRequest, "mode must be three octal digits")
	}
	n, err := strconv.ParseUint(raw, 8, 32)
	if err != nil || n > 0o777 {
		return 0, Errorf(http.StatusBadRequest, CodeBadRequest, "mode must be three octal digits")
	}
	return os.FileMode(n), nil
}

// Save handles POST /api/files/save, writing text content to a file.
func (s *Server) Save(w http.ResponseWriter, r *http.Request, u *auth.User, fsys storage.FS) {
	var req SaveRequest
	if err := DecodeJSON(w, r, &req, maxSaveSize); err != nil {
		WriteError(w, err)
		return
	}
	if strings.TrimSpace(req.Path) == "" {
		WriteError(w, Errorf(http.StatusBadRequest, CodeBadRequest, "path is required"))
		return
	}
	if !s.isEditable(req.Path) {
		WriteError(w, Errorf(http.StatusBadRequest, CodeBadRequest, "this file type is not editable"))
		return
	}
	// Save rewrites a file, so it opens the existing one for writing instead
	// of creating: Create is exclusive now, which would refuse every save.
	f, err := fsys.OpenWrite(req.Path)
	if err != nil {
		WriteError(w, mapStorageError(err))
		return
	}
	if _, err := io.WriteString(f, req.Content); err != nil {
		f.Close()
		WriteError(w, err)
		return
	}
	if err := f.Close(); err != nil {
		WriteError(w, err)
		return
	}
	e, err := fsys.Stat(req.Path)
	if err != nil {
		WriteError(w, mapStorageError(err))
		return
	}
	s.logEvent(r, u, logger.EventFileSaved, "file saved",
		logger.Fields{"target": req.Path, "bytes": len(req.Content)})
	WriteJSON(w, http.StatusOK, toEntry(e))
}

// isEditable reports whether the extension may be written by the editor.
func (s *Server) isEditable(name string) bool {
	if name == "" {
		return false
	}
	ext := strings.ToLower(path.Ext(name))
	if ext == "" {
		return false
	}
	for _, allowed := range s.Config.Get().Frontend.Editable {
		if strings.EqualFold(allowed, ext) {
			return true
		}
	}
	return false
}

// targetPath validates a parent directory and a new entry name.
//
// path.Join silently collapses ".." segments, so the parent is checked for
// traversal explicitly. Otherwise "/../bob" would become "/bob" and escape
// the home without an error.
func targetPath(parent, name string) (string, error) {
	if err := storage.ValidateName(name); err != nil {
		return "", Errorf(http.StatusBadRequest, CodeBadRequest, "invalid name")
	}
	if parent == "" {
		parent = "/"
	}
	if !strings.HasPrefix(parent, "/") {
		parent = "/" + parent
	}
	// Reject traversal in the parent. path.Join would silently collapse
	// ".." segments, turning "/../bob" into "/bob". Backslashes are treated
	// as separators too, matching the storage layer.
	parent = strings.ReplaceAll(parent, `\`, "/")
	for _, seg := range strings.Split(parent, "/") {
		if seg == ".." {
			return "", Errorf(http.StatusForbidden, CodeForbidden, "path is outside your home folder")
		}
	}
	return path.Join(parent, name), nil
}

// WriteRoutes registers state-changing file endpoints on rt.
func (s *Server) WriteRoutes(rt *router.Router) {
	write := []auth.Permission{auth.PermRead, auth.PermWrite}
	rt.Post("/api/files/mkdir", s.write(write, s.Mkdir))
	rt.Post("/api/files/create", s.write(write, s.Create))
	rt.Post("/api/files/rename", s.write(write, s.Rename))
	rt.Post("/api/files/move", s.write(write, s.Move))
	rt.Post("/api/files/copy", s.write(write, s.Copy))
	rt.Post("/api/files/delete", s.write(write, s.Delete))
	rt.Post("/api/files/chmod", s.write([]auth.Permission{auth.PermRead, auth.PermWrite, auth.PermChmod}, s.Chmod))
	rt.Post("/api/files/save", s.write(write, s.Save))
}
