package admin

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/filegator/filegator/internal/auth"
	"github.com/filegator/filegator/internal/config"
)

// settingsBody builds a complete settings request from the current config so
// tests only override what they care about.
//
// It works on the plain SettingsView rather than the pointer fields the
// request uses: a test asserting on a save should not have to dereference
// anything, and sending every field is exactly what the Settings page does.
func (w *adminTestEnv) settingsBody(mutate func(*SettingsView)) string {
	w.t.Helper()
	rec := w.req(http.MethodGet, "/api/admin/settings", "")
	if rec.Code != http.StatusOK {
		w.t.Fatalf("get settings: status %d", rec.Code)
	}
	var envelope struct {
		Data SettingsView `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		w.t.Fatal(err)
	}
	settings := envelope.Data
	if mutate != nil {
		mutate(&settings)
	}
	data, err := json.Marshal(settings)
	if err != nil {
		w.t.Fatal(err)
	}
	return string(data)
}

// getSettings returns the current settings as a plain view.
func (w *adminTestEnv) getSettings() SettingsView {
	w.t.Helper()
	rec := w.req(http.MethodGet, "/api/admin/settings", "")
	if rec.Code != http.StatusOK {
		w.t.Fatalf("get settings: status %d body %s", rec.Code, rec.Body)
	}
	var envelope struct {
		Data SettingsView `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		w.t.Fatal(err)
	}
	return envelope.Data
}

func TestGetSettings(t *testing.T) {
	w := adminEnv(t)
	rec := w.req(http.MethodGet, "/api/admin/settings", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body)
	}
	var result struct {
		Data SettingsView `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Data.AppName != "FileGator" {
		t.Errorf("app name = %q, want FileGator", result.Data.AppName)
	}
	if result.Data.Logo == "" {
		t.Error("default logo missing")
	}
	if result.Data.SessionTTL <= 0 {
		t.Error("session ttl missing")
	}
	// The session key must never be exposed.
	if containsStr(rec.Body.String(), "session_key") {
		t.Error("settings leak the session key")
	}
}

func TestUpdateSettings(t *testing.T) {
	w := adminEnv(t)
	body := w.settingsBody(func(s *SettingsView) {
		s.AppName = "My Files"
		s.Language = "german"
		s.Logo = "/img/logo.svg"
		s.UploadMaxSize = 50 << 20
		s.IPAllowlist = []string{"10.0.0.0/8"}
	})

	rec := w.submit("/api/admin/settings", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body)
	}

	cfg := w.srv.Config.Get()
	if cfg.Frontend.AppName != "My Files" {
		t.Errorf("app name = %q", cfg.Frontend.AppName)
	}
	if cfg.Frontend.Language != "german" {
		t.Errorf("language = %q", cfg.Frontend.Language)
	}
	if cfg.Frontend.UploadMaxSize != 50<<20 {
		t.Errorf("upload max = %d", cfg.Frontend.UploadMaxSize)
	}
	if len(cfg.Security.IPAllowlist) != 1 {
		t.Errorf("allowlist = %v", cfg.Security.IPAllowlist)
	}

	// And it must survive a reload from disk.
	reloaded, err := config.Load(w.srv.Config.Path())
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Get().Frontend.AppName != "My Files" {
		t.Error("settings were not persisted")
	}
}

func TestUpdateSettingsValidation(t *testing.T) {
	w := adminEnv(t)
	cases := map[string]func(*SettingsView){
		"empty app name":   func(s *SettingsView) { s.AppName = "  " },
		"bad language":     func(s *SettingsView) { s.Language = "German!" },
		"zero max size":    func(s *SettingsView) { s.UploadMaxSize = 0 },
		"zero chunk size":  func(s *SettingsView) { s.UploadChunkSize = 0 },
		"zero session ttl": func(s *SettingsView) { s.SessionTTL = 0 },
		"negative lockout": func(s *SettingsView) { s.LockoutAttempts = -1 },
		"bad ip":           func(s *SettingsView) { s.IPAllowlist = []string{"999.1.1.1"} },
		"bad header name":  func(s *SettingsView) { s.Headers = map[string]string{"Bad Name": "x"} },
		// A logo is written into an <img src>: a scheme other than http(s) or
		// a bare relative path either breaks the page or runs script here.
		"script logo":     func(s *SettingsView) { s.Logo = "javascript:alert(1)" },
		"data logo":       func(s *SettingsView) { s.LogoDark = "data:text/html,<script>" },
		"relative logo":   func(s *SettingsView) { s.Logo = "img/logo.svg" },
		"scheme-less":     func(s *SettingsView) { s.LogoDark = "//cdn.example.com/a.svg" },
		"logo w/ newline": func(s *SettingsView) { s.Logo = "https://example.com/a.svg\r\nX-Evil: 1" },
	}
	for name, mutate := range cases {
		rec := w.submit("/api/admin/settings", w.settingsBody(mutate))
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status %d, want 400 (body %s)", name, rec.Code, rec.Body)
		}
	}
}

// A logo may be a path on this server as well as an http(s) URL, and clearing
// it restores the built-in artwork rather than leaving the interface unbranded.
func TestLogoSettingsAcceptPathsAndEmpty(t *testing.T) {
	w := adminEnv(t)
	for _, value := range []string{"", "/img/filegator-logo.svg", "https://example.com/logo.svg"} {
		rec := w.submit("/api/admin/settings", w.settingsBody(func(s *SettingsView) {
			s.Logo = value
			s.LogoDark = value
		}))
		if rec.Code != http.StatusOK {
			t.Errorf("logo %q: status %d, want 200 (body %s)", value, rec.Code, rec.Body)
		}
		cfg := w.srv.Config.Get()
		if cfg.Frontend.Logo != value || cfg.Frontend.LogoDark != value {
			t.Errorf("logo %q stored as %q/%q", value, cfg.Frontend.Logo, cfg.Frontend.LogoDark)
		}
	}
}

// The pair has to round-trip through the settings page, which saves only the
// fields it shows: a save from the Files page must not clear it.
func TestLogoSettingsRoundTrip(t *testing.T) {
	w := adminEnv(t)
	rec := w.submit("/api/admin/settings", `{"logo":"/img/mine.svg","logo_dark":"/img/mine-dark.svg"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body)
	}
	var saved struct {
		Data SettingsView `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &saved); err != nil {
		t.Fatal(err)
	}
	if saved.Data.Logo != "/img/mine.svg" || saved.Data.LogoDark != "/img/mine-dark.svg" {
		t.Errorf("response = %q/%q", saved.Data.Logo, saved.Data.LogoDark)
	}

	// Another page's partial save keeps both.
	if rec := w.submit("/api/admin/settings", `{"max_width":1200}`); rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body)
	}
	cfg := w.srv.Config.Get()
	if cfg.Frontend.Logo != "/img/mine.svg" || cfg.Frontend.LogoDark != "/img/mine-dark.svg" {
		t.Errorf("a partial save cleared the logos: %q/%q", cfg.Frontend.Logo, cfg.Frontend.LogoDark)
	}
}

func TestUpdateSettingsRequiresAdmin(t *testing.T) {
	e := newEnv(t)
	e.addUser("bob", "pw", "/bob", auth.RoleUser, auth.PermRead)
	session := e.login("bob", "pw")

	for _, path := range []string{"/api/admin/settings"} {
		rec := e.do(http.MethodGet, path, "", session)
		if rec.Code != http.StatusForbidden {
			t.Errorf("GET %s: status %d, want 403", path, rec.Code)
		}
	}
}

func TestSettingsChangeAffectsPublicConfig(t *testing.T) {
	w := adminEnv(t)
	body := w.settingsBody(func(s *SettingsView) {
		s.AppName = "Renamed"
		s.Language = "french"
	})
	if rec := w.submit("/api/admin/settings", body); rec.Code != http.StatusOK {
		t.Fatalf("update status %d", rec.Code)
	}

	// /api/config is served by the app package; the settings it exposes come
	// straight from the store, so asserting on the store is equivalent here.
	cfg := w.srv.Config.Get()
	if cfg.Frontend.AppName != "Renamed" || cfg.Frontend.Language != "french" {
		t.Errorf("config = %+v", cfg.Frontend)
	}
}

func TestSettingsPreserveSessionKey(t *testing.T) {
	w := adminEnv(t)
	if err := w.srv.Config.Update(func(c *config.Config) error {
		c.Security.SessionKey = "deadbeefdeadbeef"
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if rec := w.submit("/api/admin/settings", w.settingsBody(nil)); rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	if got := w.srv.Config.Get().Security.SessionKey; got != "deadbeefdeadbeef" {
		t.Errorf("session key changed to %q", got)
	}
}

// TestPartialSettingsKeepTheRest pins the merge that lets the Settings page and
// the Files page save separately. Before it, a Files page sending only its own
// fields would have reset every other setting to the config defaults.
func TestPartialSettingsKeepTheRest(t *testing.T) {
	w := adminEnv(t)
	// Something on the Settings page, away from the Files fields.
	base := w.settingsBody(func(s *SettingsView) {
		s.AppName = "Renamed"
		s.IPAllowlist = []string{"10.0.0.0/8"}
		s.LoggingFile = "private/logs/app.log"
		s.MaxWidth = 1200
	})
	if rec := w.submit("/api/admin/settings", base); rec.Code != http.StatusOK {
		t.Fatalf("base save: status %d body %s", rec.Code, rec.Body)
	}

	// Only the Files fields, exactly as the Files page would send them.
	rec := w.submit("/api/admin/settings", `{
		"upload_max_size": 52428800,
		"upload_chunk_size": 524288,
		"upload_simultaneous": 5,
		"default_archive_name": "files.zip",
		"editable": [".txt"]
	}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("partial save: status %d body %s", rec.Code, rec.Body)
	}

	cfg := w.srv.Config.Get()
	if cfg.Frontend.UploadMaxSize != 52428800 || cfg.Frontend.UploadChunkSize != 524288 ||
		cfg.Frontend.UploadSimultaneous != 5 {
		t.Errorf("files fields were not saved: %+v", cfg.Frontend)
	}
	if cfg.Frontend.DefaultArchiveName != "files.zip" || len(cfg.Frontend.Editable) != 1 {
		t.Errorf("archive/editable not saved: %+v", cfg.Frontend)
	}
	if cfg.Frontend.AppName != "Renamed" {
		t.Errorf("app name was reset to %q by a partial save", cfg.Frontend.AppName)
	}
	if len(cfg.Security.IPAllowlist) != 1 || cfg.Security.IPAllowlist[0] != "10.0.0.0/8" {
		t.Errorf("allowlist was reset to %v", cfg.Security.IPAllowlist)
	}
	if cfg.Logging.File != "private/logs/app.log" {
		t.Errorf("log file was reset to %q", cfg.Logging.File)
	}
	if cfg.Frontend.MaxWidth != 1200 {
		t.Errorf("interface width was reset to %d", cfg.Frontend.MaxWidth)
	}
}

// TestEmptySettingsFieldsAreExplicit checks that an explicit empty value does
// replace a stored one: "clear the allowlist" must be possible, and is
// different from "do not touch it".
func TestEmptySettingsFieldsAreExplicit(t *testing.T) {
	w := adminEnv(t)
	if err := w.srv.Config.Update(func(c *config.Config) error {
		c.Security.IPAllowlist = []string{"10.0.0.0/8"}
		c.Frontend.Editable = []string{".txt", ".md"}
		c.Frontend.GuestRedirection = "https://example.com/login"
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	rec := w.submit("/api/admin/settings", `{
		"ip_allowlist": [],
		"editable": [],
		"guest_redirection": ""
	}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body)
	}
	cfg := w.srv.Config.Get()
	if len(cfg.Security.IPAllowlist) != 0 {
		t.Errorf("allowlist = %v, want empty", cfg.Security.IPAllowlist)
	}
	if len(cfg.Frontend.Editable) != 0 {
		t.Errorf("editable = %v, want empty", cfg.Frontend.Editable)
	}
	if cfg.Frontend.GuestRedirection != "" {
		t.Errorf("guest redirection = %q, want empty", cfg.Frontend.GuestRedirection)
	}
	// And the fields it did not send are still there.
	if cfg.Frontend.AppName != "FileGator" {
		t.Errorf("app name = %q", cfg.Frontend.AppName)
	}
}

// TestNullSettingsFieldsAreIgnored keeps a JSON null from erasing a field: a
// client that encodes an unset list as null means "I have nothing here", not
// "set this to nothing".
func TestNullSettingsFieldsAreIgnored(t *testing.T) {
	w := adminEnv(t)
	if err := w.srv.Config.Update(func(c *config.Config) error {
		c.Frontend.Editable = []string{".txt"}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if rec := w.submit("/api/admin/settings", `{"editable": null, "app_name": null}`); rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body)
	}
	cfg := w.srv.Config.Get()
	if len(cfg.Frontend.Editable) != 1 {
		t.Errorf("editable = %v, want the stored value", cfg.Frontend.Editable)
	}
	if cfg.Frontend.AppName != "FileGator" {
		t.Errorf("app name = %q", cfg.Frontend.AppName)
	}
}

// TestInlineExtensionsNormalised checks that the list is stored in the dotless
// lowercase form v7 used, so a value typed as ".PDF" and a config written by
// either version mean the same thing.
func TestDownloadInlineNormalised(t *testing.T) {
	w := adminEnv(t)
	rec := w.submit("/api/admin/settings", `{"download_inline": [".PDF", " jpg ", "jpg", "", "JPG"]}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body)
	}
	got := w.srv.Config.Get().DownloadInline
	want := []string{"pdf", "jpg"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("download_inline = %v, want %v", got, want)
	}
	// And the GET returns exactly what is stored, for the admin field.
	if view := w.getSettings(); strings.Join(view.DownloadInline, ",") != "pdf,jpg" {
		t.Errorf("settings report %v", view.DownloadInline)
	}
}

// TestDownloadInlineValidation refuses an entry that can never match a file,
// rather than saving something that silently does nothing.
func TestDownloadInlineValidation(t *testing.T) {
	w := adminEnv(t)
	for _, bad := range []string{"*.pdf", "a/b", "pd f", "https://example.com"} {
		body, err := json.Marshal(map[string]any{"download_inline": []string{bad}})
		if err != nil {
			t.Fatal(err)
		}
		if rec := w.submit("/api/admin/settings", string(body)); rec.Code != http.StatusBadRequest {
			t.Errorf("%q: status %d, want 400", bad, rec.Code)
		}
	}
	// "*" and an empty list are both valid: all types, and none.
	if rec := w.submit("/api/admin/settings", `{"download_inline": ["*"]}`); rec.Code != http.StatusOK {
		t.Errorf("wildcard: status %d", rec.Code)
	}
	if rec := w.submit("/api/admin/settings", `{"download_inline": []}`); rec.Code != http.StatusOK {
		t.Errorf("empty list: status %d", rec.Code)
	}
	if got := w.srv.Config.Get().DownloadInline; len(got) != 0 {
		t.Errorf("download_inline = %v, want empty", got)
	}
}

// TestSettingsResponseHasEveryField guards the GET shape: the Settings page
// sends the whole thing back, so a field missing from the response would be
// missing from its next save.
func TestSettingsResponseHasEveryField(t *testing.T) {
	w := adminEnv(t)
	rec := w.req(http.MethodGet, "/api/admin/settings", "")
	var body struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	// One per field of a complete request; names match the JSON keys.
	for _, key := range []string{
		"app_name", "language", "language_selection", "logo", "logo_dark", "date_format",
		"download_inline", "upload_max_size", "upload_chunk_size",
		"upload_simultaneous", "default_archive_name", "editable",
		"search_simultaneous", "filter_entries", "guest_redirection",
		"frontend_dir", "logging_enabled", "logging_file", "logging_level",
		"logging_max_size", "csrf_protection", "session_ttl", "secure_cookies",
		"ip_allowlist", "ip_denylist", "trusted_ip_header", "headers",
		"allow_password_change", "max_width", "allow_logout",
		"lockout_attempts", "lockout_timeout",
	} {
		if _, ok := body.Data[key]; !ok {
			t.Errorf("GET settings is missing %q", key)
		}
	}
}

// TestUpdateSettingsRefusesReadOnlyConfig pins the answer a save gets when the
// configuration cannot be written: 409 with the read_only_config code, not 200.
//
// The change still applies to the running server (Update does that before
// returning), but a 200 told the caller it was saved when it will be gone after
// a restart. Every other write in the API refuses in that case, and the admin
// UI shows the message it receives, so it needs no special case.
func TestUpdateSettingsRefusesReadOnlyConfig(t *testing.T) {
	w := adminEnv(t)
	w.makeConfigReadOnly()

	body := w.settingsBody(func(s *SettingsView) {
		s.AppName = "Applied But Not Saved"
	})
	rec := w.submit("/api/admin/settings", body)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409; body %s", rec.Code, rec.Body)
	}
	var envelope struct {
		Error struct {
			Message string `json:"message"`
			Code    string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Error.Code != "read_only_config" {
		t.Errorf("code = %q, want read_only_config", envelope.Error.Code)
	}
	if !strings.Contains(envelope.Error.Message, "could not be saved") {
		t.Errorf("message should explain the save was lost: %q", envelope.Error.Message)
	}
	// And why: a bare "read-only" sent the operator looking for a read-only
	// mount when the cause was a folder that could not be written.
	if !strings.Contains(envelope.Error.Message, "cannot be written to") {
		t.Errorf("message should name the cause, not just say read-only: %q", envelope.Error.Message)
	}
	// The running server still uses the new value for this session.
	if got := w.srv.Config.Get().Frontend.AppName; got != "Applied But Not Saved" {
		t.Errorf("in-memory app name = %q", got)
	}
	// And nothing was written: the 409 is the only truth the caller has.
	if data, err := os.ReadFile(w.srv.Config.Path()); err != nil || strings.Contains(string(data), "Applied But Not Saved") {
		t.Errorf("the read-only file was modified (%v)", err)
	}
}

// TestAuthAndStorageRefuseReadOnlyConfig checks that the other two saves which
// swap a backend behave the same way: the switch happens for this run, the
// caller is told it cannot be saved.
func TestAuthAndStorageRefuseReadOnlyConfig(t *testing.T) {
	cases := []struct {
		name string
		path string
		body string
	}{
		{
			name: "auth",
			path: "/api/admin/auth",
			body: `{"type":"json","users_file":"` + "USERFILE" + `"}`,
		},
		{
			name: "storage",
			path: "/api/admin/storage",
			body: `{"type":"local","root":"` + "ROOT" + `"}`,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w := adminEnv(t)
			body := c.body
			if c.name == "auth" {
				body = strings.Replace(body, "USERFILE", w.srv.Config.Get().Auth.UsersFile, 1)
			} else {
				body = strings.Replace(body, "ROOT", w.root.Dir(), 1)
			}
			w.makeConfigReadOnly()

			rec := w.submit(c.path, body)
			if rec.Code != http.StatusConflict {
				t.Fatalf("status = %d, want 409; body %s", rec.Code, rec.Body)
			}
			if !strings.Contains(rec.Body.String(), "read_only_config") {
				t.Errorf("body should carry the read_only_config code: %s", rec.Body)
			}
		})
	}
}
