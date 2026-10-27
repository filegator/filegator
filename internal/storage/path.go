package storage

import (
	"path"
	"strings"
)

// CleanPath normalises a virtual path shared by every backend.
//
// The returned path always starts with "/" and never contains "." or ".."
// segments. Backslashes are treated as separators, so Windows-style input
// cannot bypass the checks, and NUL bytes are rejected. A path that climbs
// above the root returns ErrOutsideRoot.
func CleanPath(p string) (string, error) {
	if strings.ContainsRune(p, 0) {
		return "", ErrInvalidPath
	}
	p = strings.ReplaceAll(p, `\`, "/")
	if p == "" {
		return "/", nil
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}

	// path.Clean silently swallows "..", so count depth first.
	depth := 0
	for _, seg := range strings.Split(p, "/") {
		switch seg {
		case "", ".":
			continue
		case "..":
			if depth == 0 {
				return "", ErrOutsideRoot
			}
			depth--
		default:
			depth++
		}
	}

	cleaned := path.Clean(p)
	if cleaned == "." {
		return "/", nil
	}
	if !strings.HasPrefix(cleaned, "/") {
		return "", ErrInvalidPath
	}
	return cleaned, nil
}

// ValidateName rejects empty, reserved or separator-containing file names.
// Backends that create entries must use it so no caller can smuggle a path
// separator or traversal into a name.
func ValidateName(name string) error {
	if name == "" || name == "." || name == ".." {
		return ErrInvalidPath
	}
	if strings.ContainsAny(name, "/\\\x00") {
		return ErrInvalidPath
	}
	return nil
}
