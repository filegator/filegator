package app

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/filegator/filegator/internal/config"
	"github.com/filegator/filegator/internal/version"
)

// frontendRequired lists the files a directory must contain to be treated as a
// complete frontend. Returning nil for an incomplete directory means the
// server silently runs the embedded copy while an administrator believes their
// edits are live, so each entry is checked before it is used.
var frontendRequired = []string{
	"index.html",
	"js/app.js",
	"css/app.css",
}

// frontendVersion reads version.json from a frontend directory.
func frontendVersion(root fs.FS) string {
	data, err := fs.ReadFile(root, versionFile)
	if err != nil {
		return ""
	}
	var file struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(data, &file); err != nil {
		log.Printf("warning: %s is not valid: %v", versionFile, err)
		return ""
	}
	return file.Version
}

// resolveFrontend picks the filesystem to serve the interface from.
//
// With no frontend_dir configured this is the copy compiled into the binary,
// which always matches the running code. When one is configured it replaces
// the embedded files entirely: templates and JavaScript live together, and
// mixing the two would produce an interface whose parts do not fit.
func resolveFrontend(cfg *config.Store) (fs.FS, string, string, error) {
	dir := strings.TrimSpace(cfg.Get().FrontendDir)
	if dir == "" {
		return frontendRoot(), "embedded", "", nil
	}

	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, "", "", fmt.Errorf("frontend_dir %q: %w", dir, err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, "", "", fmt.Errorf("frontend_dir %q: %w", dir, err)
	}
	if !info.IsDir() {
		return nil, "", "", fmt.Errorf("frontend_dir %q is not a directory", dir)
	}

	root := os.DirFS(abs)
	var missing []string
	for _, name := range frontendRequired {
		if _, err := fs.Stat(root, name); err != nil {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return nil, "", "", fmt.Errorf(
			"frontend_dir %q is not a FileGator frontend: %s missing. "+
				"Copy internal/app/frontend/ there, or remove frontend_dir to use the built-in interface",
			dir, strings.Join(missing, ", "))
	}

	// An interface served from disk is meant to be edited, so it is often on a
	// mounted volume the process cannot write to. That is fine for reading; the
	// warning exists because a failed upload of assets is otherwise mysterious.
	if err := writableDir(abs); err != nil {
		log.Printf("warning: frontend_dir %q is not writable (%v); it must be updated some other way",
			dir, err)
	}
	// version.json says which release the folder belongs to. A stale interface
	// does not stop the app — it may still work — but silently serving it is
	// how missing features are reported as bugs.
	meta := frontendVersion(root)
	staleVersion := ""
	switch {
	case meta == "":
		log.Printf("warning: %s has no %s, so its version cannot be checked against %s",
			abs, versionFile, version.Version)
	case meta != version.Version:
		log.Printf("warning: the interface in %s was exported for FileGator %s, but this binary is %s. "+
			"Export it again with -export-frontend to get a matching copy", abs, meta, version.Version)
		// Returned rather than kept in a package variable: a global would
		// outlive the app and report the old interface after a restart.
		staleVersion = meta
	}

	log.Printf("serving the interface from %s instead of the built-in copy", abs)
	return root, "directory", staleVersion, nil
}

// writableDir reports whether a file could be created in dir.
func writableDir(dir string) error {
	f, err := os.CreateTemp(dir, ".write-probe-*")
	if err != nil {
		return err
	}
	name := f.Name()
	f.Close()
	return os.Remove(name)
}
