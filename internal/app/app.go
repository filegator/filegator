// Package app wires configuration, routing and the embedded frontend into a
// ready-to-run HTTP server.
package app

import (
	"context"
	"errors"
	"io/fs"
	"log"
	"net/http"
	"path"
	"strings"
	"sync/atomic"
	"time"

	"github.com/filegator/filegator/internal/api"
	"github.com/filegator/filegator/internal/config"
	"github.com/filegator/filegator/internal/logger"
	"github.com/filegator/filegator/internal/router"
	"github.com/filegator/filegator/internal/security"
	"github.com/filegator/filegator/internal/sso"
)

// App holds the long-lived application state.
type App struct {
	config *config.Store
	// ctx is cancelled by Close, stopping background sweeps.
	ctx    context.Context
	cancel context.CancelFunc
	router *router.Router
	// frontend is the filesystem the interface is served from: the embedded
	// copy, or frontend_dir when configured.
	frontend fs.FS
	static   *staticServer
	// source names where the interface came from, for the startup log.
	source string
	// staleFrontend is the version of a custom interface built for a different
	// release than this binary, or "".
	staleFrontend string
	api           *api.Server
	// log records security-relevant events; it may be nil when logging is
	// disabled or the file could not be opened.
	log *logger.Logger
	// logLevel is the level the logger was built with, so a settings change
	// only rebuilds the logger when the level actually differs.
	logLevel atomic.Value
	// startup records what this start created or could not resolve, shown to
	// administrators in the interface (see startup_notes.go).
	startup *StartupLog
}

// New builds an application from a configuration store.
//
// basePath is the URL prefix the app is served under, such as "/browser"; ""
// or "/" means the site root. It comes from the command line rather than the
// configuration because it describes how the process was started, not how the
// application is set up.
func New(cfg *config.Store, basePath string) (*App, error) {
	ctx, cancel := context.WithCancel(context.Background())
	frontend, source, staleVersion, err := resolveFrontend(cfg)
	if err != nil {
		cancel()
		return nil, err
	}
	a := &App{
		config:        cfg,
		router:        router.New(),
		frontend:      frontend,
		static:        newStaticServer(frontend),
		source:        source,
		staleFrontend: staleVersion,
		ctx:           ctx,
		cancel:        cancel,
		startup:       &StartupLog{},
	}
	// How the configuration was found decides what an operator sees in the
	// interface: a missing file on a writable volume is created by the session
	// key below, which is a very different situation from a read-only one.
	a.startup.RecordConfigSource(cfg)
	a.router.SetBasePath(router.NormalizeBasePath(basePath))
	if err := a.initAPI(); err != nil {
		cancel()
		return nil, err
	}
	a.routes()
	return a, nil
}

// initAPI constructs the API server and its dependencies.
func (a *App) initAPI() error {
	cfg := a.config.Get()

	// Generate and persist a session key on first start so sessions survive
	// restarts.
	if cfg.Security.SessionKey == "" {
		key, err := security.CreateKeyHex()
		if err != nil {
			return err
		}
		// A missing config file is written for the first time here, carrying
		// the session key every later restart needs to keep sessions valid.
		hadFile := fileExists(a.config.Path())
		// Persist the key on the store that is actually in use: a -base-path
		// run hands the app a derived store, and writing through the original
		// would save the wrong base path (or, on a fresh install, none at all).
		if err := a.config.Update(func(c *config.Config) error {
			if c.Security.SessionKey == "" {
				c.Security.SessionKey = key
			}
			return nil
		}); err != nil {
			if !errors.Is(err, config.ErrReadOnly) {
				return err
			}
			// The key stays in memory: sessions work but do not survive a
			// restart. The reason matters: "read-only" alone sent operators
			// looking for a read-only mount when the folder simply could not
			// be created.
			log.Printf("warning: cannot save the session key in %s: %s; sessions will not survive a restart",
				a.config.Path(), readOnlyReason(a.config))
			a.startup.add(noteWarning, "config.no_session_key",
				"Cannot save the session key in "+a.config.Path()+": "+readOnlyReason(a.config)+
					". Every restart signs everybody out.")
		} else if !hadFile {
			log.Printf("created %s with a new session key", a.config.Path())
			// The "missing" note from the start of New is resolved: replace it
			// with what actually happened, so the admin sees one line, not two.
			a.startup.removeKey("config.missing")
			a.startup.add(noteInfo, "config.created",
				"Created "+a.config.Path()+" with a new session key.")
		}
		cfg = a.config.Get()
	}
	if cfg.Security.SessionKey == "" {
		// Read-only session key generation failed earlier; fall back to an
		// ephemeral key so the server can still run.
		key, err := security.CreateKeyHex()
		if err != nil {
			return err
		}
		cfg.Security.SessionKey = key
	}

	key, err := security.ParseKeyHex(cfg.Security.SessionKey)
	if err != nil {
		return err
	}
	sessions, err := security.NewSessions(
		key,
		time.Duration(cfg.Security.SessionTTL)*time.Second,
		cfg.Security.SecureCookies,
	)
	if err != nil {
		return err
	}
	// Cookies follow the mount point so they are not sent to other apps on the
	// same domain, and so a sibling app cannot read them.
	basePath := a.router.BasePath()
	sessions.SetPath(basePath)

	ipFilter, err := security.NewIPFilter(
		cfg.Security.IPAllowlist,
		cfg.Security.IPDenylist,
		cfg.Security.TrustedIPHeader,
	)
	if err != nil {
		return err
	}
	root, err := openStorage(cfg.Storage, a.startup)
	if err != nil {
		return err
	}

	users, err := openAuth(cfg.Auth, a.startup)
	if err != nil {
		return err
	}

	a.api = &api.Server{
		Config:   a.config,
		Users:    users,
		Sessions: sessions,
		CSRF:     newCSRF(cfg.Security.SecureCookies, basePath),
		Lockout:  security.NewLockout(cfg.LockoutAttempts, time.Duration(cfg.LockoutTimeout)*time.Second),
		IPFilter: ipFilter,
	}
	// Signed handoffs from other applications. The endpoint only answers when
	// at least one application is configured.
	a.api.SSO = sso.NewVerifier()
	a.api.SetBasePath(a.router.BasePath())
	a.api.SetFrontendSource(a.source)
	a.api.SetStaleFrontendVersion(a.staleFrontend)
	// The event log is optional: a container that cannot write a file still
	// serves requests, and stderr remains available.
	if logCfg := cfg.Logging; logCfg.Enabled {
		eventLog, err := logger.New(logger.Config{
			File:    logCfg.File,
			Level:   logCfg.Level,
			MaxSize: logCfg.MaxSize,
		})
		if err != nil {
			log.Printf("warning: event logging disabled: %v", err)
		} else {
			a.log = eventLog
			a.api.Log = eventLog
			a.logLevel.Store(logCfg.Level)
			// Name the destination as the operator configured it; "-" means
			// standard output, so say so rather than leaving a bare dash in
			// the startup message.
			destination := logCfg.File
			if destination == logger.Stdout {
				destination = "stdout"
			}
			log.Printf("event log: %s (level %s)", destination, logCfg.Level)
		}
	}

	a.api.SetRoot(root, root.Description())

	// Remove chunk staging left over from interrupted uploads, then keep
	// sweeping: a server that runs for weeks would otherwise never clean.
	a.api.StartUploadCleanup(a.ctx)
	return nil
}

// routes registers HTTP routes.
func (a *App) routes() {
	// HTML entrypoint.
	a.router.Get("/", func(w http.ResponseWriter, r *http.Request, p router.Params) {
		a.serveIndex(w, r)
	})

	a.registerFunctionRoutes(a.router)

	// The log level is applied per request so an administrator can switch to
	// debug (or quieten a noisy log) from the Settings tab without a restart.
	a.router.Use(func(next router.Handler) router.Handler {
		return func(w http.ResponseWriter, r *http.Request, p router.Params) {
			if a.log != nil {
				if want := a.config.Get().Logging.Level; a.logLevel.Load() != want {
					a.log.SetLevel(want)
					a.logLevel.Store(want)
				}
			}
			next(w, r, p)
		}
	})

	// The IP allow and deny lists are enforced before anything else, so a
	// blocked address cannot reach a handler, an asset or the login form.
	a.router.Use(func(next router.Handler) router.Handler {
		return func(w http.ResponseWriter, r *http.Request, p router.Params) {
			if !a.api.IPFilter.Allow(r) {
				a.api.Log.Warn(logger.EventIPBlocked, "blocked by the IP allow/deny lists",
					logger.Fields{
						"ip":     a.api.IPFilter.Address(r),
						"path":   r.URL.Path,
						"method": r.Method,
					})
				http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
				return
			}
			next(w, r, p)
		}
	})

	// Security headers apply to every response.
	a.router.Use(func(next router.Handler) router.Handler {
		return func(w http.ResponseWriter, r *http.Request, p router.Params) {
			for name, value := range security.MergeHeaders(security.DefaultHeaders(), a.config.Get().Security.Headers) {
				if value != "" {
					w.Header().Set(name, value)
				}
			}
			next(w, r, p)
		}
	})
}

// serveIndex writes the embedded index.html.
func (a *App) serveIndex(w http.ResponseWriter, r *http.Request) {
	data, err := fs.ReadFile(a.frontend, "index.html")
	if err != nil {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}
	// The mount prefix is substituted once per page load; the browser then
	// resolves every relative URL against it. "apiBase" is also handed to the
	// script as a data attribute so the API client never has to guess.
	base := a.router.BasePath()
	page := strings.ReplaceAll(string(data), "{{BASE_PATH}}", base)
	page = strings.ReplaceAll(page, "{{API_BASE}}", a.apiBase())
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Write([]byte(page))
}

// newCSRF builds the double-submit protector, scoped to the mount point.
func newCSRF(secure bool, basePath string) *security.CSRF {
	c := security.NewCSRF(secure)
	c.Path = basePath
	return c
}

// apiBase returns the prefix the API lives under, without a trailing slash.
func (a *App) apiBase() string { return a.router.BasePath() }

// ServeHTTP implements http.Handler. Requests for embedded assets are served
// directly; everything else goes through the router.
func (a *App) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	original := r
	// Assets are served before the router runs, so the mount prefix has to be
	// stripped here too: the router does it for API routes only.
	if base := a.router.BasePath(); base != "" {
		rest, ok := strings.CutPrefix(r.URL.Path, base)
		if !ok || (rest != "" && !strings.HasPrefix(rest, "/")) {
			http.NotFound(w, r)
			return
		}
		r2 := r.Clone(r.Context())
		r2.URL.Path = rest
		if r2.URL.Path == "" {
			r2.URL.Path = "/"
		}
		r = r2
	}

	if r.Method == http.MethodGet || r.Method == http.MethodHead {
		if name := assetPath(r.URL.Path); name != "" {
			if a.static.exists(name) {
				a.static.serve(w, r, name)
				return
			}
			// A missing asset must not fall through to the router: browsers
			// then report a confusing "disallowed MIME type" error for the
			// text/plain 404 body instead of a plain 404. Answer directly with
			// the expected content type so the real problem (a missing file or
			// a broken import) is obvious.
			w.Header().Set("Content-Type", contentTypeForAsset(name))
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.WriteHeader(http.StatusNotFound)
			if r.Method != http.MethodHead {
				w.Write([]byte("404 asset not found: " + name))
			}
			return
		}
	}
	// The router strips the prefix itself, so give it the original request.
	a.router.ServeHTTP(w, original)
}

// contentTypeForAsset returns the content type implied by an asset name.
func contentTypeForAsset(name string) string {
	switch path.Ext(name) {
	case ".js", ".mjs":
		return "text/javascript; charset=utf-8"
	case ".css":
		return "text/css; charset=utf-8"
	case ".html":
		return "text/html; charset=utf-8"
	case ".json":
		return "application/json; charset=utf-8"
	case ".svg":
		return "image/svg+xml"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	}
	return "text/plain; charset=utf-8"
}

// Close stops background work and releases resources.
func (a *App) Close() error {
	a.cancel()
	return a.log.Close()
}

// ListenAndServe starts the HTTP server on addr.
func (a *App) ListenAndServe(addr string) error {
	log.Printf("listening on http://%s%s/", addr, a.router.BasePath())
	return http.ListenAndServe(addr, a)
}

// assetPath converts a URL path into a relative asset name. It returns "" for
// paths that do not look like embedded files (e.g. "/" or "/api/ping").
func assetPath(urlPath string) string {
	for _, prefix := range []string{"/css/", "/js/", "/img/"} {
		if rest, ok := strings.CutPrefix(urlPath, prefix); ok {
			return prefix[1:] + rest
		}
	}
	return ""
}

// staticServer serves a subtree of an fs.FS.
type staticServer struct {
	root fs.FS
}

func newStaticServer(root fs.FS) *staticServer {
	return &staticServer{root: root}
}

func (s *staticServer) exists(name string) bool {
	_, err := fs.Stat(s.root, name)
	return err == nil
}

func (s *staticServer) serve(w http.ResponseWriter, r *http.Request, name string) {
	http.ServeFileFS(w, r, s.root, name)
}
