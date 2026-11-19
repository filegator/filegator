package api

import (
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/filegator/filegator/internal/config"
	"github.com/filegator/filegator/internal/sso"
)

const resplitNonce = "cf9683ba7d1e4a0b9c2f5e8a1d3b6c40"

// resplitURL takes a handoff an application signed for login and returns the
// same signature with the "|" boundaries moved: user and exp are read out of
// the login, and the rest of the signed bytes travel in the nonce.
func resplitURL(app string, issued int64, login, secret_ string, user, exp string) string {
	q := url.Values{
		"user":  {user},
		"app":   {app},
		"exp":   {exp},
		"nonce": {app + "|" + stamp(issued) + "|" + resplitNonce},
		"sig":   {sso.Signature(secret_, login, app, issued, resplitNonce)},
	}
	return "/auth/sso?" + q.Encode()
}

func TestSSORefusesAHandoffReadWithDifferentFieldBoundaries(t *testing.T) {
	app := config.SSOApplication{Name: "panel", Secret: ssoSecret, Mode: config.SSOModeExisting}
	e := ssoEnv(t, app)
	issued := time.Now().Add(time.Minute).Unix()
	const login = "admin|panel|9999999999"

	// Followed the way it was signed, the handoff is for a name with no
	// account here, so it opens nothing.
	rec, session := e.follow(handoffURL("panel", issued, login, resplitNonce, ssoSecret))
	if rec.Code != http.StatusForbidden || session != "" {
		t.Fatalf("as signed: status %d, session %q", rec.Code, session)
	}

	rec, session = e.follow(resplitURL("panel", issued, login, ssoSecret, "admin", "9999999999"))
	if session != "" {
		var user UserResponse
		decode(t, e.do(http.MethodGet, "/api/auth/me", "", session), &user)
		t.Fatalf("status %d: a handoff signed for %q opened a session as %q with role %q",
			rec.Code, login, user.Username, user.Role)
	}
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status %d, want 403", rec.Code)
	}
}

func TestSSOPolicyModeRefusesAHandoffReadWithDifferentFieldBoundaries(t *testing.T) {
	app := config.SSOApplication{
		Name: "portal", Secret: ssoSecret, Mode: config.SSOModePolicy,
		Home: "/clients/%s", Permissions: []string{"read", "download"},
	}
	e := ssoEnv(t, app)
	const login = "carol|portal|9999999999"
	e.write("clients/carol/contract.txt", "carol only")
	issued := time.Now().Add(time.Minute).Unix()

	// Followed the way it was signed, the visitor is not handed carol's
	// folder. Their own folder is not created here: its name holds a "|",
	// which some filesystems refuse.
	rec, own := e.follow(handoffURL("portal", issued, login, resplitNonce, ssoSecret))
	if rec.Code != http.StatusSeeOther || own == "" {
		t.Fatalf("as signed: status %d, session %q", rec.Code, own)
	}
	var ownUser UserResponse
	decode(t, e.do(http.MethodGet, "/api/auth/me", "", own), &ownUser)
	if ownUser.HomeDir == "/clients/carol" {
		t.Fatalf("as signed: the visitor's own session already has carol's home")
	}
	if got := e.do(http.MethodGet, "/api/files/download?path=/contract.txt", "", own); got.Code == http.StatusOK {
		t.Fatalf("as signed: the visitor's own session already reads carol's file")
	}

	rec, session := e.follow(resplitURL("portal", issued, login, ssoSecret, "carol", "9999999999"))
	if session != "" {
		got := e.do(http.MethodGet, "/api/files/download?path=/contract.txt", "", session)
		t.Fatalf("status %d: a handoff signed for %q opened a session in carol's folder; download: status %d, body %q",
			rec.Code, login, got.Code, got.Body.String())
	}
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status %d, want 403", rec.Code)
	}
}

func TestSSOPolicyModeAdminListRefusesAHandoffReadWithDifferentFieldBoundaries(t *testing.T) {
	app := config.SSOApplication{
		Name: "ops", Secret: ssoSecret, Mode: config.SSOModePolicy,
		Home: "/clients/%s", Permissions: []string{"read"}, AdminUsernames: []string{"jane"},
	}
	e := ssoEnv(t, app)
	const login = "jane|ops|9999999999"
	issued := time.Now().Add(time.Minute).Unix()

	// Followed the way it was signed, the session is not an administrator's.
	_, own := e.follow(handoffURL("ops", issued, login, resplitNonce, ssoSecret))
	var ownUser UserResponse
	decode(t, e.do(http.MethodGet, "/api/auth/me", "", own), &ownUser)
	if ownUser.Role == "admin" {
		t.Fatalf("as signed: the visitor's own session is already an administrator's")
	}

	rec, session := e.follow(resplitURL("ops", issued, login, ssoSecret, "jane", "9999999999"))
	if session != "" {
		var user UserResponse
		decode(t, e.do(http.MethodGet, "/api/auth/me", "", session), &user)
		t.Fatalf("status %d: a handoff signed for %q opened a session as %q with role %q and home %q",
			rec.Code, login, user.Username, user.Role, user.HomeDir)
	}
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status %d, want 403", rec.Code)
	}
}

func TestSSOCreateModeRefusesAHandoffReadWithDifferentFieldBoundaries(t *testing.T) {
	app := config.SSOApplication{
		Name: "panel", Secret: ssoSecret, Mode: config.SSOModeCreate,
		Home: "/clients/%s", Permissions: []string{"read"},
	}
	e := ssoEnv(t, app)
	const login = "admin|panel|9999999999"
	issued := time.Now().Add(time.Minute).Unix()

	// The login is not followed the way it was signed here: that would create
	// the account, which is not what this test is about.
	rec, session := e.follow(resplitURL("panel", issued, login, ssoSecret, "admin", "9999999999"))
	if session != "" {
		var user UserResponse
		decode(t, e.do(http.MethodGet, "/api/auth/me", "", session), &user)
		t.Fatalf("status %d: a handoff signed for %q opened a session as %q with role %q",
			rec.Code, login, user.Username, user.Role)
	}
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status %d, want 403", rec.Code)
	}
}

// In create mode a name with no account gets one, so a handoff read with the
// boundaries moved must not be what creates it.
func TestSSOCreateModeAddsNoAccountForAHandoffReadWithDifferentFieldBoundaries(t *testing.T) {
	app := config.SSOApplication{
		Name: "panel", Secret: ssoSecret, Mode: config.SSOModeCreate,
		Home: "/clients/%s", Permissions: []string{"read"},
	}
	e := ssoEnv(t, app)
	const login = "victor|panel|9999999999"
	issued := time.Now().Add(time.Minute).Unix()

	rec, session := e.follow(resplitURL("panel", issued, login, ssoSecret, "victor", "9999999999"))
	if created, err := e.srv.Users.Find("victor"); err == nil {
		t.Fatalf("status %d: a handoff signed for %q created the account %q with home %q",
			rec.Code, login, created.Username, created.HomeDir)
	}
	if rec.Code != http.StatusForbidden || session != "" {
		t.Fatalf("status %d, session %q, want 403 and no session", rec.Code, session)
	}
}
