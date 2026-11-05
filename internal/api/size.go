package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/filegator/filegator/internal/auth"
	"github.com/filegator/filegator/internal/storage"
)

// sizeTimeout bounds an on-demand folder walk. A huge tree must not hold a
// worker for minutes; the user can retry from a narrower folder.
const sizeTimeout = 30 * time.Second

// SizeResponse is the body of GET /api/files/size.
type SizeResponse struct {
	Path string `json:"path"`
	Size int64  `json:"size"`
}

// FolderSize handles GET /api/files/size?path=/folder and returns the total
// bytes below the folder. The walk is on demand and confined to the user's
// home, like every other file operation.
func (s *Server) FolderSize(w http.ResponseWriter, r *http.Request, u *auth.User) {
	fsys, err := s.requirePermission(u, auth.PermRead)
	if err != nil {
		WriteError(w, err)
		return
	}

	p := r.URL.Query().Get("path")
	if p == "" {
		p = "/"
	}
	entry, err := fsys.Stat(p)
	if err != nil {
		WriteError(w, mapStorageError(err))
		return
	}
	if !entry.IsDir {
		WriteError(w, Errorf(http.StatusBadRequest, CodeBadRequest, "path must be a directory"))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), sizeTimeout)
	defer cancel()

	total, err := dirSize(ctx, fsys, p)
	switch {
	case err == nil:
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		WriteError(w, Errorf(http.StatusRequestTimeout, CodeBadRequest,
			"calculating the folder size took too long"))
		return
	default:
		WriteError(w, mapStorageError(err))
		return
	}
	WriteJSON(w, http.StatusOK, SizeResponse{Path: p, Size: total})
}

// dirSize sums the file sizes below dir. Directories themselves add no bytes;
// their children are walked recursively. The context check happens per
// directory so a cancelled request stops promptly.
func dirSize(ctx context.Context, fsys storage.FS, dir string) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	entries, err := fsys.ReadDir(dir)
	if err != nil {
		return 0, err
	}
	var total int64
	for _, entry := range entries {
		if entry.IsDir {
			n, err := dirSize(ctx, fsys, entry.Path)
			if err != nil {
				return 0, err
			}
			if n > maxInt64-total {
				return maxInt64, nil
			}
			total += n
			continue
		}
		if entry.Size > maxInt64-total {
			return maxInt64, nil
		}
		total += entry.Size
	}
	return total, nil
}
