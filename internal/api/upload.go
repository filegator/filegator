package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/filegator/filegator/internal/auth"
	"github.com/filegator/filegator/internal/logger"
	"github.com/filegator/filegator/internal/router"
	"github.com/filegator/filegator/internal/storage"
)

// Upload protocol
//
// Files are sent in chunks so large uploads survive flaky connections and can
// resume. The chunk fields follow the Resumable.js naming used by v7, so the
// existing client can talk to v8 unchanged:
//
//	resumableChunkNumber, resumableTotalChunks, resumableChunkSize,
//	resumableTotalSize, resumableIdentifier, resumableFilename,
//	resumableRelativePath
//
// Chunks are staged in private/tmp/uploads/<user>/<folder>/ and assembled into
// a temporary file on the final chunk, then moved into place. Nothing is
// written into the user's folder until every chunk has arrived.
//
// <folder> is derived from the upload's shape (destination, name, chunk count)
// as well as the client's identifier. The identifier alone is not unique across
// clients: Resumable.js and this project's own client derive it from the file's
// size and modification time, so two people uploading the same file produce the
// same value. Keying the folder by the shape as well means two different files
// can never share one, while a resumed upload still finds its own chunks.

// maxChunkBytes bounds a single chunk request. Generous because clients may
// send the entire file as one chunk.
const maxChunkBytes = 64 << 20

// UploadStagingTTL is how long interrupted uploads are kept before the
// housekeeping sweep discards their chunks.
//
// v7 kept temp files for two days and cleaned them on a 10% chance per
// request; a timer is simpler and predictable.
const UploadStagingTTL = 48 * time.Hour

// uploadSessionTTL is kept as an alias for the existing tests.
const uploadSessionTTL = UploadStagingTTL

// uploadStagingDirName is the directory under the config dir used for chunks.
const uploadStagingDirName = "tmp/uploads"

// UploadResponse reports the state of an upload.
type UploadResponse struct {
	// Status is "uploaded" while more chunks are needed, or "stored" when the
	// file has been saved.
	Status string `json:"status"`
	// Entry is set once the file is stored.
	Entry *Entry `json:"entry,omitempty"`
	// Received and Total report assembly progress.
	Received int `json:"received"`
	Total    int `json:"total"`
}

// Upload handles POST /api/files/upload with one chunk per request.
func (s *Server) Upload(w http.ResponseWriter, r *http.Request, u *auth.User, fsys storage.FS) {
	cfg := s.Config.Get().Frontend

	// Bound the request body. One byte of slack covers multipart framing.
	r.Body = http.MaxBytesReader(w, r.Body, maxChunkBytes+1<<20)
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		WriteError(w, Errorf(http.StatusBadRequest, CodeBadRequest, "invalid upload request"))
		return
	}
	defer r.MultipartForm.RemoveAll()

	meta, err := parseChunkMeta(r)
	if err != nil {
		WriteError(w, err)
		return
	}
	meta.username = safeUserKey(u.Username)
	if cfg.UploadMaxSize > 0 && meta.totalSize > cfg.UploadMaxSize {
		WriteError(w, Errorf(http.StatusRequestEntityTooLarge, CodeBadRequest,
			fmt.Sprintf("file is too large, the limit is %d bytes", cfg.UploadMaxSize)))
		return
	}

	// A refusal costs one request, not a whole upload, so the collision is
	// checked before a single byte is staged. With "upcount" or "replace" the
	// final name is decided at assembly time instead, because the target may
	// appear while a long upload is still running.
	if meta.chunk == 1 {
		switch meta.onCollision {
		case CollisionUpcount, CollisionReplace:
			// Decided in assemble.
		default:
			if fsys.Exists(path.Join(meta.destDir, meta.fileName)) {
				WriteError(w, Errorf(http.StatusConflict, CodeConflict, "target already exists"))
				return
			}
		}
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		WriteError(w, Errorf(http.StatusBadRequest, CodeBadRequest, "chunk file is missing"))
		return
	}
	written, err := s.stageChunk(meta, file)
	file.Close()
	if err != nil {
		WriteError(w, err)
		return
	}
	if cfg.UploadChunkSize > 0 && written > int64(maxChunkBytes) {
		WriteError(w, Errorf(http.StatusBadRequest, CodeBadRequest, "chunk is too large"))
		return
	}

	received, complete, err := s.chunkProgress(meta)
	if err != nil {
		WriteError(w, err)
		return
	}
	if !complete {
		WriteJSON(w, http.StatusOK, UploadResponse{
			Status:   "uploaded",
			Received: received,
			Total:    meta.total,
		})
		return
	}

	entry, err := s.assemble(meta, fsys)
	if err != nil {
		WriteError(w, err)
		return
	}
	s.logEvent(r, u, logger.EventFileUpload, "upload stored",
		logger.Fields{
			"target": path.Join(meta.destDir, meta.fileName),
			"bytes":  entry.Size,
		})
	WriteJSON(w, http.StatusCreated, UploadResponse{
		Status:   "stored",
		Entry:    entry,
		Received: received,
		Total:    meta.total,
	})
}

// UploadStatus handles GET /api/files/upload to let a client resume: it
// reports how many chunks are already stored.
func (s *Server) UploadStatus(w http.ResponseWriter, r *http.Request, u *auth.User, _ storage.FS) {
	meta, err := parseChunkMeta(r)
	if err != nil {
		WriteError(w, err)
		return
	}
	meta.username = safeUserKey(u.Username)
	received, _, err := s.chunkProgress(meta)
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, UploadResponse{
		Status:   "uploaded",
		Received: received,
		Total:    meta.total,
	})
}

// chunkMeta describes one upload.
type chunkMeta struct {
	username    string
	identifier  string
	fileName    string
	destDir     string
	onCollision string
	chunk       int
	total       int
	totalSize   int64
}

// parseChunkMeta reads and validates the upload parameters.
func parseChunkMeta(r *http.Request) (chunkMeta, error) {
	get := func(key string) string {
		if v := r.FormValue(key); v != "" {
			return v
		}
		return r.URL.Query().Get(key)
	}

	meta := chunkMeta{
		fileName: get("resumableFilename"),
		destDir:  get("resumableRelativePath"),
	}
	if meta.fileName == "" {
		return meta, Errorf(http.StatusBadRequest, CodeBadRequest, "resumableFilename is required")
	}
	if err := storage.ValidateName(meta.fileName); err != nil {
		return meta, Errorf(http.StatusBadRequest, CodeBadRequest, "invalid file name")
	}

	// Identifiers are client-supplied and used as a path segment, so they are
	// reduced to a safe character set (as v7 did) and kept short.
	raw := get("resumableIdentifier")
	var b strings.Builder
	for _, ru := range raw {
		if ru >= 'a' && ru <= 'z' || ru >= 'A' && ru <= 'Z' || ru >= '0' && ru <= '9' || ru == '_' || ru == '-' {
			b.WriteRune(ru)
		}
	}
	if b.Len() == 0 {
		return meta, Errorf(http.StatusBadRequest, CodeBadRequest, "resumableIdentifier is required")
	}
	if b.Len() > 128 {
		meta.identifier = b.String()[:128]
	} else {
		meta.identifier = b.String()
	}

	chunk, err := strconv.Atoi(get("resumableChunkNumber"))
	if err != nil || chunk < 1 || chunk > 100000 {
		return meta, Errorf(http.StatusBadRequest, CodeBadRequest, "invalid resumableChunkNumber")
	}
	total, err := strconv.Atoi(get("resumableTotalChunks"))
	if err != nil || total < 1 || total > 100000 {
		return meta, Errorf(http.StatusBadRequest, CodeBadRequest, "invalid resumableTotalChunks")
	}
	if chunk > total {
		return meta, Errorf(http.StatusBadRequest, CodeBadRequest, "chunk number exceeds the total")
	}
	meta.chunk = chunk
	meta.total = total

	if v := get("resumableOnCollision"); v != "" {
		policy, err := normalizeCollision(v)
		if err != nil {
			return meta, err
		}
		meta.onCollision = policy
	}
	if v := get("resumableTotalSize"); v != "" {
		size, err := strconv.ParseInt(v, 10, 64)
		if err != nil || size < 0 {
			return meta, Errorf(http.StatusBadRequest, CodeBadRequest, "invalid resumableTotalSize")
		}
		meta.totalSize = size
	}
	return meta, nil
}

// sessionDir returns the staging directory for one upload.
//
// The folder is derived from the user, the client's identifier and the upload's
// shape: the destination directory, the file name and the number of chunks. The
// identifier on its own is not a unique key — clients derive it from the file's
// size and modification time, so two clients uploading the same file send the
// same identifier, and a folder per identifier mixed their chunks into one
// corrupted file. Including the destination and name separates such uploads,
// while a resumed upload, which repeats both, still lands in the folder holding
// its earlier chunks.
func (s *Server) sessionDir(username, identifier, destDir, fileName string, total int) string {
	h := sha256.New()
	for _, part := range []string{
		username, identifier, destDir, fileName, strconv.Itoa(total),
	} {
		// Length-prefixed so field boundaries cannot be shifted by a name.
		fmt.Fprintf(h, "%d:%s", len(part), part)
	}
	return filepath.Join(s.stagingDir(), username, hex.EncodeToString(h.Sum(nil)))
}

// uploadDirs returns every staging folder under userDir that holds the upload
// named by identifier.
//
// The folder name is a hash of the upload's shape as well as its identifier, so
// it cannot be computed from the identifier alone. The index record beside the
// chunks is what makes the folder findable: a status check and a cancel both
// know only the identifier the client sent, not the destination the chunks were
// staged for.
func (s *Server) uploadDirs(username, identifier string) []string {
	userDir := filepath.Join(s.stagingDir(), username)
	entries, err := os.ReadDir(userDir)
	if err != nil {
		return nil
	}
	var dirs []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dir := filepath.Join(userDir, entry.Name())
		if index, ok := readUploadIndex(dir); ok && index.Identifier == identifier {
			dirs = append(dirs, dir)
		}
	}
	return dirs
}

// uploadDir returns the staging folder for the upload meta describes.
//
// The upload's own shape is known here, so the folder is computed directly.
// A client that renamed the file between chunks stages into a new folder
// rather than mixing its chunks with the earlier ones.
func (s *Server) uploadDir(meta chunkMeta) string {
	return s.sessionDir(meta.username, meta.identifier, meta.destDir, meta.fileName, meta.total)
}

// stagingDir returns the absolute chunk staging directory.
func (s *Server) stagingDir() string {
	cfg := s.Config.Get()
	base := filepath.Dir(cfg.Auth.UsersFile)
	if cfg.Auth.UsersFile == "" || base == "." {
		base = "private"
	}
	return filepath.Join(base, filepath.FromSlash(uploadStagingDirName))
}

// stageChunk writes one chunk to the staging directory.
func (s *Server) stageChunk(meta chunkMeta, src multipart.File) (int64, error) {
	dir := s.uploadDir(meta)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return 0, Errorf(http.StatusInternalServerError, CodeInternal, "cannot stage the upload")
	}
	// The folder name is a hash, so the upload it belongs to is recorded
	// beside the chunks. Cancelling an upload has to find its folder, and a
	// hash cannot be searched for by identifier.
	writeUploadIndex(dir, uploadIndex{
		Identifier: meta.identifier,
		FileName:   meta.fileName,
		DestDir:    meta.destDir,
		Total:      meta.total,
	})
	part := filepath.Join(dir, fmt.Sprintf("%06d.part", meta.chunk))

	out, err := os.OpenFile(part, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return 0, Errorf(http.StatusInternalServerError, CodeInternal, "cannot stage the upload")
	}
	written, err := io.Copy(out, io.LimitReader(src, maxChunkBytes))
	closeErr := out.Close()
	if err != nil || closeErr != nil {
		os.Remove(part)
		return 0, Errorf(http.StatusInternalServerError, CodeInternal, "cannot stage the upload")
	}
	return written, nil
}

// uploadIndexFile names the record stored beside the chunks of one upload.
const uploadIndexFile = "index.json"

// uploadIndex describes the upload a staging folder holds.
type uploadIndex struct {
	Identifier string `json:"identifier"`
	FileName   string `json:"file_name"`
	DestDir    string `json:"dest_dir"`
	Total      int    `json:"total"`
}

// writeUploadIndex records the upload's identity in its staging folder.
//
// A failure is not fatal: the index exists so a cancel can find the folder, and
// the upload itself works without it. The housekeeping sweep clears a folder it
// cannot identify.
func writeUploadIndex(dir string, index uploadIndex) {
	data, err := json.Marshal(index)
	if err != nil {
		return
	}
	_ = os.WriteFile(filepath.Join(dir, uploadIndexFile), data, 0o600)
}

// readUploadIndex reads the record from a staging folder, reporting false when
// it is missing or unreadable.
func readUploadIndex(dir string) (uploadIndex, bool) {
	data, err := os.ReadFile(filepath.Join(dir, uploadIndexFile))
	if err != nil {
		return uploadIndex{}, false
	}
	var index uploadIndex
	if err := json.Unmarshal(data, &index); err != nil {
		return uploadIndex{}, false
	}
	return index, true
}

// chunkProgress counts staged chunks and reports whether all are present.
//
// A resumed upload may have been staged under a slightly different name in an
// earlier attempt, so the folders the identifier matches are searched rather
// than only the one this request computes. Only chunks within the declared
// total count: a restart with a smaller chunk size leaves stale parts behind,
// and counting them would make "received" larger than "total".
func (s *Server) chunkProgress(meta chunkMeta) (int, bool, error) {
	dirs := s.uploadDirs(meta.username, meta.identifier)
	if own := s.uploadDir(meta); !slices.Contains(dirs, own) {
		dirs = append(dirs, own)
	}
	seen := make(map[int]bool, meta.total)
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !strings.HasSuffix(e.Name(), ".part") {
				continue
			}
			index, err := strconv.Atoi(strings.TrimSuffix(e.Name(), ".part"))
			if err != nil || meta.total <= 0 || index < 1 || index > meta.total {
				continue
			}
			seen[index] = true
		}
	}
	received := len(seen)
	return received, received >= meta.total, nil
}

// assemble joins all chunks into the target file and cleans up.
//
// Only the folder this upload owns is read. Another upload sharing the
// identifier — a different file staged under the same identifier, which happens
// whenever two clients upload same-sized files — keeps its own chunks and
// assembles or expires them separately.
func (s *Server) assemble(meta chunkMeta, fsys storage.FS) (*Entry, error) {
	dir := s.uploadDir(meta)
	defer os.RemoveAll(dir)

	target := path.Join(meta.destDir, meta.fileName)
	if meta.destDir == "" {
		target = "/" + meta.fileName
	}

	// Assemble in the staging area first so a partial upload never appears in
	// the user's folder. The name is generated, not derived from the client's
	// identifier: a predictable name in a shared directory would let a local
	// user replace or symlink the file being assembled.
	out, err := os.CreateTemp(s.stagingDir(), "assemble-*.tmp")
	if err != nil {
		return nil, Errorf(http.StatusInternalServerError, CodeInternal, "cannot assemble the upload")
	}
	assembled := out.Name()
	defer os.Remove(assembled)
	for i := 1; i <= meta.total; i++ {
		part := filepath.Join(dir, fmt.Sprintf("%06d.part", i))
		in, err := os.Open(part)
		if err != nil {
			out.Close()
			os.Remove(assembled)
			return nil, Errorf(http.StatusBadRequest, CodeBadRequest, "upload is incomplete")
		}
		_, copyErr := io.Copy(out, in)
		in.Close()
		if copyErr != nil {
			out.Close()
			os.Remove(assembled)
			return nil, Errorf(http.StatusInternalServerError, CodeInternal, "cannot assemble the upload")
		}
	}
	if err := out.Close(); err != nil {
		os.Remove(assembled)
		return nil, Errorf(http.StatusInternalServerError, CodeInternal, "cannot assemble the upload")
	}

	// The final name is decided here, not when chunk 1 arrived: a long upload
	// gives another client time to create the same file.
	target, _, err = resolveTarget(fsys, target, meta.onCollision)
	if err != nil {
		return nil, err
	}

	in, err := os.Open(assembled)
	if err != nil {
		return nil, Errorf(http.StatusInternalServerError, CodeInternal, "cannot assemble the upload")
	}
	defer in.Close()

	dst, err := fsys.Create(target)
	if err != nil {
		return nil, mapStorageError(err)
	}
	if _, err := io.Copy(dst, in); err != nil {
		dst.Close()
		return nil, Errorf(http.StatusInternalServerError, CodeInternal, "cannot store the upload")
	}
	if err := dst.Close(); err != nil {
		return nil, Errorf(http.StatusInternalServerError, CodeInternal, "cannot store the upload")
	}

	entry, err := fsys.Stat(target)
	if err != nil {
		return nil, mapStorageError(err)
	}
	return ptrEntry(toEntry(entry)), nil
}

// ptrEntry returns a pointer to e.
func ptrEntry(e Entry) *Entry { return &e }

// CleanUploadStaging removes upload sessions older than the TTL.
//
// A session folder is the hash of user + identifier, created when the first
// chunk arrives and updated with each chunk, so its mtime tracks activity: an
// upload still in progress is never swept, and an abandoned one is removed
// once it has been idle past the TTL.
//   - at startup, so a restart tidies up after whatever ran before
//   - every hour while the server runs, because a long-lived server would
//     otherwise never clean again and abandoned uploads would accumulate
func (s *Server) CleanUploadStaging() {
	root := s.stagingDir()
	_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil || !d.IsDir() || p == root {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		if time.Since(info.ModTime()) > UploadStagingTTL {
			os.RemoveAll(p)
			return filepath.SkipDir
		}
		return nil
	})
}

// UploadStagingInterval is how often the staging sweep runs.
const UploadStagingInterval = time.Hour

// StartUploadCleanup runs the staging sweep at startup and then on a timer
// until ctx is cancelled. It returns immediately; the sweep runs in the
// background so it never delays serving.
func (s *Server) StartUploadCleanup(ctx context.Context) {
	s.CleanUploadStaging()

	go func() {
		ticker := time.NewTicker(UploadStagingInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.CleanUploadStaging()
			}
		}
	}()
}

// CancelRequest names an upload to discard.
type CancelRequest struct {
	// Filename, Identifier and TotalChunks identify the staging folder using
	// the same Resumable.js fields the upload used.
	Filename    string `json:"filename"`
	Identifier  string `json:"identifier"`
	TotalChunks int    `json:"total_chunks"`
}

// Cancel handles POST /api/files/upload/cancel and discards the staged chunks
// of an upload the user abandoned.
//
// The folder is derived from the authenticated user as well as the request, so
// one user can never cancel another's upload. Because the folder also covers
// the upload's destination, file name and chunk count, a cancel removes every
// staging folder that matches the identifier the caller sent — there can be
// more than one, since the same identifier may describe uploads of different
// files. Without this endpoint, a cancelled upload keeps its chunks until the
// housekeeping sweep clears them.
func (s *Server) Cancel(w http.ResponseWriter, r *http.Request, u *auth.User, _ storage.FS) {
	var req CancelRequest
	if err := DecodeJSON(w, r, &req, maxJSONBody); err != nil {
		WriteError(w, err)
		return
	}
	if strings.TrimSpace(req.Identifier) == "" {
		WriteError(w, Errorf(http.StatusBadRequest, CodeBadRequest, "identifier is required"))
		return
	}

	userDir := filepath.Join(s.stagingDir(), safeUserKey(u.Username))
	if err := removeUploads(userDir, req.Identifier, req.Filename, req.TotalChunks); err != nil {
		WriteError(w, Errorf(http.StatusInternalServerError, CodeInternal, "cannot discard the upload"))
		return
	}
	WriteNoContent(w)
}

// removeUploads deletes the staging folders under userDir whose index matches
// the given identifier.
//
// Each staging folder holds an index naming the upload it belongs to, because
// the folder name is a hash and cannot be matched directly. A folder without a
// readable index is left alone: it is either from an older release, which the
// housekeeping sweep will clear, or not this code's to delete.
func removeUploads(userDir, identifier, fileName string, totalChunks int) error {
	entries, err := os.ReadDir(userDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dir := filepath.Join(userDir, entry.Name())
		index, ok := readUploadIndex(dir)
		if !ok || index.Identifier != identifier {
			continue
		}
		// The request may name the file it is cancelling; when it does, only
		// that file's folder is removed, so a second upload that happens to
		// share the identifier is untouched.
		if fileName != "" && index.FileName != fileName {
			continue
		}
		if totalChunks > 0 && index.Total != totalChunks {
			continue
		}
		if err := os.RemoveAll(dir); err != nil {
			return err
		}
	}
	return nil
}

// UploadRoutes registers upload endpoints.
func (s *Server) UploadRoutes(rt *router.Router) {
	perm := []auth.Permission{auth.PermRead, auth.PermUpload}
	rt.Post("/api/files/upload", s.write(perm, s.Upload))
	rt.Get("/api/files/upload", s.withUser(s.uploadStatus))
	// The request body carries the identifier, so POST rather than DELETE:
	// the project router has no DELETE method and the CSRF check lives in
	// s.write.
	rt.Post("/api/files/upload/cancel", s.write(perm, s.Cancel))
}

// uploadStatus resolves the user and delegates, so the status endpoint shares
// the permission checks of the POST path.
func (s *Server) uploadStatus(w http.ResponseWriter, r *http.Request, u *auth.User) {
	fsys, err := s.requirePermission(u, auth.PermRead, auth.PermUpload)
	if err != nil {
		WriteError(w, err)
		return
	}
	s.UploadStatus(w, r, u, fsys)
}

// safeUserKey reduces a username to characters usable as a path segment.
func safeUserKey(username string) string {
	sum := sha256.Sum256([]byte(username))
	return hex.EncodeToString(sum[:8])
}
