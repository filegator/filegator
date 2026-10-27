package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/filegator/filegator/internal/auth"
	"github.com/filegator/filegator/internal/auth/jsonfile"
	"github.com/filegator/filegator/internal/config"
	"github.com/filegator/filegator/internal/router"
	"github.com/filegator/filegator/internal/security"
	"github.com/filegator/filegator/internal/sso"
	"github.com/filegator/filegator/internal/storage/local"
)

// env is a fully wired API server with a populated repository.
type env struct {
	t      *testing.T
	srv    *Server
	router *router.Router
	root   *local.Root
}

func newEnv(t *testing.T) *env {
	t.Helper()
	dir := t.TempDir()

	cfgPath := filepath.Join(dir, "config.json")
	store, err := config.Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Update(func(c *config.Config) error {
		c.Auth.UsersFile = filepath.Join(dir, "users.json")
		c.Storage.Root = filepath.Join(dir, "repository")
		c.Security.CSRFProtection = false
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	cfg := store.Get()

	root, err := local.NewRoot(cfg.Storage.Root)
	if err != nil {
		t.Fatal(err)
	}

	sessions, err := security.NewSessions([]byte("test-session-key"), time.Hour, false)
	if err != nil {
		t.Fatal(err)
	}
	ipFilter, err := security.NewIPFilter(nil, nil, "")
	if err != nil {
		t.Fatal(err)
	}

	srv := &Server{
		Config:   store,
		Users:    jsonfile.NewUsersFile(cfg.Auth.UsersFile),
		Sessions: sessions,
		CSRF:     security.NewCSRF(false),
		Lockout:  security.NewLockout(3, time.Minute),
		IPFilter: ipFilter,
		SSO:      sso.NewVerifier(),
	}

	srv.SetRoot(root, root.Dir())

	rt := router.New()
	srv.AuthRoutes(rt)
	srv.Routes(rt)
	srv.WriteRoutes(rt)
	srv.UploadRoutes(rt)
	srv.EditRoutes(rt)
	srv.ArchiveRoutes(rt)
	srv.SearchRoutes(rt)
	srv.SSOAuthRoutes(rt)
	srv.CapabilitiesRoutes(rt)

	e := &env{t: t, srv: srv, router: rt, root: root}
	// The shipped users.json enables guest access by default. Most tests are
	// about authenticated behaviour, so the baseline switches it off; the
	// guest tests turn it back on explicitly.
	e.setGuestPermissions()
	e.write("alice/docs/report.txt", "alice report")
	e.write("alice/notes.md", "notes")
	e.write("bob/secret.txt", "bob secret")
	e.write("top.txt", "top level")
	e.write("secret.txt", "admin only")
	return e
}

// setGuestPermissions replaces the guest's permissions. With none, anonymous
// requests are refused exactly as they were before guest access existed.
func (e *env) setGuestPermissions(perms ...auth.Permission) {
	e.t.Helper()
	guest, err := e.srv.Users.Find("guest")
	if err != nil {
		e.t.Fatalf("no guest account: %v", err)
	}
	guest.Permissions = perms
	if _, err := e.srv.Users.Update("guest", guest, ""); err != nil {
		e.t.Fatalf("set guest permissions: %v", err)
	}
}

// write creates a file below the repository root.
func (e *env) write(rel, content string) {
	e.t.Helper()
	full := filepath.Join(e.root.Dir(), filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		e.t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		e.t.Fatal(err)
	}
}

// addUser creates a user with a bcrypt password.
func (e *env) addUser(username, password, home string, role auth.Role, perms ...auth.Permission) {
	e.t.Helper()
	_, err := e.srv.Users.Add(&auth.User{
		Username:    username,
		Name:        username,
		Role:        role,
		HomeDir:     home,
		Permissions: perms,
	}, password)
	if err != nil {
		e.t.Fatalf("addUser %s: %v", username, err)
	}
}

// login returns the session cookie value for username.
func (e *env) login(username, password string) string {
	e.t.Helper()
	body := `{"username":"` + username + `","password":"` + password + `"}`
	rec := e.do(http.MethodPost, "/api/auth/login", body, "")
	if rec.Code != http.StatusOK {
		e.t.Fatalf("login %s: status %d body %s", username, rec.Code, rec.Body)
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == security.CookieName {
			return c.Value
		}
	}
	e.t.Fatalf("login %s: no session cookie", username)
	return ""
}

// do performs a request with an optional session cookie.
func (e *env) do(method, path, body, session string) *httptest.ResponseRecorder {
	e.t.Helper()
	var r *http.Request
	if body == "" {
		r = httptest.NewRequest(method, path, nil)
	} else {
		r = httptest.NewRequest(method, path, strings.NewReader(body))
	}
	r.RemoteAddr = "127.0.0.1:12345"
	if session != "" {
		r.AddCookie(&http.Cookie{Name: security.CookieName, Value: session})
	}
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, r)
	return rec
}

// decode unmarshals the "data" field of a response into dst.
func decode(t *testing.T, rec *httptest.ResponseRecorder, dst any) {
	t.Helper()
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode body %q: %v", rec.Body, err)
	}
	if err := json.Unmarshal(envelope.Data, dst); err != nil {
		t.Fatalf("decode data %q: %v", envelope.Data, err)
	}
}

func TestLoginAndMe(t *testing.T) {
	e := newEnv(t)
	e.addUser("alice", "pw-alice", "/alice", auth.RoleUser, auth.PermRead)

	session := e.login("alice", "pw-alice")

	rec := e.do(http.MethodGet, "/api/auth/me", "", session)
	if rec.Code != http.StatusOK {
		t.Fatalf("me: status %d (body %s)", rec.Code, rec.Body)
	}
	var me UserResponse
	decode(t, rec, &me)
	if me.Username != "alice" || me.HomeDir != "/alice" {
		t.Errorf("me = %+v", me)
	}
	if len(me.Permissions) != 1 || me.Permissions[0] != "read" {
		t.Errorf("permissions = %v", me.Permissions)
	}
}

func TestLoginResponseHasNoPassword(t *testing.T) {
	e := newEnv(t)
	e.addUser("alice", "pw-alice", "/alice", auth.RoleUser, auth.PermRead)

	rec := e.do(http.MethodPost, "/api/auth/login", `{"username":"alice","password":"pw-alice"}`, "")
	body := rec.Body.String()
	if strings.Contains(body, "password") || strings.Contains(body, "$2y$") {
		t.Errorf("login response leaks password data: %s", body)
	}
}

func TestLoginWrongPassword(t *testing.T) {
	e := newEnv(t)
	e.addUser("alice", "pw-alice", "/alice", auth.RoleUser, auth.PermRead)

	rec := e.do(http.MethodPost, "/api/auth/login", `{"username":"alice","password":"wrong"}`, "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "invalid username or password") {
		t.Errorf("body = %s", rec.Body)
	}
}

func TestLoginUnknownUserSameMessage(t *testing.T) {
	e := newEnv(t)
	rec := e.do(http.MethodPost, "/api/auth/login", `{"username":"ghost","password":"x"}`, "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "invalid username or password") {
		t.Errorf("unknown user leaks existence: %s", rec.Body)
	}
}

func TestLoginLockout(t *testing.T) {
	e := newEnv(t)
	e.addUser("alice", "pw-alice", "/alice", auth.RoleUser, auth.PermRead)

	for i := 0; i < 3; i++ {
		e.do(http.MethodPost, "/api/auth/login", `{"username":"alice","password":"bad"}`, "")
	}
	rec := e.do(http.MethodPost, "/api/auth/login", `{"username":"alice","password":"pw-alice"}`, "")
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", rec.Code)
	}
}

func TestMeWithoutSession(t *testing.T) {
	e := newEnv(t)
	rec := e.do(http.MethodGet, "/api/auth/me", "", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestLogout(t *testing.T) {
	e := newEnv(t)
	e.addUser("alice", "pw-alice", "/alice", auth.RoleUser, auth.PermRead)
	rec := e.do(http.MethodPost, "/api/auth/logout", "", "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d", rec.Code)
	}
	found := false
	for _, c := range rec.Result().Cookies() {
		if c.Name == security.CookieName && c.MaxAge < 0 {
			found = true
		}
	}
	if !found {
		t.Error("logout did not clear the session cookie")
	}
}

func TestSessionInvalidatedWhenPermissionsChange(t *testing.T) {
	e := newEnv(t)
	e.addUser("alice", "pw-alice", "/alice", auth.RoleUser, auth.PermRead)
	session := e.login("alice", "pw-alice")

	u, err := e.srv.Users.Find("alice")
	if err != nil {
		t.Fatal(err)
	}
	u.Permissions = []auth.Permission{auth.PermRead, auth.PermWrite}
	if _, err := e.srv.Users.Update("alice", u, ""); err != nil {
		t.Fatal(err)
	}

	rec := e.do(http.MethodGet, "/api/auth/me", "", session)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 after permission change", rec.Code)
	}
}

func TestSessionInvalidatedWhenPasswordChanges(t *testing.T) {
	e := newEnv(t)
	e.addUser("alice", "pw-alice", "/alice", auth.RoleUser, auth.PermRead)
	session := e.login("alice", "pw-alice")

	u, _ := e.srv.Users.Find("alice")
	if _, err := e.srv.Users.Update("alice", u, "new-password"); err != nil {
		t.Fatal(err)
	}

	if rec := e.do(http.MethodGet, "/api/auth/me", "", session); rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestListDir(t *testing.T) {
	e := newEnv(t)
	e.addUser("alice", "pw-alice", "/alice", auth.RoleUser, auth.PermRead, auth.PermDownload)
	session := e.login("alice", "pw-alice")

	rec := e.do(http.MethodGet, "/api/files?path=/", "", session)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body)
	}
	var listing DirListing
	decode(t, rec, &listing)

	if listing.Path != "/" {
		t.Errorf("path = %q", listing.Path)
	}
	if listing.Parent != nil {
		t.Errorf("parent at home root = %v, want nil", *listing.Parent)
	}
	// Directories first, then files.
	if len(listing.Entries) != 2 {
		t.Fatalf("entries = %+v", listing.Entries)
	}
	if listing.Entries[0].Name != "docs" || listing.Entries[0].Type != "dir" {
		t.Errorf("first entry = %+v", listing.Entries[0])
	}
	if listing.Entries[1].Name != "notes.md" || listing.Entries[1].Type != "file" {
		t.Errorf("second entry = %+v", listing.Entries[1])
	}
	if listing.Entries[1].Path != "/notes.md" {
		t.Errorf("path = %q", listing.Entries[1].Path)
	}
	if listing.Entries[1].Mode != "644" {
		t.Errorf("mode = %q, want 644", listing.Entries[1].Mode)
	}
	if listing.Entries[1].Size != int64(len("notes")) {
		t.Errorf("size = %d", listing.Entries[1].Size)
	}
	if _, err := time.Parse(time.RFC3339, listing.Entries[1].Modified); err != nil {
		t.Errorf("modified %q is not RFC3339: %v", listing.Entries[1].Modified, err)
	}
}

func TestListDirSubdirectoryHasParent(t *testing.T) {
	e := newEnv(t)
	e.addUser("alice", "pw-alice", "/alice", auth.RoleUser, auth.PermRead)
	session := e.login("alice", "pw-alice")

	rec := e.do(http.MethodGet, "/api/files?path=/docs", "", session)
	var listing DirListing
	decode(t, rec, &listing)
	if listing.Path != "/docs" {
		t.Errorf("path = %q", listing.Path)
	}
	if listing.Parent == nil || *listing.Parent != "/" {
		t.Errorf("parent = %v, want /", listing.Parent)
	}
}

func TestListDirRequiresPermission(t *testing.T) {
	e := newEnv(t)
	e.addUser("alice", "pw-alice", "/alice", auth.RoleUser) // no permissions
	session := e.login("alice", "pw-alice")

	rec := e.do(http.MethodGet, "/api/files?path=/", "", session)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}

func TestListDirCannotEscapeHome(t *testing.T) {
	e := newEnv(t)
	e.addUser("alice", "pw-alice", "/alice", auth.RoleUser, auth.PermRead)
	session := e.login("alice", "pw-alice")

	for _, p := range []string{"/..", "/../bob", "/../../etc"} {
		rec := e.do(http.MethodGet, "/api/files?path="+p, "", session)
		if rec.Code != http.StatusForbidden {
			t.Errorf("path %q: status %d, want 403", p, rec.Code)
		}
	}
}

func TestListDirCannotSeeSiblings(t *testing.T) {
	e := newEnv(t)
	e.addUser("alice", "pw-alice", "/alice", auth.RoleUser, auth.PermRead)
	session := e.login("alice", "pw-alice")

	rec := e.do(http.MethodGet, "/api/files?path=/secret.txt", "", session)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestListDirMissingHome(t *testing.T) {
	e := newEnv(t)
	e.addUser("alice", "pw-alice", "/nonexistent", auth.RoleUser, auth.PermRead)
	session := e.login("alice", "pw-alice")

	rec := e.do(http.MethodGet, "/api/files?path=/", "", session)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}

func TestDownload(t *testing.T) {
	e := newEnv(t)
	e.addUser("alice", "pw-alice", "/alice", auth.RoleUser, auth.PermRead, auth.PermDownload)
	session := e.login("alice", "pw-alice")

	rec := e.do(http.MethodGet, "/api/files/download?path=/docs/report.txt", "", session)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body)
	}
	if got := rec.Body.String(); got != "alice report" {
		t.Errorf("body = %q", got)
	}
	cd := rec.Header().Get("Content-Disposition")
	if !strings.HasPrefix(cd, "attachment") || !strings.Contains(cd, "report.txt") {
		t.Errorf("Content-Disposition = %q", cd)
	}
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Error("missing nosniff")
	}
}

func TestDownloadInlinePDF(t *testing.T) {
	e := newEnv(t)
	e.addUser("alice", "pw-alice", "/alice", auth.RoleUser, auth.PermRead, auth.PermDownload)
	e.write("alice/doc.pdf", "%PDF-1.4")
	session := e.login("alice", "pw-alice")

	rec := e.do(http.MethodGet, "/api/files/download?path=/doc.pdf", "", session)
	if cd := rec.Header().Get("Content-Disposition"); !strings.HasPrefix(cd, "inline") {
		t.Errorf("Content-Disposition = %q, want inline", cd)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/pdf" {
		t.Errorf("Content-Type = %q", ct)
	}
}

// TestDownloadInlineListIsHonoured pins the meaning of the admin "Inline
// download extensions" list: only a listed extension is rendered, and a listed
// scriptable one is rendered with its real type rather than quietly refused
// (v7 behaviour, user decision 2026-10-26). The admin UI warns about that; the
// server does not second-guess the administrator.
func TestDownloadInlineListIsHonoured(t *testing.T) {
	e := newEnv(t)
	e.addUser("alice", "pw-alice", "/alice", auth.RoleUser, auth.PermRead, auth.PermDownload)
	e.write("alice/logo.svg", `<svg xmlns="http://www.w3.org/2000/svg"/>`)
	e.write("alice/page.html", `<p>hi</p>`)
	e.write("alice/notes.txt", "hi")
	e.write("alice/report.pdf", "%PDF-1.4")
	session := e.login("alice", "pw-alice")

	// By default SVG and HTML are attachments: they can carry script.
	for _, name := range []string{"/logo.svg", "/page.html"} {
		rec := e.do(http.MethodGet, "/api/files/download?path="+name, "", session)
		if cd := rec.Header().Get("Content-Disposition"); !strings.HasPrefix(cd, "attachment") {
			t.Errorf("%s: Content-Disposition = %q, want attachment by default", name, cd)
		}
		if ct := rec.Header().Get("Content-Type"); ct != "application/octet-stream" {
			t.Errorf("%s: Content-Type = %q, want octet-stream by default", name, ct)
		}
	}
	// A text file is inline only if listed; it is not by default either.
	if rec := e.do(http.MethodGet, "/api/files/download?path=/notes.txt", "", session); !strings.HasPrefix(rec.Header().Get("Content-Disposition"), "attachment") {
		t.Error("txt should be an attachment until it is listed")
	}

	if err := e.srv.Config.Update(func(c *config.Config) error {
		c.DownloadInline = []string{"pdf", "svg", "html", "md"} // typed with and without the dot
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	cases := map[string]string{
		"/logo.svg":   "image/svg+xml",
		"/page.html":  "text/html; charset=utf-8",
		"/report.pdf": "application/pdf",
	}
	for name, wantType := range cases {
		rec := e.do(http.MethodGet, "/api/files/download?path="+name, "", session)
		if cd := rec.Header().Get("Content-Disposition"); !strings.HasPrefix(cd, "inline") {
			t.Errorf("%s: Content-Disposition = %q, want inline once listed", name, cd)
		}
		if ct := rec.Header().Get("Content-Type"); ct != wantType {
			t.Errorf("%s: Content-Type = %q, want %q", name, ct, wantType)
		}
		// The content-type must not be sniffable into something else.
		if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Errorf("%s: missing nosniff", name)
		}
	}
	// An extension that is not listed stays an attachment even when a
	// sibling type is.
	if rec := e.do(http.MethodGet, "/api/files/download?path=/notes.txt", "", session); !strings.HasPrefix(rec.Header().Get("Content-Disposition"), "attachment") {
		t.Error("txt was not listed and must stay an attachment")
	}
}

// TestDownloadInlineWildcard covers the "*" entry v7 documented: every file
// renders, which is the dangerous setting the admin page warns about.
func TestDownloadInlineWildcard(t *testing.T) {
	e := newEnv(t)
	e.addUser("alice", "pw-alice", "/alice", auth.RoleUser, auth.PermRead, auth.PermDownload)
	e.write("alice/page.html", `<p>hi</p>`)
	session := e.login("alice", "pw-alice")

	if err := e.srv.Config.Update(func(c *config.Config) error {
		c.DownloadInline = []string{"*"}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	rec := e.do(http.MethodGet, "/api/files/download?path=/page.html", "", session)
	if cd := rec.Header().Get("Content-Disposition"); !strings.HasPrefix(cd, "inline") {
		t.Errorf("Content-Disposition = %q, want inline", cd)
	}
}

func TestDownloadRequiresPermission(t *testing.T) {
	e := newEnv(t)
	e.addUser("alice", "pw-alice", "/alice", auth.RoleUser, auth.PermRead)
	session := e.login("alice", "pw-alice")

	rec := e.do(http.MethodGet, "/api/files/download?path=/notes.md", "", session)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}

func TestDownloadCannotEscapeHome(t *testing.T) {
	e := newEnv(t)
	e.addUser("alice", "pw-alice", "/alice", auth.RoleUser, auth.PermRead, auth.PermDownload)
	session := e.login("alice", "pw-alice")

	rec := e.do(http.MethodGet, "/api/files/download?path=/../secret.txt", "", session)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "admin only") {
		t.Fatal("leaked data outside home")
	}
}

func TestDownloadDirectoryRejected(t *testing.T) {
	e := newEnv(t)
	e.addUser("alice", "pw-alice", "/alice", auth.RoleUser, auth.PermRead, auth.PermDownload)
	session := e.login("alice", "pw-alice")

	rec := e.do(http.MethodGet, "/api/files/download?path=/docs", "", session)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestAPIRequiresLogin(t *testing.T) {
	e := newEnv(t)
	for _, path := range []string{"/api/files", "/api/files/stat?path=/x", "/api/files/download?path=/x"} {
		rec := e.do(http.MethodGet, path, "", "")
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s: status %d, want 401", path, rec.Code)
		}
	}
}

func TestModeString(t *testing.T) {
	cases := map[os.FileMode]string{
		0o644: "644",
		0o755: "755",
		0o600: "600",
		0o777: "777",
		0:     "000",
	}
	for in, want := range cases {
		if got := modeString(in); got != want {
			t.Errorf("modeString(%o) = %q, want %q", in, got, want)
		}
	}
}

func TestContentDispositionEscapes(t *testing.T) {
	cd := contentDisposition("attachment", `evil"name.txt`)
	if strings.Count(cd, `"`) != 2 {
		t.Errorf("unbalanced quotes: %q", cd)
	}
	if !strings.Contains(cd, "evil_name.txt") {
		t.Errorf("quotes not sanitised: %q", cd)
	}

	cd = contentDisposition("attachment", "résumé.pdf")
	if !strings.Contains(cd, "filename*=UTF-8''") {
		t.Errorf("missing RFC 5987 filename: %q", cd)
	}
}

func TestParentOf(t *testing.T) {
	if p := parentOf("/"); p != nil {
		t.Errorf("parent of / = %v", *p)
	}
	if p := parentOf("/docs"); p == nil || *p != "/" {
		t.Errorf("parent of /docs = %v", p)
	}
	if p := parentOf("/a/b"); p == nil || *p != "/a" {
		t.Errorf("parent of /a/b = %v", p)
	}
}

func TestLogoutDisabledByAdmin(t *testing.T) {
	e := newEnv(t)
	if err := e.srv.Config.Update(func(c *config.Config) error {
		c.Security.AllowLogout = false
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	e.addUser("alice", "pw-alice", "/alice", auth.RoleUser, auth.PermRead)
	session := e.login("alice", "pw-alice")

	rec := e.do(http.MethodPost, "/api/auth/logout", "", session)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status %d, want 403", rec.Code)
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == security.CookieName && c.MaxAge < 0 {
			t.Error("logout cleared the session although it is disabled")
		}
	}
	// The session must still work: refusing to log out must not log the user
	// out by accident.
	if r := e.do(http.MethodGet, "/api/auth/me", "", session); r.Code != http.StatusOK {
		t.Errorf("session ended although logout is disabled: status %d", r.Code)
	}

	// The flag drives the hidden button.
	caps := e.do(http.MethodGet, "/api/capabilities", "", "")
	var payload struct {
		Data Capabilities `json:"data"`
	}
	if err := json.Unmarshal(caps.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Data.AuthLogout {
		t.Error("auth_logout true while logout is disabled")
	}
}

func TestLogoutAllowedByDefault(t *testing.T) {
	// A config.json written before the key existed must keep the button.
	if !config.Default().Security.AllowLogout {
		t.Fatal("default is disabled: upgrading would remove the Log out button")
	}
	e := newEnv(t)
	caps := e.do(http.MethodGet, "/api/capabilities", "", "")
	var payload struct {
		Data Capabilities `json:"data"`
	}
	if err := json.Unmarshal(caps.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if !payload.Data.AuthLogout {
		t.Error("auth_logout false on a default configuration")
	}
}
