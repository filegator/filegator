package admin

import (
	"bytes"
	"encoding/json"
	"errors"
	"maps"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/filegator/filegator/internal/api/httpx"
	"github.com/filegator/filegator/internal/config"
	"github.com/filegator/filegator/internal/logger"
	"github.com/filegator/filegator/internal/router"
)

// Settings is the editable part of the configuration. Fields that need a
// restart (storage root) or must never be exposed (session key) live
// elsewhere.
//
// Every field is a pointer so a request says which fields it is setting: a
// field that is absent from the JSON keeps its stored value, while a field that
// is present - including an explicit empty list or "" - replaces it. That is
// what lets the Settings page and the Files page save separately even though
// they edit one configuration block; without it, a page that sent only its own
// fields would reset everything it did not show to the defaults. An explicit
// null is treated as absent, because that is what a JSON encoder emits for a
// nil list and it does not mean "set this to nothing".
type Settings struct {
	// Appearance
	AppName           *string `json:"app_name"`
	Language          *string `json:"language"`
	LanguageSelection *bool   `json:"language_selection"`
	// The logo pair: light and dark. Empty means the FileGator artwork
	// embedded in the binary, so clearing a field restores it rather than
	// leaving the interface unbranded. One pair covers every surface — the
	// header of the browser and of the admin area, and the login card.
	Logo       *string `json:"logo"`
	LogoDark   *string `json:"logo_dark"`
	DateFormat *string `json:"date_format"`

	// Behaviour
	DownloadInline     *[]string `json:"download_inline"`
	UploadMaxSize      *int64    `json:"upload_max_size"`
	UploadChunkSize    *int64    `json:"upload_chunk_size"`
	UploadSimultaneous *int      `json:"upload_simultaneous"`
	DefaultArchiveName *string   `json:"default_archive_name"`
	Editable           *[]string `json:"editable"`
	SearchSimultaneous *int      `json:"search_simultaneous"`
	FilterEntries      *[]string `json:"filter_entries"`
	// GuestRedirection sends logged-out visitors to this URL instead of
	// showing the login form, for an external login.
	GuestRedirection *string `json:"guest_redirection"`
	// FrontendDir serves the interface from a folder instead of the built-in
	// copy. Applies after a restart, because the assets are loaded at startup.
	FrontendDir *string `json:"frontend_dir"`

	// Logging
	LoggingEnabled *bool   `json:"logging_enabled"`
	LoggingFile    *string `json:"logging_file"`
	LoggingLevel   *string `json:"logging_level"`
	LoggingMaxSize *int64  `json:"logging_max_size"`

	// Security
	CSRFProtection  *bool              `json:"csrf_protection"`
	SessionTTL      *int               `json:"session_ttl"`
	SecureCookies   *bool              `json:"secure_cookies"`
	IPAllowlist     *[]string          `json:"ip_allowlist"`
	IPDenylist      *[]string          `json:"ip_denylist"`
	TrustedIPHeader *string            `json:"trusted_ip_header"`
	Headers         *map[string]string `json:"headers"`
	// AllowPasswordChange lets users change their own password. When false the
	// profile button is hidden and POST /api/auth/password is refused.
	AllowPasswordChange *bool `json:"allow_password_change"`
	// MaxWidth caps the interface width in pixels; 0 means no cap.
	MaxWidth *int `json:"max_width"`
	// AllowLogout shows the Log out button. When false the button is hidden and
	// POST /api/auth/logout is refused: with SSO the identity provider keeps
	// its session, so a local logout only looks broken.
	AllowLogout *bool `json:"allow_logout"`

	// Lockout
	LockoutAttempts *int `json:"lockout_attempts"`
	LockoutTimeout  *int `json:"lockout_timeout"`
}

// Pointer helpers: settingsFrom returns a fully populated snapshot, so every
// field it sets is "present".
func ptr[T any](v T) *T { return &v }

// settingsFrom builds a Settings snapshot from the configuration. Every field
// is filled, so the snapshot is both the GET response and the shape of a
// request that sends everything.
func settingsFrom(c *config.Config) Settings {
	return Settings{
		AppName:             ptr(c.Frontend.AppName),
		Language:            ptr(c.Frontend.Language),
		LanguageSelection:   ptr(c.Frontend.LanguageSelection),
		Logo:                ptr(c.Frontend.Logo),
		LogoDark:            ptr(c.Frontend.LogoDark),
		DateFormat:          ptr(c.Frontend.DateFormat),
		DownloadInline:      ptr(c.DownloadInline),
		UploadMaxSize:       ptr(c.Frontend.UploadMaxSize),
		UploadChunkSize:     ptr(c.Frontend.UploadChunkSize),
		UploadSimultaneous:  ptr(c.Frontend.UploadSimultaneous),
		DefaultArchiveName:  ptr(c.Frontend.DefaultArchiveName),
		Editable:            ptr(c.Frontend.Editable),
		SearchSimultaneous:  ptr(c.Frontend.SearchSimultaneous),
		FilterEntries:       ptr(c.Frontend.FilterEntries),
		GuestRedirection:    ptr(c.Frontend.GuestRedirection),
		FrontendDir:         ptr(c.FrontendDir),
		LoggingEnabled:      ptr(c.Logging.Enabled),
		LoggingFile:         ptr(c.Logging.File),
		LoggingLevel:        ptr(c.Logging.Level),
		LoggingMaxSize:      ptr(c.Logging.MaxSize),
		CSRFProtection:      ptr(c.Security.CSRFProtection),
		SessionTTL:          ptr(c.Security.SessionTTL),
		SecureCookies:       ptr(c.Security.SecureCookies),
		IPAllowlist:         ptr(c.Security.IPAllowlist),
		IPDenylist:          ptr(c.Security.IPDenylist),
		TrustedIPHeader:     ptr(c.Security.TrustedIPHeader),
		Headers:             ptr(c.Security.Headers),
		AllowPasswordChange: ptr(c.Security.AllowPasswordChange),
		AllowLogout:         ptr(c.Security.AllowLogout),
		MaxWidth:            ptr(c.Frontend.MaxWidth),
		LockoutAttempts:     ptr(c.LockoutAttempts),
		LockoutTimeout:      ptr(c.LockoutTimeout),
	}
}

// GetSettings handles GET /api/admin/settings.
func (h *Handler) GetSettings(w http.ResponseWriter, r *http.Request, _ router.Params) {
	cfg := h.Core.Config.Get()
	httpx.WriteJSON(w, http.StatusOK, settingsFrom(&cfg).view())
}

// SetSettings handles POST /api/admin/settings. Only supplied fields are
// changed: a field absent from the request keeps its stored value, so the
// Settings page and the Files page can save their own parts of one block. An
// explicit "" or empty list does replace the field.
func (h *Handler) SetSettings(w http.ResponseWriter, r *http.Request, _ router.Params) {
	var req SettingsIn
	if err := httpx.DecodeJSON(w, r, &req, httpx.MaxJSONBody); err != nil {
		httpx.WriteError(w, err)
		return
	}
	if err := validateSettings(&req); err != nil {
		httpx.WriteError(w, err)
		return
	}

	err := h.Core.Config.Update(func(c *config.Config) error {
		applySettings(c, &req)
		return nil
	})
	if errors.Is(err, config.ErrReadOnly) {
		// Update() applies the change in memory before returning ErrReadOnly, so
		// the running server already uses the new values. Reporting 200 would
		// tell the caller the setting was saved when it will be gone after a
		// restart; every other write in the API refuses in that case, and the
		// admin UI shows the message it gets back.
		h.Core.LogAdmin(r, logger.EventAdminSettings, "settings applied but not saved",
			logger.Fields{"read_only": true})
		httpx.WriteError(w, httpx.Errorf(http.StatusConflict, httpx.CodeReadOnlyConfig,
			"the settings were applied for this session but could not be saved: "+readOnlyReason(h.Core.Config)))
		return
	}
	if err != nil {
		httpx.WriteError(w, mapConfigError(err))
		return
	}

	cfg := h.Core.Config.Get()
	h.Core.LogAdmin(r, logger.EventAdminSettings, "settings saved", nil)
	httpx.WriteJSON(w, http.StatusOK, settingsFrom(&cfg).view())
}

// SettingsIn carries the fields that were actually sent. It exists so a
// request can be told apart from a decoded struct: a JSON decoder leaves an
// absent field and an explicit null both nil, and they do not mean the same
// thing. An absent key keeps the stored value; an explicit null is what a JSON
// encoder writes for a nil list, so it also changes nothing rather than
// erasing the field. A field is applied only when it is in seen.
type SettingsIn struct {
	Settings
	seen map[string]bool
}

// UnmarshalJSON decodes the settings and notes which keys were present.
func (s *SettingsIn) UnmarshalJSON(data []byte) error {
	type plain Settings
	var decoded plain
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	s.Settings = Settings(decoded)

	var keys map[string]json.RawMessage
	if err := json.Unmarshal(data, &keys); err != nil {
		return err
	}
	s.seen = make(map[string]bool, len(keys))
	for name, raw := range keys {
		if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			continue
		}
		s.seen[name] = true
	}
	return nil
}

// set reports whether a field was supplied and should be applied.
func (s *SettingsIn) set(name string) bool { return s.seen[name] }

// applySettings copies the supplied values into the configuration. A field
// that was not supplied keeps the stored value.
//
// List and map values are copied rather than referenced: the store clones its
// configuration on every write, but a slice from the decoded request must not
// be shared with callers that keep the snapshot.
func applySettings(c *config.Config, req *SettingsIn) {
	if req.set("app_name") {
		c.Frontend.AppName = strings.TrimSpace(*req.AppName)
	}
	if req.set("language") {
		c.Frontend.Language = *req.Language
	}
	if req.set("language_selection") {
		c.Frontend.LanguageSelection = *req.LanguageSelection
	}
	if req.set("logo") {
		c.Frontend.Logo = strings.TrimSpace(*req.Logo)
	}
	if req.set("logo_dark") {
		c.Frontend.LogoDark = strings.TrimSpace(*req.LogoDark)
	}
	if req.set("date_format") {
		c.Frontend.DateFormat = strings.TrimSpace(*req.DateFormat)
	}
	if req.set("download_inline") {
		c.DownloadInline = slices.Clone(*req.DownloadInline)
	}
	if req.set("upload_max_size") {
		c.Frontend.UploadMaxSize = *req.UploadMaxSize
	}
	if req.set("upload_chunk_size") {
		c.Frontend.UploadChunkSize = *req.UploadChunkSize
	}
	if req.set("upload_simultaneous") {
		c.Frontend.UploadSimultaneous = *req.UploadSimultaneous
	}
	if req.set("default_archive_name") {
		c.Frontend.DefaultArchiveName = strings.TrimSpace(*req.DefaultArchiveName)
	}
	if req.set("editable") {
		c.Frontend.Editable = slices.Clone(*req.Editable)
	}
	if req.set("download_inline") {
		// Entries are stored in the dotless lowercase form v7 used ("pdf"),
		// so the same list keeps working in either version.
		c.DownloadInline = inlineExtensions(*req.DownloadInline)
	}
	if req.set("search_simultaneous") {
		c.Frontend.SearchSimultaneous = *req.SearchSimultaneous
	}
	if req.set("filter_entries") {
		c.Frontend.FilterEntries = slices.Clone(*req.FilterEntries)
	}
	if req.set("guest_redirection") {
		c.Frontend.GuestRedirection = strings.TrimSpace(*req.GuestRedirection)
	}
	if req.set("frontend_dir") {
		c.FrontendDir = strings.TrimSpace(*req.FrontendDir)
	}
	if req.set("logging_enabled") {
		c.Logging.Enabled = *req.LoggingEnabled
	}
	if req.set("logging_file") {
		c.Logging.File = strings.TrimSpace(*req.LoggingFile)
	}
	if req.set("logging_level") {
		c.Logging.Level = *req.LoggingLevel
	}
	if req.set("logging_max_size") {
		c.Logging.MaxSize = *req.LoggingMaxSize
	}
	if req.set("csrf_protection") {
		c.Security.CSRFProtection = *req.CSRFProtection
	}
	if req.set("session_ttl") {
		c.Security.SessionTTL = *req.SessionTTL
	}
	if req.set("secure_cookies") {
		c.Security.SecureCookies = *req.SecureCookies
	}
	if req.set("ip_allowlist") {
		c.Security.IPAllowlist = slices.Clone(*req.IPAllowlist)
	}
	if req.set("ip_denylist") {
		c.Security.IPDenylist = slices.Clone(*req.IPDenylist)
	}
	if req.set("trusted_ip_header") {
		c.Security.TrustedIPHeader = strings.TrimSpace(*req.TrustedIPHeader)
	}
	if req.set("headers") {
		c.Security.Headers = maps.Clone(*req.Headers)
	}
	if req.set("allow_password_change") {
		c.Security.AllowPasswordChange = *req.AllowPasswordChange
	}
	if req.set("allow_logout") {
		c.Security.AllowLogout = *req.AllowLogout
	}
	if req.set("max_width") {
		c.Frontend.MaxWidth = *req.MaxWidth
	}
	if req.set("lockout_attempts") {
		c.LockoutAttempts = *req.LockoutAttempts
	}
	if req.set("lockout_timeout") {
		c.LockoutTimeout = *req.LockoutTimeout
	}
}

// inlineExtensions normalises a download_inline list: entries lose a leading
// dot and are lowercased, blanks are dropped and duplicates removed. The
// stored form is v7's ("pdf"), so a config written by either version reads the
// same; "*" is kept as it is, because it means "every type".
func inlineExtensions(list []string) []string {
	out := make([]string, 0, len(list))
	seen := make(map[string]bool, len(list))
	for _, raw := range list {
		ext := strings.ToLower(strings.TrimSpace(raw))
		if ext == "" || seen[ext] {
			continue
		}
		// A bare wildcard is the one entry that is not an extension.
		if ext != "*" {
			ext = strings.TrimPrefix(ext, ".")
		}
		if ext == "" || seen[ext] {
			continue
		}
		seen[ext] = true
		out = append(out, ext)
	}
	return out
}

// validateSettings reports the first problem with a settings request. Checks
// are skipped for fields the request did not carry: an omitted field is left
// as it is, and the stored configuration has already been validated.
func validateSettings(req *SettingsIn) error {
	if req.set("app_name") && strings.TrimSpace(*req.AppName) == "" {
		return httpx.Errorf(http.StatusBadRequest, httpx.CodeBadRequest, "app name must not be empty")
	}
	if req.set("language") && !config.ValidLanguage(*req.Language) {
		return httpx.Errorf(http.StatusBadRequest, httpx.CodeBadRequest, "invalid language")
	}
	if req.set("date_format") {
		if err := config.ValidateDateFormat(strings.TrimSpace(*req.DateFormat)); err != nil {
			return httpx.Errorf(http.StatusBadRequest, httpx.CodeBadRequest,
				strings.TrimPrefix(err.Error(), "config: "))
		}
	}
	// Each logo is either an http(s) URL or a path on this server. A typo in a
	// scheme ("htp://") or a bare relative path would leave the page without a
	// logo, so it is refused here rather than discovered in a screenshot.
	// An ordered list, not a map: two bad fields must report the same one
	// every time.
	for _, logo := range []struct {
		field string
		value *string
	}{
		{"logo", req.Logo},
		{"logo_dark", req.LogoDark},
	} {
		if !req.set(logo.field) {
			continue
		}
		if err := config.ValidateLogoURL(*logo.value); err != nil {
			return httpx.Errorf(http.StatusBadRequest, httpx.CodeBadRequest,
				logo.field+" "+err.Error()+"; leave it empty to use the built-in logo")
		}
	}
	if req.set("upload_max_size") && *req.UploadMaxSize <= 0 {
		return httpx.Errorf(http.StatusBadRequest, httpx.CodeBadRequest, "upload max size must be positive")
	}
	if req.set("upload_chunk_size") && *req.UploadChunkSize <= 0 {
		return httpx.Errorf(http.StatusBadRequest, httpx.CodeBadRequest, "upload chunk size must be positive")
	}
	if req.set("session_ttl") && *req.SessionTTL <= 0 {
		return httpx.Errorf(http.StatusBadRequest, httpx.CodeBadRequest, "session lifetime must be positive")
	}
	if req.set("frontend_dir") {
		if err := config.ValidateFrontendDir(*req.FrontendDir); err != nil {
			return httpx.Errorf(http.StatusBadRequest, httpx.CodeBadRequest,
				strings.TrimPrefix(err.Error(), "config: "))
		}
	}
	if req.set("max_width") && *req.MaxWidth < 0 {
		return httpx.Errorf(http.StatusBadRequest, httpx.CodeBadRequest,
			"max width must be 0 (no limit) or a positive number of pixels")
	}
	if (req.set("lockout_attempts") && *req.LockoutAttempts < 0) ||
		(req.set("lockout_timeout") && *req.LockoutTimeout < 0) {
		return httpx.Errorf(http.StatusBadRequest, httpx.CodeBadRequest, "lockout values must not be negative")
	}
	if req.set("guest_redirection") {
		if url := strings.TrimSpace(*req.GuestRedirection); url != "" {
			if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
				return httpx.Errorf(http.StatusBadRequest, httpx.CodeBadRequest,
					"guest redirection must be an http:// or https:// URL")
			}
		}
	}
	if req.set("download_inline") {
		// A bad entry would silently never match a file, so it is refused at
		// save time rather than left to look configured.
		for _, entry := range *req.DownloadInline {
			ext := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(entry), "."))
			if ext == "" || ext == "*" {
				continue
			}
			for _, r := range ext {
				if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '+' && r != '-' && r != '_' {
					return httpx.Errorf(http.StatusBadRequest, httpx.CodeBadRequest,
						"inline download extension "+strconv.Quote(entry)+
							" is not an extension; use a bare extension such as pdf, or * for all")
				}
			}
		}
	}
	if req.set("logging_level") {
		switch *req.LoggingLevel {
		case "", "debug", "info", "warn", "warning", "error":
		default:
			return httpx.Errorf(http.StatusBadRequest, httpx.CodeBadRequest,
				"log level must be debug, info, warn or error")
		}
	}
	if req.set("logging_max_size") && *req.LoggingMaxSize < 0 {
		return httpx.Errorf(http.StatusBadRequest, httpx.CodeBadRequest, "log size must not be negative")
	}
	// Reuse the config package's validation for the fiddly parts so both
	// paths agree on what is acceptable. The probe starts from the defaults
	// and carries only what the request sets, so an omitted field is validated
	// against its default rather than against a zero value that would fail.
	probe := config.Default()
	applySettings(probe, req)
	if err := probe.Validate(); err != nil {
		return httpx.Errorf(http.StatusBadRequest, httpx.CodeBadRequest, err.Error())
	}
	return nil
}

// mapConfigError converts a configuration error into a client-safe error.
func mapConfigError(err error) error {
	var coded *httpx.CodedError
	if errors.As(err, &coded) {
		return err
	}
	return httpx.Errorf(http.StatusBadRequest, httpx.CodeBadRequest, err.Error())
}

// SettingsView is the shape of GET /api/admin/settings: every field filled and
// no pointer, so a client reads what is stored rather than what was sent. It
// is also the shape of a complete request - the Settings page sends every
// field it displays - and of the merges the Files page makes.
//
// It is written out rather than derived from Settings because a snapshot has
// no "absent" case: a nil list on the way out must still be [] in the JSON so
// the frontend can join it.
type SettingsView struct {
	AppName             string            `json:"app_name"`
	Language            string            `json:"language"`
	LanguageSelection   bool              `json:"language_selection"`
	Logo                string            `json:"logo"`
	LogoDark            string            `json:"logo_dark"`
	DateFormat          string            `json:"date_format"`
	DownloadInline      []string          `json:"download_inline"`
	UploadMaxSize       int64             `json:"upload_max_size"`
	UploadChunkSize     int64             `json:"upload_chunk_size"`
	UploadSimultaneous  int               `json:"upload_simultaneous"`
	DefaultArchiveName  string            `json:"default_archive_name"`
	Editable            []string          `json:"editable"`
	SearchSimultaneous  int               `json:"search_simultaneous"`
	FilterEntries       []string          `json:"filter_entries"`
	GuestRedirection    string            `json:"guest_redirection"`
	FrontendDir         string            `json:"frontend_dir"`
	LoggingEnabled      bool              `json:"logging_enabled"`
	LoggingFile         string            `json:"logging_file"`
	LoggingLevel        string            `json:"logging_level"`
	LoggingMaxSize      int64             `json:"logging_max_size"`
	CSRFProtection      bool              `json:"csrf_protection"`
	SessionTTL          int               `json:"session_ttl"`
	SecureCookies       bool              `json:"secure_cookies"`
	IPAllowlist         []string          `json:"ip_allowlist"`
	IPDenylist          []string          `json:"ip_denylist"`
	TrustedIPHeader     string            `json:"trusted_ip_header"`
	Headers             map[string]string `json:"headers"`
	AllowPasswordChange bool              `json:"allow_password_change"`
	MaxWidth            int               `json:"max_width"`
	AllowLogout         bool              `json:"allow_logout"`
	LockoutAttempts     int               `json:"lockout_attempts"`
	LockoutTimeout      int               `json:"lockout_timeout"`
}

// view converts a full snapshot to the response shape.
func (s Settings) view() SettingsView {
	return SettingsView{
		AppName:             deref(s.AppName),
		Language:            deref(s.Language),
		LanguageSelection:   deref(s.LanguageSelection),
		Logo:                deref(s.Logo),
		LogoDark:            deref(s.LogoDark),
		DateFormat:          deref(s.DateFormat),
		DownloadInline:      emptyIfNil(deref(s.DownloadInline)),
		UploadMaxSize:       deref(s.UploadMaxSize),
		UploadChunkSize:     deref(s.UploadChunkSize),
		UploadSimultaneous:  deref(s.UploadSimultaneous),
		DefaultArchiveName:  deref(s.DefaultArchiveName),
		Editable:            emptyIfNil(deref(s.Editable)),
		SearchSimultaneous:  deref(s.SearchSimultaneous),
		FilterEntries:       emptyIfNil(deref(s.FilterEntries)),
		GuestRedirection:    deref(s.GuestRedirection),
		FrontendDir:         deref(s.FrontendDir),
		LoggingEnabled:      deref(s.LoggingEnabled),
		LoggingFile:         deref(s.LoggingFile),
		LoggingLevel:        deref(s.LoggingLevel),
		LoggingMaxSize:      deref(s.LoggingMaxSize),
		CSRFProtection:      deref(s.CSRFProtection),
		SessionTTL:          deref(s.SessionTTL),
		SecureCookies:       deref(s.SecureCookies),
		IPAllowlist:         emptyIfNil(deref(s.IPAllowlist)),
		IPDenylist:          emptyIfNil(deref(s.IPDenylist)),
		TrustedIPHeader:     deref(s.TrustedIPHeader),
		Headers:             deref(s.Headers),
		AllowPasswordChange: deref(s.AllowPasswordChange),
		MaxWidth:            deref(s.MaxWidth),
		AllowLogout:         deref(s.AllowLogout),
		LockoutAttempts:     deref(s.LockoutAttempts),
		LockoutTimeout:      deref(s.LockoutTimeout),
	}
}

func deref[T any](v *T) T {
	var zero T
	if v == nil {
		return zero
	}
	return *v
}

// emptyIfNil keeps a list [] in the JSON rather than null, so the frontend can
// join it without a guard.
func emptyIfNil[T any](v []T) []T {
	if v == nil {
		return []T{}
	}
	return v
}
