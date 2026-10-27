package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/filegator/filegator/internal/auth"
	"github.com/filegator/filegator/internal/config"
	"github.com/filegator/filegator/internal/security"
)

// writeEnv extends env with a user holding read+write+chmod and a session.
type writeEnv struct {
	*env
	session string
	csrf    string
}

func newWriteEnv(t *testing.T) *writeEnv {
	t.Helper()
	e := newEnv(t)
	e.addUser("alice", "pw-alice", "/alice", auth.RoleUser,
		auth.PermRead, auth.PermWrite, auth.PermChmod)
	session := e.login("alice", "pw-alice")

	// Issue a CSRF token the way the real frontend would.
	e.srv.Config.Update(func(c *config.Config) error {
		c.Security.CSRFProtection = true
		return nil
	})
	token, _, err := e.srv.CSRF.Token(httptest.NewRequest(http.MethodGet, "/", nil))
	if err != nil {
		t.Fatal(err)
	}
	return &writeEnv{env: e, session: session, csrf: token}
}

// post performs a state-changing request with CSRF and session cookies.
func (w *writeEnv) post(path, body string) *httptest.ResponseRecorder {
	w.t.Helper()
	r := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	r.RemoteAddr = "127.0.0.1:12345"
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set(security.CSRFHeader, w.csrf)
	r.AddCookie(&http.Cookie{Name: security.CookieName, Value: w.session})
	r.AddCookie(&http.Cookie{Name: security.CSRFCookieName, Value: w.csrf})
	rec := httptest.NewRecorder()
	w.router.ServeHTTP(rec, r)
	return rec
}

// exists reports whether a path exists inside alice's home.
func (w *writeEnv) exists(rel string) bool {
	return fileExists(w.t, filepath.Join(w.root.Dir(), "alice", filepath.FromSlash(strings.TrimPrefix(rel, "/"))))
}

func fileExists(t *testing.T, full string) bool {
	t.Helper()
	_, err := os.Stat(full)
	return err == nil
}

func TestMkdir(t *testing.T) {
	w := newWriteEnv(t)
	rec := w.post("/api/files/mkdir", `{"parent":"/","name":"newdir"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status %d body %s", rec.Code, rec.Body)
	}
	var e Entry
	decode(t, rec, &e)
	if e.Name != "newdir" || e.Type != "dir" || e.Path != "/newdir" {
		t.Errorf("entry = %+v", e)
	}
	if !w.exists("/newdir") {
		t.Error("directory not created on disk")
	}
}

func TestMkdirDuplicate(t *testing.T) {
	w := newWriteEnv(t)
	if rec := w.post("/api/files/mkdir", `{"parent":"/","name":"docs"}`); rec.Code != http.StatusConflict {
		t.Fatalf("status %d, want 409", rec.Code)
	}
}

func TestMkdirInvalidName(t *testing.T) {
	w := newWriteEnv(t)
	for _, name := range []string{"", ".", "..", "a/b", "../escape"} {
		body := `{"parent":"/","name":"` + name + `"}`
		rec := w.post("/api/files/mkdir", body)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("name %q: status %d, want 400", name, rec.Code)
		}
	}
}

func TestMkdirCannotEscapeHome(t *testing.T) {
	w := newWriteEnv(t)
	rec := w.post("/api/files/mkdir", `{"parent":"/../bob","name":"evil"}`)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status %d, want 403", rec.Code)
	}
	if fileExists(t, filepath.Join(w.root.Dir(), "bob", "evil")) {
		t.Fatal("created a directory outside the home")
	}
}

func TestCreateFile(t *testing.T) {
	w := newWriteEnv(t)
	rec := w.post("/api/files/create", `{"parent":"/","name":"notes.txt"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status %d body %s", rec.Code, rec.Body)
	}
	var e Entry
	decode(t, rec, &e)
	if e.Size != 0 || e.Type != "file" {
		t.Errorf("entry = %+v", e)
	}
	info, err := os.Stat(filepath.Join(w.root.Dir(), "alice", "notes.txt"))
	if err != nil {
		t.Fatalf("file not created: %v", err)
	}
	if info.Size() != 0 {
		t.Errorf("size = %d, want 0", info.Size())
	}
}

func TestCreateFileDuplicate(t *testing.T) {
	w := newWriteEnv(t)
	if rec := w.post("/api/files/create", `{"parent":"/","name":"notes.md"}`); rec.Code != http.StatusConflict {
		t.Fatalf("status %d, want 409", rec.Code)
	}
}

func TestRename(t *testing.T) {
	w := newWriteEnv(t)
	rec := w.post("/api/files/rename", `{"from":"/notes.md","to":"/renamed.md"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body)
	}
	if w.exists("/notes.md") || !w.exists("/renamed.md") {
		t.Error("rename did not take effect")
	}
}

func TestRenameIntoDirectory(t *testing.T) {
	w := newWriteEnv(t)
	rec := w.post("/api/files/rename", `{"from":"/notes.md","to":"/docs/notes.md"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body)
	}
	if !w.exists("/docs/notes.md") {
		t.Error("moved file not in target directory")
	}
}

func TestRenameRefusesOverwrite(t *testing.T) {
	w := newWriteEnv(t)
	rec := w.post("/api/files/rename", `{"from":"/notes.md","to":"/docs/report.txt"}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status %d, want 409", rec.Code)
	}
	if !w.exists("/docs/report.txt") {
		t.Fatal("existing file was overwritten")
	}
}

func TestRenameCannotEscapeHome(t *testing.T) {
	w := newWriteEnv(t)
	rec := w.post("/api/files/rename", `{"from":"/notes.md","to":"/../bob/stolen.md"}`)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status %d, want 403", rec.Code)
	}
	if fileExists(t, filepath.Join(w.root.Dir(), "bob", "stolen.md")) {
		t.Fatal("file escaped the home")
	}
}

func TestMove(t *testing.T) {
	w := newWriteEnv(t)
	rec := w.post("/api/files/move", `{"paths":["/notes.md"],"target":"/docs"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body)
	}
	var moved []Entry
	decode(t, rec, &moved)
	if len(moved) != 1 || moved[0].Path != "/docs/notes.md" {
		t.Errorf("moved = %+v", moved)
	}
	if w.exists("/notes.md") {
		t.Error("source still present")
	}
}

func TestCopy(t *testing.T) {
	w := newWriteEnv(t)
	rec := w.post("/api/files/copy", `{"paths":["/notes.md"],"target":"/docs"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body)
	}
	if !w.exists("/notes.md") || !w.exists("/docs/notes.md") {
		t.Error("copy should leave the source in place")
	}
	data, err := os.ReadFile(filepath.Join(w.root.Dir(), "alice", "docs", "notes.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "notes" {
		t.Errorf("content = %q", data)
	}
}

func TestCopyDirectory(t *testing.T) {
	w := newWriteEnv(t)
	rec := w.post("/api/files/copy", `{"paths":["/docs"],"target":"/"}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status %d, want 409 (target exists)", rec.Code)
	}

	rec = w.post("/api/files/mkdir", `{"parent":"/","name":"backup"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("mkdir status %d", rec.Code)
	}
	rec = w.post("/api/files/copy", `{"paths":["/docs"],"target":"/backup"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("copy dir status %d body %s", rec.Code, rec.Body)
	}
	if !w.exists("/backup/docs/report.txt") {
		t.Error("directory contents not copied")
	}
}

func TestDelete(t *testing.T) {
	w := newWriteEnv(t)
	rec := w.post("/api/files/delete", `{"paths":["/notes.md"]}`)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status %d body %s", rec.Code, rec.Body)
	}
	if w.exists("/notes.md") {
		t.Error("file still exists")
	}
}

func TestDeleteDirectoryRecursive(t *testing.T) {
	w := newWriteEnv(t)
	rec := w.post("/api/files/delete", `{"paths":["/docs"]}`)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status %d", rec.Code)
	}
	if w.exists("/docs/report.txt") {
		t.Error("nested file survived")
	}
}

func TestDeleteCannotRemoveHome(t *testing.T) {
	w := newWriteEnv(t)
	rec := w.post("/api/files/delete", `{"paths":["/"]}`)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status %d, want 403", rec.Code)
	}
	if !w.exists("/notes.md") {
		t.Fatal("home was deleted")
	}
}

func TestDeleteCannotEscapeHome(t *testing.T) {
	w := newWriteEnv(t)
	rec := w.post("/api/files/delete", `{"paths":["/../secret.txt"]}`)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status %d, want 403", rec.Code)
	}
	if !fileExists(t, filepath.Join(w.root.Dir(), "secret.txt")) {
		t.Fatal("deleted a file outside the home")
	}
}

func TestChmod(t *testing.T) {
	w := newWriteEnv(t)
	rec := w.post("/api/files/chmod", `{"paths":["/notes.md"],"mode":"600"}`)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status %d body %s", rec.Code, rec.Body)
	}
	info, err := os.Stat(filepath.Join(w.root.Dir(), "alice", "notes.md"))
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("mode = %o, want 600", perm)
	}
}

func TestChmodAcceptsFourDigits(t *testing.T) {
	w := newWriteEnv(t)
	rec := w.post("/api/files/chmod", `{"paths":["/notes.md"],"mode":"0640"}`)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestChmodInvalidMode(t *testing.T) {
	w := newWriteEnv(t)
	for _, mode := range []string{"", "abc", "99", "7777", "8", "1234"} {
		body := `{"paths":["/notes.md"],"mode":"` + mode + `"}`
		if rec := w.post("/api/files/chmod", body); rec.Code != http.StatusBadRequest {
			t.Errorf("mode %q: status %d, want 400", mode, rec.Code)
		}
	}
}

func TestChmodRequiresPermission(t *testing.T) {
	e := newEnv(t)
	e.addUser("bob", "pw-bob", "/bob", auth.RoleUser, auth.PermRead, auth.PermWrite)
	session := e.login("bob", "pw-bob")

	r := httptest.NewRequest(http.MethodPost, "/api/files/chmod", strings.NewReader(`{"paths":["/secret.txt"],"mode":"777"}`))
	r.AddCookie(&http.Cookie{Name: security.CookieName, Value: session})
	r.AddCookie(&http.Cookie{Name: security.CSRFCookieName, Value: "t"})
	r.Header.Set(security.CSRFHeader, "t")
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, r)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status %d, want 403", rec.Code)
	}
}

func TestSave(t *testing.T) {
	w := newWriteEnv(t)
	rec := w.post("/api/files/save", `{"path":"/notes.md","content":"updated content"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body)
	}
	data, err := os.ReadFile(filepath.Join(w.root.Dir(), "alice", "notes.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "updated content" {
		t.Errorf("content = %q", data)
	}
}

func TestSaveRejectsNonEditableType(t *testing.T) {
	w := newWriteEnv(t)
	w.write("alice/image.png", "binary")
	rec := w.post("/api/files/save", `{"path":"/image.png","content":"x"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d, want 400", rec.Code)
	}
}

func TestWriteEndpointsRequireCSRF(t *testing.T) {
	// newEnv disables CSRF so other tests stay simple; enable it here.
	e := newEnv(t)
	if err := e.srv.Config.Update(func(c *config.Config) error {
		c.Security.CSRFProtection = true
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	e.addUser("alice", "pw-alice", "/alice", auth.RoleUser, auth.PermRead, auth.PermWrite)
	session := e.login("alice", "pw-alice")

	for _, path := range []string{
		"/api/files/mkdir", "/api/files/create", "/api/files/rename",
		"/api/files/move", "/api/files/copy", "/api/files/delete",
		"/api/files/chmod", "/api/files/save",
	} {
		r := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"path":"/x","name":"y"}`))
		r.AddCookie(&http.Cookie{Name: security.CookieName, Value: session})
		rec := httptest.NewRecorder()
		e.router.ServeHTTP(rec, r)
		if rec.Code != http.StatusForbidden {
			t.Errorf("%s without csrf: status %d, want 403 (body %s)", path, rec.Code, rec.Body)
		}
	}
}

func TestWriteRequiresLogin(t *testing.T) {
	e := newEnv(t)
	r := httptest.NewRequest(http.MethodPost, "/api/files/mkdir", strings.NewReader(`{"name":"x"}`))
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, r)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d, want 401", rec.Code)
	}
}

func TestWriteRequiresWritePermission(t *testing.T) {
	e := newEnv(t)
	e.addUser("ro", "pw-ro", "/alice", auth.RoleUser, auth.PermRead)
	session := e.login("ro", "pw-ro")

	r := httptest.NewRequest(http.MethodPost, "/api/files/delete", strings.NewReader(`{"paths":["/notes.md"]}`))
	r.AddCookie(&http.Cookie{Name: security.CookieName, Value: session})
	r.Header.Set(security.CSRFHeader, "t")
	r.AddCookie(&http.Cookie{Name: security.CSRFCookieName, Value: "t"})
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, r)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status %d, want 403", rec.Code)
	}
	if !fileExists(t, filepath.Join(e.root.Dir(), "alice", "notes.md")) {
		t.Fatal("read-only user deleted a file")
	}
}

func TestParseMode(t *testing.T) {
	good := map[string]os.FileMode{"644": 0o644, "600": 0o600, "777": 0o777, "000": 0, "0755": 0o755}
	for in, want := range good {
		got, err := parseMode(in)
		if err != nil {
			t.Errorf("parseMode(%q): %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("parseMode(%q) = %o, want %o", in, got, want)
		}
	}
	for _, bad := range []string{"", "abc", "99", "7777", "1234", "-x"} {
		if _, err := parseMode(bad); err == nil {
			t.Errorf("parseMode(%q) accepted", bad)
		}
	}
}

func TestTargetPath(t *testing.T) {
	got, err := targetPath("/", "a.txt")
	if err != nil || got != "/a.txt" {
		t.Errorf("got %q err %v", got, err)
	}
	got, err = targetPath("/docs", "a.txt")
	if err != nil || got != "/docs/a.txt" {
		t.Errorf("got %q err %v", got, err)
	}
	got, err = targetPath("", "a.txt")
	if err != nil || got != "/a.txt" {
		t.Errorf("empty parent: got %q err %v", got, err)
	}
	if _, err := targetPath("/", "../x"); err == nil {
		t.Error("traversal name accepted")
	}
}

// TestTargetPathRejectsTraversalParent guards the bug where path.Join
// silently collapsed "/../bob" into "/bob".
func TestTargetPathRejectsTraversalParent(t *testing.T) {
	for _, parent := range []string{"/..", "/../bob", "..", "/a/../../b", `/..\bob`} {
		if got, err := targetPath(parent, "x"); err == nil {
			t.Errorf("parent %q accepted, produced %q", parent, got)
		}
	}
}

// modeOf reads a file's permission bits from the repository.
func (w *writeEnv) modeOf(rel string) os.FileMode {
	w.t.Helper()
	info, err := os.Stat(filepath.Join(w.root.Dir(), "alice", filepath.FromSlash(strings.TrimPrefix(rel, "/"))))
	if err != nil {
		w.t.Fatalf("stat %s: %v", rel, err)
	}
	return info.Mode().Perm()
}

func TestChmodSingleEntry(t *testing.T) {
	w := chmodEnv(t)
	w.write("alice/notes.txt", "x")

	rec := w.post("/api/files/chmod", `{"paths":["/notes.txt"],"mode":"600"}`)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status %d body %s", rec.Code, rec.Body)
	}
	if got := w.modeOf("/notes.txt"); got != 0o600 {
		t.Errorf("mode = %o, want 600", got)
	}
}

func TestChmodRecursiveAll(t *testing.T) {
	w := chmodEnv(t)
	w.write("alice/tree/a.txt", "a")
	w.write("alice/tree/sub/b.txt", "b")

	rec := w.post("/api/files/chmod",
		`{"paths":["/tree"],"mode":"700","recursive":"all"}`)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status %d body %s", rec.Code, rec.Body)
	}
	for _, rel := range []string{"/tree", "/tree/a.txt", "/tree/sub", "/tree/sub/b.txt"} {
		if got := w.modeOf(rel); got != 0o700 {
			t.Errorf("%s mode = %o, want 700", rel, got)
		}
	}
}

func TestChmodRecursiveFoldersOnlyLeavesFiles(t *testing.T) {
	w := chmodEnv(t)
	w.write("alice/tree/a.txt", "a")
	w.write("alice/tree/sub/b.txt", "b")

	if rec := w.post("/api/files/chmod",
		`{"paths":["/tree"],"mode":"700","recursive":"folders"}`); rec.Code != http.StatusNoContent {
		t.Fatalf("status %d", rec.Code)
	}
	if got := w.modeOf("/tree/sub"); got != 0o700 {
		t.Errorf("folder mode = %o, want 700", got)
	}
	// Files keep the mode the write left them with (644).
	if got := w.modeOf("/tree/a.txt"); got == 0o700 {
		t.Error("recursive=folders changed a file")
	}
}

func TestChmodRecursiveFilesOnlyLeavesFolders(t *testing.T) {
	w := chmodEnv(t)
	w.write("alice/tree/a.txt", "a")
	before := w.modeOf("/tree")

	if rec := w.post("/api/files/chmod",
		`{"paths":["/tree"],"mode":"700","recursive":"files"}`); rec.Code != http.StatusNoContent {
		t.Fatalf("status %d", rec.Code)
	}
	if got := w.modeOf("/tree/a.txt"); got != 0o700 {
		t.Errorf("file mode = %o, want 700", got)
	}
	if got := w.modeOf("/tree"); got != before {
		t.Errorf("folder mode changed from %o to %o", before, got)
	}
}

func TestChmodRejectsUnknownRecursive(t *testing.T) {
	w := chmodEnv(t)
	rec := w.post("/api/files/chmod",
		`{"paths":["/notes.md"],"mode":"700","recursive":"deep"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d, want 400", rec.Code)
	}
}

func TestChmodRejectsSpecialBits(t *testing.T) {
	// 4755 would set setuid; the dialog must not be able to request it and the
	// API must not accept it.
	w := chmodEnv(t)
	for _, mode := range []string{"4755", "2755", "1755", "999", "abc"} {
		rec := w.post("/api/files/chmod", `{"paths":["/notes.md"],"mode":"`+mode+`"}`)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("mode %q: status %d, want 400", mode, rec.Code)
		}
	}
}

// chmodEnv returns an environment whose user may read, write and chmod.
func chmodEnv(t *testing.T) *writeEnv {
	t.Helper()
	e := newEnv(t)
	e.addUser("permer", "pw", "/alice", auth.RoleUser,
		auth.PermRead, auth.PermWrite, auth.PermChmod)
	session := e.login("permer", "pw")
	return &writeEnv{env: e, session: session}
}
