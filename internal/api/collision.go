package api

import (
	"fmt"
	"net/http"
	"path"
	"strings"
)

// Collision policies. A request that would create an entry whose name is
// already taken chooses one of these with the "on_collision" field.
const (
	// CollisionRefuse is the default: the request fails with 409 and nothing
	// is written. v7 silently renamed instead; an HTTP API must not rename
	// what the caller asked for unless it is told to.
	CollisionRefuse = "refuse"
	// CollisionReplace deletes the existing entry first. Destructive, so it is
	// never chosen implicitly.
	CollisionReplace = "replace"
	// CollisionUpcount picks the next free name, the way v7 did: "report.pdf"
	// becomes "report (1).pdf", then "report (2).pdf".
	CollisionUpcount = "upcount"
)

// maxUpcountTries bounds the search for a free name. A folder holding
// thousands of duplicates should report a clear error rather than scanning
// forever.
const maxUpcountTries = 1000

// collisionError is returned when a policy is unknown.
func collisionError(policy string) error {
	return Errorf(http.StatusBadRequest, CodeBadRequest,
		fmt.Sprintf("unknown on_collision %q, use refuse, replace or upcount", policy))
}

// normalizeCollision maps an empty policy to the default and validates it.
func normalizeCollision(policy string) (string, error) {
	switch p := strings.ToLower(strings.TrimSpace(policy)); p {
	case "":
		return CollisionRefuse, nil
	case CollisionRefuse, CollisionReplace, CollisionUpcount:
		return p, nil
	default:
		return "", collisionError(policy)
	}
}

// upcountName returns the name with a " (n)" suffix, keeping the extension.
func upcountName(name string, n int) string {
	base, ext := splitName(name)
	return fmt.Sprintf("%s (%d)%s", base, n, ext)
}

// splitName separates a file name from its extension.
//
// Rules:
//   - no dot, or a leading dot only  → no extension   ("Makefile", ".bashrc")
//   - a trailing dot is part of the name              ("notes." → "notes (1).")
//   - compound archives stay whole                    ("archive.tar.gz")
//   - anything else                    → the last part ("report.pdf")
func splitName(name string) (string, string) {
	for _, compound := range compoundExtensions {
		if len(name) > len(compound) && strings.EqualFold(name[len(name)-len(compound):], compound) {
			return name[:len(name)-len(compound)], name[len(name)-len(compound):]
		}
	}
	dot := strings.LastIndex(name, ".")
	if dot <= 0 || dot == len(name)-1 {
		return name, ""
	}
	return name[:dot], name[dot:]
}

// compoundExtensions are treated as a single extension. Only the ones where
// the middle part is a format rather than a name are listed: "my.photos.zip"
// must not become "my (1).photos.zip".
var compoundExtensions = []string{".tar.gz", ".tar.bz2", ".tar.xz", ".tar.zst"}

// resolveTarget decides the path a write should use, applying the collision
// policy to an existing entry.
//
// It returns the path to write and, when the name had to change, the original
// path so the caller can report it.
func resolveTarget(fsys storagePathChecker, target string, policy string) (string, string, error) {
	if !fsys.Exists(target) {
		return target, "", nil
	}
	switch policy {
	case CollisionReplace:
		if err := fsys.Remove(target); err != nil {
			return "", "", err
		}
		return target, "", nil
	case CollisionUpcount:
		dir, name := path.Split(target)
		for n := 1; n <= maxUpcountTries; n++ {
			candidate := path.Join(dir, upcountName(name, n))
			if !fsys.Exists(candidate) {
				return candidate, target, nil
			}
		}
		return "", "", Errorf(http.StatusConflict, CodeConflict, "too many files with a similar name")
	default:
		return "", "", Errorf(http.StatusConflict, CodeConflict, "target already exists")
	}
}

// storagePathChecker is the slice of storage.FS that collision handling needs.
// Keeping it narrow makes the logic testable without a backend.
type storagePathChecker interface {
	Exists(p string) bool
	Remove(p string) error
}
