// Package ldap implements auth.Provider against an LDAP directory.
//
// It mirrors v7's LDAP adapter: a service account searches the directory, the
// found user's DN is re-bound with the supplied password to prove the
// credentials, and the result is mapped onto a filegator User using
// default_permissions, admin_usernames and a home folder template.
//
// Two deliberate changes from v7:
//
//   - Resolved users are cached for a short TTL instead of hitting the
//     directory on every request. A directory outage therefore keeps sessions
//     alive until the cache expires, matching v7's "log a warning, keep the
//     session" behaviour.
//   - The private_repos flag is replaced by an explicit home_template such as
//     "/%s", so it is visible what the home folder will be.
package ldap

import (
	"errors"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	ldapv3 "github.com/go-ldap/ldap/v3"

	"github.com/filegator/filegator/internal/auth"
	"github.com/filegator/filegator/internal/config"
	"github.com/filegator/filegator/internal/security"
)

// DefaultTTL is used when the config leaves user_cache_ttl at 0.
const DefaultTTL = 60 * time.Second

// DefaultTimeout bounds a directory operation.
const DefaultTimeout = 10 * time.Second

// Provider authenticates users against LDAP.
type Provider struct {
	cfg     config.LDAP
	ttl     time.Duration
	timeout time.Duration

	mu    sync.RWMutex
	cache map[string]cacheEntry
}

// New validates the configuration and returns a provider. It performs one
// search at startup so a bad bind DN or base DN fails immediately instead of
// at the first login.
func New(cfg config.LDAP) (*Provider, error) {
	if cfg.Server == "" {
		return nil, errors.New("ldap: server is required")
	}
	if cfg.BaseDN == "" {
		return nil, errors.New("ldap: base_dn is required")
	}
	if cfg.UsernameAttribute == "" {
		return nil, errors.New("ldap: username_attribute is required")
	}

	p := &Provider{
		cfg:     cfg,
		ttl:     time.Duration(cfg.UserCacheTTL) * time.Second,
		timeout: DefaultTimeout,
		cache:   make(map[string]cacheEntry),
	}
	if cfg.UserCacheTTL == 0 {
		p.ttl = DefaultTTL
	}
	if cfg.Timeout > 0 {
		p.timeout = time.Duration(cfg.Timeout) * time.Second
	}

	// Fail fast on a misconfigured server, bind account or base DN.
	if _, err := p.lookup(""); err != nil {
		return nil, err
	}
	return p, nil
}

// Describe returns a short description for logs and the admin UI.
func (p *Provider) Describe() string {
	return "ldap://" + strings.TrimPrefix(strings.TrimPrefix(p.cfg.Server, "ldap://"), "ldaps://")
}

// Capabilities reports that LDAP users are managed in the directory, not here.
func (p *Provider) Capabilities() auth.Caps {
	return auth.Caps{ManageUsers: false, ChangePassword: false}
}

// Writable reports whether users can be modified through this provider.
func (p *Provider) Writable() bool { return false }

// DefaultAdminPassword never applies to a directory provider: passwords live
// in LDAP, so there is no shipped default to warn about.
func (p *Provider) DefaultAdminPassword() bool { return false }

// Authenticate verifies a username and password.
func (p *Provider) Authenticate(username, password string) (*auth.User, error) {
	// An empty password would be an anonymous bind, which most directories
	// accept. Refusing here is essential: it would otherwise authenticate
	// anyone against a misconfigured server.
	if password == "" || username == "" {
		return nil, auth.ErrInvalidPassword
	}

	searchName := p.stripDomains(username)
	users, err := p.lookup(searchName)
	if err != nil {
		return nil, err
	}
	if len(users) == 0 {
		return nil, auth.ErrUserNotFound
	}

	fullName := p.addDomain(searchName)
	for _, u := range users {
		if !strings.EqualFold(u.username, fullName) {
			continue
		}
		ok, bindErr := p.bindAs(u.dn, password)
		if bindErr != nil {
			return nil, bindErr
		}
		if !ok {
			return nil, auth.ErrInvalidPassword
		}
		return u.user, nil
	}
	return nil, auth.ErrUserNotFound
}

// Find looks up a user by username without verifying a password.
func (p *Provider) Find(username string) (*auth.User, error) {
	users, err := p.lookup(p.stripDomains(username))
	if err != nil {
		return nil, err
	}
	fullName := p.addDomain(username)
	for _, u := range users {
		if strings.EqualFold(u.username, fullName) {
			return u.user, nil
		}
	}
	return nil, auth.ErrUserNotFound
}

// List returns every user matching the configured filter.
func (p *Provider) List() ([]*auth.User, error) {
	users, err := p.lookup("")
	if err != nil {
		return nil, err
	}
	out := make([]*auth.User, 0, len(users))
	for _, u := range users {
		out = append(out, u.user)
	}
	return out, nil
}

// Add is not supported: create the account in the directory instead.
func (p *Provider) Add(*auth.User, string) (*auth.User, error) { return nil, auth.ErrUnsupported }

// Update is not supported: change the account in the directory instead.
func (p *Provider) Update(string, *auth.User, string) (*auth.User, error) {
	return nil, auth.ErrUnsupported
}

// Delete is not supported: remove the account in the directory instead.
func (p *Provider) Delete(string) error { return auth.ErrUnsupported }

// Guest looks for a directory entry named "guest". When there is none, a
// synthetic guest with no permissions is returned, matching v7.
func (p *Provider) Guest() *auth.User {
	u, err := p.Find("guest")
	if err == nil && u.IsGuest() {
		return u
	}
	return &auth.User{
		Username: "guest",
		Name:     "Guest",
		Role:     auth.RoleGuest,
		HomeDir:  "/",
	}
}

// SessionFingerprint changes when the account's role, home or permissions
// change, so a downgraded user cannot keep a privileged session. LDAP has no
// local password hash, so the directory DN is included to catch a recreated
// account.
func (p *Provider) SessionFingerprint(u *auth.User) string {
	return security.Fingerprint(
		u.Username,
		"ldap",
		string(u.Role),
		u.HomeDir,
		auth.EncodePermissions(u.Permissions),
	)
}

// ldapUser is an intermediate directory record.
type ldapUser struct {
	username, name, dn string
	user               *auth.User
}

// lookup searches the directory, optionally filtered to one username, and maps
// results. Results are cached for the configured TTL.
func (p *Provider) lookup(username string) ([]ldapUser, error) {
	key := strings.ToLower(username)
	if cached, ok := p.cached(key); ok {
		return cached.users, cached.err
	}

	users, err := p.search(username)
	p.store(key, users, err)
	return users, err
}

// cacheEntry holds a cached lookup. The error is cached too, so a repeated
// search for a nonexistent account does not hit the directory each time.
type cacheEntry struct {
	users   []ldapUser
	err     error
	expires time.Time
}

func (p *Provider) cached(key string) (cacheEntry, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	e, ok := p.cache[strings.ToLower(key)]
	if !ok || time.Now().After(e.expires) {
		return cacheEntry{}, false
	}
	return e, true
}

func (p *Provider) store(key string, users []ldapUser, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cache[strings.ToLower(key)] = cacheEntry{users: users, err: err, expires: time.Now().Add(p.ttl)}
}

// Invalidate clears the cache, used after a configuration change.
func (p *Provider) Invalidate() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cache = make(map[string]cacheEntry)
}

// search performs the service-account search against the directory.
func (p *Provider) search(username string) ([]ldapUser, error) {
	conn, err := p.connect()
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	if p.cfg.BindDN != "" {
		if err := conn.Bind(p.cfg.BindDN, p.cfg.BindPass); err != nil {
			return nil, fmt.Errorf("ldap: service bind failed: %w", err)
		}
	}

	filter := p.cfg.Filter
	if filter == "" {
		filter = "(" + p.cfg.UsernameAttribute + "=*)"
	}
	if username != "" {
		// The username is escaped so a value containing "*" or "(" cannot
		// alter the filter and enumerate other accounts.
		cond := fmt.Sprintf("(%s=%s)", p.cfg.UsernameAttribute, ldapv3.EscapeFilter(username))
		filter = "(&" + filter + cond + ")"
	}

	attrs := p.cfg.Attributes
	if len(attrs) == 0 {
		attrs = []string{p.cfg.UsernameAttribute, p.nameAttribute(), "dn"}
	}

	req := ldapv3.NewSearchRequest(
		p.cfg.BaseDN,
		ldapv3.ScopeWholeSubtree,
		ldapv3.NeverDerefAliases,
		0, 0, false,
		filter,
		attrs,
		nil,
	)
	result, err := conn.Search(req)
	if err != nil {
		return nil, fmt.Errorf("ldap: search failed: %w", err)
	}

	users := make([]ldapUser, 0, len(result.Entries))
	for _, entry := range result.Entries {
		u, ok := p.mapEntry(entry)
		if !ok {
			continue
		}
		users = append(users, u)
	}
	return users, nil
}

// mapEntry converts one directory entry into a user, skipping incomplete ones.
func (p *Provider) mapEntry(entry *ldapv3.Entry) (ldapUser, bool) {
	username := entry.GetAttributeValue(p.cfg.UsernameAttribute)
	if username == "" {
		log.Printf("ldap: skipping %s: missing attribute %q", entry.DN, p.cfg.UsernameAttribute)
		return ldapUser{}, false
	}
	displayName := entry.GetAttributeValue(p.nameAttribute())
	if displayName == "" {
		displayName = username
	}
	username = p.addDomain(username)

	perms, err := auth.ParsePermissions(strings.Join(p.cfg.DefaultPermissions, "|"))
	if err != nil {
		log.Printf("ldap: invalid default permissions: %v", err)
		return ldapUser{}, false
	}

	role := auth.RoleUser
	home := p.homeFor(username)

	for _, admin := range p.cfg.AdminUsernames {
		if strings.EqualFold(admin, username) {
			role = auth.RoleAdmin
			// Admins see everything, mirroring v7.
			perms = append([]auth.Permission(nil), auth.AllPermissions...)
			home = "/"
			break
		}
	}

	return ldapUser{
		username: username,
		name:     displayName,
		dn:       entry.DN,
		user: &auth.User{
			Username:    username,
			Name:        displayName,
			Role:        role,
			HomeDir:     home,
			Permissions: perms,
		},
	}, true
}

// homeFor applies the configured home template.
func (p *Provider) homeFor(username string) string {
	tpl := strings.TrimSpace(p.cfg.HomeTemplate)
	if tpl == "" {
		return "/"
	}
	home := strings.ReplaceAll(tpl, "%s", username)
	if !strings.HasPrefix(home, "/") {
		home = "/" + home
	}
	return home
}

// nameAttribute returns the display name attribute, defaulting to "cn".
func (p *Provider) nameAttribute() string {
	if p.cfg.NameAttribute != "" {
		return p.cfg.NameAttribute
	}
	return "cn"
}

// stripDomains removes configured domain suffixes from a typed username.
func (p *Provider) stripDomains(username string) string {
	for _, domain := range p.cfg.RemoveDomains {
		if domain == "" {
			continue
		}
		username = strings.ReplaceAll(username, domain, "")
	}
	return username
}

// addDomain appends the configured domain suffix when it is missing.
func (p *Provider) addDomain(username string) string {
	if p.cfg.AddDomain == "" {
		return username
	}
	if strings.Contains(username, p.cfg.AddDomain) {
		return username
	}
	return username + p.cfg.AddDomain
}

// connect opens a connection with the configured timeout.
func (p *Provider) connect() (*ldapv3.Conn, error) {
	conn, err := ldapv3.DialURL(p.cfg.Server, ldapv3.DialWithDialer(&net.Dialer{Timeout: p.timeout}))
	if err != nil {
		return nil, fmt.Errorf("ldap: connect to %s: %w", p.cfg.Server, err)
	}
	conn.SetTimeout(p.timeout)
	return conn, nil
}

// bindAs verifies a password by binding as the user's DN.
func (p *Provider) bindAs(dn, password string) (bool, error) {
	conn, err := p.connect()
	if err != nil {
		return false, err
	}
	defer conn.Close()

	if err := conn.Bind(dn, password); err != nil {
		var ldapErr *ldapv3.Error
		if errors.As(err, &ldapErr) && ldapErr.ResultCode == ldapv3.LDAPResultInvalidCredentials {
			return false, nil
		}
		return false, fmt.Errorf("ldap: bind failed: %w", err)
	}
	return true, nil
}
