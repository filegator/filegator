package ldap

import (
	"testing"
	"time"

	ldapv3 "github.com/go-ldap/ldap/v3"

	"github.com/filegator/filegator/internal/auth"
	"github.com/filegator/filegator/internal/config"
)

// testConfig returns a valid configuration that never connects anywhere.
func testConfig() config.LDAP {
	return config.LDAP{
		Server:             "ldap://127.0.0.1:1",
		BindDN:             "cn=admin,dc=example,dc=com",
		BindPass:           "pw",
		BaseDN:             "dc=example,dc=com",
		Filter:             "(uid=*)",
		UsernameAttribute:  "uid",
		NameAttribute:      "cn",
		DefaultPermissions: []string{"read", "write"},
		AdminUsernames:     []string{"boss"},
		HomeTemplate:       "/users/%s",
		AddDomain:          "",
		RemoveDomains:      nil,
		UserCacheTTL:       30,
		Timeout:            1,
	}
}

// newProvider builds a provider without the startup connection check.
func newProvider(t *testing.T, cfg config.LDAP) *Provider {
	t.Helper()
	ttl := time.Duration(cfg.UserCacheTTL) * time.Second
	if cfg.UserCacheTTL == 0 {
		ttl = DefaultTTL
	}
	return &Provider{
		cfg:     cfg,
		ttl:     ttl,
		timeout: time.Second,
		cache:   make(map[string]cacheEntry),
	}
}

func TestNewRejectsMissingServer(t *testing.T) {
	cfg := testConfig()
	cfg.Server = ""
	if _, err := New(cfg); err == nil {
		t.Fatal("expected an error without a server")
	}
}

func TestNewRejectsMissingBaseDN(t *testing.T) {
	cfg := testConfig()
	cfg.BaseDN = ""
	if _, err := New(cfg); err == nil {
		t.Fatal("expected an error without a base DN")
	}
}

func TestNewRejectsMissingUsernameAttribute(t *testing.T) {
	cfg := testConfig()
	cfg.UsernameAttribute = ""
	if _, err := New(cfg); err == nil {
		t.Fatal("expected an error without a username attribute")
	}
}

// TestAuthenticateRejectsEmptyPassword guards the anonymous-bind hole: most
// directories accept an empty password as an anonymous bind.
func TestAuthenticateRejectsEmptyPassword(t *testing.T) {
	p := newProvider(t, testConfig())
	if _, err := p.Authenticate("alice", ""); err != auth.ErrInvalidPassword {
		t.Fatalf("got %v, want ErrInvalidPassword", err)
	}
	if _, err := p.Authenticate("", "secret"); err != auth.ErrInvalidPassword {
		t.Fatalf("got %v, want ErrInvalidPassword", err)
	}
}

func TestStripDomains(t *testing.T) {
	cfg := testConfig()
	cfg.RemoveDomains = []string{"@dept1.example.com", "@dept2.example.com"}
	p := newProvider(t, cfg)

	cases := map[string]string{
		"alice":                   "alice",
		"alice@dept1.example.com": "alice",
		"alice@dept2.example.com": "alice",
		"alice@other.example.com": "alice@other.example.com",
	}
	for in, want := range cases {
		if got := p.stripDomains(in); got != want {
			t.Errorf("stripDomains(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestAddDomain(t *testing.T) {
	cfg := testConfig()
	cfg.AddDomain = "@example.com"
	p := newProvider(t, cfg)

	if got := p.addDomain("alice"); got != "alice@example.com" {
		t.Errorf("got %q", got)
	}
	// Already present: not duplicated.
	if got := p.addDomain("alice@example.com"); got != "alice@example.com" {
		t.Errorf("got %q", got)
	}
}

func TestHomeForTemplate(t *testing.T) {
	cfg := testConfig()
	cfg.HomeTemplate = "/users/%s"
	p := newProvider(t, cfg)
	if got := p.homeFor("alice"); got != "/users/alice" {
		t.Errorf("got %q", got)
	}

	// Empty template means everyone shares the root.
	cfg.HomeTemplate = ""
	p = newProvider(t, cfg)
	if got := p.homeFor("alice"); got != "/" {
		t.Errorf("got %q", got)
	}

	// A template without a leading slash is normalised.
	cfg.HomeTemplate = "home/%s"
	p = newProvider(t, cfg)
	if got := p.homeFor("alice"); got != "/home/alice" {
		t.Errorf("got %q", got)
	}
}

func TestMapEntryDefaults(t *testing.T) {
	cfg := testConfig()
	cfg.HomeTemplate = "/users/%s"
	p := newProvider(t, cfg)

	u, ok := p.mapEntry(entry("alice", "Alice Smith", "cn=alice,dc=example,dc=com"))
	if !ok {
		t.Fatal("entry rejected")
	}
	if u.user.Username != "alice" || u.user.Name != "Alice Smith" {
		t.Errorf("user = %+v", u.user)
	}
	if u.user.Role != auth.RoleUser {
		t.Errorf("role = %q", u.user.Role)
	}
	if u.user.HomeDir != "/users/alice" {
		t.Errorf("home = %q", u.user.HomeDir)
	}
	if len(u.user.Permissions) != 2 {
		t.Errorf("permissions = %v", u.user.Permissions)
	}
	if u.dn != "cn=alice,dc=example,dc=com" {
		t.Errorf("dn = %q", u.dn)
	}
}

func TestMapEntryAdmin(t *testing.T) {
	cfg := testConfig()
	cfg.AdminUsernames = []string{"boss"}
	cfg.HomeTemplate = "/users/%s"
	p := newProvider(t, cfg)

	u, ok := p.mapEntry(entry("boss", "The Boss", "cn=boss,dc=example,dc=com"))
	if !ok {
		t.Fatal("entry rejected")
	}
	if u.user.Role != auth.RoleAdmin {
		t.Errorf("role = %q, want admin", u.user.Role)
	}
	// Admins see everything, mirroring v7.
	if u.user.HomeDir != "/" {
		t.Errorf("home = %q, want /", u.user.HomeDir)
	}
	if len(u.user.Permissions) != len(auth.AllPermissions) {
		t.Errorf("admin permissions = %v", u.user.Permissions)
	}
}

func TestMapEntryAdminIsCaseInsensitive(t *testing.T) {
	cfg := testConfig()
	cfg.AdminUsernames = []string{"BOSS"}
	p := newProvider(t, cfg)

	u, ok := p.mapEntry(entry("boss", "Boss", "cn=boss,dc=x"))
	if !ok {
		t.Fatal("entry rejected")
	}
	if u.user.Role != auth.RoleAdmin {
		t.Errorf("role = %q", u.user.Role)
	}
}

func TestMapEntryMissingUsername(t *testing.T) {
	p := newProvider(t, testConfig())
	if _, ok := p.mapEntry(entry("", "Nobody", "cn=x")); ok {
		t.Error("entry without a username should be skipped")
	}
}

func TestMapEntryInvalidPermissions(t *testing.T) {
	cfg := testConfig()
	cfg.DefaultPermissions = []string{"teleport"}
	p := newProvider(t, cfg)
	if _, ok := p.mapEntry(entry("alice", "Alice", "cn=alice")); ok {
		t.Error("invalid permissions should skip the entry")
	}
}

func TestMapEntryNameFallback(t *testing.T) {
	p := newProvider(t, testConfig())
	u, ok := p.mapEntry(entry("alice", "", "cn=alice"))
	if !ok {
		t.Fatal("entry rejected")
	}
	if u.user.Name != "alice" {
		t.Errorf("name = %q, want the username as fallback", u.user.Name)
	}
}

func TestSessionFingerprintChangesWithRole(t *testing.T) {
	p := newProvider(t, testConfig())
	base := &auth.User{Username: "alice", Role: auth.RoleUser, HomeDir: "/users/alice", Permissions: []auth.Permission{auth.PermRead}}
	promoted := &auth.User{Username: "alice", Role: auth.RoleAdmin, HomeDir: "/users/alice", Permissions: []auth.Permission{auth.PermRead}}

	if p.SessionFingerprint(base) == p.SessionFingerprint(promoted) {
		t.Error("fingerprint must change when the role changes")
	}

	// Same data means the same fingerprint, across calls.
	if p.SessionFingerprint(base) != p.SessionFingerprint(base) {
		t.Error("fingerprint is not stable")
	}
}

func TestCapabilities(t *testing.T) {
	p := newProvider(t, testConfig())
	caps := p.Capabilities()
	if caps.ManageUsers {
		t.Error("LDAP users are managed in the directory")
	}
	if caps.ChangePassword {
		t.Error("LDAP password changes are not supported")
	}
	if p.Writable() {
		t.Error("LDAP provider must not be writable")
	}
	if p.DefaultAdminPassword() {
		t.Error("there is no shipped default password for LDAP")
	}
}

func TestUnsupportedOperations(t *testing.T) {
	p := newProvider(t, testConfig())
	u := &auth.User{Username: "alice"}
	if _, err := p.Add(u, "pw"); err != auth.ErrUnsupported {
		t.Errorf("Add: %v", err)
	}
	if _, err := p.Update("alice", u, "pw"); err != auth.ErrUnsupported {
		t.Errorf("Update: %v", err)
	}
	if err := p.Delete("alice"); err != auth.ErrUnsupported {
		t.Errorf("Delete: %v", err)
	}
}

func TestGuestFallback(t *testing.T) {
	// A provider whose directory is unreachable: Find fails, so the synthetic
	// guest is returned.
	p := newProvider(t, testConfig())
	p.cache["guest"] = cacheEntry{err: errFake, expires: time.Now().Add(time.Minute)}
	g := p.Guest()
	if g == nil || g.Username != "guest" || g.Role != auth.RoleGuest {
		t.Errorf("guest = %+v", g)
	}
	if len(g.Permissions) != 0 {
		t.Errorf("synthetic guest should have no permissions: %v", g.Permissions)
	}
}

func TestCacheStoresAndExpires(t *testing.T) {
	cfg := testConfig()
	cfg.UserCacheTTL = 0 // use the default TTL
	p := newProvider(t, cfg)

	if p.ttl != DefaultTTL {
		t.Errorf("default ttl = %v", p.ttl)
	}

	p.store("alice", []ldapUser{{username: "alice"}}, nil)
	if _, ok := p.cached("alice"); !ok {
		t.Error("entry not cached")
	}

	p.Invalidate()
	if _, ok := p.cached("alice"); ok {
		t.Error("invalidate did not clear the cache")
	}
}

func TestCacheIsCaseInsensitive(t *testing.T) {
	p := newProvider(t, testConfig())
	p.store("alice", []ldapUser{{username: "alice"}}, nil)
	if _, ok := p.cached("ALICE"); !ok {
		t.Error("cache keys should be case-insensitive")
	}
}

var errFake = &fakeError{}

type fakeError struct{}

func (e *fakeError) Error() string { return "fake" }

// entry builds a minimal LDAP entry for mapping tests.
func entry(username, name, dn string) *ldapv3.Entry {
	attrs := []*ldapv3.EntryAttribute{
		{Name: "uid", Values: []string{username}},
	}
	if name != "" {
		attrs = append(attrs, &ldapv3.EntryAttribute{Name: "cn", Values: []string{name}})
	}
	return &ldapv3.Entry{DN: dn, Attributes: attrs}
}
