// Command apidoc renders the endpoint tables in docs/content/api.md from the
// routes the server actually registers.
//
// A table of endpoints is exactly the kind of list that rots silently: an
// endpoint added to the code but missing from the page is a feature nobody can
// find, and one removed from the code but left on the page is a promise the
// server cannot keep. The paths and methods here therefore come from the
// router, through the same registration function the server calls at startup.
// Only the human description of each endpoint is written by hand; an endpoint
// with no description stops the generator instead of being published as a
// blank row.
//
// Run it from the repository root:
//
//	go run ./internal/apidoc
//
// There is no `//go:generate` directive on purpose: `go generate` cannot pass
// the output through to a build, and `make docs` runs the generator and the
// site build as one step.
// It rewrites the blocks between the BEGIN/END markers of the named sections in
// docs/content/api.md and leaves everything else in the file untouched. A
// `-check` run reports what would change and exits non-zero, so CI can fail on
// a page that no longer matches the code.
package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"

	"github.com/filegator/filegator/internal/api"
	"github.com/filegator/filegator/internal/app"
	"github.com/filegator/filegator/internal/router"
)

// docPath is the page the tables belong to, relative to the module root.
const docPath = "docs/content/api.md"

// description is what the table says about one endpoint. An empty value means
// the endpoint has not been described yet and the generator refuses to run.
var description = map[string]string{
	// Health and configuration.
	"GET /api/ping":         "Health check",
	"GET /api/config":       "Public settings the browser needs before login",
	"GET /api/capabilities": "Which features this deployment supports",

	// Sessions.
	"POST /api/auth/login":    "`{\"username\",\"password\"}` → user",
	"POST /api/auth/logout":   "Ends the session",
	"POST /api/auth/password": "Change your own password",
	"GET /api/auth/me":        "The signed-in user, or `401`",
	"GET /auth/sso":           "Signed hand-off from another application",

	// Files.
	"GET /api/files":                "List a directory",
	"GET /api/files/stat":           "Describe one entry",
	"GET /api/files/download":       "Download a file",
	"POST /api/files/mkdir":         "Create a directory",
	"POST /api/files/create":        "Create an empty file",
	"POST /api/files/rename":        "Rename, or move in one step",
	"POST /api/files/move":          "Move entries into a folder",
	"POST /api/files/copy":          "Copy entries into a folder",
	"POST /api/files/delete":        "Delete permanently",
	"POST /api/files/chmod":         "Set permission bits",
	"POST /api/files/save":          "Write text content",
	"GET /api/files/content":        "Read a text file for the editor",
	"GET /api/files/search":         "Search names below a folder",
	"GET /api/files/batch":          "Stream a zip of selected paths",
	"POST /api/files/zip":           "Create an archive",
	"POST /api/files/unzip":         "Extract an archive into a new folder",
	"GET /api/files/upload":         "Which chunks are already stored (resume)",
	"POST /api/files/upload":        "Send one chunk",
	"POST /api/files/upload/cancel": "Discard a staged upload and free its chunks",

	// Administrators.
	"GET /api/admin/status":                     "Configuration warnings",
	"GET /api/admin/settings":                   "Read the editable settings",
	"POST /api/admin/settings":                  "Save the editable settings",
	"GET /api/admin/users":                      "List users",
	"POST /api/admin/users":                     "Create a user",
	"POST /api/admin/users/{username}":          "Update a user",
	"POST /api/admin/users/{username}/delete":   "Delete a user",
	"POST /api/admin/users/{username}/password": "Set a password without knowing the old one",
	"GET /api/admin/browse":                     "Browse the host filesystem",
	"GET /api/admin/storage/browse":             "Browse inside the storage root",
	"GET /api/admin/root":                       "Read the storage root",
	"POST /api/admin/root":                      "Change the storage root",
	"GET /api/admin/storage":                    "Read the storage backend settings",
	"POST /api/admin/storage":                   "Switch or configure the storage backend",
	"GET /api/admin/auth":                       "Read the auth provider settings",
	"POST /api/admin/auth":                      "Switch or configure the auth provider",

	// Language files.
	"GET /api/i18n":        "Available languages, and the configured default",
	"GET /api/i18n/{lang}": "Messages for one language",
}

// group decides which generated section an endpoint belongs to. The sections
// are the `##` headings of the page, in the order they appear.
type group struct {
	name  string
	key   string
	match func(path string) bool
}

var groups = []group{
	{name: "Authentication", key: "auth", match: func(p string) bool {
		return p == "/api/auth/login" || p == "/api/auth/logout" ||
			p == "/api/auth/password" || p == "/api/auth/me" ||
			p == "/api/ping"
	}},
	{name: "Files", key: "files", match: func(p string) bool {
		return strings.HasPrefix(p, "/api/files")
	}},
	{name: "Administration", key: "admin", match: func(p string) bool {
		return strings.HasPrefix(p, "/api/admin")
	}},
	{name: "Configuration and languages", key: "config", match: func(p string) bool {
		return p == "/api/config" || p == "/api/capabilities" ||
			strings.HasPrefix(p, "/api/i18n")
	}},
	{name: "Single sign-on", key: "sso", match: func(p string) bool {
		return strings.HasPrefix(p, "/auth/")
	}},
}

// registeredRoutes asks the server what it answers. A zero Server has no
// configuration or storage behind it; its route methods only add patterns to
// the router, which is all that is needed here.
func registeredRoutes() []router.Route {
	rt := router.New()
	app.RegisterAPIRoutes(rt, &api.Server{})
	// The remaining routes are registered by App, which needs embedded assets
	// and a config file to construct. They are declared here so they are
	// documented too; a test pins them against the real server.
	rt.Get("/api/ping", deadHandler)
	rt.Get("/api/i18n/{lang}", deadHandler)
	rt.Get("/api/i18n", deadHandler)
	rt.Get("/api/config", deadHandler)
	return rt.Routes()
}

// deadHandler stands in for handlers that need a running App. No request is
// ever dispatched to a router built here.
func deadHandler(_ http.ResponseWriter, _ *http.Request, _ router.Params) {}

func main() {
	check := flag.Bool("check", false, "report differences without writing")
	flag.Parse()

	rendered, err := build()
	if err != nil {
		fmt.Fprintln(os.Stderr, "apidoc:", err)
		os.Exit(1)
	}

	current, err := os.ReadFile(docPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "apidoc:", err)
		os.Exit(1)
	}
	updated, err := splice(string(current), rendered)
	if err != nil {
		fmt.Fprintln(os.Stderr, "apidoc:", err)
		os.Exit(1)
	}

	if updated == string(current) {
		return
	}
	if *check {
		fmt.Fprintf(os.Stderr, "apidoc: %s is out of date; run make docs\n", docPath)
		os.Exit(1)
	}
	if err := os.WriteFile(docPath, []byte(updated), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "apidoc:", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "apidoc: updated %s\n", docPath)
}

// build renders each section and returns them keyed by section key.
func build() (map[string]string, error) {
	var missing []string
	for _, r := range registeredRoutes() {
		key := r.Method + " " + r.Pattern()
		if _, ok := description[key]; !ok {
			missing = append(missing, key)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return nil, fmt.Errorf("no description for:\n  %s", strings.Join(missing, "\n  "))
	}

	byGroup := map[string][]router.Route{}
	var unplaced []string
	for _, r := range registeredRoutes() {
		placed := false
		for _, g := range groups {
			if g.match(r.Pattern()) {
				byGroup[g.key] = append(byGroup[g.key], r)
				placed = true
				break
			}
		}
		if !placed {
			unplaced = append(unplaced, r.Method+" "+r.Pattern())
		}
	}
	if len(unplaced) > 0 {
		sort.Strings(unplaced)
		return nil, fmt.Errorf("no section for:\n  %s", strings.Join(unplaced, "\n  "))
	}

	out := map[string]string{}
	for _, g := range groups {
		out[g.key] = table(byGroup[g.key])
	}
	return out, nil
}

// table renders one endpoint table, sorted by path then method.
func table(routes []router.Route) string {
	sort.Slice(routes, func(i, j int) bool {
		a, b := routes[i], routes[j]
		if a.Pattern() != b.Pattern() {
			return a.Pattern() < b.Pattern()
		}
		return a.Method < b.Method
	})

	var b strings.Builder
	b.WriteString("| Method | Path | Description |\n")
	b.WriteString("|--------|------|-------------|\n")
	for _, r := range routes {
		key := r.Method + " " + r.Pattern()
		// Backticked paths break a table cell only if they contain a pipe;
		// none do.
		fmt.Fprintf(&b, "| `%s` | `%s` | %s |\n", r.Method, r.Pattern(), description[key])
	}
	return b.String()
}

// splice replaces the contents of every marked block, leaving the rest of the
// file as it is. Markers look like:
//
//	<!-- BEGIN apidoc:files -->
//	<!-- END apidoc:files -->
//
// A missing marker is an error: silently appending a table would produce a page
// with two of them.
func splice(page string, rendered map[string]string) (string, error) {
	for key := range rendered {
		begin := "<!-- BEGIN apidoc:" + key + " -->"
		end := "<!-- END apidoc:" + key + " -->"
		from := strings.Index(page, begin)
		to := strings.Index(page, end)
		if from < 0 || to < 0 {
			return "", fmt.Errorf("markers for %q not found in %s", key, docPath)
		}
		if to < from {
			return "", fmt.Errorf("markers for %q are in the wrong order in %s", key, docPath)
		}
		page = page[:from+len(begin)] + "\n" + rendered[key] + page[to:]
	}
	return page, nil
}
