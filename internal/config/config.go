// Package config loads and saves the application configuration.
//
// Configuration is stored in a single JSON file. Defaults are applied for any
// field that is missing from the file so that older or hand-written configs
// keep working.
package config

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/filegator/filegator/internal/auth"
	"github.com/filegator/filegator/internal/security"
)

// Frontend holds settings exposed to the browser.
type Frontend struct {
	// AppName is the page title, shown in the browser tab and in the login
	// heading. It is not repeated next to the logo. It may be empty, in which
	// case only the logo is shown.
	AppName string `json:"app_name"`
	// Language is the default interface language, set by the administrator.
	Language string `json:"language"`
	// LanguageSelection controls whether users may override the default
	// language from the profile panel.
	LanguageSelection bool `json:"language_selection"`
	// Logo is the image shown in light mode: the header and the login card.
	// Empty means the FileGator logo embedded in the binary — which is what
	// every slot falls back to, so a fresh install is branded before anything
	// is configured and never fetches its own logo from another server.
	//
	// The key and its meaning are unchanged from v7, so an upgraded config
	// keeps its logo.
	Logo string `json:"logo"`
	// LogoDark is the logo shown in dark mode; empty means the dark variant of
	// the embedded FileGator logo. The interface swaps to it while the theme is
	// dark.
	LogoDark           string   `json:"logo_dark"`
	UploadMaxSize      int64    `json:"upload_max_size"`
	UploadChunkSize    int64    `json:"upload_chunk_size"`
	UploadSimultaneous int      `json:"upload_simultaneous"`
	DefaultArchiveName string   `json:"default_archive_name"`
	Editable           []string `json:"editable"`
	DateFormat         string   `json:"date_format"`
	GuestRedirection   string   `json:"guest_redirection"`
	SearchSimultaneous int      `json:"search_simultaneous"`
	// FilterEntries is a list of glob patterns (names or "/full/paths", a
	// trailing "/" meaning a folder) that a listing hides until the user
	// ticks "Show hidden". Ported from v7's filter_entries.
	FilterEntries []string `json:"filter_entries"`
	// MaxWidth caps the width of the interface in pixels; 0 means no cap.
	// On a wide monitor a file listing stretched across 2500px is hard to
	// read: the eye loses the row it was on between the name and the actions.
	MaxWidth int `json:"max_width"`
}

// Storage describes where repositories live.
type Storage struct {
	// Type selects the backend: "local" (default), "sftp" or "s3".
	Type string `json:"type"`
	// Root is the base directory containing managed repositories. For the
	// local backend it is a path on this machine; for sftp it is a directory
	// on the remote server.
	Root string `json:"root"`
	// SFTP holds the connection settings for the sftp backend.
	SFTP SFTP `json:"sftp"`
	// S3 holds the connection settings for the s3 backend.
	S3 S3 `json:"s3"`
}

// S3 is the connection configuration for the s3 storage backend. It works with
// AWS and with S3-compatible services (DigitalOcean Spaces, MinIO, Backblaze)
// through a custom endpoint.
type S3 struct {
	// Bucket is the bucket name.
	Bucket string `json:"bucket"`
	// Region is the AWS region, e.g. "us-east-1". Some S3-compatible
	// services ignore it but still require a value.
	Region string `json:"region"`
	// Endpoint overrides the AWS host, e.g. "https://nyc3.digitaloceanspaces.com".
	// Empty means AWS. A plain http:// endpoint is allowed for local testing.
	Endpoint string `json:"endpoint"`
	// AccessKey and SecretKey are the credentials.
	AccessKey string `json:"access_key"`
	SecretKey string `json:"secret_key"`
	// UsePathStyle puts the bucket in the URL path instead of the host name,
	// which most self-hosted S3 services (MinIO) require.
	UsePathStyle bool `json:"use_path_style"`
	// Prefix is an optional key prefix, so one bucket can hold several
	// repositories.
	Prefix string `json:"prefix"`
	// Timeout in seconds per request.
	Timeout int `json:"timeout"`
}

// SFTP is the connection configuration for the sftp storage backend. It
// mirrors v7's FTP adapter options (host, username, password, port) and adds
// the settings SFTP needs: a key file and a host key policy.
type SFTP struct {
	Host string `json:"host"`
	Port int    `json:"port"`
	// User is the login name.
	User string `json:"user"`
	// Password is optional when a private key is configured.
	Password string `json:"password"`
	// PrivateKey is a path to an unencrypted PEM/OpenSSH private key.
	PrivateKey string `json:"private_key"`
	// KnownHosts is a path to a known_hosts file. When empty, the server's
	// host key is accepted on first use, which is convenient but not safe
	// against interception; set this for production.
	KnownHosts string `json:"known_hosts"`
	// Timeout in seconds for dialling and for individual operations.
	Timeout int `json:"timeout"`
}

// StorageTypeLocal and StorageTypeSFTP are the supported backend identifiers.
const (
	StorageTypeLocal = "local"
	StorageTypeSFTP  = "sftp"
	StorageTypeS3    = "s3"
)

// Auth describes where users come from.
type Auth struct {
	// Type selects the provider: "json" (default) or "ldap".
	Type string `json:"type"`
	// UsersFile is the path to the users JSON file (json provider).
	UsersFile string `json:"users_file"`
	// LDAP holds the settings for the ldap provider.
	LDAP LDAP `json:"ldap"`
	// SQL holds the settings for the mysql and postgres providers.
	SQL SQL `json:"sql"`
}

// SQL configures a users table in MySQL or PostgreSQL.
//
// The table is never created by the application: run the schema from
// docs/content/database.md first. The column layout matches users.json
// (username, name, role, permissions, homedir, password).
type SQL struct {
	// DSN is the driver-specific connection string, e.g.
	// "user:pass@tcp(host:3306)/filegator" for MySQL or
	// "postgres://user:pass@host:5432/filegator?sslmode=require" for PostgreSQL.
	DSN string `json:"dsn"`
	// Table is the users table name. Defaults to "users".
	Table string `json:"table"`
	// UserCacheTTL caches resolved users for this many seconds so a busy
	// client does not query the database on every request. 0 disables caching.
	UserCacheTTL int `json:"user_cache_ttl"`
	// MaxOpenConns and MaxIdleConns bound the connection pool.
	MaxOpenConns int `json:"max_open_conns"`
	MaxIdleConns int `json:"max_idle_conns"`
	// Timeout in seconds for individual queries.
	Timeout int `json:"timeout"`
}

// LDAP mirrors v7's LDAP adapter options, plus the home folder template that
// replaces the opaque private_repos flag.
type LDAP struct {
	// Server is the URL, for example "ldap://192.168.1.1" or "ldaps://host:636".
	Server string `json:"server"`
	// BindDN and BindPass are the service account used to search the
	// directory. Searching anonymously is refused.
	BindDN   string `json:"bind_dn"`
	BindPass string `json:"bind_pass"`
	// BaseDN is the search base, for example "cn=users,dc=ldap,dc=example,dc=com".
	BaseDN string `json:"base_dn"`
	// Filter is the search filter, for example "(uid=*)". The username
	// condition is ANDed onto it.
	Filter string `json:"filter"`
	// Attributes lists what to fetch. Empty means the defaults below.
	Attributes []string `json:"attributes"`

	// UsernameAttribute is the attribute holding the login name.
	UsernameAttribute string `json:"username_attribute"`
	// NameAttribute is the attribute holding the display name.
	NameAttribute string `json:"name_attribute"`
	// AddDomain is appended to usernames when missing (e.g. "@example.com").
	AddDomain string `json:"add_domain"`
	// RemoveDomains are stripped from the typed username before searching.
	RemoveDomains []string `json:"remove_domains"`

	// DefaultPermissions are granted to every LDAP user.
	DefaultPermissions []string `json:"default_permissions"`
	// AdminUsernames are promoted to admin, which also grants all
	// permissions and the storage root as home.
	AdminUsernames []string `json:"admin_usernames"`
	// HomeTemplate is the home folder for users, with %s replaced by the
	// username. Empty means "/" (all users share the root). v7's
	// private_repos=true is equivalent to "/%s".
	HomeTemplate string `json:"home_template"`

	// UserCacheTTL is how many seconds a resolved user is cached, so a busy
	// client does not hit the directory on every request. 0 disables caching.
	UserCacheTTL int `json:"user_cache_ttl"`
	// Timeout in seconds for directory operations.
	Timeout int `json:"timeout"`
}

// Auth provider identifiers.
const (
	AuthTypeJSON     = "json"
	AuthTypeLDAP     = "ldap"
	AuthTypeMySQL    = "mysql"
	AuthTypePostgres = "postgres"
)

// Security holds protection-related settings.
type Security struct {
	CSRFProtection bool     `json:"csrf_protection"`
	SessionKey     string   `json:"session_key"`
	SessionTTL     int      `json:"session_ttl"`
	SecureCookies  bool     `json:"secure_cookies"`
	IPAllowlist    []string `json:"ip_allowlist"`
	IPDenylist     []string `json:"ip_denylist"`
	// TrustedIPHeader names a request header that carries the real client IP,
	// behind a reverse proxy. Examples: "CF-Connecting-IP", "X-Real-IP",
	// "X-Forwarded-For". Empty means the header is ignored and RemoteAddr is
	// used, which is the safe default.
	TrustedIPHeader string `json:"trusted_ip_header"`
	// Headers overrides or adds response headers. Values merged over the
	// built-in defaults; an empty value removes a default header.
	Headers map[string]string `json:"headers"`
	// AllowPasswordChange lets signed-in users change their own password and
	// hides the profile button when false. Defaults to true, so a config.json
	// written before this setting existed keeps working: Load unmarshals into
	// Default(), and a missing key leaves the default in place.
	//
	// Administrators are unaffected: they can still set any user's password
	// from Admin -> Users, which is the way to help someone who is locked out
	// when this is off.
	AllowPasswordChange bool `json:"allow_password_change"`
	// AllowLogout shows the Log out button and accepts POST /api/auth/logout.
	// Defaults to true.
	//
	// With an SSO handoff, logging out of FileGator does nothing useful: the
	// identity provider still holds a session, so the next visit signs the user
	// straight back in and the button only looks broken. Turning it off hides
	// the button. The session cookie still expires on its own at SessionTTL,
	// so a shared machine is not left signed in forever, and an administrator
	// can still end a session by changing the user's password (the fingerprint
	// covers it).
	AllowLogout bool `json:"allow_logout"`
}

// Logging controls the event log.
type Logging struct {
	// Enabled turns the event log on. Disabled means no file is written, which
	// suits a container that logs to stdout.
	Enabled bool `json:"enabled"`
	// File is the log destination: a path, resolved from the working directory
	// so the v7 layout (private/logs/app.log) works unchanged, or "-" for
	// standard output so a container supervisor can collect and rotate it.
	File string `json:"file"`
	// Level is debug, info, warn or error.
	Level string `json:"level"`
	// MaxSize rotates the file once it grows past this many bytes, keeping one
	// previous file. Zero disables rotation.
	MaxSize int64 `json:"max_size"`
}

// SSOApplication authorises an external application to hand a user into
// FileGator with a signed, single-use URL.
type SSOApplication struct {
	// Name is the identifier the handoff must name.
	Name string `json:"name"`
	// Secret is shared with the other application and signs the handoff. It is
	// write-only: the API never returns it.
	Secret string `json:"secret"`
	// Mode decides what happens to the username in a handoff:
	//   "existing" - only a user FileGator already knows, with its own
	//                permissions and home;
	//   "policy"   - always this application's permissions and home template,
	//                with no user record at all;
	//   "create"   - existing users keep their own settings, unknown names get
	//                an account built from the policy.
	Mode string `json:"mode"`
	// Home is the home template; %s becomes the username. Required for
	// "policy", used when creating accounts in "create".
	Home string `json:"home"`
	// Permissions granted in "policy" mode and to accounts created in "create".
	Permissions []string `json:"permissions"`
	// AdminUsernames are administrators inside this application's sessions
	// only. An empty list means the application grants no administrator
	// access, which is the normal case.
	AdminUsernames []string `json:"admin_usernames"`
	// Expiry is how long a handoff URL stays valid, in seconds.
	Expiry int `json:"expiry"`
}

// SSO holds the applications allowed to hand users in.
type SSO struct {
	Applications []SSOApplication `json:"applications"`
}

// SSO modes.
const (
	SSOModeExisting = "existing"
	SSOModePolicy   = "policy"
	SSOModeCreate   = "create"
)

// Config is the root configuration object.
type Config struct {
	// FrontendDir serves the interface from this directory instead of the
	// copy compiled into the binary, so HTML, CSS and JavaScript can be
	// changed without rebuilding.
	FrontendDir     string   `json:"frontend_dir"`
	DownloadInline  []string `json:"download_inline"`
	LockoutAttempts int      `json:"lockout_attempts"`
	LockoutTimeout  int      `json:"lockout_timeout"`
	Frontend        Frontend `json:"frontend_config"`
	Storage         Storage  `json:"storage"`
	Auth            Auth     `json:"auth"`
	Security        Security `json:"security"`
	Logging         Logging  `json:"logging"`
	SSO             SSO      `json:"sso"`
}

// The logo defaults point at the artwork embedded in the binary, not at the
// project website: a fresh install has to work offline and behind an egress
// filter, and the page must not depend on a third party to show its own
// branding. The files are internal/app/frontend/img/, served as assets.
const (
	DefaultLogo     = "/img/filegator-logo.svg"
	DefaultLogoDark = "/img/filegator-logo-dark.svg"
)

// DefaultUsersFile is where the JSON users provider keeps its accounts unless
// configured otherwise. Exported so callers that must not name a configured
// path (the public configuration) can still refer to the default one.
const DefaultUsersFile = "private/users.json"

// legacyLogoURL was the logo default before the artwork was embedded.
// config.json written by that release has it stored as an ordinary value, so
// Load treats it as "the default was never changed" and replaces it. The admin
// API is not affected: a logo typed in the interface is saved verbatim.
const legacyLogoURL = "https://filegator.io/filegator_logo.svg"

// Default returns a configuration with sensible defaults.
func Default() *Config {
	return &Config{
		// Images and PDFs render in the browser. Everything else is served as
		// an attachment, which also stops uploaded HTML or SVG from executing
		// on this origin. '*' is allowed but dangerous: it makes every
		// uploaded file render here.
		DownloadInline:  []string{"pdf", "jpg", "jpeg", "png", "gif", "webp", "bmp", "avif"},
		LockoutAttempts: 5,
		LockoutTimeout:  15,
		Frontend: Frontend{
			AppName:            "FileGator",
			Language:           "english",
			LanguageSelection:  true,
			Logo:               DefaultLogo,
			LogoDark:           DefaultLogoDark,
			UploadMaxSize:      100 * 1024 * 1024,
			UploadChunkSize:    1 * 1024 * 1024,
			UploadSimultaneous: 3,
			DefaultArchiveName: "archive.zip",
			Editable:           []string{".txt", ".css", ".js", ".ts", ".html", ".json", ".md"},
			DateFormat:         "YYYY/MM/DD HH:mm:ss",
			SearchSimultaneous: 5,
			FilterEntries:      []string{},
			MaxWidth:           960,
		},
		Storage: Storage{Type: StorageTypeLocal, Root: "repository"},
		Auth:    Auth{Type: AuthTypeJSON, UsersFile: DefaultUsersFile},
		Security: Security{
			CSRFProtection: true,
			SessionTTL:     8 * 60 * 60,
			SecureCookies:  false,
			IPAllowlist:    []string{},
			IPDenylist:     []string{},
			Headers:        map[string]string{},
			// On by default: turning either off is a deliberate lockdown.
			AllowPasswordChange: true,
			AllowLogout:         true,
		},
		// Stdout by default: a container supervisor collects and rotates it,
		// and a fresh install works on a read-only volume. Set a path to keep
		// a file instead, as v7 did with private/logs/app.log.
		SSO: SSO{Applications: []SSOApplication{}},
		Logging: Logging{
			Enabled: true,
			File:    "-",
			Level:   "info",
			MaxSize: 0,
		},
	}
}

// Store loads, saves and provides synchronized access to the configuration.
type Store struct {
	mu   *sync.RWMutex
	path string
	cfg  *Config
	// frontendDir overrides FrontendDir for this run only (see WithFrontendDir).
	frontendDir *string
	// readOnly is set when the configuration cannot be persisted, which is
	// common on read-only volumes. The server keeps running on in-memory
	// values, but callers must tell the administrator that changes will not
	// survive a restart.
	readOnly bool
}

// Load reads the config file at path. A missing file yields defaults and is
// not treated as an error.
//
// When the directory cannot be written, Load still succeeds and the store is
// marked read-only, so the server can start on in-memory settings instead of
// refusing to boot on a read-only volume.
func Load(path string) (*Store, error) {
	s := &Store{mu: &sync.RWMutex{}, path: path, cfg: Default()}

	data, err := os.ReadFile(path)
	switch {
	case errors.Is(err, os.ErrNotExist):
		s.readOnly = !canWriteDir(filepath.Dir(path))
		return s, nil
	case err != nil:
		return nil, fmt.Errorf("config: read %s: %w", path, err)
	}

	// Unmarshal into defaults so unspecified fields keep default values.
	if err := json.Unmarshal(data, s.cfg); err != nil {
		return nil, fmt.Errorf("config: parse %s: %w", path, err)
	}
	// A file written by an older release carries that release's logo default
	// as a real value, so it would keep loading the artwork from the project
	// website instead of the copy embedded in this binary. The old default is
	// migrated once, on load: it was never a deliberate choice, and the file is
	// rewritten with the current default on the next save. Any other logo an
	// administrator typed is left exactly as it is.
	if s.cfg.Frontend.Logo == legacyLogoURL {
		s.cfg.Frontend.Logo = DefaultLogo
	}
	// A file that exists but cannot be replaced is read-only too.
	s.readOnly = !canWriteDir(filepath.Dir(path))
	return s, nil
}

// canWriteDir reports whether the configuration writer can create files in dir.
//
// A missing directory is writable when it can be created: the writer calls
// MkdirAll before writing, and on a fresh install private/ is exactly that —
// it does not exist yet. Probing it as if it did made the very first start look
// read-only (no session key, no config.json), and the second start succeed,
// because by then the users provider had created private/ on its own. The check
// is therefore about the nearest directory that exists: whether a new entry can
// be put there, and whether the missing ones can be created below it.
func canWriteDir(dir string) bool {
	probeDir := dir
	for {
		if info, err := os.Stat(probeDir); err == nil {
			if !info.IsDir() {
				return false
			}
			break
		}
		parent := filepath.Dir(probeDir)
		if parent == probeDir {
			return false
		}
		probeDir = parent
	}
	probe, err := os.CreateTemp(probeDir, ".write-probe-*")
	if err != nil {
		return false
	}
	name := probe.Name()
	probe.Close()
	os.Remove(name)
	return true
}

// writeReason explains why the configuration cannot be persisted.
//
// "read-only" alone sends operators looking for a read-only mount when the real
// cause is a folder that cannot be created, or a path whose parent is a file.
// Callers put this in the message an administrator reads.
func writeReason(dir string) string {
	probeDir := dir
	for {
		info, err := os.Stat(probeDir)
		if err == nil {
			if !info.IsDir() {
				return probeDir + " is not a folder"
			}
			probe, err := os.CreateTemp(probeDir, ".write-probe-*")
			if err != nil {
				return probeDir + " cannot be written to: " + err.Error()
			}
			name := probe.Name()
			probe.Close()
			os.Remove(name)
			if probeDir != dir {
				return dir + " cannot be created, because " + probeDir + " is not writable"
			}
			return dir + " is not writable"
		}
		parent := filepath.Dir(probeDir)
		if parent == probeDir {
			return dir + " does not exist"
		}
		probeDir = parent
	}
}

// Why returns what is wrong when the configuration cannot be persisted, or ""
// when it can. It is meant for the message an administrator reads: "read-only"
// alone suggests a read-only mount even when the cause is a folder that could
// not be created.
func (s *Store) Why() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.readOnly {
		return ""
	}
	return writeReason(filepath.Dir(s.path))
}

// Path returns the configuration file path.
func (s *Store) Path() string { return s.path }

// ReadOnly reports whether the configuration cannot be persisted. Callers
// should warn the administrator instead of failing.
func (s *Store) ReadOnly() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.readOnly
}

// Get returns a snapshot of the current configuration. Slices are copied so
// callers cannot mutate shared state.
func (s *Store) Get() Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := s.cfg.clone()
	if s.frontendDir != nil {
		out.FrontendDir = *s.frontendDir
	}
	return out
}

// clone returns a deep copy of the configuration.
func (c *Config) clone() Config {
	out := *c
	out.DownloadInline = slices.Clone(c.DownloadInline)
	out.Frontend = c.Frontend
	out.Frontend.Editable = slices.Clone(c.Frontend.Editable)
	out.Frontend.FilterEntries = slices.Clone(c.Frontend.FilterEntries)
	out.Security.IPAllowlist = slices.Clone(c.Security.IPAllowlist)
	out.Security.IPDenylist = slices.Clone(c.Security.IPDenylist)
	out.Security.Headers = maps.Clone(c.Security.Headers)
	return out
}

// ErrReadOnly is returned when the configuration cannot be persisted.
var ErrReadOnly = errors.New("config: configuration is read-only")

// Update replaces the configuration and persists it to disk. If fn returns an
// error or validation fails, the in-memory configuration is left untouched.
//
// When the store is read-only the change is still applied in memory so the
// server keeps working, but ErrReadOnly is returned so the caller can tell the
// administrator that the change will not survive a restart.
func (s *Store) Update(fn func(*Config) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	next := s.cfg.clone()
	if err := fn(&next); err != nil {
		return err
	}
	if err := next.Validate(); err != nil {
		return err
	}
	if s.readOnly {
		s.cfg = &next
		return ErrReadOnly
	}
	if err := s.write(&next); err != nil {
		// A write may fail despite the earlier probe (full disk, revoked
		// permission). Keep serving on in-memory values rather than crashing.
		s.cfg = &next
		return fmt.Errorf("%w: %v", ErrReadOnly, err)
	}
	s.cfg = &next
	return nil
}

func (s *Store) write(cfg *Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("config: marshal: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("config: create dir: %w", err)
	}
	// Write to a temp file then rename for atomicity.
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o600); err != nil {
		return fmt.Errorf("config: write temp: %w", err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return fmt.Errorf("config: rename: %w", err)
	}
	return nil
}

// Validate checks invariants that must hold before a config is saved.
func (c *Config) Validate() error {
	if strings.TrimSpace(c.Frontend.AppName) == "" {
		return errors.New("config: app_name must not be empty")
	}
	if !ValidLanguage(c.Frontend.Language) {
		return fmt.Errorf("config: frontend_config.language %q is not a valid language code", c.Frontend.Language)
	}
	if c.Frontend.UploadMaxSize <= 0 {
		return errors.New("config: upload_max_size must be positive")
	}
	if c.Frontend.UploadChunkSize <= 0 {
		return errors.New("config: upload_chunk_size must be positive")
	}
	if err := ValidateDateFormat(c.Frontend.DateFormat); err != nil {
		return err
	}
	for name, logo := range map[string]string{
		"logo":      c.Frontend.Logo,
		"logo_dark": c.Frontend.LogoDark,
	} {
		if err := ValidateLogoURL(logo); err != nil {
			return fmt.Errorf("config: frontend_config.%s: %w", name, err)
		}
	}
	if err := ValidateSSO(c.SSO); err != nil {
		return err
	}
	switch c.Logging.Level {
	case "", "debug", "info", "warn", "warning", "error":
	default:
		return fmt.Errorf("config: logging.level %q must be debug, info, warn or error", c.Logging.Level)
	}
	// Empty is accepted when logging is enabled and means standard error: a
	// server that cannot write a volume must still be able to start.
	if strings.TrimSpace(c.Logging.File) == "-" && c.Logging.MaxSize > 0 {
		// stdout is rotated by whoever collects it, not by us.
		return errors.New("config: logging.max_size must be 0 when logging to stdout")
	}
	if c.Logging.MaxSize < 0 {
		return errors.New("config: logging.max_size must not be negative")
	}
	if c.LockoutAttempts < 0 {
		return errors.New("config: lockout_attempts must not be negative")
	}
	if c.LockoutTimeout < 0 {
		return errors.New("config: lockout_timeout must not be negative")
	}
	// Root is a path for local and sftp; S3 addresses objects by bucket and
	// key, so it has no root folder.
	if c.Storage.Root == "" && c.Storage.Type != StorageTypeS3 {
		return errors.New("config: storage.root must not be empty")
	}
	// An unknown type would silently fall back to local storage, which could
	// point at the wrong place, so reject it instead.
	switch c.Storage.Type {
	case "", StorageTypeLocal:
	case StorageTypeSFTP:
		if c.Storage.SFTP.Host == "" {
			return errors.New("config: storage.sftp.host must not be empty")
		}
		if c.Storage.SFTP.User == "" {
			return errors.New("config: storage.sftp.user must not be empty")
		}
		if c.Storage.SFTP.Password == "" && c.Storage.SFTP.PrivateKey == "" {
			return errors.New("config: storage.sftp needs a password or a private_key")
		}
		if p := c.Storage.SFTP.Port; p != 0 && (p < 1 || p > 65535) {
			return errors.New("config: storage.sftp.port must be between 1 and 65535")
		}
	case StorageTypeS3:
		if c.Storage.S3.Bucket == "" {
			return errors.New("config: storage.s3.bucket must not be empty")
		}
		if c.Storage.S3.AccessKey == "" || c.Storage.S3.SecretKey == "" {
			return errors.New("config: storage.s3 needs access_key and secret_key")
		}
		if c.Storage.S3.Region == "" {
			return errors.New("config: storage.s3.region must not be empty")
		}
		if c.Storage.S3.Timeout < 0 {
			return errors.New("config: storage.s3.timeout must not be negative")
		}
	default:
		return fmt.Errorf("config: unknown storage.type %q", c.Storage.Type)
	}
	switch c.Auth.Type {
	case "", AuthTypeJSON:
		if c.Auth.UsersFile == "" {
			return errors.New("config: auth.users_file must not be empty")
		}
	case AuthTypeLDAP:
		if c.Auth.LDAP.Server == "" {
			return errors.New("config: auth.ldap.server must not be empty")
		}
		if c.Auth.LDAP.BaseDN == "" {
			return errors.New("config: auth.ldap.base_dn must not be empty")
		}
		if c.Auth.LDAP.UsernameAttribute == "" {
			return errors.New("config: auth.ldap.username_attribute must not be empty")
		}
		if c.Auth.LDAP.UserCacheTTL < 0 {
			return errors.New("config: auth.ldap.user_cache_ttl must not be negative")
		}
		if c.Auth.LDAP.Timeout < 0 {
			return errors.New("config: auth.ldap.timeout must not be negative")
		}
		for _, p := range c.Auth.LDAP.DefaultPermissions {
			if !auth.ValidPermission(auth.Permission(p)) {
				return fmt.Errorf("config: auth.ldap.default_permissions: unknown permission %q", p)
			}
		}
	case AuthTypeMySQL, AuthTypePostgres:
		if c.Auth.SQL.DSN == "" {
			return errors.New("config: auth.sql.dsn must not be empty")
		}
		if c.Auth.SQL.UserCacheTTL < 0 {
			return errors.New("config: auth.sql.user_cache_ttl must not be negative")
		}
		if c.Auth.SQL.MaxOpenConns < 0 || c.Auth.SQL.MaxIdleConns < 0 {
			return errors.New("config: auth.sql connection limits must not be negative")
		}
		if c.Auth.SQL.Timeout < 0 {
			return errors.New("config: auth.sql.timeout must not be negative")
		}
	default:
		return fmt.Errorf("config: unknown auth.type %q", c.Auth.Type)
	}
	if c.Security.SessionTTL <= 0 {
		return errors.New("config: security.session_ttl must be positive")
	}
	if c.Security.SessionKey != "" {
		if _, err := hex.DecodeString(c.Security.SessionKey); err != nil {
			return errors.New("config: security.session_key must be a hex string")
		}
	}
	if h := c.Security.TrustedIPHeader; h != "" {
		// Header names are case-insensitive tokens. Rejecting anything else
		// also prevents injection-style values from reaching the config.
		for _, r := range h {
			if !isTokenChar(r) {
				return fmt.Errorf("config: security.trusted_ip_header %q is not a valid header name", h)
			}
		}
	}
	for _, entry := range append(append([]string{}, c.Security.IPAllowlist...), c.Security.IPDenylist...) {
		if err := validateIPEntry(entry); err != nil {
			return fmt.Errorf("config: %w", err)
		}
	}
	for name, value := range c.Security.Headers {
		if err := security.ValidateHeaders(map[string]string{name: value}); err != nil {
			return fmt.Errorf("config: security.headers: %w", err)
		}
	}
	return nil
}

// DefaultSSOExpiry is how long a handoff URL stays valid unless configured.
const DefaultSSOExpiry = 60

// ValidateFrontendDir checks a frontend directory setting.
//
// It is called when saving settings so a typo is reported immediately rather
// than at the next restart. An empty value means "use the built-in interface".
func ValidateFrontendDir(dir string) error {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return nil
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("config: frontend_dir %q: %w", dir, err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return fmt.Errorf("config: frontend_dir %q: %w", dir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("config: frontend_dir %q is not a directory", dir)
	}
	return nil
}

// WithFrontendDir returns a view of the store that serves the interface from
// dir, without writing it to the configuration file.
func (s *Store) WithFrontendDir(dir string) *Store {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return &Store{mu: s.mu, path: s.path, cfg: s.cfg, readOnly: s.readOnly, frontendDir: &dir}
}

// ValidateSSO checks the applications allowed to hand users in.
func ValidateSSO(sso SSO) error {
	seen := make(map[string]bool, len(sso.Applications))
	for i, app := range sso.Applications {
		name := strings.TrimSpace(app.Name)
		if name == "" {
			return fmt.Errorf("config: sso application %d has no name", i+1)
		}
		if seen[name] {
			return fmt.Errorf("config: sso application %q is listed twice", name)
		}
		seen[name] = true

		if len(strings.TrimSpace(app.Secret)) < 32 {
			return fmt.Errorf("config: sso application %q needs a secret of at least 32 characters", name)
		}
		switch app.Mode {
		case SSOModeExisting, SSOModePolicy, SSOModeCreate:
		case "":
			return fmt.Errorf("config: sso application %q needs a mode (existing, policy or create)", name)
		default:
			return fmt.Errorf("config: sso application %q has an unknown mode %q", name, app.Mode)
		}
		if app.Mode == SSOModePolicy && strings.TrimSpace(app.Home) == "" {
			return fmt.Errorf("config: sso application %q uses policy mode and needs a home template", name)
		}
		if app.Mode == SSOModeCreate && strings.TrimSpace(app.Home) == "" {
			return fmt.Errorf("config: sso application %q creates accounts and needs a home template", name)
		}
		for _, raw := range app.Permissions {
			p := auth.Permission(strings.TrimSpace(raw))
			if !auth.ValidPermission(p) {
				return fmt.Errorf("config: sso application %q has an invalid permission %q", name, raw)
			}
		}
		if app.Expiry < 0 || app.Expiry > 3600 {
			return fmt.Errorf("config: sso application %q expiry must be between 0 and 3600 seconds", name)
		}
	}
	return nil
}

// FindSSOApplication returns the application with the given name, or nil.
func (s SSO) FindSSOApplication(name string) *SSOApplication {
	for i := range s.Applications {
		if s.Applications[i].Name == name {
			return &s.Applications[i]
		}
	}
	return nil
}

// ValidateDateFormat checks an administrator's date pattern, written in the
// moment.js tokens v7 used (for example "YY/MM/DD hh:mm:ss") so an upgraded
// configuration keeps its format. The pattern is rendered by the browser, so
// this only rejects what cannot be a pattern at all: an empty value (meaning
// "use the default"), control characters, and an unreasonable length.
func ValidateDateFormat(pattern string) error {
	if pattern == "" {
		return nil
	}
	if len(pattern) > 64 {
		return errors.New("config: frontend_config.date_format must be at most 64 characters")
	}
	for _, r := range pattern {
		if r < 0x20 || r == 0x7f {
			return errors.New("config: frontend_config.date_format must not contain control characters")
		}
	}
	// Text between brackets is printed literally, so an unclosed one would
	// swallow the rest of the pattern.
	if strings.Count(pattern, "[") != strings.Count(pattern, "]") {
		return errors.New("config: frontend_config.date_format has an unclosed [ or ]")
	}
	return nil
}

// ValidateLogoURL checks one of the four logo settings. Empty is always
// accepted: it means "use the FileGator logo embedded in the binary", so a
// fresh install needs no URL at all.
//
// An absolute http(s) URL is accepted, and so is a path on this server
// ("/img/logo.svg") because that is what the built-in artwork and anything the
// administrator puts in a custom interface folder are. A relative path, a
// scheme such as javascript: or data:, or anything with control characters is
// refused: the value is written into an <img src>, where those either break the
// page or run script in this origin.
func ValidateLogoURL(logo string) error {
	logo = strings.TrimSpace(logo)
	if logo == "" {
		return nil
	}
	for _, r := range logo {
		if r < 0x20 || r == 0x7f {
			return errors.New("must not contain control characters")
		}
	}
	if strings.HasPrefix(logo, "https://") || strings.HasPrefix(logo, "http://") {
		if u, err := url.Parse(logo); err != nil || u.Host == "" {
			return errors.New("is not a valid URL")
		}
		return nil
	}
	// A path on this server. "//host/x" is a scheme-relative URL, not a path.
	if strings.HasPrefix(logo, "/") && !strings.HasPrefix(logo, "//") {
		return nil
	}
	return errors.New("must be an http(s) URL or a path starting with /")
}

// ValidLanguage reports whether name is a well-formed language code. Codes
// are lowercase letters, digits and underscores, matching the file names used
// for translations (for example "english", "portuguese_br").
func ValidLanguage(name string) bool {
	if name == "" || len(name) > 32 {
		return false
	}
	for _, r := range name {
		ok := r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '_'
		if !ok {
			return false
		}
	}
	return true
}

// isTokenChar reports whether r may appear in an HTTP header name.
func isTokenChar(r rune) bool {
	if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
		return true
	}
	return strings.ContainsRune("!#$%&'*+-.^_`|~", r)
}

// validateIPEntry checks that an allow/deny entry is an IP address or CIDR.
func validateIPEntry(entry string) error {
	entry = strings.TrimSpace(entry)
	if entry == "" {
		return nil
	}
	var err error
	if strings.Contains(entry, "/") {
		_, err = netip.ParsePrefix(entry)
	} else {
		_, err = netip.ParseAddr(entry)
	}
	if err != nil {
		return fmt.Errorf("invalid IP entry %q: %w", entry, err)
	}
	return nil
}
