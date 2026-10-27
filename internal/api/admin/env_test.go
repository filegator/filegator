package admin

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/filegator/filegator/internal/api"
	"github.com/filegator/filegator/internal/auth"
	"github.com/filegator/filegator/internal/auth/jsonfile"
	"github.com/filegator/filegator/internal/config"
	"github.com/filegator/filegator/internal/router"
	"github.com/filegator/filegator/internal/security"
	"github.com/filegator/filegator/internal/sso"
	"github.com/filegator/filegator/internal/storage/local"
)

// testEnv wires the admin API on top of a real api.Server, mirroring how the
// application assembles them.
type testEnv struct {
	t      *testing.T
	srv    *api.Server
	handle *Handler
	router *router.Router
	root   *local.Root
	// session and csrf hold the admin credentials created by adminEnv.
	session string
	csrf    string
}

// newTestEnv builds an environment the way a fresh install looks: the storage
// root and the users file do not exist yet and are created on the way.
func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	dir := t.TempDir()

	store, err := config.Load(filepath.Join(dir, "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Update(func(c *config.Config) error {
		c.Auth.UsersFile = filepath.Join(dir, "users.json")
		c.Storage.Root = filepath.Join(dir, "repository")
		c.Security.CSRFProtection = true
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	// Update above writes config.json, because a store change is persisted.
	// The file is removed again so a test sees the folder as a first start
	// leaves it: a fresh install on disk, not one that depends on the fixture.
	// (New() writing the file on a first start is covered in the app package.)
	if err := os.Remove(store.Path()); err != nil {
		t.Fatal(err)
	}
	// This environment wires the API by hand instead of calling app.New, so the
	// file New writes on a first start is not created here. What matters for
	// the tests below is that every handler agrees on one session key, and
	// makeConfigReadOnly asserts that it can see it.
	if err := store.Update(func(c *config.Config) error {
		c.Security.SessionKey = strings.Repeat("ab", 32)
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

	srv := &api.Server{
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

	handle := New(srv)
	handle.Routes(rt)

	e := &testEnv{t: t, srv: srv, handle: handle, router: rt, root: root}
	// The shipped users.json enables guest access. These tests are about
	// administrator behaviour, so the baseline switches it off; otherwise an
	// anonymous request resolves to the guest and is refused with 403 rather
	// than 401.
	if guest, err := srv.Users.Find("guest"); err == nil {
		guest.Permissions = nil
		if _, err := srv.Users.Update("guest", guest, ""); err != nil {
			t.Fatalf("set guest permissions: %v", err)
		}
	}
	e.write("alice/docs/report.txt", "alice report")
	e.write("alice/notes.md", "notes")
	e.write("bob/secret.txt", "bob secret")
	e.write("top.txt", "top level")
	e.write("secret.txt", "admin only")
	return e
}

// write creates a file below the repository root.
func (e *testEnv) write(rel, content string) {
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
func (e *testEnv) addUser(username, password, home string, role auth.Role, perms ...auth.Permission) {
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

// login returns a session cookie value.
func (e *testEnv) login(username, password string) string {
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

// do issues a GET or unauthenticated POST.
func (e *testEnv) do(method, path, body, session string) *httptest.ResponseRecorder {
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

// adminSession logs in as an administrator with write access and returns the
// session plus a valid CSRF token.
func (e *testEnv) adminSession() (string, string) {
	e.t.Helper()
	e.addUser("root", "pw-root", "/", auth.RoleAdmin, auth.PermRead, auth.PermWrite)
	session := e.login("root", "pw-root")
	token, _, err := e.srv.CSRF.Token(httptest.NewRequest(http.MethodGet, "/", nil))
	if err != nil {
		e.t.Fatal(err)
	}
	return session, token
}

// post issues an authenticated state-changing request.
func (e *testEnv) post(path, body, session, token string) *httptest.ResponseRecorder {
	e.t.Helper()
	r := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	r.RemoteAddr = "127.0.0.1:12345"
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set(security.CSRFHeader, token)
	r.AddCookie(&http.Cookie{Name: security.CookieName, Value: session})
	r.AddCookie(&http.Cookie{Name: security.CSRFCookieName, Value: token})
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

// adminTestEnv is the helper receiver for the body-building functions the
// ported tests use to construct complete request payloads.
type adminTestEnv = testEnv

// adminEnv is the shared fixture: a fully wired environment with an admin
// session selected.
func adminEnv(t *testing.T) *testEnv {
	t.Helper()
	e := newTestEnv(t)
	session, token := e.adminSession()
	e.session = session
	e.csrf = token
	return e
}

// newEnv is kept for tests ported from the api package: it returns a plain
// environment without an admin session.
func newEnv(t *testing.T) *testEnv { return newTestEnv(t) }

// makeConfigReadOnly replaces the environment's config store with one that
// cannot be written, the way a read-only volume looks to the server.
//
// The store is reloaded from a directory the process cannot write to, so the
// real detection path is exercised rather than a flag. Running as root skips
// permissions, so those runs skip the test instead of asserting the wrong
// thing.
func (e *testEnv) makeConfigReadOnly() {
	e.t.Helper()
	if os.Geteuid() == 0 {
		e.t.Skip("permission checks do not apply to root")
	}
	source := filepath.Dir(e.srv.Config.Path())
	data, err := os.ReadFile(e.srv.Config.Path())
	if err != nil {
		e.t.Fatal(err)
	}
	dir := e.t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		e.t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o500); err != nil {
		e.t.Fatal(err)
	}
	e.t.Cleanup(func() { os.Chmod(dir, 0o700) })

	store, err := config.Load(path)
	if err != nil {
		e.t.Fatal(err)
	}
	if !store.ReadOnly() {
		e.t.Fatalf("%s is writable; the read-only case cannot be tested", source)
	}
	// The read-only store reads the same file, so it sees the same session
	// key; without one the handlers disagree about which key is in use.
	key := e.srv.Config.Get().Security.SessionKey
	if err := store.Update(func(c *config.Config) error {
		c.Security.SessionKey = key
		return nil
	}); !errors.Is(err, config.ErrReadOnly) {
		e.t.Fatalf("read-only store accepted a write: %v", err)
	}
	if got := store.Get().Security.SessionKey; got != key {
		e.t.Fatalf("read-only store session key = %q, want %q", got, key)
	}
	e.srv.Config = store
	e.handle = New(e.srv)
}

// req issues a request as the current admin session.
func (e *testEnv) req(method, path, body string) *httptest.ResponseRecorder {
	return e.do(method, path, body, e.session)
}

// submit issues a state-changing request with the current admin token.
func (e *testEnv) submit(path, body string) *httptest.ResponseRecorder {
	return e.post(path, body, e.session, e.csrf)
}
