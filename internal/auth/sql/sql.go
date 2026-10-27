// Package sql implements auth.Provider against a MySQL or PostgreSQL users
// table.
//
// The same code serves both databases: only the placeholder syntax and
// identifier quoting differ, and those live behind the Dialect type. The table
// layout matches users.json so an existing FileGator database keeps working:
//
//	username   varchar  primary key
//	name       varchar
//	role       varchar   ('guest' | 'user' | 'admin')
//	permissions varchar  ('read|write|upload|...')
//	homedir    varchar
//	password   varchar   (bcrypt hash)
//
// The application never creates or migrates the table; see
// docs/content/database.md for the schema.
package sql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	// Drivers are registered by importing them.
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/filegator/filegator/internal/auth"
	"github.com/filegator/filegator/internal/auth/jsonfile"
	"github.com/filegator/filegator/internal/config"
	"github.com/filegator/filegator/internal/security"
)

// DefaultTTL is used when the config leaves user_cache_ttl at 0.
const DefaultTTL = 60 * time.Second

// DefaultTimeout bounds a single query.
const DefaultTimeout = 10 * time.Second

// Dialect captures the small differences between supported databases.
type Dialect struct {
	// Name is "mysql" or "postgres", used in messages.
	Name string
	// Placeholder renders the nth (1-based) bind parameter.
	Placeholder func(n int) string
	// Quote wraps an identifier in the database's quoting characters.
	Quote func(ident string) string
	// Driver is the database/sql driver name.
	Driver string
}

// MySQL is the MySQL/MariaDB dialect.
var MySQL = Dialect{
	Name:        "mysql",
	Driver:      "mysql",
	Placeholder: func(int) string { return "?" },
	Quote:       func(id string) string { return "`" + id + "`" },
}

// Postgres is the PostgreSQL dialect.
var Postgres = Dialect{
	Name:        "postgres",
	Driver:      "pgx",
	Placeholder: func(n int) string { return fmt.Sprintf("$%d", n) },
	Quote:       func(id string) string { return `"` + id + `"` },
}

// Provider authenticates users from a database table.
type Provider struct {
	db       *sql.DB
	dialect  Dialect
	table    string
	timeout  time.Duration
	ttl      time.Duration
	describe string

	mu    sync.RWMutex
	cache map[string]cacheEntry
}

type cacheEntry struct {
	user    *auth.User
	err     error
	expires time.Time
}

// New opens the database, verifies the table is usable and returns a provider.
//
// It fails fast on an unreachable database or a missing table, so a
// misconfiguration is reported at startup rather than at the first login.
func New(cfg config.SQL, dialect Dialect) (*Provider, error) {
	if cfg.DSN == "" {
		return nil, errors.New("sql: dsn is required")
	}
	db, err := sql.Open(dialect.Driver, cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("sql: open %s: %w", dialect.Name, err)
	}

	if cfg.MaxOpenConns > 0 {
		db.SetMaxOpenConns(cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns > 0 {
		db.SetMaxIdleConns(cfg.MaxIdleConns)
	}

	table := cfg.Table
	if table == "" {
		table = "users"
	}

	p := &Provider{
		db:       db,
		dialect:  dialect,
		table:    table,
		timeout:  DefaultTimeout,
		ttl:      DefaultTTL,
		describe: dialect.Name + ":" + table,
		cache:    make(map[string]cacheEntry),
	}
	if cfg.Timeout > 0 {
		p.timeout = time.Duration(cfg.Timeout) * time.Second
	}
	if cfg.UserCacheTTL > 0 {
		p.ttl = time.Duration(cfg.UserCacheTTL) * time.Second
	}

	// Verify connectivity and that the table exists. The error names the
	// expected schema so a missing table is actionable.
	if err := p.verify(); err != nil {
		db.Close()
		return nil, err
	}
	return p, nil
}

// verify checks the connection and the table.
func (p *Provider) verify() error {
	ctx, cancel := p.context()
	defer cancel()

	if err := p.db.PingContext(ctx); err != nil {
		return fmt.Errorf("sql: cannot connect to %s: %w", p.dialect.Name, err)
	}
	query := "SELECT username, name, role, permissions, homedir, password FROM " +
		p.quotedTable() + " WHERE 1 = 0"
	rows, err := p.db.QueryContext(ctx, query)
	if err != nil {
		return fmt.Errorf("sql: table %q is not usable (run the schema from docs/content/database.md): %w",
			p.table, err)
	}
	rows.Close()
	return nil
}

// Close releases the connection pool.
func (p *Provider) Close() error { return p.db.Close() }

// Describe returns a short label for logs and the admin UI.
func (p *Provider) Describe() string { return p.describe }

// Capabilities reports that users are managed here, like the JSON provider.
func (p *Provider) Capabilities() auth.Caps {
	return auth.Caps{ManageUsers: true, ChangePassword: true}
}

// Writable reports whether the table accepts writes.
func (p *Provider) Writable() bool { return true }

// DefaultAdminPassword reports whether any admin still uses the shipped
// default password.
func (p *Provider) DefaultAdminPassword() bool {
	users, err := p.List()
	if err != nil {
		return false
	}
	for _, u := range users {
		if u.IsAdmin() && jsonfile.VerifyPassword(jsonfile.DefaultAdminPassword, u.Credential()) {
			return true
		}
	}
	return false
}

// SessionFingerprint changes when the account's password, role, home or
// permissions change, so a stale session is rejected.
func (p *Provider) SessionFingerprint(u *auth.User) string {
	return security.Fingerprint(
		u.Username,
		u.Credential(),
		string(u.Role),
		u.HomeDir,
		auth.EncodePermissions(u.Permissions),
	)
}

// Authenticate verifies a username and password against the stored hash.
func (p *Provider) Authenticate(username, password string) (*auth.User, error) {
	if username == "" || password == "" {
		return nil, auth.ErrInvalidPassword
	}
	u, hash, err := p.fetch(username)
	if err != nil {
		return nil, err
	}
	if !jsonfile.VerifyPassword(password, hash) {
		return nil, auth.ErrInvalidPassword
	}
	return u, nil
}

// Find returns a user by username.
func (p *Provider) Find(username string) (*auth.User, error) {
	u, _, err := p.fetch(username)
	return u, err
}

// List returns every user in the table.
func (p *Provider) List() ([]*auth.User, error) {
	ctx, cancel := p.context()
	defer cancel()

	query := "SELECT username, name, role, permissions, homedir, password FROM " +
		p.quotedTable() + " ORDER BY username"
	rows, err := p.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("sql: list users: %w", err)
	}
	defer rows.Close()

	var out []*auth.User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sql: list users: %w", err)
	}
	return out, nil
}

// Guest returns the guest account, or nil when the table has none.
func (p *Provider) Guest() *auth.User {
	u, _, err := p.fetch("guest")
	if err != nil || !u.IsGuest() {
		return nil
	}
	return u
}

// Add inserts a new user with a bcrypt-hashed password.
func (p *Provider) Add(u *auth.User, password string) (*auth.User, error) {
	if err := validateUser(u); err != nil {
		return nil, err
	}
	if password == "" {
		return nil, auth.ErrInvalidPassword
	}
	if _, _, err := p.fetch(u.Username); err == nil {
		return nil, auth.ErrUsernameTaken
	} else if !errors.Is(err, auth.ErrUserNotFound) {
		return nil, err
	}

	hash, err := jsonfile.HashPassword(password)
	if err != nil {
		return nil, err
	}

	ctx, cancel := p.context()
	defer cancel()

	values := make([]string, 6)
	for i := range values {
		values[i] = p.dialect.Placeholder(i + 1)
	}
	query := fmt.Sprintf(
		"INSERT INTO %s (username, name, role, permissions, homedir, password) VALUES (%s)",
		p.quotedTable(), strings.Join(values, ", "),
	)
	_, err = p.db.ExecContext(ctx, query,
		u.Username, u.Name, string(u.Role), auth.EncodePermissions(u.Permissions),
		normaliseHome(u.HomeDir), hash,
	)
	if err != nil {
		return nil, fmt.Errorf("sql: add user: %w", err)
	}
	p.invalidate()
	created, _, err := p.fetch(u.Username)
	return created, err
}

// Update changes an existing user. An empty password keeps the stored hash.
func (p *Provider) Update(oldUsername string, u *auth.User, password string) (*auth.User, error) {
	if err := validateUser(u); err != nil {
		return nil, err
	}
	// Renaming onto an existing username would violate the primary key; report
	// it as a plain conflict instead of a database error.
	if u.Username != oldUsername {
		if _, _, err := p.fetch(u.Username); err == nil {
			return nil, auth.ErrUsernameTaken
		} else if !errors.Is(err, auth.ErrUserNotFound) {
			return nil, err
		}
	}

	// Build the SET clause and its arguments together so placeholder numbering
	// stays correct for PostgreSQL.
	var (
		sets []string
		args []any
	)
	add := func(column string, value any) {
		sets = append(sets, column+" = "+p.dialect.Placeholder(len(args)+1))
		args = append(args, value)
	}

	if u.Username != oldUsername {
		add("username", u.Username)
	}
	add("name", u.Name)
	add("role", string(u.Role))
	add("permissions", auth.EncodePermissions(u.Permissions))
	add("homedir", normaliseHome(u.HomeDir))

	if password != "" {
		hash, err := jsonfile.HashPassword(password)
		if err != nil {
			return nil, err
		}
		add("password", hash)
	}

	where := p.dialect.Placeholder(len(args) + 1)
	args = append(args, oldUsername)

	query := fmt.Sprintf("UPDATE %s SET %s WHERE username = %s",
		p.quotedTable(), strings.Join(sets, ", "), where)

	ctx, cancel := p.context()
	defer cancel()

	result, err := p.db.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("sql: update user: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return nil, auth.ErrUserNotFound
	}

	p.invalidate()
	updated, _, err := p.fetch(u.Username)
	return updated, err
}

// Delete removes a user.
func (p *Provider) Delete(username string) error {
	ctx, cancel := p.context()
	defer cancel()

	query := fmt.Sprintf("DELETE FROM %s WHERE username = %s", p.quotedTable(), p.dialect.Placeholder(1))
	result, err := p.db.ExecContext(ctx, query, username)
	if err != nil {
		return fmt.Errorf("sql: delete user: %w", err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return auth.ErrUserNotFound
	}
	p.invalidate()
	return nil
}

// fetch loads one user and its password hash, using the cache when enabled.
func (p *Provider) fetch(username string) (*auth.User, string, error) {
	key := strings.ToLower(username)
	if p.ttl > 0 {
		p.mu.RLock()
		entry, ok := p.cache[key]
		p.mu.RUnlock()
		if ok && time.Now().Before(entry.expires) {
			if entry.err != nil {
				return nil, "", entry.err
			}
			return entry.user, entry.user.Credential(), nil
		}
	}

	ctx, cancel := p.context()
	defer cancel()

	query := fmt.Sprintf(
		"SELECT username, name, role, permissions, homedir, password FROM %s WHERE username = %s",
		p.quotedTable(), p.dialect.Placeholder(1),
	)
	row := p.db.QueryRowContext(ctx, query, username)

	u, err := scanUser(row)
	if errors.Is(err, sql.ErrNoRows) {
		p.storeCache(key, nil, auth.ErrUserNotFound)
		return nil, "", auth.ErrUserNotFound
	}
	if err != nil {
		return nil, "", err
	}
	p.storeCache(key, u, nil)
	return u, u.Credential(), nil
}

// storeCache records a lookup result.
func (p *Provider) storeCache(key string, u *auth.User, err error) {
	if p.ttl <= 0 {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cache[key] = cacheEntry{user: u, err: err, expires: time.Now().Add(p.ttl)}
}

// invalidate clears the cache after a write.
func (p *Provider) invalidate() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cache = make(map[string]cacheEntry)
}

// context returns a bounded context for one query.
func (p *Provider) context() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), p.timeout)
}

// quotedTable returns the quoted table name.
func (p *Provider) quotedTable() string { return p.dialect.Quote(p.table) }

// scanner is satisfied by both *sql.Row and *sql.Rows.
type scanner interface {
	Scan(dest ...any) error
}

// scanUser reads one row into a User.
func scanUser(s scanner) (*auth.User, error) {
	var (
		username, name, role, permissions, homedir, password string
	)
	if err := s.Scan(&username, &name, &role, &permissions, &homedir, &password); err != nil {
		return nil, err
	}

	r := auth.Role(role)
	if !auth.ValidRole(r) {
		return nil, fmt.Errorf("sql: user %q has invalid role %q", username, role)
	}
	perms, err := auth.ParsePermissions(permissions)
	if err != nil {
		return nil, fmt.Errorf("sql: user %q: %w", username, err)
	}

	u := &auth.User{
		Username:    username,
		Name:        name,
		Role:        r,
		HomeDir:     normaliseHome(homedir),
		Permissions: perms,
	}
	u.SetCredential(password)
	return u, nil
}

// validateUser checks the fields before a write.
func validateUser(u *auth.User) error {
	if strings.TrimSpace(u.Username) == "" {
		return errors.New("sql: username must not be empty")
	}
	if !auth.ValidRole(u.Role) {
		return auth.ErrInvalidRole
	}
	for _, p := range u.Permissions {
		if !auth.ValidPermission(p) {
			return auth.ErrInvalidPerm
		}
	}
	return nil
}

// normaliseHome defaults an empty home to the storage root.
func normaliseHome(h string) string {
	if h == "" {
		return "/"
	}
	return h
}

// Ensure the provider satisfies the shared interface.
var _ auth.Provider = (*Provider)(nil)
