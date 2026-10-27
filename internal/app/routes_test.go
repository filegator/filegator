package app

import (
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/filegator/filegator/internal/router"
)

// TestDocumentedRoutesMatchServer walks the route table of a real App and
// checks that every route appears in a generated block of docs/content/api.md.
//
// The generator (internal/apidoc) reads a zero Server, which is enough for the
// api and admin packages but not for the routes the App itself registers. This
// test is the other half: it answers the question "does the page document what
// the server will actually answer?" by asking a started server.
//
// It runs `go test` from the package directory, so the documentation is found
// by walking up rather than by assuming the working directory.
func TestDocumentedRoutesMatchServer(t *testing.T) {
	page := readDoc(t, "docs/content/api.md")

	rt := router.New()
	a := &App{router: rt}
	a.registerFunctionRoutes(rt)

	var undocumented []string
	for _, r := range rt.Routes() {
		row := "| `" + r.Method + "` | `" + r.Pattern() + "` |"
		if !strings.Contains(page, row) {
			undocumented = append(undocumented, r.Method+" "+r.Pattern())
		}
	}
	sort.Strings(undocumented)
	if len(undocumented) > 0 {
		t.Fatalf("docs/content/api.md does not document these routes:\n  %s\n"+
			"run: make docs", strings.Join(undocumented, "\n  "))
	}
}

// TestNoDuplicateDocumentedRoutes catches a stale generated block that was not
// rewritten, which would list the same endpoint twice and hide a removed one.
func TestNoDuplicateDocumentedRoutes(t *testing.T) {
	page := readDoc(t, "docs/content/api.md")

	seen := map[string]int{}
	for _, line := range strings.Split(page, "\n") {
		if !strings.HasPrefix(line, "| `") {
			continue
		}
		cells := strings.Split(line, "|")
		if len(cells) < 4 {
			continue
		}
		key := strings.TrimSpace(cells[1]) + " " + strings.TrimSpace(cells[2])
		seen[key]++
	}
	for key, n := range seen {
		if n > 1 {
			t.Errorf("%s appears %d times in docs/content/api.md; run: make docs", key, n)
		}
	}
}

// readDoc reads a repository file, walking up from the test's working
// directory until it finds it.
func readDoc(t *testing.T, name string) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for i := 0; i < 8; i++ {
		path := dir + string(os.PathSeparator) + name
		if data, err := os.ReadFile(path); err == nil {
			return string(data)
		}
		parent := dir[:strings.LastIndex(dir, string(os.PathSeparator))]
		if parent == dir || parent == "" {
			break
		}
		dir = parent
	}
	t.Fatalf("%s not found above the test directory", name)
	return ""
}
