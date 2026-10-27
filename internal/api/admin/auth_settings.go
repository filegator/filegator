package admin

import (
	"errors"
	"net/http"

	"github.com/filegator/filegator/internal/api/httpx"
	"strings"

	"github.com/filegator/filegator/internal/auth"
	"github.com/filegator/filegator/internal/auth/jsonfile"
	"github.com/filegator/filegator/internal/auth/ldap"
	sqlprovider "github.com/filegator/filegator/internal/auth/sql"
	"github.com/filegator/filegator/internal/config"
	"github.com/filegator/filegator/internal/logger"
	"github.com/filegator/filegator/internal/router"
)

// AuthConfig is the admin view of the auth provider settings. The LDAP bind
// password is never returned; HasBindPass reports whether one is stored.
type AuthConfig struct {
	Type      string `json:"type"`
	UsersFile string `json:"users_file"`

	LDAPServer             string   `json:"ldap_server"`
	LDAPBindDN             string   `json:"ldap_bind_dn"`
	LDAPBindPass           string   `json:"ldap_bind_pass,omitempty"`
	LDAPHasBindPass        bool     `json:"ldap_has_bind_pass"`
	LDAPBaseDN             string   `json:"ldap_base_dn"`
	LDAPFilter             string   `json:"ldap_filter"`
	LDAPAttributes         []string `json:"ldap_attributes"`
	LDAPUsernameAttribute  string   `json:"ldap_username_attribute"`
	LDAPNameAttribute      string   `json:"ldap_name_attribute"`
	LDAPAddDomain          string   `json:"ldap_add_domain"`
	LDAPRemoveDomains      []string `json:"ldap_remove_domains"`
	LDAPDefaultPermissions []string `json:"ldap_default_permissions"`
	LDAPAdminUsernames     []string `json:"ldap_admin_usernames"`
	LDAPHomeTemplate       string   `json:"ldap_home_template"`
	LDAPUserCacheTTL       int      `json:"ldap_user_cache_ttl"`
	LDAPTimeout            int      `json:"ldap_timeout"`

	// SQLDSN is write-only: a DSN commonly embeds the password. An empty value
	// keeps the stored DSN.
	SQLDSN          string `json:"sql_dsn,omitempty"`
	SQLHasDSN       bool   `json:"sql_has_dsn"`
	SQLTable        string `json:"sql_table"`
	SQLUserCacheTTL int    `json:"sql_user_cache_ttl"`
	SQLMaxOpenConns int    `json:"sql_max_open_conns"`
	SQLMaxIdleConns int    `json:"sql_max_idle_conns"`
	SQLTimeout      int    `json:"sql_timeout"`
}

// GetAuth handles GET /api/admin/auth.
func (h *Handler) GetAuth(w http.ResponseWriter, r *http.Request, _ router.Params) {
	cfg := h.Core.Config.Get().Auth
	h.Core.LogAdmin(r, logger.EventAdminAuthChanged, "authentication backend changed",
		logger.Fields{"type": h.Core.Config.Get().Auth.Type})
	httpx.WriteJSON(w, http.StatusOK, AuthConfig{
		Type:                   cfg.Type,
		UsersFile:              cfg.UsersFile,
		LDAPServer:             cfg.LDAP.Server,
		LDAPBindDN:             cfg.LDAP.BindDN,
		LDAPHasBindPass:        cfg.LDAP.BindPass != "",
		LDAPBaseDN:             cfg.LDAP.BaseDN,
		LDAPFilter:             cfg.LDAP.Filter,
		LDAPAttributes:         cfg.LDAP.Attributes,
		LDAPUsernameAttribute:  cfg.LDAP.UsernameAttribute,
		LDAPNameAttribute:      cfg.LDAP.NameAttribute,
		LDAPAddDomain:          cfg.LDAP.AddDomain,
		LDAPRemoveDomains:      cfg.LDAP.RemoveDomains,
		LDAPDefaultPermissions: cfg.LDAP.DefaultPermissions,
		LDAPAdminUsernames:     cfg.LDAP.AdminUsernames,
		LDAPHomeTemplate:       cfg.LDAP.HomeTemplate,
		LDAPUserCacheTTL:       cfg.LDAP.UserCacheTTL,
		LDAPTimeout:            cfg.LDAP.Timeout,
		SQLHasDSN:              cfg.SQL.DSN != "",
		SQLTable:               cfg.SQL.Table,
		SQLUserCacheTTL:        cfg.SQL.UserCacheTTL,
		SQLMaxOpenConns:        cfg.SQL.MaxOpenConns,
		SQLMaxIdleConns:        cfg.SQL.MaxIdleConns,
		SQLTimeout:             cfg.SQL.Timeout,
	})
}

// SetAuth handles POST /api/admin/auth and switches the auth
// provider at runtime.
//
// The candidate provider is validated before anything is saved: for LDAP this
// means the server must accept the service bind and the base DN must be
// searchable. A failed switch leaves the current provider active, so an admin
// cannot lock everyone out with a typo.
func (h *Handler) SetAuth(w http.ResponseWriter, r *http.Request, _ router.Params) {
	var req AuthConfig
	if err := httpx.DecodeJSON(w, r, &req, httpx.MaxJSONBody); err != nil {
		httpx.WriteError(w, err)
		return
	}
	if req.Type == "" {
		req.Type = config.AuthTypeJSON
	}

	current := h.Core.Config.Get().Auth
	if req.LDAPBindPass == "" && req.LDAPHasBindPass && req.Type == config.AuthTypeLDAP {
		req.LDAPBindPass = current.LDAP.BindPass
	}

	candidate := config.Auth{
		Type:      req.Type,
		UsersFile: strings.TrimSpace(req.UsersFile),
		LDAP: config.LDAP{
			Server:             strings.TrimSpace(req.LDAPServer),
			BindDN:             strings.TrimSpace(req.LDAPBindDN),
			BindPass:           req.LDAPBindPass,
			BaseDN:             strings.TrimSpace(req.LDAPBaseDN),
			Filter:             strings.TrimSpace(req.LDAPFilter),
			Attributes:         cleanList(req.LDAPAttributes),
			UsernameAttribute:  strings.TrimSpace(req.LDAPUsernameAttribute),
			NameAttribute:      strings.TrimSpace(req.LDAPNameAttribute),
			AddDomain:          strings.TrimSpace(req.LDAPAddDomain),
			RemoveDomains:      cleanList(req.LDAPRemoveDomains),
			DefaultPermissions: cleanList(req.LDAPDefaultPermissions),
			AdminUsernames:     cleanList(req.LDAPAdminUsernames),
			HomeTemplate:       strings.TrimSpace(req.LDAPHomeTemplate),
			UserCacheTTL:       req.LDAPUserCacheTTL,
			Timeout:            req.LDAPTimeout,
		},
		SQL: config.SQL{
			DSN:          strings.TrimSpace(req.SQLDSN),
			Table:        strings.TrimSpace(req.SQLTable),
			UserCacheTTL: req.SQLUserCacheTTL,
			MaxOpenConns: req.SQLMaxOpenConns,
			MaxIdleConns: req.SQLMaxIdleConns,
			Timeout:      req.SQLTimeout,
		},
	}
	if candidate.UsersFile == "" {
		// Keep the existing users file so switching away from LDAP and back
		// does not lose the path.
		candidate.UsersFile = current.UsersFile
	}

	probe := h.Core.Config.Get()
	probe.Auth = candidate
	if err := probe.Validate(); err != nil {
		httpx.WriteError(w, httpx.Errorf(http.StatusBadRequest, httpx.CodeBadRequest, err.Error()))
		return
	}

	provider, err := openAuthProvider(candidate)
	if err != nil {
		httpx.WriteError(w, httpx.Errorf(http.StatusBadRequest, httpx.CodeBadRequest, err.Error()))
		return
	}

	// Persist, then swap. A read-only config still switches for this run, but
	// the caller is told the choice will be lost on restart: answering 200
	// made a switch that cannot survive a restart look saved.
	if err := h.Core.Config.Update(func(c *config.Config) error {
		c.Auth = candidate
		return nil
	}); err != nil {
		if !errors.Is(err, config.ErrReadOnly) {
			httpx.WriteError(w, err)
			return
		}
		h.Core.Users = provider
		h.Core.LogAdmin(r, logger.EventAdminAuthChanged, "authentication backend changed but was not saved",
			logger.Fields{"type": candidate.Type, "read_only": true})
		httpx.WriteError(w, httpx.Errorf(http.StatusConflict, httpx.CodeReadOnlyConfig,
			"the authentication backend was switched for this session but could not be saved: "+readOnlyReason(h.Core.Config)))
		return
	}

	h.Core.Users = provider
	h.Core.LogAdmin(r, logger.EventAdminAuthChanged, "authentication backend changed",
		logger.Fields{"type": h.Core.Config.Get().Auth.Type})
	httpx.WriteJSON(w, http.StatusOK, AuthConfig{
		Type:                   candidate.Type,
		UsersFile:              candidate.UsersFile,
		LDAPServer:             candidate.LDAP.Server,
		LDAPBindDN:             candidate.LDAP.BindDN,
		LDAPHasBindPass:        candidate.LDAP.BindPass != "",
		LDAPBaseDN:             candidate.LDAP.BaseDN,
		LDAPFilter:             candidate.LDAP.Filter,
		LDAPAttributes:         candidate.LDAP.Attributes,
		LDAPUsernameAttribute:  candidate.LDAP.UsernameAttribute,
		LDAPNameAttribute:      candidate.LDAP.NameAttribute,
		LDAPAddDomain:          candidate.LDAP.AddDomain,
		LDAPRemoveDomains:      candidate.LDAP.RemoveDomains,
		LDAPDefaultPermissions: candidate.LDAP.DefaultPermissions,
		LDAPAdminUsernames:     candidate.LDAP.AdminUsernames,
		LDAPHomeTemplate:       candidate.LDAP.HomeTemplate,
		LDAPUserCacheTTL:       candidate.LDAP.UserCacheTTL,
		LDAPTimeout:            candidate.LDAP.Timeout,
		SQLHasDSN:              candidate.SQL.DSN != "",
		SQLTable:               candidate.SQL.Table,
		SQLUserCacheTTL:        candidate.SQL.UserCacheTTL,
		SQLMaxOpenConns:        candidate.SQL.MaxOpenConns,
		SQLMaxIdleConns:        candidate.SQL.MaxIdleConns,
		SQLTimeout:             candidate.SQL.Timeout,
	})
}

// openAuthProvider builds an auth provider without the first-run file setup
// that the app bootstrap performs.
func openAuthProvider(cfg config.Auth) (auth.Provider, error) {
	switch cfg.Type {
	case "", config.AuthTypeJSON:
		users := jsonfile.NewUsersFile(cfg.UsersFile)
		if err := users.CheckWritable(); err != nil {
			return nil, err
		}
		return users, nil
	case config.AuthTypeMySQL:
		return sqlprovider.New(cfg.SQL, sqlprovider.MySQL)
	case config.AuthTypePostgres:
		return sqlprovider.New(cfg.SQL, sqlprovider.Postgres)
	case config.AuthTypeLDAP:
		return ldap.New(cfg.LDAP)
	default:
		return nil, errors.New("unknown auth type " + cfg.Type)
	}
}

// cleanList trims entries and drops empties.
func cleanList(in []string) []string {
	out := make([]string, 0, len(in))
	for _, item := range in {
		if v := strings.TrimSpace(item); v != "" {
			out = append(out, v)
		}
	}
	return out
}
