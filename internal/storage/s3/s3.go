// Package s3 implements the storage.Storage interface for Amazon S3 and
// S3-compatible services.
//
// S3 has no directories: an object is just a key. Folders are emulated with a
// trailing "/" and discovered through ListObjectsV2's CommonPrefixes. The
// backend presents them as ordinary directories so the rest of the application
// behaves exactly as it does with local storage.
//
// Limitations, reflected in Capabilities and in the UI:
//   - Chmod is meaningless and reported as unsupported.
//   - Rename and Copy are implemented as copy plus delete, since S3 has no
//     server-side move.
//   - A single object is limited to 5 GiB because multipart upload is not
//     implemented.
package s3

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path"
	"sort"
	"strings"

	"github.com/filegator/filegator/internal/config"
	"github.com/filegator/filegator/internal/storage"
)

// errNotFound marks a missing object inside this package.
var errNotFound = errors.New("s3: object not found")

// Backend is the S3 implementation of storage.Storage.
type Backend struct {
	client *client
	prefix string
	label  string
}

// New connects to the configured bucket and verifies that it is reachable, so
// a bad credential or bucket name fails at startup.
func New(cfg config.S3) (*Backend, error) {
	c, err := newClient(cfg)
	if err != nil {
		return nil, err
	}
	prefix := strings.Trim(cfg.Prefix, "/")
	if prefix != "" {
		prefix += "/"
	}
	b := &Backend{client: c, prefix: prefix}

	label := "s3://" + cfg.Bucket
	if prefix != "" {
		label += "/" + strings.TrimSuffix(prefix, "/")
	}
	if cfg.Endpoint != "" {
		label += " (" + cfg.Endpoint + ")"
	}
	b.label = label

	// One listing proves the credentials and bucket work.
	if _, err := c.listPrefix(prefix, "/", ""); err != nil {
		return nil, fmt.Errorf("s3: cannot read bucket %q: %w", cfg.Bucket, err)
	}
	return b, nil
}

// Description returns the bucket and optional prefix.
func (b *Backend) Description() string { return b.label }

// Capabilities reports what S3 can do.
func (b *Backend) Capabilities() storage.Caps {
	return storage.Caps{Chmod: false, ServerBrowse: false}
}

// Scope returns a view confined to a home folder inside the bucket.
func (b *Backend) Scope(home string) (storage.FS, error) {
	clean, err := storage.CleanPath(home)
	if err != nil {
		return nil, err
	}
	if clean != "/" {
		// The home prefix must exist (as a placeholder object) so a typo is
		// caught instead of silently creating an empty tree.
		meta, err := b.client.headObject(b.key(clean + "/"))
		if err != nil || meta == nil {
			return nil, storage.ErrNotFound
		}
	}
	return &Scope{backend: b, home: clean}, nil
}

// EnsureDir creates the pseudo-directory for path, including parents.
func (b *Backend) EnsureDir(p string) error {
	clean, err := storage.CleanPath(p)
	if err != nil {
		return err
	}
	if clean == "/" {
		return storage.ErrExists
	}
	if meta, err := b.client.headObject(b.key(clean + "/")); err == nil && meta != nil {
		return storage.ErrExists
	}
	if err := b.client.putObject(b.key(clean+"/"), nil, ""); err != nil {
		return err
	}
	return nil
}

// key joins the configured prefix with a virtual path.
func (b *Backend) key(virtual string) string {
	return b.prefix + strings.TrimPrefix(virtual, "/")
}

// virtualKey strips the configured prefix from a raw object key and returns the
// virtual path without a trailing slash.
func (b *Backend) virtualKey(raw string) string {
	trimmed := strings.TrimPrefix(raw, b.prefix)
	trimmed = strings.TrimSuffix(trimmed, "/")
	if trimmed == "" {
		return "/"
	}
	return "/" + trimmed
}

// Scope is a confined S3 view rooted at home.
type Scope struct {
	backend *Backend
	home    string
}

// Home returns the home folder path.
func (s *Scope) Home() string { return s.home }

// dirKey returns the raw key prefix for a virtual directory.
func (s *Scope) dirKey(p string) (string, error) {
	clean, err := storage.CleanPath(p)
	if err != nil {
		return "", err
	}
	if s.home != "/" && !withinPath(s.home, clean) {
		return "", storage.ErrOutsideScope
	}
	full := path.Join(s.home, strings.TrimPrefix(clean, "/"))
	if full == "." {
		full = "/"
	}
	if full == "/" {
		return s.backend.prefix, nil
	}
	return s.backend.key(full) + "/", nil
}

// objectKey returns the raw key for a file, rejecting directories.
func (s *Scope) objectKey(p string) (string, error) {
	clean, err := storage.CleanPath(p)
	if err != nil {
		return "", err
	}
	if clean == "/" {
		return "", storage.ErrInvalidPath
	}
	dir, err := s.dirKey(path.Dir(clean))
	if err != nil {
		return "", err
	}
	return dir + path.Base(clean), nil
}

// Stat describes a file or directory.
func (s *Scope) Stat(p string) (*storage.Entry, error) {
	clean, err := storage.CleanPath(p)
	if err != nil {
		return nil, err
	}
	// Directories are keyed with a trailing slash.
	if clean == "/" || s.isDir(clean) {
		return &storage.Entry{
			Name:  baseName(clean),
			Path:  clean,
			IsDir: true,
			Mode:  fs.ModeDir | 0o755,
		}, nil
	}

	key, err := s.objectKey(clean)
	if err != nil {
		return nil, err
	}
	meta, err := s.backend.client.headObject(key)
	if err != nil {
		if err == errNotFound {
			return nil, storage.ErrNotFound
		}
		return nil, err
	}
	return &storage.Entry{
		Name:     baseName(clean),
		Path:     clean,
		Size:     meta.size,
		Modified: meta.modified,
		Mode:     0o644,
	}, nil
}

// isDir reports whether a virtual path has children or a placeholder object.
func (s *Scope) isDir(clean string) bool {
	if clean == "/" {
		return true
	}
	if _, err := s.dirKey(clean); err != nil {
		return false
	}
	if meta, err := s.backend.client.headObject(s.backend.key(clean) + "/"); err == nil && meta != nil {
		return true
	}
	// A directory can also exist implicitly because children are stored under
	// it, so a one-item listing decides.
	result, err := s.backend.client.listPrefix(s.backend.key(clean)+"/", "/", "")
	return err == nil && len(result.Entries) > 0
}

// ReadDir lists a directory. S3 groups keys into pseudo-directories through
// the "/" delimiter, which is what makes folders appear.
func (s *Scope) ReadDir(dir string) ([]*storage.Entry, error) {
	clean, err := storage.CleanPath(dir)
	if err != nil {
		return nil, err
	}
	prefix, err := s.dirKey(clean)
	if err != nil {
		return nil, err
	}

	entries := make([]*storage.Entry, 0, 32)
	token := ""
	for {
		result, err := s.backend.client.listPrefix(prefix, "/", token)
		if err != nil {
			return nil, err
		}
		for _, item := range result.Entries {
			virtual := s.backend.virtualKey(item.Key)
			// Skip the directory's own placeholder object.
			if virtual == clean {
				continue
			}
			if item.IsPrefix {
				entries = append(entries, &storage.Entry{
					Name:  baseName(virtual),
					Path:  virtual,
					IsDir: true,
					Mode:  fs.ModeDir | 0o755,
				})
				continue
			}
			entries = append(entries, &storage.Entry{
				Name:     baseName(virtual),
				Path:     virtual,
				Size:     item.Size,
				Modified: item.LastModified,
				Mode:     0o644,
			})
		}
		if !result.Truncated {
			break
		}
		token = result.NextToken
	}
	sortEntries(entries)
	return entries, nil
}

// Open reads a file.
func (s *Scope) Open(p string) (io.ReadCloser, error) {
	key, err := s.objectKey(p)
	if err != nil {
		return nil, err
	}
	body, err := s.backend.client.getObject(key)
	if err != nil {
		if strings.Contains(err.Error(), "NoSuchKey") || strings.Contains(err.Error(), "HTTP 404") {
			return nil, storage.ErrNotFound
		}
		return nil, err
	}
	return body, nil
}

// Create writes a file. The whole body is buffered because S3 requires a known
// content length for a single PutObject.
//
// S3 has no exclusive create, so the existence check happens here and again
// as a conditional PUT where the server supports it.
func (s *Scope) Create(p string) (io.WriteCloser, error) {
	key, err := s.objectKey(p)
	if err != nil {
		return nil, err
	}
	if s.Exists(p) {
		return nil, storage.ErrExists
	}
	return &writer{client: s.backend.client, key: key}, nil
}

// OpenWrite truncates an existing object for rewriting.
func (s *Scope) OpenWrite(p string) (io.WriteCloser, error) {
	key, err := s.objectKey(p)
	if err != nil {
		return nil, err
	}
	if !s.Exists(p) {
		return nil, storage.ErrNotFound
	}
	return &writer{client: s.backend.client, key: key}, nil
}

// Mkdir creates one pseudo-directory.
func (s *Scope) Mkdir(p string) error {
	clean, err := storage.CleanPath(p)
	if err != nil {
		return err
	}
	if s.isDir(clean) {
		return storage.ErrExists
	}
	key, err := s.dirKey(clean)
	if err != nil {
		return err
	}
	return s.backend.client.putObject(key, nil, "")
}

// MkdirAll creates a directory and its parents.
func (s *Scope) MkdirAll(p string) error {
	clean, err := storage.CleanPath(p)
	if err != nil {
		return err
	}
	if s.isDir(clean) {
		return nil
	}
	key, err := s.dirKey(clean)
	if err != nil {
		return err
	}
	// A single placeholder at the leaf is enough: S3 has no parent objects.
	return s.backend.client.putObject(key, nil, "")
}

// Rename moves a file or directory, refusing to replace an existing target.
func (s *Scope) Rename(src, dst string) error {
	if s.Exists(dst) {
		return storage.ErrExists
	}
	if err := s.Copy(src, dst); err != nil {
		return err
	}
	return s.Remove(src)
}

// Copy copies a file or directory tree.
func (s *Scope) Copy(src, dst string) error {
	cleanSrc, err := storage.CleanPath(src)
	if err != nil {
		return err
	}
	cleanDst, err := storage.CleanPath(dst)
	if err != nil {
		return err
	}
	if s.Exists(cleanDst) {
		return storage.ErrExists
	}
	if withinPath(cleanSrc, cleanDst) {
		return storage.ErrInvalidPath
	}

	if !s.isDir(cleanSrc) {
		srcKey, err := s.objectKey(cleanSrc)
		if err != nil {
			return err
		}
		dstKey, err := s.objectKey(cleanDst)
		if err != nil {
			return err
		}
		return s.backend.client.copyObject(srcKey, dstKey)
	}

	// Directory: list everything beneath it and copy key by key.
	srcPrefix, err := s.dirKey(cleanSrc)
	if err != nil {
		return err
	}
	dstPrefix, err := s.dirKey(cleanDst)
	if err != nil {
		return err
	}
	token := ""
	for {
		result, err := s.backend.client.listPrefix(srcPrefix, "", token)
		if err != nil {
			return err
		}
		for _, item := range result.Entries {
			relative := strings.TrimPrefix(item.Key, srcPrefix)
			if err := s.backend.client.copyObject(item.Key, dstPrefix+relative); err != nil {
				return err
			}
		}
		if !result.Truncated {
			break
		}
		token = result.NextToken
	}
	return nil
}

// Remove deletes a file, or a directory and everything beneath it.
func (s *Scope) Remove(p string) error {
	clean, err := storage.CleanPath(p)
	if err != nil {
		return err
	}
	if clean == "/" {
		return storage.ErrOutsideScope
	}

	if !s.isDir(clean) {
		key, err := s.objectKey(clean)
		if err != nil {
			return err
		}
		return s.backend.client.deleteObject(key)
	}

	prefix, err := s.dirKey(clean)
	if err != nil {
		return err
	}
	token := ""
	for {
		result, err := s.backend.client.listPrefix(prefix, "", token)
		if err != nil {
			return err
		}
		for _, item := range result.Entries {
			if err := s.backend.client.deleteObject(item.Key); err != nil {
				return err
			}
		}
		if !result.Truncated {
			break
		}
		token = result.NextToken
	}
	// The placeholder object itself.
	return s.backend.client.deleteObject(prefix)
}

// Chmod is not supported: S3 has no permission bits. It returns
// storage.ErrUnsupported so callers can report it plainly.
func (s *Scope) Chmod(p string, mode fs.FileMode) error {
	return storage.ErrUnsupported
}

// Exists reports whether a path exists.
func (s *Scope) Exists(p string) bool {
	_, err := s.Stat(p)
	return err == nil
}

// writer buffers a file and uploads it on Close.
type writer struct {
	client *client
	key    string
	buf    bytes.Buffer
	closed bool
}

// Write appends to the buffer.
func (w *writer) Write(p []byte) (int, error) {
	if w.closed {
		return 0, errors.New("s3: writer is closed")
	}
	if int64(w.buf.Len()+len(p)) > maxSinglePut {
		return 0, fmt.Errorf("s3: object exceeds the %d GiB single-upload limit", maxSinglePut>>30)
	}
	return w.buf.Write(p)
}

// Close uploads the object.
func (w *writer) Close() error {
	if w.closed {
		return nil
	}
	w.closed = true
	return w.client.putObject(w.key, w.buf.Bytes(), "")
}

// baseName returns the last segment of a virtual path.
func baseName(p string) string {
	if p == "/" {
		return "/"
	}
	return path.Base(p)
}

// withinPath reports whether p is base or beneath it.
func withinPath(base, p string) bool {
	if base == "/" {
		return true
	}
	return p == base || strings.HasPrefix(p, base+"/")
}

// sortEntries orders directories first, then names ignoring case.
func sortEntries(entries []*storage.Entry) {
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir != entries[j].IsDir {
			return entries[i].IsDir
		}
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})
}
