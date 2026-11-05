package local

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/filegator/filegator/internal/storage"
)

// view holds the constraints shared by Root and Scope.
type view struct {
	// base is the absolute directory every path is resolved against.
	base string
	// real is the symlink-resolved form of base, used to stop symlinks from
	// escaping. Empty means symlink escape checks are skipped.
	real string
	// confined selects Scope semantics: escapes are reported as
	// storage.ErrOutsideScope rather than storage.ErrOutsideRoot.
	confined bool
}

// outside returns the appropriate escape error for the view.
func (v view) outside() error {
	if v.confined {
		return storage.ErrOutsideScope
	}
	return storage.ErrOutsideRoot
}

// clean normalises p and maps traversal errors onto the view's error.
func (v view) clean(p string) (string, error) {
	clean, err := cleanPath(p)
	if errors.Is(err, storage.ErrOutsideRoot) {
		return "", v.outside()
	}
	return clean, err
}

// abs resolves a virtual path to an absolute path inside base.
func (v view) abs(p string) (string, error) {
	clean, err := v.clean(p)
	if err != nil {
		return "", err
	}
	abs := filepath.Join(v.base, filepath.FromSlash(strings.TrimPrefix(clean, "/")))
	if !withinRoot(v.base, abs) {
		return "", v.outside()
	}
	return abs, nil
}

// resolve returns both the clean virtual path and the absolute path, after
// verifying that following symlinks does not leave the view.
func (v view) resolve(p string) (string, string, error) {
	clean, err := v.clean(p)
	if err != nil {
		return "", "", err
	}
	abs, err := v.abs(clean)
	if err != nil {
		return "", "", err
	}
	if v.real != "" {
		if _, err := v.realWithin(abs); err != nil {
			return "", "", err
		}
	}
	return clean, abs, nil
}

// realWithin resolves symlinks in abs and reports whether the result is still
// inside the view's real base.
func (v view) realWithin(abs string) (string, error) {
	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return "", storage.ErrInvalidPath
		}
		// The target does not exist yet: check its parent instead so files
		// can be created safely.
		parent, perr := filepath.EvalSymlinks(filepath.Dir(abs))
		if perr != nil {
			return "", storage.ErrNotFound
		}
		if !withinRoot(v.real, parent) {
			return "", v.outside()
		}
		return filepath.Join(parent, filepath.Base(abs)), nil
	}
	if !withinRoot(v.real, real) {
		return "", v.outside()
	}
	return real, nil
}

// Stat returns information about p.
func (v view) Stat(p string) (*storage.Entry, error) {
	clean, abs, err := v.resolve(p)
	if err != nil {
		return nil, err
	}
	info, err := os.Lstat(abs)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, storage.ErrNotFound
		}
		return nil, err
	}
	return toEntry(clean, info), nil
}

// ReadDir lists the immediate children of dir, directories first.
func (v view) ReadDir(dir string) ([]*storage.Entry, error) {
	clean, abs, err := v.resolve(dir)
	if err != nil {
		return nil, err
	}
	items, err := os.ReadDir(abs)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, storage.ErrNotFound
		}
		return nil, err
	}
	entries := make([]*storage.Entry, 0, len(items))
	for _, item := range items {
		info, err := item.Info()
		if err != nil {
			// The file may vanish between ReadDir and Info; skip it.
			continue
		}
		entries = append(entries, toEntry(virtualPath(clean, info.Name()), info))
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir != entries[j].IsDir {
			return entries[i].IsDir
		}
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})
	return entries, nil
}

// Open opens p for reading.
func (v view) Open(p string) (io.ReadCloser, error) {
	_, abs, err := v.resolve(p)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(abs)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, storage.ErrNotFound
		}
		return nil, err
	}
	return f, nil
}

// Create creates or truncates p.
func (v view) Create(p string) (io.WriteCloser, error) {
	_, abs, err := v.resolve(p)
	if err != nil {
		return nil, err
	}
	// O_EXCL, not O_TRUNC: a target that appeared between the caller's
	// existence check and this call must fail rather than be silently
	// overwritten — that race is exactly what two concurrent uploads of the
	// same name produce.
	f, err := os.OpenFile(abs, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return nil, storage.ErrNotFound
	case errors.Is(err, fs.ErrExist):
		return nil, storage.ErrExists
	case err != nil:
		return nil, err
	}
	return f, nil
}

// OpenWrite truncates an existing file for rewriting, without creating one.
func (v view) OpenWrite(p string) (io.WriteCloser, error) {
	_, abs, err := v.resolve(p)
	if err != nil {
		return nil, err
	}
	f, err := os.OpenFile(abs, os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, storage.ErrNotFound
		}
		return nil, err
	}
	return f, nil
}

// Mkdir creates the directory p.
func (v view) Mkdir(p string) error {
	_, abs, err := v.resolve(p)
	if err != nil {
		return err
	}
	if err := os.Mkdir(abs, 0o755); err != nil {
		if errors.Is(err, fs.ErrExist) {
			return storage.ErrExists
		}
		if errors.Is(err, fs.ErrNotExist) {
			return storage.ErrNotFound
		}
		return err
	}
	return nil
}

// MkdirAll creates p and any missing parents.
func (v view) MkdirAll(p string) error {
	clean, err := v.clean(p)
	if err != nil {
		return err
	}
	abs, err := v.abs(clean)
	if err != nil {
		return err
	}
	// Verify each existing ancestor before creating anything.
	if v.real != "" {
		if _, err := v.realWithin(abs); err != nil && !errors.Is(err, storage.ErrNotFound) {
			return err
		}
	}
	return os.MkdirAll(abs, 0o755)
}

// Rename moves src to dst, refusing to replace an existing target.
func (v view) Rename(src, dst string) error {
	_, absSrc, err := v.resolve(src)
	if err != nil {
		return err
	}
	_, absDst, err := v.resolve(dst)
	if err != nil {
		return err
	}
	if _, err := os.Lstat(absDst); err == nil {
		return storage.ErrExists
	}
	if err := os.Rename(absSrc, absDst); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return storage.ErrNotFound
		}
		return err
	}
	return nil
}

// Copy copies src to dst, recursively for directories.
func (v view) Copy(src, dst string) error {
	return v.copy(src, dst, nil)
}

// CopyWithLimit copies src to dst while writing at most limit bytes. It returns
// the bytes written, so a caller copying several entries can share one budget.
func (v view) CopyWithLimit(src, dst string, limit int64) (int64, error) {
	remaining := limit
	if err := v.copy(src, dst, &remaining); err != nil {
		return limit - remaining, err
	}
	return limit - remaining, nil
}

// copy performs the shared validation and then copies the tree. A non-nil
// budget limits how many bytes may be written.
func (v view) copy(src, dst string, budget *int64) error {
	_, absSrc, err := v.resolve(src)
	if err != nil {
		return err
	}
	_, absDst, err := v.resolve(dst)
	if err != nil {
		return err
	}
	if _, err := os.Lstat(absDst); err == nil {
		return storage.ErrExists
	}
	// Refuse to copy a directory into itself.
	if withinRoot(absSrc, absDst) {
		return storage.ErrInvalidPath
	}
	return copyTree(absSrc, absDst, budget)
}

// Remove deletes p, recursively for directories.
func (v view) Remove(p string) error {
	_, abs, err := v.resolve(p)
	if err != nil {
		return err
	}
	if abs == v.base {
		// Never delete the view's own root.
		return v.outside()
	}
	if err := os.RemoveAll(abs); err != nil {
		return err
	}
	return nil
}

// Exists reports whether p exists.
func (v view) Exists(p string) bool {
	_, err := v.Stat(p)
	return err == nil
}

// Chmod changes the permission bits of p.
func (v view) Chmod(p string, mode fs.FileMode) error {
	_, abs, err := v.resolve(p)
	if err != nil {
		return err
	}
	if abs == v.base {
		// Changing the permissions of the home folder itself could lock the
		// user out of their own files.
		return v.outside()
	}
	if err := os.Chmod(abs, mode.Perm()); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return storage.ErrNotFound
		}
		return err
	}
	return nil
}

// copyTree copies a file or directory recursively. A non-nil budget is the
// number of bytes the whole copy may still write.
func copyTree(src, dst string, budget *int64) error {
	info, err := os.Lstat(src)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return storage.ErrNotFound
		}
		return err
	}
	if !info.IsDir() {
		return copyFile(src, dst, info, budget)
	}
	if err := os.Mkdir(dst, info.Mode().Perm()|0o700); err != nil {
		return err
	}
	items, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, item := range items {
		if err := copyTree(filepath.Join(src, item.Name()), filepath.Join(dst, item.Name()), budget); err != nil {
			return err
		}
	}
	return nil
}

// copyFile copies a single regular file, checking its known size first and
// using a limited writer for the copy so a file that grows cannot overshoot.
func copyFile(src, dst string, info fs.FileInfo, budget *int64) error {
	if budget != nil && info.Size() > *budget {
		return storage.ErrInsufficientStorage
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode().Perm())
	if err != nil {
		return err
	}
	var writer io.Writer = out
	if budget != nil {
		writer = &budgetWriter{w: out, remaining: budget}
	}
	if _, err := io.Copy(writer, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// budgetWriter stops a copy before it writes past the space it was allowed.
type budgetWriter struct {
	w         io.Writer
	remaining *int64
}

func (b *budgetWriter) Write(p []byte) (int, error) {
	if int64(len(p)) > *b.remaining {
		return 0, storage.ErrInsufficientStorage
	}
	n, err := b.w.Write(p)
	*b.remaining -= int64(n)
	return n, err
}

// cleanPath is the shared normaliser, kept local so call sites stay short.
func cleanPath(p string) (string, error) { return storage.CleanPath(p) }

// withinRoot reports whether abs is root itself or a descendant of it.
func withinRoot(root, abs string) bool {
	if abs == root {
		return true
	}
	return strings.HasPrefix(abs, root+string(os.PathSeparator))
}

// toEntry builds an storage.Entry for the file described by info.
func toEntry(virtual string, info fs.FileInfo) *storage.Entry {
	return &storage.Entry{
		Name:     info.Name(),
		Path:     virtual,
		Size:     info.Size(),
		Modified: info.ModTime(),
		IsDir:    info.IsDir(),
		Mode:     info.Mode().Perm(),
	}
}

// virtualPath returns the virtual path for name inside dir.
func virtualPath(dir, name string) string {
	if dir == "/" {
		return "/" + name
	}
	return dir + "/" + name
}
