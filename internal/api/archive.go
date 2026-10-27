package api

import (
	"errors"
	"io"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/filegator/filegator/internal/archiver"
	"github.com/filegator/filegator/internal/auth"
	"github.com/filegator/filegator/internal/logger"
	"github.com/filegator/filegator/internal/router"
	"github.com/filegator/filegator/internal/storage"
)

// ZipRequest asks for an archive to be created in the storage.
type ZipRequest struct {
	// Paths are the entries to include.
	Paths []string `json:"paths"`
	// Destination is the folder to write the archive into; "/" by default.
	Destination string `json:"destination"`
	// Name is the archive file name. Empty means the configured default.
	Name string `json:"name"`
}

// UnzipRequest asks for an archive to be extracted.
type UnzipRequest struct {
	// Path is the archive to extract.
	Path string `json:"path"`
	// Destination is where to extract; "/" by default.
	Destination string `json:"destination"`
	// Folder, when set, is a subfolder created inside Destination. Empty uses
	// the archive's base name, matching v7's behaviour of never unpacking
	// loose files into the current folder.
	Folder string `json:"folder"`
}

// Zip handles POST /api/files/zip and creates an archive in place.
func (s *Server) Zip(w http.ResponseWriter, r *http.Request, u *auth.User) {
	fsys, err := s.requirePermission(u, auth.PermRead, auth.PermWrite, auth.PermZip)
	if err != nil {
		WriteError(w, err)
		return
	}
	if err := s.RequireCSRF(r); err != nil {
		WriteError(w, err)
		return
	}

	var req ZipRequest
	if err := DecodeJSON(w, r, &req, maxJSONBody); err != nil {
		WriteError(w, err)
		return
	}
	if len(req.Paths) == 0 {
		WriteError(w, Errorf(http.StatusBadRequest, CodeBadRequest, "paths is required"))
		return
	}

	dest := req.Destination
	if dest == "" {
		dest = "/"
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = s.Config.Get().Frontend.DefaultArchiveName
	}
	if name == "" {
		name = "archive.zip"
	}
	if err := storage.ValidateName(name); err != nil {
		WriteError(w, Errorf(http.StatusBadRequest, CodeBadRequest, "invalid archive name"))
		return
	}

	target := path.Join(dest, name)
	if fsys.Exists(target) {
		WriteError(w, Errorf(http.StatusConflict, CodeConflict, "target already exists"))
		return
	}

	// Write into a temporary file first: the archive size is unknown until it
	// is complete, and a failure must not leave a half-written zip in the
	// user's folder.
	tmp, err := os.CreateTemp("", "filegator-zip-*")
	if err != nil {
		WriteError(w, Errorf(http.StatusInternalServerError, CodeInternal, "cannot create the archive"))
		return
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()

	if err := archiver.Write(fsys, tmp, req.Paths); err != nil {
		WriteError(w, mapArchiveError(err))
		return
	}
	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		WriteError(w, Errorf(http.StatusInternalServerError, CodeInternal, "cannot read the archive"))
		return
	}

	dst, err := fsys.Create(target)
	if err != nil {
		WriteError(w, mapStorageError(err))
		return
	}
	if _, err := io.Copy(dst, tmp); err != nil {
		dst.Close()
		WriteError(w, Errorf(http.StatusInternalServerError, CodeInternal, "cannot store the archive"))
		return
	}
	if err := dst.Close(); err != nil {
		WriteError(w, Errorf(http.StatusInternalServerError, CodeInternal, "cannot store the archive"))
		return
	}

	entry, err := fsys.Stat(target)
	if err != nil {
		WriteError(w, mapStorageError(err))
		return
	}
	s.logEvent(r, u, logger.EventZip, "archive created",
		logger.Fields{"paths": req.Paths, "target": target})
	WriteJSON(w, http.StatusCreated, toEntry(entry))
}

// Unzip handles POST /api/files/unzip and extracts an archive.
func (s *Server) Unzip(w http.ResponseWriter, r *http.Request, u *auth.User) {
	fsys, err := s.requirePermission(u, auth.PermRead, auth.PermWrite, auth.PermZip)
	if err != nil {
		WriteError(w, err)
		return
	}
	if err := s.RequireCSRF(r); err != nil {
		WriteError(w, err)
		return
	}

	var req UnzipRequest
	if err := DecodeJSON(w, r, &req, maxJSONBody); err != nil {
		WriteError(w, err)
		return
	}
	if strings.TrimSpace(req.Path) == "" {
		WriteError(w, Errorf(http.StatusBadRequest, CodeBadRequest, "path is required"))
		return
	}

	entry, err := fsys.Stat(req.Path)
	if err != nil {
		WriteError(w, mapStorageError(err))
		return
	}
	if entry.IsDir {
		WriteError(w, Errorf(http.StatusBadRequest, CodeBadRequest, "not a file"))
		return
	}

	// Read the archive into a temporary file: archive/zip needs random access
	// and storage backends only offer a stream.
	src, err := fsys.Open(req.Path)
	if err != nil {
		WriteError(w, mapStorageError(err))
		return
	}
	defer src.Close()

	tmp, err := os.CreateTemp("", "filegator-unzip-*")
	if err != nil {
		WriteError(w, Errorf(http.StatusInternalServerError, CodeInternal, "cannot read the archive"))
		return
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()

	size, err := io.Copy(tmp, io.LimitReader(src, maxArchiveBytes))
	if err != nil {
		WriteError(w, Errorf(http.StatusInternalServerError, CodeInternal, "cannot read the archive"))
		return
	}
	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		WriteError(w, Errorf(http.StatusInternalServerError, CodeInternal, "cannot read the archive"))
		return
	}

	// v7 always extracted into a subfolder so loose files never spill into the
	// current directory. The caller may override the name.
	dest := req.Destination
	if dest == "" {
		dest = "/"
	}
	folder := strings.TrimSpace(req.Folder)
	if folder == "" {
		folder = archiver.SuggestFolder(req.Path)
	}
	if err := storage.ValidateName(folder); err != nil {
		WriteError(w, Errorf(http.StatusBadRequest, CodeBadRequest, "invalid folder name"))
		return
	}
	target := path.Join(dest, folder)

	if fsys.Exists(target) {
		WriteError(w, Errorf(http.StatusConflict, CodeConflict, "target already exists"))
		return
	}
	if err := fsys.MkdirAll(target); err != nil {
		WriteError(w, mapStorageError(err))
		return
	}

	created, err := archiver.Extract(fsys, tmp, size, target)
	if err != nil {
		WriteError(w, mapArchiveError(err))
		return
	}

	out := make([]Entry, 0, len(created))
	for _, e := range created {
		out = append(out, toEntry(e))
	}
	s.logEvent(r, u, logger.EventUnzip, "archive extracted",
		logger.Fields{"archive": req.Path, "folder": target, "entries": len(out)})
	WriteJSON(w, http.StatusCreated, map[string]any{
		"folder":  target,
		"entries": out,
	})
}

// Batch handles GET /api/files/batch?path=/a&path=/b and streams a zip of the
// selected entries.
//
// Streaming straight to the response avoids temp files, session-bound archive
// ids and cleanup, which is what made v7's batch download security-sensitive.
func (s *Server) Batch(w http.ResponseWriter, r *http.Request, u *auth.User) {
	fsys, err := s.requirePermission(u, auth.PermRead, auth.PermDownload, auth.PermBatchDownload)
	if err != nil {
		WriteError(w, err)
		return
	}

	paths := r.URL.Query()["path"]
	if len(paths) == 0 {
		WriteError(w, Errorf(http.StatusBadRequest, CodeBadRequest, "path is required"))
		return
	}

	name := strings.TrimSpace(r.URL.Query().Get("name"))
	if name == "" {
		name = s.Config.Get().Frontend.DefaultArchiveName
	}
	if name == "" {
		name = "archive.zip"
	}

	// A batch download is recorded even though the body is the payload: the
	// administrator needs to know what left the server.
	s.logEvent(r, u, logger.EventBatchDownload, "batch download", logger.Fields{"paths": paths})

	// Headers must be set before the first byte of the body is written.
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", contentDisposition("attachment", name))
	w.Header().Set("X-Content-Type-Options", "nosniff")

	if err := archiver.Write(fsys, w, paths); err != nil {
		// The body is already partially written, so the status cannot change.
		// Log-worthy, but nothing more can be done for the client.
		return
	}
}

// maxArchiveBytes bounds how large an archive may be for extraction.
const maxArchiveBytes = 2 << 30 // 2 GiB

// mapArchiveError converts archiver errors into client-safe errors.
func mapArchiveError(err error) error {
	var coded *APICodeError
	if errors.As(err, &coded) {
		return err
	}
	switch {
	case errors.Is(err, archiver.ErrNoEntries):
		return Errorf(http.StatusBadRequest, CodeBadRequest, "no entries to archive")
	case errors.Is(err, archiver.ErrUnsafeEntry):
		return Errorf(http.StatusBadRequest, CodeBadRequest,
			"the archive contains an entry outside the destination")
	case errors.Is(err, archiver.ErrInvalidArchive):
		return Errorf(http.StatusBadRequest, CodeBadRequest, "the file is not a valid zip archive")
	}
	return mapStorageError(err)
}

// ArchiveRoutes registers the archiver endpoints.
func (s *Server) ArchiveRoutes(rt *router.Router) {
	rt.Post("/api/files/zip", s.withUser(s.Zip))
	rt.Post("/api/files/unzip", s.withUser(s.Unzip))
	rt.Get("/api/files/batch", s.withUser(s.Batch))
}
