package httpx

import (
	"errors"
	"net/http"
	"path"
	"strings"

	"github.com/filegator/filegator/internal/auth"
	"github.com/filegator/filegator/internal/auth/jsonfile"
	"github.com/filegator/filegator/internal/storage"
)

// MapUserError converts auth provider errors into client-safe errors.
func MapUserError(err error) error {
	var coded *CodedError
	if errors.As(err, &coded) {
		return err
	}
	switch {
	case errors.Is(err, auth.ErrUserNotFound):
		return Errorf(http.StatusNotFound, CodeNotFound, "user not found")
	case errors.Is(err, auth.ErrUsernameTaken):
		return Errorf(http.StatusConflict, CodeConflict, "username already taken")
	case errors.Is(err, auth.ErrInvalidRole):
		return Errorf(http.StatusBadRequest, CodeBadRequest, "invalid role")
	case errors.Is(err, auth.ErrInvalidPerm):
		return Errorf(http.StatusBadRequest, CodeBadRequest, "invalid permission")
	case errors.Is(err, auth.ErrInvalidPassword):
		return Errorf(http.StatusBadRequest, CodeBadRequest, "password is required")
	case errors.Is(err, jsonfile.ErrPasswordTooLong):
		return Errorf(http.StatusBadRequest, CodeBadRequest, "password is too long")
	case errors.Is(err, auth.ErrUnsupported):
		return Errorf(http.StatusForbidden, CodeForbidden,
			"this authentication provider manages users externally")
	}
	return err
}

// MapStorageError converts a backend error into a client-safe error. Every
// backend maps its own failures onto the shared sentinels, so one mapping
// serves all of them.
func MapStorageError(err error) error {
	var coded *CodedError
	if errors.As(err, &coded) {
		return err
	}
	switch {
	case errors.Is(err, storage.ErrNotFound):
		return Errorf(http.StatusNotFound, CodeNotFound, "path not found")
	case errors.Is(err, storage.ErrExists):
		return Errorf(http.StatusConflict, CodeConflict, "target already exists")
	case errors.Is(err, storage.ErrOutsideRoot), errors.Is(err, storage.ErrOutsideScope):
		return Errorf(http.StatusForbidden, CodeForbidden, "path is outside your home folder")
	case errors.Is(err, storage.ErrInvalidPath):
		return Errorf(http.StatusBadRequest, CodeBadRequest, "invalid path")
	case errors.Is(err, storage.ErrPermission):
		return Errorf(http.StatusForbidden, CodeForbidden, "permission denied")
	case errors.Is(err, storage.ErrUnsupported):
		return Errorf(http.StatusBadRequest, CodeBadRequest, "this operation is not supported by the storage backend")
	}
	return err
}

// CleanPath normalises a client-supplied path for display in a response.
func CleanPath(p string) (string, error) {
	if p == "" {
		return "/", nil
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	cleaned := path.Clean(p)
	if !strings.HasPrefix(cleaned, "/") {
		return "", Errorf(http.StatusBadRequest, CodeBadRequest, "invalid path")
	}
	return cleaned, nil
}
