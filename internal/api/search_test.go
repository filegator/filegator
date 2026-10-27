package api

import (
	"net/http"
	"testing"

	"github.com/filegator/filegator/internal/auth"
)

// searchEnv returns an environment with a reader whose home holds a small tree.
//
// The base environment creates /alice/notes.md and /alice/docs/report.txt.
func searchEnv(t *testing.T) *env {
	t.Helper()
	e := newEnv(t)
	e.addUser("searcher", "pw", "/alice", auth.RoleUser, auth.PermRead)
	return e
}

// search asks for results as the given user.
func (e *env) search(session, query string) (*http.Response, SearchResponse) {
	e.t.Helper()
	rec := e.do(http.MethodGet, "/api/files/search?"+query, "", session)
	var body SearchResponse
	if rec.Code == http.StatusOK {
		decode(e.t, rec, &body)
	}
	return rec.Result(), body
}

func TestSearchFindsNestedFiles(t *testing.T) {
	e := searchEnv(t)
	session := e.login("searcher", "pw")

	_, body := e.search(session, "q=report")
	if len(body.Results) != 1 {
		t.Fatalf("results = %+v", body.Results)
	}
	got := body.Results[0]
	if got.Path != "/docs/report.txt" {
		t.Errorf("path = %q", got.Path)
	}
	if got.Parent != "/docs" {
		t.Errorf("parent = %q, want /docs", got.Parent)
	}
	if got.Type != "file" {
		t.Errorf("type = %q", got.Type)
	}
}

func TestSearchIsCaseInsensitive(t *testing.T) {
	e := searchEnv(t)
	session := e.login("searcher", "pw")

	_, body := e.search(session, "q=NOTES")
	if len(body.Results) != 1 || body.Results[0].Name != "notes.md" {
		t.Fatalf("results = %+v", body.Results)
	}
}

func TestSearchScopedToPath(t *testing.T) {
	e := searchEnv(t)
	session := e.login("searcher", "pw")

	// /docs must not leak the match in /.
	_, body := e.search(session, "path=/docs&q=notes")
	if len(body.Results) != 0 {
		t.Errorf("results = %+v", body.Results)
	}

	_, body = e.search(session, "path=/docs&q=report")
	if len(body.Results) != 1 {
		t.Errorf("results = %+v", body.Results)
	}
}

func TestSearchRequiresTerm(t *testing.T) {
	e := searchEnv(t)
	session := e.login("searcher", "pw")

	res, _ := e.search(session, "")
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("status %d, want 400", res.StatusCode)
	}
}

func TestSearchRequiresReadPermission(t *testing.T) {
	e := newEnv(t)
	e.addUser("blind", "pw", "/alice", auth.RoleUser) // no permissions
	session := e.login("blind", "pw")

	res, _ := e.search(session, "q=notes")
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("status %d, want 403", res.StatusCode)
	}
}

func TestSearchRequiresLogin(t *testing.T) {
	e := searchEnv(t)
	res, _ := e.search("", "q=notes")
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status %d, want 401", res.StatusCode)
	}
}

func TestSearchRejectsFileAsPath(t *testing.T) {
	e := searchEnv(t)
	session := e.login("searcher", "pw")

	res, _ := e.search(session, "path=/notes.md&q=notes")
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("status %d, want 400", res.StatusCode)
	}
}

func TestSearchMissingPath(t *testing.T) {
	e := searchEnv(t)
	session := e.login("searcher", "pw")

	res, _ := e.search(session, "path=/nope&q=notes")
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("status %d, want 404", res.StatusCode)
	}
}

func TestSearchConfinedToHome(t *testing.T) {
	e := newEnv(t)
	// A file outside the user's home must never appear in their results.
	e.write("outside/secret.txt", "x")
	e.addUser("searcher", "pw", "/alice", auth.RoleUser, auth.PermRead)
	session := e.login("searcher", "pw")

	_, body := e.search(session, "q=secret")
	if len(body.Results) != 0 {
		t.Errorf("search escaped the home folder: %+v", body.Results)
	}
}

func TestSearchFindsDirectories(t *testing.T) {
	e := searchEnv(t)
	session := e.login("searcher", "pw")

	_, body := e.search(session, "q=docs")
	if len(body.Results) != 1 || body.Results[0].Type != "dir" {
		t.Fatalf("results = %+v", body.Results)
	}
}

func TestSearchNoMatches(t *testing.T) {
	e := searchEnv(t)
	session := e.login("searcher", "pw")

	_, body := e.search(session, "q=zzzzz")
	if len(body.Results) != 0 || body.Limited {
		t.Fatalf("body = %+v", body)
	}
}
