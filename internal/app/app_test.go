package app

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/filegator/filegator/internal/config"
)

// TestConfigStockLoadIsNotAStart documents why Load does not create the file.
//
// A stock `store, _ := config.Load(path)` followed by an Update — what every
// API test does — used to write config.json *before* New() ran. That start then
// looked like a warm start, and the file it read back claimed settings the
// operator never made: the default storage root and users file resolved
// relative to the working directory.
//
// The behaviour that makes this harmless is asserted here, so it cannot be
// "optimised" into Load writing the file:
//
//  1. Load itself writes nothing.
//  2. If something does write the file first, the session key is still absent,
//     so New() recognises a first start and writes the key.
func TestConfigStockLoadIsNotAStart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	store, err := config.Load(path)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("Load created %s; it must only read", path)
	}

	// A pre-start write, as a test fixture does it, persists defaults but no
	// key: the file must not strip the "first start" signal.
	if err := store.Update(func(c *config.Config) error { return nil }); err != nil {
		t.Fatalf("config.Update: %v", err)
	}
	reloaded, err := config.Load(path)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	if key := reloaded.Get().Security.SessionKey; key != "" {
		t.Fatalf("session key = %q after a pre-start write; New() would not know this is a first start", key)
	}
}

// TestSessionKeyIsWrittenWithoutAnEarlierUpdate is the real first start of a
// binary: Load reads a missing file and New() writes it, session key included.
// No Update may be needed beforehand — that is what made the file appear on a
// later run in the application and in every API test fixture.
func TestSessionKeyIsWrittenWithoutAnEarlierUpdate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	store, err := config.Load(path)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	a, err := New(store, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	a.Close()

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("New() did not create %s: %v", path, err)
	}
	reloaded, err := config.Load(path)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	if reloaded.Get().Security.SessionKey == "" {
		t.Error("the created file has no session key")
	}
	if !hasNote(a.startup.Notes(), "config.created") {
		t.Error("the first start should be reported as having created the file")
	}
}

func newTestApp(t *testing.T) *App {
	t.Helper()
	dir := t.TempDir()
	store, err := config.Load(dir + "/config.json")
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	// Use absolute paths inside the temp dir so tests never create
	// directories next to the source files.
	if err := store.Update(func(c *config.Config) error {
		c.Storage.Root = dir + "/repository"
		c.Auth.UsersFile = dir + "/private/users.json"
		return nil
	}); err != nil {
		t.Fatalf("config.Update: %v", err)
	}
	a, err := New(store, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return a
}

func get(a *App, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	a.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func TestPing(t *testing.T) {
	rec := get(newTestApp(t), "/api/ping")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("content type = %q", ct)
	}
	if body := rec.Body.String(); !strings.Contains(body, `"ok"`) {
		t.Errorf("body = %q", body)
	}
}

func TestFrontendAssetsAreServed(t *testing.T) {
	files := map[string]string{
		"/":                            "js/app.js",
		"/css/app.css":                 "--primary",
		"/css/login.css":               ".login-card",
		"/css/browser.css":             ".file-table",
		"/js/app.js":                   "./modules/api.js",
		"/js/modules/api.js":           "/api/files",
		"/js/modules/format.js":        "formatSize",
		"/js/modules/datetime.js":      "formatDatePattern",
		"/js/modules/i18n.js":          "export function t",
		"/js/modules/dom.js":           "export function el",
		"/js/modules/header_search.js": "createHeaderSearch",
		"/js/modules/theme.js":         "createTheme",
		"/js/views/login.js":           "createLogin",
		"/js/views/browser.js":         "createBrowser",
	}
	for path, want := range files {
		rec := get(newTestApp(t), path)
		if rec.Code != http.StatusOK {
			t.Errorf("%s: status = %d", path, rec.Code)
			continue
		}
		body, _ := io.ReadAll(rec.Body)
		if want != "" && !strings.Contains(string(body), want) {
			t.Errorf("%s: body does not contain %q", path, want)
		}
	}
}

func TestIndexReferencesExistingModules(t *testing.T) {
	rec := get(newTestApp(t), "/")
	body, _ := io.ReadAll(rec.Body)
	page := string(body)

	// Assets are referenced relatively and resolved against <base>, so the
	// page works under any mount point. Resolve them the way a browser would.
	base := "/"
	for _, part := range strings.Split(page, `<base href="`)[1:] {
		base = strings.Split(part, `"`)[0]
	}
	refs := []string{}
	for _, prefix := range []string{"css/", "js/"} {
		for _, part := range strings.Split(page, `"`+prefix)[1:] {
			name := prefix + strings.FieldsFunc(part, func(r rune) bool {
				return r == '"' || r == ' ' || r == '>'
			})[0]
			refs = append(refs, strings.TrimSuffix(base, "/")+"/"+name)
		}
	}
	if len(refs) < 4 {
		t.Fatalf("expected several asset references, got %v", refs)
	}
	for _, ref := range refs {
		if rec := get(newTestApp(t), ref); rec.Code != http.StatusOK {
			t.Errorf("%s referenced by index.html is not served (status %d)", ref, rec.Code)
		}
	}
}

func TestUnknownPathIsNotFound(t *testing.T) {
	rec := get(newTestApp(t), "/does-not-exist")
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestAssetPath(t *testing.T) {
	cases := map[string]string{
		"/css/app.css":       "css/app.css",
		"/js/app.js":         "js/app.js",
		"/js/modules/api.js": "js/modules/api.js",
		"/api/ping":          "",
		"/":                  "",
		"/favicon.ico":       "",
	}
	for in, want := range cases {
		if got := assetPath(in); got != want {
			t.Errorf("assetPath(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestEmbeddedFilesExist(t *testing.T) {
	for _, name := range []string{"index.html", "css/app.css", "js/app.js", "js/modules/api.js"} {
		if !newStaticServer(frontendRoot()).exists(name) {
			t.Errorf("embedded file missing: %s", name)
		}
	}
}

// newAppWithBase builds an app mounted under a URL prefix.
func newAppWithBase(t *testing.T, base string) *App {
	t.Helper()
	dir := t.TempDir()
	store, err := config.Load(filepath.Join(dir, "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Update(func(c *config.Config) error {
		c.Auth.UsersFile = filepath.Join(dir, "users.json")
		c.Storage.Root = filepath.Join(dir, "repository")
		c.Security.CSRFProtection = false
		c.Logging.Enabled = false
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	a, err := New(store, base)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { a.Close() })
	return a
}

func TestBasePathServesEverythingUnderThePrefix(t *testing.T) {
	a := newAppWithBase(t, "/browser")

	cases := map[string]int{
		"/browser":             http.StatusOK,
		"/browser/":            http.StatusOK,
		"/browser/js/app.js":   http.StatusOK,
		"/browser/css/app.css": http.StatusOK,
		"/browser/api/ping":    http.StatusOK,
		"/browser/api/i18n":    http.StatusOK,
		// Outside the mount point the app must not answer at all.
		"/":          http.StatusNotFound,
		"/api/ping":  http.StatusNotFound,
		"/js/app.js": http.StatusNotFound,
		// A lookalike prefix is not the mount point.
		"/browserextra/api/ping": http.StatusNotFound,
	}
	for path, want := range cases {
		if got := get(a, path).Code; got != want {
			t.Errorf("%s: status %d, want %d", path, got, want)
		}
	}
}

func TestBasePathIsInjectedIntoThePage(t *testing.T) {
	a := newAppWithBase(t, "/browser")
	body, _ := io.ReadAll(get(a, "/browser/").Body)
	page := string(body)

	if !strings.Contains(page, `<base href="/browser/"`) {
		t.Errorf("base element missing:\n%s", page[:min(len(page), 400)])
	}
	if strings.Contains(page, "{{BASE_PATH}}") || strings.Contains(page, "{{API_BASE}}") {
		t.Error("a placeholder was left in the page")
	}
	// Relative asset URLs are what make the page portable.
	if strings.Contains(page, `href="/css/`) {
		t.Error("absolute asset URL survived: it would escape the prefix")
	}
}

func TestBasePathRootByDefault(t *testing.T) {
	a := newTestApp(t)
	body, _ := io.ReadAll(get(a, "/").Body)
	if !strings.Contains(string(body), `<base href="/"`) {
		t.Error("default base should be the site root")
	}
}

func TestBasePathIsNotStoredInTheConfig(t *testing.T) {
	// The prefix describes how the process was started, so it must not appear
	// in the saved configuration at all.
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	store, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Update(func(c *config.Config) error {
		c.Auth.UsersFile = filepath.Join(dir, "users.json")
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "base_path") {
		t.Errorf("base_path was written to the config:\n%s", data)
	}
}
