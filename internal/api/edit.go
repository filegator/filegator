package api

import (
	"io"
	"net/http"
	"time"

	"github.com/filegator/filegator/internal/auth"
	"github.com/filegator/filegator/internal/router"
)

// maxEditSize is the largest file the editor will open. Files bigger than this
// are better downloaded than edited in a textarea.
const maxEditSize = 2 << 20 // 2 MB

// EditContent is the response for the editor.
type EditContent struct {
	Path string `json:"path"`
	// Name is the base name, for display.
	Name string `json:"name"`
	// Content is the file text.
	Content string `json:"content"`
	// Modified is the RFC 3339 modification time.
	Modified string `json:"modified"`
}

// ReadContent handles GET /api/files/content?path=/notes.md and returns the
// text of one editable file.
//
// This is separate from /api/files/download because the editor needs the raw
// text as JSON and must not be able to read binary files or anything outside
// the editable extensions.
func (s *Server) ReadContent(w http.ResponseWriter, r *http.Request, u *auth.User) {
	fsys, err := s.requirePermission(u, auth.PermRead)
	if err != nil {
		WriteError(w, err)
		return
	}

	p := r.URL.Query().Get("path")
	if p == "" {
		WriteError(w, Errorf(http.StatusBadRequest, CodeBadRequest, "path is required"))
		return
	}
	if !s.isEditable(p) {
		WriteError(w, Errorf(http.StatusBadRequest, CodeBadRequest, "this file type is not editable"))
		return
	}

	entry, err := fsys.Stat(p)
	if err != nil {
		WriteError(w, mapStorageError(err))
		return
	}
	if entry.IsDir {
		WriteError(w, Errorf(http.StatusBadRequest, CodeBadRequest, "cannot edit a directory"))
		return
	}
	if entry.Size > maxEditSize {
		WriteError(w, Errorf(http.StatusRequestEntityTooLarge, CodeBadRequest, "file is too large to edit"))
		return
	}

	f, err := fsys.Open(p)
	if err != nil {
		WriteError(w, mapStorageError(err))
		return
	}
	defer f.Close()

	// One byte of slack detects a file that grew between Stat and Open.
	data, err := io.ReadAll(io.LimitReader(f, maxEditSize+1))
	if err != nil {
		WriteError(w, Errorf(http.StatusInternalServerError, CodeInternal, "cannot read the file"))
		return
	}
	if len(data) > maxEditSize {
		WriteError(w, Errorf(http.StatusRequestEntityTooLarge, CodeBadRequest, "file is too large to edit"))
		return
	}
	if isBinary(data) {
		WriteError(w, Errorf(http.StatusBadRequest, CodeBadRequest, "this file is not text"))
		return
	}

	WriteJSON(w, http.StatusOK, EditContent{
		Path:     entry.Path,
		Name:     entry.Name,
		Content:  string(data),
		Modified: entry.Modified.UTC().Format(time.RFC3339),
	})
}

// isBinary reports whether data looks like binary content. A NUL byte in the
// first kilobyte is the usual heuristic and catches executables, images and
// archives reliably enough for an editor.
func isBinary(data []byte) bool {
	limit := len(data)
	if limit > 1024 {
		limit = 1024
	}
	for _, b := range data[:limit] {
		if b == 0 {
			return true
		}
	}
	return false
}

// EditRoutes registers the editor endpoint.
func (s *Server) EditRoutes(rt *router.Router) {
	rt.Get("/api/files/content", s.withUser(s.ReadContent))
}
