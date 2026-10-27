package app

import (
	"encoding/json"
	"fmt"
	"github.com/filegator/filegator/internal/version"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/filegator/filegator/internal/config"
)

// newAppWithFrontend builds an app serving the interface from dir.
func newAppWithFrontend(t *testing.T, dir string) (*App, error) {
	t.Helper()
	root := t.TempDir()
	store, err := config.Load(filepath.Join(root, "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Update(func(c *config.Config) error {
		c.FrontendDir = dir
		c.Auth.UsersFile = filepath.Join(root, "users.json")
		c.Storage.Root = filepath.Join(root, "repository")
		c.Security.CSRFProtection = false
		c.Logging.Enabled = false
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	a, err := New(store, "")
	if err != nil {
		return nil, err
	}
	t.Cleanup(func() { a.Close() })
	return a, nil
}

// writeFrontend creates a minimal interface in dir.
func writeFrontend(t *testing.T, dir string) {
	t.Helper()
	files := map[string]string{
		"index.html":  `<html><body>EXTERNAL</body></html>`,
		"js/app.js":   `console.log("external");`,
		"css/app.css": "body { color: red; }",
	}
	for name, content := range files {
		full := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestFrontendDefaultsToTheEmbeddedCopy(t *testing.T) {
	a := newTestApp(t)
	body, _ := io.ReadAll(get(a, "/").Body)
	if !strings.Contains(string(body), "js/app.js") {
		t.Fatalf("index looks wrong:\n%s", body)
	}
	if a.source != "embedded" {
		t.Errorf("source = %q, want embedded", a.source)
	}
}

func TestFrontendServedFromADirectory(t *testing.T) {
	dir := t.TempDir()
	writeFrontend(t, dir)

	a, err := newAppWithFrontend(t, dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if a.source != "directory" {
		t.Errorf("source = %q, want directory", a.source)
	}

	// The page must be the external one, not the embedded copy.
	body, _ := io.ReadAll(get(a, "/").Body)
	if !strings.Contains(string(body), "EXTERNAL") {
		t.Fatalf("served the wrong index:\n%s", body)
	}

	// And its assets come from the same place.
	js := get(a, "/js/app.js")
	if js.Code != http.StatusOK {
		t.Fatalf("js status %d", js.Code)
	}
	code, _ := io.ReadAll(js.Body)
	if !strings.Contains(string(code), "external") {
		t.Errorf("js came from the embedded copy: %s", code)
	}
}

func TestFrontendDirectoryEditsAreLive(t *testing.T) {
	// The whole point: change a file, no rebuild.
	dir := t.TempDir()
	writeFrontend(t, dir)

	a, err := newAppWithFrontend(t, dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte(`<html>EDITED</html>`), 0o644); err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(get(a, "/").Body)
	if !strings.Contains(string(body), "EDITED") {
		t.Error("an edit was not picked up")
	}
}

func TestFrontendDirectoryMustBeComplete(t *testing.T) {
	// A half-copied directory must stop startup rather than silently falling
	// back: an administrator would edit files that are never served.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html></html>"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := newAppWithFrontend(t, dir)
	if err == nil {
		t.Fatal("an incomplete frontend was accepted")
	}
	for _, want := range []string{"js/app.js", "css/app.css"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error does not name %s: %v", want, err)
		}
	}
}

func TestFrontendDirectoryMustExist(t *testing.T) {
	_, err := newAppWithFrontend(t, filepath.Join(t.TempDir(), "nope"))
	if err == nil {
		t.Fatal("a missing directory was accepted")
	}
	if !strings.Contains(err.Error(), "frontend_dir") {
		t.Errorf("error should name the setting: %v", err)
	}
}

func TestFrontendDirectoryServesItsOwnTranslations(t *testing.T) {
	// Languages live inside the frontend, so an external copy brings its own.
	dir := t.TempDir()
	writeFrontend(t, dir)
	trans := filepath.Join(dir, "translations")
	if err := os.MkdirAll(trans, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(trans, "english.json"),
		[]byte(`{"Hello":"EXTERNAL HELLO"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	a, err := newAppWithFrontend(t, dir)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(get(a, "/api/i18n/english").Body)
	if !strings.Contains(string(body), "EXTERNAL HELLO") {
		t.Errorf("translations did not come from the external folder: %s", body)
	}
}

func TestFrontendDirDoesNotPersist(t *testing.T) {
	// Like -base-path, the override is for one run: a deployment choice, not a
	// setting to write back.
	dir := t.TempDir()
	root := t.TempDir()
	path := filepath.Join(root, "config.json")
	store, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	override := store.WithFrontendDir(dir)
	if got := override.Get().FrontendDir; got != dir {
		t.Fatalf("override reads %q", got)
	}
	if err := override.Update(func(c *config.Config) error {
		c.Security.SessionKey = strings.Repeat("ab", 32)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), dir) {
		t.Error("the frontend override was written to the config")
	}
}

func TestExportFrontend(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "ui")
	written, err := ExportFrontend(dir)
	if err != nil {
		t.Fatalf("ExportFrontend: %v", err)
	}
	if written < 10 {
		t.Errorf("wrote only %d files", written)
	}

	// Every file the loader requires must be there.
	for _, name := range FrontendRequired() {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(name))); err != nil {
			t.Errorf("%s missing after export: %v", name, err)
		}
	}

	// And the export must be accepted as a frontend.
	a, err := newAppWithFrontend(t, dir)
	if err != nil {
		t.Fatalf("exported frontend rejected: %v", err)
	}
	if a.source != "directory" {
		t.Errorf("source = %q", a.source)
	}
	body, _ := io.ReadAll(get(a, "/").Body)
	if !strings.Contains(string(body), "js/app.js") {
		t.Error("exported page looks wrong")
	}
}

func TestExportFrontendMatchesTheEmbeddedCopy(t *testing.T) {
	// The export exists so an administrator can obtain exactly what this
	// binary serves; anything else would defeat the purpose.
	dir := filepath.Join(t.TempDir(), "ui")
	if _, err := ExportFrontend(dir); err != nil {
		t.Fatal(err)
	}

	err := fs.WalkDir(frontendRoot(), ".", func(name string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil || d.IsDir() {
			return walkErr
		}
		want, err := fs.ReadFile(frontendRoot(), name)
		if err != nil {
			return err
		}
		got, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(name)))
		if err != nil {
			return err
		}
		if string(got) != string(want) {
			t.Errorf("%s differs from the embedded copy", name)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestExportFrontendRefusesUnrelatedDirectory(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("keep me"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := ExportFrontend(dir); err == nil {
		t.Fatal("export over an unrelated directory was allowed")
	}
	// Nothing may have been written before the check.
	if data, _ := os.ReadFile(filepath.Join(dir, "notes.txt")); string(data) != "keep me" {
		t.Error("existing file was damaged")
	}
	if _, err := os.Stat(filepath.Join(dir, "index.html")); err == nil {
		t.Error("files were written into a refused directory")
	}
}

func TestExportFrontendRefusesAFile(t *testing.T) {
	file := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(file, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ExportFrontend(file); err == nil {
		t.Fatal("export over a file was allowed")
	}
}

func TestExportFrontendRefusesEmptyTarget(t *testing.T) {
	if _, err := ExportFrontend(""); err == nil {
		t.Fatal("export with no target was allowed")
	}
}

func TestExportFrontendTwiceIsAllowed(t *testing.T) {
	// Refreshing an existing export after an upgrade is the normal case, as
	// long as nothing in it has been customised.
	dir := filepath.Join(t.TempDir(), "ui")
	if _, err := ExportFrontend(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := ExportFrontend(dir); err != nil {
		t.Fatalf("second export refused: %v", err)
	}
}

// TestExportFrontendRefusesCustomisedExport covers the one way the command can
// destroy work: overwriting an export that has since been edited. Each case
// changes the folder in a different way, because a single comparison could
// easily miss one of the three.
func TestExportFrontendRefusesCustomisedExport(t *testing.T) {
	cases := map[string]func(t *testing.T, dir string){
		"edited file": func(t *testing.T, dir string) {
			css := filepath.Join(dir, "css", "app.css")
			data, err := os.ReadFile(css)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(css, append(data, []byte("\n/* mine */\n")...), 0o644); err != nil {
				t.Fatal(err)
			}
		},
		"added file": func(t *testing.T, dir string) {
			if err := os.WriteFile(filepath.Join(dir, "css", "site.css"), []byte("body{}"), 0o644); err != nil {
				t.Fatal(err)
			}
		},
		"deleted file": func(t *testing.T, dir string) {
			if err := os.Remove(filepath.Join(dir, "css", "login.css")); err != nil {
				t.Fatal(err)
			}
		},
	}

	for name, customise := range cases {
		t.Run(name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "ui")
			if _, err := ExportFrontend(dir); err != nil {
				t.Fatal(err)
			}
			customise(t, dir)

			_, err := ExportFrontend(dir)
			if err == nil {
				t.Fatal("export over a customised folder was allowed")
			}
			if !strings.Contains(err.Error(), "customised") {
				t.Errorf("error does not explain why: %v", err)
			}

			// The customisation must survive the refusal.
			if name == "edited file" {
				data, _ := os.ReadFile(filepath.Join(dir, "css", "app.css"))
				if !strings.Contains(string(data), "/* mine */") {
					t.Error("the edit was overwritten")
				}
			}
			if name == "added file" {
				if _, statErr := os.Stat(filepath.Join(dir, "css", "site.css")); statErr != nil {
					t.Error("the added file was removed")
				}
			}
		})
	}
}

// TestExportFrontendNamesWhatDiffers checks the refusal is actionable: an
// administrator needs to know which file makes the folder un-overwritable.
func TestExportFrontendNamesWhatDiffers(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "ui")
	if _, err := ExportFrontend(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "css", "site.css"), []byte("body{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := ExportFrontend(dir)
	if err == nil {
		t.Fatal("expected a refusal")
	}
	if !strings.Contains(err.Error(), "css/site.css") {
		t.Errorf("error does not name the differing file: %v", err)
	}
}

// TestSummariseDifferencesIsBounded keeps a wildly diverged folder from
// producing an unreadable error message.
func TestSummariseDifferencesIsBounded(t *testing.T) {
	var diff []string
	for i := 0; i < differenceLimit+5; i++ {
		diff = append(diff, fmt.Sprintf("f%02d", i))
	}
	got := summariseDifferences(diff)
	if !strings.Contains(got, "and 5 more") {
		t.Errorf("summary not truncated: %s", got)
	}
	if strings.Contains(got, fmt.Sprintf("f%02d", differenceLimit+4)) {
		t.Errorf("summary lists beyond the limit: %s", got)
	}
	if small := summariseDifferences([]string{"b", "a"}); small != "a, b" {
		t.Errorf("small summary = %q, want sorted", small)
	}
}

func TestExportWritesVersionJSON(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "ui")
	if _, err := ExportFrontend(dir); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "version.json"))
	if err != nil {
		t.Fatalf("version.json missing: %v", err)
	}
	var meta struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(data, &meta); err != nil {
		t.Fatal(err)
	}
	if meta.Version != version.Version {
		t.Errorf("version.json says %q, binary is %q", meta.Version, version.Version)
	}
}

func TestFrontendVersionMismatchIsReported(t *testing.T) {
	dir := t.TempDir()
	writeFrontend(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "version.json"),
		[]byte(`{"version":"v0.0.1"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	a, err := newAppWithFrontend(t, dir)
	if err != nil {
		t.Fatal(err)
	}
	// A stale interface still runs, but must be reported.
	if got := a.staleFrontend; got != "v0.0.1" {
		t.Errorf("stale version = %q, want v0.0.1", got)
	}
}

func TestFrontendVersionMatchIsNotReported(t *testing.T) {
	dir := t.TempDir()
	writeFrontend(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "version.json"),
		[]byte(`{"version":"`+version.Version+`"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	a, err := newAppWithFrontend(t, dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := a.staleFrontend; got != "" {
		t.Errorf("matching interface reported as stale: %q", got)
	}
}

func TestFrontendWithoutVersionIsStillServed(t *testing.T) {
	// A hand-made interface predates version.json; refusing to serve it would
	// break existing customisations.
	dir := t.TempDir()
	writeFrontend(t, dir)

	a, err := newAppWithFrontend(t, dir)
	if err != nil {
		t.Fatalf("version-less frontend rejected: %v", err)
	}
	if a.source != "directory" {
		t.Errorf("source = %q", a.source)
	}
}
