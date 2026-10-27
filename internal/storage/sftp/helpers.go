package sftp

import (
	"errors"
	"io/fs"
	"os"
	"sort"
	"strings"

	pkgsftp "github.com/pkg/sftp"

	"github.com/filegator/filegator/internal/storage"
)

// within reports whether abs is root itself or a descendant of it.
func within(root, abs string) bool {
	if abs == root {
		return true
	}
	return strings.HasPrefix(abs, root+"/")
}

// joinVirtual returns the virtual path for name inside dir.
func joinVirtual(dir, name string) string {
	if dir == "/" {
		return "/" + name
	}
	return dir + "/" + name
}

// mapErr converts an SFTP error into a shared storage sentinel.
func mapErr(err error) error {
	if err == nil {
		return nil
	}
	var status *pkgsftp.StatusError
	if errors.As(err, &status) {
		switch status.FxCode() {
		case pkgsftp.ErrSSHFxNoSuchFile:
			return storage.ErrNotFound
		case pkgsftp.ErrSSHFxPermissionDenied:
			return storage.ErrPermission
		}
	}
	if errors.Is(err, os.ErrNotExist) || errors.Is(err, fs.ErrNotExist) {
		return storage.ErrNotFound
	}
	if errors.Is(err, os.ErrPermission) || errors.Is(err, fs.ErrPermission) {
		return storage.ErrPermission
	}
	var pathErr *fs.PathError
	if errors.As(err, &pathErr) && pathErr.Err != nil {
		return pathErr.Err
	}
	return err
}

// isNotExist reports whether err means the remote path is missing.
func isNotExist(err error) bool {
	if errors.Is(err, os.ErrNotExist) || errors.Is(err, fs.ErrNotExist) {
		return true
	}
	var status *pkgsftp.StatusError
	if errors.As(err, &status) {
		return status.FxCode() == pkgsftp.ErrSSHFxNoSuchFile
	}
	return false
}

// toEntry converts SFTP file info into a storage entry.
func toEntry(virtual string, info os.FileInfo) *storage.Entry {
	return &storage.Entry{
		Name:     info.Name(),
		Path:     virtual,
		Size:     info.Size(),
		Modified: info.ModTime(),
		IsDir:    info.IsDir(),
		Mode:     info.Mode().Perm(),
	}
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
