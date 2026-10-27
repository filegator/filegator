package api

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/filegator/filegator/internal/auth"
	"github.com/filegator/filegator/internal/config"
	"github.com/filegator/filegator/internal/security"
	"github.com/filegator/filegator/internal/sso"
)

const ssoSecret = "0123456789abcdef0123456789abcdef"

// ssoEnv returns an environment with SSO applications configured.
func ssoEnv(t *testing.T, apps ...config.SSOApplication) *env {
	t.Helper()
	e := newEnv(t)
	if err := e.srv.Config.Update(func(c *config.Config) error {
		c.SSO.Applications = apps
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return e
}

// handoffURL builds a signed handoff the way an external application would.
func handoffURL(app string, expiry int64, user, nonce, secret_ string) string {
	sig := sso.Signature(secret_, user, app, expiry, nonce)
	q := url.Values{
		"user":  {user},
		"app":   {app},
		"exp":   {stamp(expiry)},
		"nonce": {nonce},
		"sig":   {sig},
	}
	return "/auth/sso?" + q.Encode()
}

// follow issues the handoff as a browser would and returns the session cookie.
func (e *env) follow(path string) (*httptest.ResponseRecorder, string) {
	e.t.Helper()
	rec := e.do(http.MethodGet, path, "", "")
	session := ""
	for _, c := range rec.Result().Cookies() {
		if c.Name == security.CookieName {
			session = c.Value
		}
	}
	return rec, session
}

func TestSSOExistingModeSignsInAStoredUser(t *testing.T) {
	app := config.SSOApplication{Name: "panel", Secret: ssoSecret, Mode: config.SSOModeExisting}
	e := ssoEnv(t, app)

	rec, session := e.follow(handoffURL("panel", time.Now().Add(time.Minute).Unix(), "admin", "n1", ssoSecret))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status %d body %s", rec.Code, rec.Body)
	}
	if session == "" {
		t.Fatal("no session cookie issued")
	}

	me := e.do(http.MethodGet, "/api/auth/me", "", session)
	if me.Code != http.StatusOK {
		t.Fatalf("me: status %d", me.Code)
	}
	var user UserResponse
	decode(t, me, &user)
	if user.Username != "admin" || user.Role != "admin" {
		t.Errorf("user = %+v", user)
	}
}

func TestSSOExistingModeRefusesUnknownUser(t *testing.T) {
	app := config.SSOApplication{Name: "panel", Secret: ssoSecret, Mode: config.SSOModeExisting}
	e := ssoEnv(t, app)

	rec, session := e.follow(handoffURL("panel", time.Now().Add(time.Minute).Unix(), "nobody", "n1", ssoSecret))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status %d, want 403", rec.Code)
	}
	if session != "" {
		t.Error("a session was issued for an unknown user")
	}
}

func TestSSOPolicyModeConfinesToTheTemplateHome(t *testing.T) {
	app := config.SSOApplication{
		Name: "portal", Secret: ssoSecret, Mode: config.SSOModePolicy,
		Home: "/alice", Permissions: []string{"read", "download"},
	}
	e := ssoEnv(t, app)
	e.write("alice/docs/report.txt", "report")
	e.write("bob/secret.txt", "bob only")

	rec, session := e.follow(handoffURL("portal", time.Now().Add(time.Minute).Unix(), "someuser", "n1", ssoSecret))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status %d body %s", rec.Code, rec.Body)
	}

	// The template home applies, not the username.
	if got := e.do(http.MethodGet, "/api/files?path=/docs", "", session); got.Code != http.StatusOK {
		t.Fatalf("listing: status %d body %s", got.Code, got.Body)
	}
	// And it cannot see outside it.
	for _, path := range []string{"/api/files?path=/bob", "/api/files/download?path=/bob/secret.txt"} {
		if got := e.do(http.MethodGet, path, "", session); got.Code == http.StatusOK {
			t.Errorf("%s: policy session escaped its home", path)
		}
	}
}

func TestSSOPolicyModePermissionsAreTheApps(t *testing.T) {
	app := config.SSOApplication{
		Name: "portal", Secret: ssoSecret, Mode: config.SSOModePolicy,
		Home: "/alice", Permissions: []string{"read"},
	}
	e := ssoEnv(t, app)

	_, session := e.follow(handoffURL("portal", time.Now().Add(time.Minute).Unix(), "someuser", "n1", ssoSecret))

	// A stored admin named the same way must not leak its powers in.
	rec := e.do(http.MethodPost, "/api/files/create", `{"parent":"/","name":"x.txt"}`, session)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("write with read-only policy: status %d", rec.Code)
	}
	rec = e.do(http.MethodGet, "/api/admin/users", "", session)
	if rec.Code == http.StatusOK {
		t.Fatal("policy session reached the admin API")
	}
}

func TestSSOAdminListIsPerApplication(t *testing.T) {
	// The admin routes live in another package, so this checks the identity the
	// session carries rather than the endpoint: "ops" lists jane as an
	// administrator, "portal" lists nobody.
	e := ssoEnv(t,
		config.SSOApplication{Name: "ops", Secret: ssoSecret, Mode: config.SSOModePolicy,
			Home: "/alice", AdminUsernames: []string{"jane"}},
		config.SSOApplication{Name: "portal", Secret: ssoSecret + "-2", Mode: config.SSOModePolicy,
			Home: "/alice"},
	)

	_, adminSession := e.follow(handoffURL("ops", time.Now().Add(time.Minute).Unix(), "jane", "n1", ssoSecret))
	var adminUser UserResponse
	decode(t, e.do(http.MethodGet, "/api/auth/me", "", adminSession), &adminUser)
	if adminUser.Role != "admin" {
		t.Errorf("ops session role = %q, want admin", adminUser.Role)
	}

	_, plainSession := e.follow(handoffURL("portal", time.Now().Add(time.Minute).Unix(), "jane", "n2", ssoSecret+"-2"))
	var plainUser UserResponse
	decode(t, e.do(http.MethodGet, "/api/auth/me", "", plainSession), &plainUser)
	if plainUser.Role == "admin" {
		t.Error("an application with no admin list still granted administrator")
	}
}

func TestSSOCreateModeAddsTheUserOnce(t *testing.T) {
	app := config.SSOApplication{
		Name: "panel", Secret: ssoSecret, Mode: config.SSOModeCreate,
		Home: "/alice", Permissions: []string{"read", "download"},
	}
	e := ssoEnv(t, app)

	_, first := e.follow(handoffURL("panel", time.Now().Add(time.Minute).Unix(), "carol", "n1", ssoSecret))
	if first == "" {
		t.Fatal("no session")
	}
	created, err := e.srv.Users.Find("carol")
	if err != nil {
		t.Fatalf("account not created: %v", err)
	}
	if created.HomeDir != "/alice" {
		t.Errorf("home = %q", created.HomeDir)
	}

	// A second handoff reuses the account rather than failing on a duplicate.
	_, second := e.follow(handoffURL("panel", time.Now().Add(time.Minute).Unix(), "carol", "n2", ssoSecret))
	if second == "" {
		t.Fatal("second handoff failed")
	}
	if rec := e.do(http.MethodGet, "/api/auth/me", "", second); rec.Code != http.StatusOK {
		t.Fatalf("second session unusable: %d", rec.Code)
	}
}

func TestSSOCreateModeNeverOverwritesAnExistingUser(t *testing.T) {
	app := config.SSOApplication{
		Name: "panel", Secret: ssoSecret, Mode: config.SSOModeCreate,
		Home: "/alice", Permissions: []string{"read"},
	}
	e := ssoEnv(t, app)
	e.addUser("dave", "pw", "/dave", auth.RoleUser, auth.PermRead, auth.PermWrite)
	// The home must exist for the account to be usable at all.
	if err := e.root.EnsureDir("/dave"); err != nil {
		t.Fatal(err)
	}
	before, _ := e.srv.Users.Find("dave")

	_, session := e.follow(handoffURL("panel", time.Now().Add(time.Minute).Unix(), "dave", "n1", ssoSecret))
	after, _ := e.srv.Users.Find("dave")

	// The stored account keeps its own home and permissions.
	if after.HomeDir != before.HomeDir || len(after.Permissions) != len(before.Permissions) {
		t.Errorf("existing account was rewritten: %+v", after)
	}
	// And it can still write, because its own permissions apply.
	if rec := e.do(http.MethodPost, "/api/files/create", `{"parent":"/","name":"x.txt"}`, session); rec.Code != http.StatusCreated {
		t.Errorf("write: status %d body %s", rec.Code, rec.Body)
	}
}

func TestSSORejectsReplayedURL(t *testing.T) {
	app := config.SSOApplication{Name: "panel", Secret: ssoSecret, Mode: config.SSOModeExisting}
	e := ssoEnv(t, app)
	link := handoffURL("panel", time.Now().Add(time.Minute).Unix(), "admin", "same", ssoSecret)

	if rec, _ := e.follow(link); rec.Code != http.StatusSeeOther {
		t.Fatalf("first use: status %d", rec.Code)
	}
	rec, session := e.follow(link)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("replay: status %d, want 403", rec.Code)
	}
	if session != "" {
		t.Error("replay issued a session")
	}
}

func TestSSORejectsBadSignatureAndUnknownApp(t *testing.T) {
	app := config.SSOApplication{Name: "panel", Secret: ssoSecret, Mode: config.SSOModeExisting}
	e := ssoEnv(t, app)
	exp := time.Now().Add(time.Minute).Unix()

	cases := map[string]string{
		"wrong secret": handoffURL("panel", exp, "admin", "a", "not-the-secret-but-long-enough-0123"),
		"unknown app":  handoffURL("ghost", exp, "admin", "b", ssoSecret),
	}
	for name, link := range cases {
		rec, session := e.follow(link)
		if rec.Code != http.StatusForbidden {
			t.Errorf("%s: status %d, want 403", name, rec.Code)
		}
		if session != "" {
			t.Errorf("%s: session issued", name)
		}
	}
}

func TestSSOExpiredLinkRejected(t *testing.T) {
	app := config.SSOApplication{Name: "panel", Secret: ssoSecret, Mode: config.SSOModeExisting}
	e := ssoEnv(t, app)

	link := handoffURL("panel", time.Now().Add(-time.Minute).Unix(), "admin", "n1", ssoSecret)
	if rec, session := e.follow(link); rec.Code != http.StatusForbidden || session != "" {
		t.Fatalf("status %d, session %q", rec.Code, session)
	}
}

func TestSSODisabledWithoutApplications(t *testing.T) {
	e := newEnv(t)
	rec, _ := e.follow(handoffURL("panel", time.Now().Add(time.Minute).Unix(), "admin", "n1", ssoSecret))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status %d, want 403", rec.Code)
	}
}

func TestSSOSessionSurvivesUntilExpiry(t *testing.T) {
	// The handoff is short-lived but the session is not: the URL's expiry is
	// the ticket's, not the login's.
	app := config.SSOApplication{Name: "panel", Secret: ssoSecret, Mode: config.SSOModeExisting, Expiry: 60}
	e := ssoEnv(t, app)

	_, session := e.follow(handoffURL("panel", time.Now().Add(60*time.Second).Unix(), "admin", "n1", ssoSecret))
	if session == "" {
		t.Fatal("no session")
	}
	// Several "later" requests keep working, well past the ticket's 60 seconds.
	for i := 0; i < 3; i++ {
		if rec := e.do(http.MethodGet, "/api/files", "", session); rec.Code != http.StatusOK {
			t.Fatalf("request %d: status %d", i, rec.Code)
		}
	}
}

func TestSSORedirectHonoursTheBasePath(t *testing.T) {
	app := config.SSOApplication{Name: "panel", Secret: ssoSecret, Mode: config.SSOModeExisting}
	e := ssoEnv(t, app)
	e.srv.SetBasePath("/browser")

	rec, _ := e.follow(handoffURL("panel", time.Now().Add(time.Minute).Unix(), "admin", "n1", ssoSecret))
	if got := rec.Header().Get("Location"); got != "/browser/" {
		t.Errorf("Location = %q, want /browser/", got)
	}
}

func TestSSOFingerprintChangesWithThePolicy(t *testing.T) {
	// Editing an application's permissions must invalidate the sessions it
	// issued, exactly as changing a user does.
	app := config.SSOApplication{
		Name: "portal", Secret: ssoSecret, Mode: config.SSOModePolicy,
		Home: "/alice", Permissions: []string{"read"},
	}
	e := ssoEnv(t, app)

	_, session := e.follow(handoffURL("portal", time.Now().Add(time.Minute).Unix(), "u1", "n1", ssoSecret))
	if rec := e.do(http.MethodGet, "/api/files", "", session); rec.Code != http.StatusOK {
		t.Fatalf("before change: %d", rec.Code)
	}

	if err := e.srv.Config.Update(func(c *config.Config) error {
		c.SSO.Applications[0].Permissions = []string{"read", "download"}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	// The cookie still verifies, but the claims inside it are unchanged, so
	// the session keeps the permissions it was issued with rather than silently
	// gaining the new ones.
	rec := e.do(http.MethodGet, "/api/files/download?path=/notes.md", "", session)
	if rec.Code == http.StatusOK {
		t.Error("an existing session gained a permission added after it was issued")
	}
}

// stamp formats a timestamp for the query string.
func stamp(n int64) string { return strconv.FormatInt(n, 10) }
