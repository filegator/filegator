package api

import (
	"errors"
	"io"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"github.com/filegator/filegator/internal/api/httpx"
	"github.com/filegator/filegator/internal/auth"
	"github.com/filegator/filegator/internal/config"
	"github.com/filegator/filegator/internal/logger"
	"github.com/filegator/filegator/internal/router"
	"github.com/filegator/filegator/internal/security"
	"github.com/filegator/filegator/internal/sso"
	"github.com/filegator/filegator/internal/storage"
)

// Server holds the dependencies shared by all handlers.
type Server struct {
	Config   *config.Store
	Users    auth.Provider
	Sessions *security.Sessions
	CSRF     *security.CSRF
	Lockout  *security.Lockout
	IPFilter *security.IPFilter
	// Log records security-relevant events. Nil disables logging, which is
	// what tests use.
	Log *logger.Logger

	// roots holds the active storage backend; the admin may swap it at runtime.
	roots rootHolder

	// SSO verifies signed handoffs from other applications; nil disables the
	// endpoint entirely.
	SSO *sso.Verifier
	// basePath is the URL prefix the app is mounted under, used for redirects.
	basePath string
	// frontendSource is "embedded" or "directory", reported to the admin UI.
	frontendSource string
	// staleFrontend is the version of a custom interface built for another
	// release, reported so the admin knows to refresh it.
	staleFrontend string
}

// Entry is the JSON representation of a file or directory.
type Entry struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	Type     string `json:"type"`
	Size     int64  `json:"size"`
	Modified string `json:"modified"`
	Mode     string `json:"mode"`
}

// DirListing is the response for a directory listing.
type DirListing struct {
	Path    string  `json:"path"`
	Parent  *string `json:"parent"`
	Entries []Entry `json:"entries"`
}

// toEntry converts a storage entry into its JSON form.
func toEntry(e *storage.Entry) Entry {
	kind := "file"
	if e.IsDir {
		kind = "dir"
	}
	return Entry{
		Name:     e.Name,
		Path:     e.Path,
		Type:     kind,
		Size:     e.Size,
		Modified: e.Modified.UTC().Format(time.RFC3339),
		Mode:     modeString(e.Mode),
	}
}

// modeString renders permission bits as a three or four digit octal string,
// matching what chmod expects.
func modeString(m os.FileMode) string {
	digits := []byte{0, 0, 0}
	perm := uint32(m.Perm())
	for i := 2; i >= 0; i-- {
		digits[i] = byte('0' + perm&7)
		perm >>= 3
	}
	return string(digits)
}

// parentOf returns the parent path of p, or nil when p is the home root.
func parentOf(p string) *string {
	if p == "/" {
		return nil
	}
	parent := path.Dir(p)
	if parent == "." {
		parent = "/"
	}
	return &parent
}

// userFS returns the storage view for a user.
//
// Administrators always get the storage root, regardless of their stored
// homedir. Without this, a mistyped homedir would lock the only account that
// can repair it out of the entire repository.
func (s *Server) userFS(u *auth.User) (storage.FS, error) {
	root := s.ActiveStorage()
	if root == nil {
		return nil, Errorf(http.StatusServiceUnavailable, CodeInternal, "storage is not configured")
	}
	home := u.HomeDir
	if u.IsAdmin() {
		home = "/"
	}
	scope, err := root.Scope(home)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, Errorf(http.StatusForbidden, CodeForbidden,
				"your home folder does not exist; an administrator must create it or change it in the users settings")
		}
		return nil, Errorf(http.StatusForbidden, CodeForbidden, "home folder is not accessible")
	}
	return scope, nil
}

// requirePermission checks that u holds every permission and returns the
// matching storage view.
func (s *Server) requirePermission(u *auth.User, perms ...auth.Permission) (storage.FS, error) {
	if !u.Can(perms...) {
		s.Log.Warn(logger.EventPermissionDenied, "permission denied",
			logger.UserFields(logger.Fields{
				"required": perms,
				"has":      u.Permissions,
			}, u.Username))
		return nil, Errorf(http.StatusForbidden, CodeForbidden, "permission denied")
	}
	return s.userFS(u)
}

// mapStorageError converts a storage error into a client-safe API error.
func mapStorageError(err error) error { return httpx.MapStorageError(err) }

// ListDir handles GET /api/files?path=/.
func (s *Server) ListDir(w http.ResponseWriter, r *http.Request, u *auth.User) {
	fsys, err := s.requirePermission(u, auth.PermRead)
	if err != nil {
		WriteError(w, err)
		return
	}

	dir := r.URL.Query().Get("path")
	if dir == "" {
		dir = "/"
	}
	entries, err := fsys.ReadDir(dir)
	if err != nil {
		WriteError(w, mapStorageError(err))
		return
	}

	clean, err := cleanForClient(dir)
	if err != nil {
		WriteError(w, err)
		return
	}

	out := DirListing{
		Path:    clean,
		Parent:  parentOf(clean),
		Entries: make([]Entry, 0, len(entries)),
	}
	for _, e := range entries {
		out.Entries = append(out.Entries, toEntry(e))
	}
	WriteJSON(w, http.StatusOK, out)
}

// Stat handles GET /api/files/stat?path=.
func (s *Server) Stat(w http.ResponseWriter, r *http.Request, u *auth.User) {
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
	e, err := fsys.Stat(p)
	if err != nil {
		WriteError(w, mapStorageError(err))
		return
	}
	WriteJSON(w, http.StatusOK, toEntry(e))
}

// Download handles GET /api/files/download?path=.
//
// Files are sent with a Content-Disposition header. Extensions listed in the
// download_inline config are shown in the browser instead, because inline
// rendering of untrusted types can enable cross-site scripting.
func (s *Server) Download(w http.ResponseWriter, r *http.Request, u *auth.User) {
	fsys, err := s.requirePermission(u, auth.PermDownload)
	if err != nil {
		WriteError(w, err)
		return
	}
	p := r.URL.Query().Get("path")
	if p == "" {
		WriteError(w, Errorf(http.StatusBadRequest, CodeBadRequest, "path is required"))
		return
	}
	e, err := fsys.Stat(p)
	if err != nil {
		WriteError(w, mapStorageError(err))
		return
	}
	if e.IsDir {
		WriteError(w, Errorf(http.StatusBadRequest, CodeBadRequest, "cannot download a directory"))
		return
	}

	f, err := fsys.Open(p)
	if err != nil {
		WriteError(w, mapStorageError(err))
		return
	}
	defer f.Close()

	disposition := "attachment"
	if s.isInline(e.Name) {
		// Serve with an inert content type unless the browser needs it.
		disposition = "inline"
		w.Header().Set("Content-Type", contentTypeFor(e.Name))
	} else {
		w.Header().Set("Content-Type", "application/octet-stream")
	}
	w.Header().Set("Content-Disposition", contentDisposition(disposition, e.Name))
	w.Header().Set("X-Content-Type-Options", "nosniff")

	if seeker, ok := f.(io.ReadSeeker); ok {
		http.ServeContent(w, r, e.Name, e.Modified, seeker)
		return
	}
	w.Header().Set("Content-Length", itoa(e.Size))
	io.Copy(w, f)
}

// isInline reports whether the extension of name should render in-browser.
// Config entries are written with a leading dot (".pdf") but a bare extension
// ("pdf") is also accepted, since that is the form v7 used.
func (s *Server) isInline(name string) bool {
	cfg := s.Config.Get()
	ext := strings.ToLower(path.Ext(name))
	bare := strings.TrimPrefix(ext, ".")
	for _, allowed := range cfg.DownloadInline {
		allowed = strings.ToLower(strings.TrimSpace(allowed))
		if allowed == "*" || allowed == ext || allowed == bare {
			return true
		}
	}
	return false
}

// contentTypeFor returns the content type for an inline download.
//
// Only an extension the administrator listed in download_inline reaches this
// function, so the type is reported for any file type rather than a safe
// subset: an entry that said "inline" but arrived typed application/octet-
// stream would not render in the browser, which makes the setting look broken.
// That is v7's behaviour too, where PHP reported the type of whatever was
// listed. The security comes from the list being explicit and from the warning
// on the admin Files page, not from this function second-guessing it. An
// unknown extension still falls back to octet-stream, which browsers download.
func contentTypeFor(name string) string {
	switch strings.ToLower(path.Ext(name)) {
	case ".pdf":
		return "application/pdf"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".bmp":
		return "image/bmp"
	case ".avif":
		return "image/avif"
	case ".svg":
		return "image/svg+xml"
	case ".ico":
		return "image/x-icon"
	case ".tif", ".tiff":
		return "image/tiff"
	// Audio and video page through a player rather than a blank tab.
	case ".mp3":
		return "audio/mpeg"
	case ".ogg", ".oga":
		return "audio/ogg"
	case ".wav":
		return "audio/wav"
	case ".m4a":
		return "audio/mp4"
	case ".mp4", ".m4v":
		return "video/mp4"
	case ".webm":
		return "video/webm"
	case ".mov":
		return "video/quicktime"
	case ".ogv":
		return "video/ogg"
	case ".txt", ".md", ".csv", ".log":
		return "text/plain; charset=utf-8"
	case ".html", ".htm":
		return "text/html; charset=utf-8"
	case ".xml":
		return "text/xml; charset=utf-8"
	case ".css":
		return "text/css; charset=utf-8"
	case ".js", ".mjs":
		return "text/javascript; charset=utf-8"
	case ".json":
		return "application/json"
	default:
		return "application/octet-stream"
	}
}

// contentDisposition builds a header value that is safe for any filename.
// Non-ASCII names also get an RFC 5987 filename* parameter.
func contentDisposition(kind, name string) string {
	ascii := make([]rune, 0, len(name))
	for _, r := range name {
		if r == '"' || r == '\\' || r < 0x20 || r > 0x7e {
			ascii = append(ascii, '_')
			continue
		}
		ascii = append(ascii, r)
	}
	value := kind + `; filename="` + string(ascii) + `"`
	if string(ascii) != name {
		value += "; filename*=UTF-8''" + urlPathEscape(name)
	}
	return value
}

// cleanForClient normalises a directory path for display in a response.
func cleanForClient(p string) (string, error) { return httpx.CleanPath(p) }

// Routes registers the file API on rt.
func (s *Server) Routes(rt *router.Router) {
	rt.Get("/api/files", s.withUser(s.ListDir))
	rt.Get("/api/files/stat", s.withUser(s.Stat))
	rt.Get("/api/files/download", s.withUser(s.Download))
}
